/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * 	Unless required by applicable law or agreed to in writing, software
 * 	distributed under the License is distributed on an "AS IS" BASIS,
 * 	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * 	See the License for the specific language governing permissions and
 * 	limitations under the License.
 */

package ha

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
)

// Default Lease coordinates and timings. These mirror the ADR (#293) defaults
// and are overridable through Options / controller flags.
const (
	DefaultLeaseName      = "kubeslice-controller-ha"
	DefaultLeaseNamespace = "kubeslice-controller"
	DefaultLeaseDuration  = 15 * time.Second
	DefaultRenewDeadline  = 10 * time.Second
	DefaultRetryPeriod    = 2 * time.Second
	DefaultPaddingSeconds = 5 * time.Second

	// DefaultPromotionDialTimeout bounds every read a Standby makes of a Lease
	// over the network: each periodic poll of the Active's Lease, the
	// self-health check against its own API server, and the final dial. All
	// must be bounded — main.go builds the remote client with a plain uncached
	// client.New and no timeout, so a read from a black-holed API server blocks
	// until the OS TCP timeout, which is minutes and far outside the failover
	// budget. The periodic poll matters most: the watch loop calls it
	// synchronously, so a blocked read stalls detection entirely.
	DefaultPromotionDialTimeout = 5 * time.Second

	// DefaultPromotionGracePeriod bounds each step of the promotion sequence
	// that waits on another component: stopping the mirror, publishing
	// status.activeController, re-enqueuing objects, and emitting the event.
	// Expiry aborts the promotion only for the mirror stop, where proceeding
	// would mean two writers; the rest log and continue, because a hub that
	// cannot publish or emit is still a better Active than none. Distinct from
	// PaddingSeconds, which is a detection threshold and has nothing to do with
	// sequencing.
	DefaultPromotionGracePeriod = 10 * time.Second
)

// Options configures a ClusterLeaderElector. Zero-valued fields fall back to the
// Default* constants (or the OS hostname, for Identity; the downward-API
// KUBESLICE_CONTROLLER_MANAGER_NAMESPACE env var, for LeaseNamespace).
type Options struct {
	Mode      HAMode
	Identity  string
	LeaseName string
	// LeaseNamespace, if empty, defaults to KUBESLICE_CONTROLLER_MANAGER_NAMESPACE
	// (the controller's own namespace, injected via the downward API) so the
	// Lease always lands where the leader-election Role grants access to it,
	// regardless of which namespace the controller is actually deployed into.
	// Only falls back to DefaultLeaseNamespace when that env var is unset too
	// (e.g. running outside a pod).
	LeaseNamespace string
	LeaseDuration  time.Duration
	RenewDeadline  time.Duration
	RetryPeriod    time.Duration
	PaddingSeconds time.Duration
	// PromotionDialTimeout bounds every networked Lease read: the periodic poll
	// of the Active's Lease and both pre-promotion guard reads.
	PromotionDialTimeout time.Duration
	// PromotionGracePeriod bounds each promotion step that waits on another
	// component.
	PromotionGracePeriod time.Duration
	// EventRecorder, if set, enables the HA lifecycle Events of issue #298:
	// BecameActive / BecameStandby at start-up, LeadershipLost when an Active
	// gives up its Lease, PromotionAborted when a Standby declines to take over.
	// Optional — nil records nothing.
	EventRecorder events.EventRecorder
	Log           *zap.SugaredLogger
}

// ClusterLeaderElector coordinates leadership between two hub clusters. Unlike
// controller-runtime's in-cluster --leader-elect (which coordinates pods sharing
// one API server), a Standby elector reads the Active's Lease across a cluster
// boundary through remoteClient. See ADR #293.
type ClusterLeaderElector struct {
	localClient  client.Client // own cluster — create and renew the Lease
	remoteClient client.Client // Standby only — read the Active's Lease (may be nil otherwise)

	// mode is an atomic.Value holding an HAMode, not a plain field, because
	// promotion mutates it from its own goroutine while Mode(), StartLeaseRenewal
	// and WatchRemoteLease read it from theirs — a plain field would be a data
	// race, and -race would rightly say so.
	mode      atomic.Value
	identity  string
	leaseName string
	leaseNS   string

	// promoting is held for the whole promotion sequence. IsLeader() reports
	// false while it is set, regardless of isLeader, so the write fence stays
	// shut from the first step until the last — see promote().
	promoting atomic.Bool

	// hooks are promotion's effects outside the elector. Injected so pkg/ha
	// stays independent of the mirror, the publisher and the manager.
	hooks PromotionHooks

	// eventRecorder records the lifecycle Events of issue #298 — the mode this
	// hub started in, a lost leadership, an abandoned promotion. Optional: nil
	// disables them and changes nothing else, which is what keeps every existing
	// test constructing an elector without one.
	//
	// Held directly rather than injected as a hook, unlike EmitPromotedEvent.
	// The distinction is which object the Event hangs off. Promotion's Event
	// attaches to the Lease it has just acquired, so only promote() can supply
	// it; these three attach to the Lease as an identifier rather than as an
	// object, which the elector can name unaided from leaseName and leaseNS.
	// RemoteSyncer already takes a recorder the same way.
	eventRecorder events.EventRecorder

	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
	padding       time.Duration

	promotionDialTimeout time.Duration
	promotionGracePeriod time.Duration

	// isLeader is the single source of truth read by IsLeader(). The background
	// renewal loop keeps it current, so readers never touch the API server.
	isLeader atomic.Bool
	// lastRenew is the time of the last successful Lease renewal. It is written
	// once by promote() when it takes the Lease, before it starts the renewal
	// goroutine, and from then on only that goroutine reads and writes it — so
	// there is exactly one writer at any time and no lock is needed.
	lastRenew time.Time

	// lastSeenLease is the newest Lease successfully read from the Active hub,
	// and nil until the very first successful read. It is the whole of the
	// promotion trigger (issue #297).
	//
	// A failed read deliberately leaves it untouched rather than clearing it or
	// treating the failure as health. "The Active's controller stopped renewing"
	// and "the Active's API server stopped answering" are the same event from
	// here — in both, the newest proof of life this hub holds stops advancing —
	// so a retained stale Lease ages on its own against a moving clock and one
	// comparison covers both. Before this, a read failure reported "not stale",
	// which made the loss of an entire hub undetectable.
	//
	// Only WatchRemoteLease's single goroutine touches this and lastGoodRead.
	lastSeenLease *coordinationv1.Lease
	// lastGoodRead is the local wall-clock time of that read. Not used by the
	// verdict, which anchors on the Lease's own renewTime; carried so an
	// optional local-only staleness floor stays available without a redesign
	// if clock skew between hubs ever becomes a practical problem.
	lastGoodRead time.Time

	log *zap.SugaredLogger
}

// NewClusterLeaderElector builds an elector. local is a client to this
// controller's own cluster; remote is a client to the Active hub and is required
// only in Standby mode (it may be nil otherwise). Everything else is passed
// through Options, because the Lease timings are operator-configurable flags.
// applyDefaults fills every zero-valued Option from the Default* constants (or
// the OS hostname / downward-API namespace). Extracted from the constructor so
// a caller that must read the Lease BEFORE constructing an elector — see
// ResumeAsActive — resolves the same name, namespace, identity and padding the
// elector itself would, rather than duplicating the fallbacks and drifting.
func applyDefaults(opts Options) Options {
	if opts.Mode == "" {
		opts.Mode = ModeStandalone
	}
	if opts.LeaseName == "" {
		opts.LeaseName = DefaultLeaseName
	}
	if opts.LeaseNamespace == "" {
		if ns := os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE"); ns != "" {
			opts.LeaseNamespace = ns
		} else {
			opts.LeaseNamespace = DefaultLeaseNamespace
		}
	}
	if opts.LeaseDuration == 0 {
		opts.LeaseDuration = DefaultLeaseDuration
	}
	if opts.RenewDeadline == 0 {
		opts.RenewDeadline = DefaultRenewDeadline
	}
	if opts.RetryPeriod == 0 {
		opts.RetryPeriod = DefaultRetryPeriod
	}
	if opts.PaddingSeconds == 0 {
		opts.PaddingSeconds = DefaultPaddingSeconds
	}
	if opts.PromotionDialTimeout == 0 {
		opts.PromotionDialTimeout = DefaultPromotionDialTimeout
	}
	if opts.PromotionGracePeriod == 0 {
		opts.PromotionGracePeriod = DefaultPromotionGracePeriod
	}
	if opts.Identity == "" {
		if hostname, err := os.Hostname(); err == nil {
			opts.Identity = hostname
		} else {
			opts.Identity = "kubeslice-controller"
		}
	}
	if opts.Log == nil {
		opts.Log = util.NewLogger().With("name", "ha-leader-elector")
	}

	return opts
}

func NewClusterLeaderElector(local, remote client.Client, opts Options) *ClusterLeaderElector {
	opts = applyDefaults(opts)

	e := &ClusterLeaderElector{
		localClient:          local,
		remoteClient:         remote,
		identity:             opts.Identity,
		leaseName:            opts.LeaseName,
		leaseNS:              opts.LeaseNamespace,
		leaseDuration:        opts.LeaseDuration,
		renewDeadline:        opts.RenewDeadline,
		retryPeriod:          opts.RetryPeriod,
		padding:              opts.PaddingSeconds,
		promotionDialTimeout: opts.PromotionDialTimeout,
		promotionGracePeriod: opts.PromotionGracePeriod,
		eventRecorder:        opts.EventRecorder,
		log:                  opts.Log,
	}
	e.mode.Store(opts.Mode)
	// Standalone is always the leader: no Lease, no remote watch — identical to
	// the controller's behaviour before HA (the no-regression guarantee).
	if opts.Mode == ModeStandalone {
		e.isLeader.Store(true)
	}

	// Publish the two gauges whose value at 0 is the alertable condition, so the
	// series exist from start-up rather than appearing at the first transition.
	// This matters more than it looks: a Standby that never promotes never calls
	// setLeader, and a Standby that never reads the Active never arms, so on the
	// exact hubs an operator most needs to see these, nothing would ever create
	// the series and `ha_leader_status == 0` would match no rows at all.
	//
	// Every other gauge in metrics.go stays deliberately unset until it has a
	// real value — see the note there on why a zeroed timestamp is worse than a
	// missing one.
	haLeaderStatus.Set(boolGauge(e.isLeader.Load()))
	if opts.Mode == ModeStandby {
		haArmed.WithLabelValues(string(ModeStandby)).Set(0)
	}
	return e
}

// IsLeader reports whether this controller may perform mutating reconciles right
// now. It reads an in-memory flag kept current by the background loops, so it is
// cheap enough to call at the top of every Reconcile. The value reflects live
// leadership (refreshed every retryPeriod), never a value frozen at startup.
// It also reports false for the whole of a promotion sequence, regardless of
// isLeader: steps 0 and 8 of promote() bracket the sequence with the promoting
// latch, so the write fence stays shut until the new Active has stopped the
// mirror, taken its Lease and published who it is. That is what gives "the
// reconcilers are not live until promotion finishes" real teeth without any
// external status surface.
func (e *ClusterLeaderElector) IsLeader() bool {
	return e.isLeader.Load() && !e.promoting.Load()
}

// Mode returns the current HA mode. It is read live rather than captured at
// startup, because promotion changes it.
func (e *ClusterLeaderElector) Mode() HAMode {
	mode, _ := e.mode.Load().(HAMode)
	return mode
}

// setMode swaps the running mode. Only promote() calls it.
func (e *ClusterLeaderElector) setMode(mode HAMode) {
	e.mode.Store(mode)
	e.log.Infow("HA mode changed", "mode", mode, "identity", e.identity)
}

// SetPromotionHooks installs the effects promotion has outside the elector.
// Called from main.go once the mirror, publisher and manager exist — they are
// constructed after the elector, so they cannot be constructor arguments.
//
// A Standby with no hooks still promotes correctly in the narrow sense (it
// takes the Lease and opens the fence); the hooks are what make the promotion
// safe and complete.
func (e *ClusterLeaderElector) SetPromotionHooks(hooks PromotionHooks) {
	e.hooks = hooks
}

// Identity returns this instance's Lease holder identity.
func (e *ClusterLeaderElector) Identity() string {
	return e.identity
}

// StartLeaseRenewal runs the Active's renewal loop until ctx is cancelled. It is
// a no-op in any other mode. It renews immediately, then every retryPeriod.
func (e *ClusterLeaderElector) StartLeaseRenewal(ctx context.Context) error {
	if e.Mode() != ModeActive {
		e.log.Infow("lease renewal not started; not in active mode", "mode", e.Mode())
		return nil
	}
	e.log.Infow("starting lease renewal",
		"lease", e.leaseName, "namespace", e.leaseNS, "identity", e.identity,
		"leaseDuration", e.leaseDuration, "renewDeadline", e.renewDeadline, "retryPeriod", e.retryPeriod)

	_ = e.renewOnce(ctx)
	ticker := time.NewTicker(e.retryPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			e.setLeader(false)
			e.log.Infow("lease renewal stopped", "reason", ctx.Err())
			return nil
		case <-ticker.C:
			_ = e.renewOnce(ctx)
		}
	}
}

// renewOnce performs a single acquire/renew attempt and updates leadership state.
// On success it (re)acquires leadership. On failure it keeps leadership until
// renewDeadline elapses without any successful renewal, then releases it — the
// natural-fencing behaviour ADR #293 relies on: a dead API server means no renewal and
// therefore no writes.
func (e *ClusterLeaderElector) renewOnce(ctx context.Context) error {
	if _, err := acquireOrRenewLease(ctx, e.localClient, e.leaseName, e.leaseNS, e.identity, e.leaseDuration); err != nil {
		haLeaseRenewErrorsTotal.Inc()
		switch {
		case e.lastRenew.IsZero():
			e.log.Warnw("failed to acquire lease", "error", err)
		case time.Since(e.lastRenew) > e.renewDeadline:
			e.log.Warnw("failed to renew lease within renew deadline; releasing leadership",
				"error", err, "renewDeadline", e.renewDeadline, "sinceLastRenew", time.Since(e.lastRenew))
			// Emitted here rather than inside setLeader, and only on this branch.
			// setLeader's other caller for a false value is StartLeaseRenewal's
			// ctx.Done path — an ordinary graceful shutdown, where a Warning
			// Event would be pure noise and the write would in any case be racing
			// the pod's own termination. This branch is the one issue #298
			// describes: renewal has failed for longer than renewDeadline and
			// leadership is being given up while the process keeps running.
			e.emitLifecycleEvent(ctx, ossEvents.EventHALeadershipLost)
			e.setLeader(false)
		default:
			e.log.Warnw("failed to renew lease; still within renew deadline, keeping leadership", "error", err)
		}
		return err
	}
	e.lastRenew = time.Now()
	haLeaseLastRenewTime.WithLabelValues(string(ModeActive)).Set(float64(e.lastRenew.Unix()))
	e.setLeader(true)
	return nil
}

// WatchRemoteLease runs the Standby's watch loop until ctx is cancelled. It
// reads the Active's Lease every retryPeriod and, once that Lease has aged past
// leaseDuration + padding, runs the promotion sequence (issue #297).
//
// It returns as soon as promotion succeeds: this hub is an Active now, and
// there is no longer any Active to watch. Promotion has already started the
// renewal loop that replaces it.
func (e *ClusterLeaderElector) WatchRemoteLease(ctx context.Context) error {
	if e.Mode() != ModeStandby {
		e.log.Infow("remote lease watch not started; not in standby mode", "mode", e.Mode())
		return nil
	}
	if e.remoteClient == nil {
		return fmt.Errorf("standby mode requires a remote client to the active hub")
	}
	e.log.Infow("watching active hub lease", "lease", e.leaseName, "namespace", e.leaseNS)

	ticker := time.NewTicker(e.retryPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			e.log.Infow("remote lease watch stopped", "reason", ctx.Err())
			return nil
		case <-ticker.C:
			candidate, _ := e.checkRemoteLeaseOnce(ctx)
			if !candidate {
				continue
			}
			promoted, err := e.promote(ctx)
			if err != nil {
				// Stay a Standby and try again next tick. Every failure path in
				// promote() leaves the hub exactly as it was — still fenced,
				// still armed — so retrying is safe rather than merely tolerable.
				e.log.Errorw("promotion attempt failed; remaining standby", "error", err)
				continue
			}
			if promoted {
				e.log.Infow("remote lease watch stopping; this hub is now the active")
				return nil
			}
		}
	}
}

// checkRemoteLeaseOnce reads the Active's Lease once, updates the cached view,
// and reports whether this hub is now a promotion *candidate*. It never changes
// leadership itself — the guards and the promotion sequence are separate, so
// that "we think the Active is gone" and "we took over" stay independently
// testable.
//
// err is the read error, returned for logging and tests; the verdict does not
// depend on it. A read that fails is not evidence of health, it is the absence
// of new evidence — see lastSeenLease.
func (e *ClusterLeaderElector) checkRemoteLeaseOnce(ctx context.Context) (candidate bool, err error) {
	// Bounded, for the same reason the guards' reads are. The watch loop calls
	// this synchronously, so a read that blocks blocks the loop — and while it is
	// blocked no staleness is evaluated at all. main.go builds the remote client
	// with a plain uncached client.New and no timeout, so an API server that
	// accepts the connection and then stops answering leaves the read hanging
	// until the transport gives up: minutes against a black-holed host.
	//
	// This is not hypothetical. Live-testing an Active whose API server was shut
	// down showed a single read blocking for ~12s, with no staleness evaluated in
	// the whole window, before the connection finally broke. That was a graceful
	// container shutdown; a powered-off node or a dropped-packet partition has
	// nothing to break the connection at all, and the failover budget would be
	// blown by an unbounded wait rather than by the detection rule.
	readCtx, cancel := context.WithTimeout(ctx, e.promotionDialTimeout)
	defer cancel()

	lease, err := getLease(readCtx, e.remoteClient, e.leaseName, e.leaseNS)
	if err != nil {
		haRemoteLeaseReadsTotal.WithLabelValues(readResultError).Inc()
		e.log.Warnw("could not read active hub lease; retaining last known view",
			"error", err, "armed", e.lastSeenLease != nil)
	} else {
		haRemoteLeaseReadsTotal.WithLabelValues(readResultOK).Inc()
		e.lastSeenLease = lease
		e.lastGoodRead = time.Now()
	}
	haArmed.WithLabelValues(string(ModeStandby)).Set(boolGauge(e.lastSeenLease != nil))
	// Deliberately outside the else: the age is published on failed reads too,
	// and that is the whole value of it as a leading indicator. A retained stale
	// Lease ageing against a moving clock is exactly how this loop models "no new
	// evidence of life", so the gauge climbs through an outage rather than
	// freezing at the last good value and looking healthy.
	if age, ok := remoteLeaseAge(e.lastSeenLease, time.Now()); ok {
		haRemoteLeaseAgeSeconds.WithLabelValues(string(ModeStandby)).Set(age.Seconds())
	}

	// This nil check MUST stay a separate statement and must never be folded
	// into the isLeaseStale call below. isLeaseStale(nil, ...) returns TRUE
	// (lease.go) — correct for its original caller, where a Lease that does not
	// exist on your own cluster is stale and should be created. Here it would
	// mean a Standby that has never once read the Active's Lease concludes the
	// Active is dead and promotes on its very first tick: a broken kubeconfig,
	// a missing RBAC grant or a mistyped namespace would each become a
	// guaranteed split brain. TestNeverArmed_NeverBecomesCandidate pins this.
	if e.lastSeenLease == nil {
		e.log.Warnw("the active hub's lease has never been read successfully; refusing to consider promotion",
			"lease", e.leaseName, "namespace", e.leaseNS,
			"hint", "check --ha-active-kubeconfig, RBAC for coordination.k8s.io/leases, and the lease namespace")
		return false, err
	}

	if !isLeaseStale(e.lastSeenLease, e.padding, time.Now()) {
		e.log.Debugw("active hub lease is fresh",
			"lease", e.leaseName, "holder", leaseHolder(e.lastSeenLease), "renewTime", leaseRenewStr(e.lastSeenLease))
		return false, err
	}

	e.log.Warnw("active hub lease is STALE; evaluating promotion",
		"lease", e.leaseName, "holder", leaseHolder(e.lastSeenLease),
		"renewTime", leaseRenewStr(e.lastSeenLease), "readable", err == nil)
	return true, err
}

// setLeader updates the leadership flag and logs LeadershipAcquired /
// LeadershipLost only on an actual transition.
func (e *ClusterLeaderElector) setLeader(leader bool) {
	if e.isLeader.Swap(leader) == leader {
		return
	}
	haLeaderStatus.Set(boolGauge(leader))
	if leader {
		e.log.Infow("LeadershipAcquired", "identity", e.identity, "lease", e.leaseName)
	} else {
		e.log.Infow("LeadershipLost", "identity", e.identity, "lease", e.leaseName)
	}
}

func leaseHolder(lease *coordinationv1.Lease) string {
	if lease == nil || lease.Spec.HolderIdentity == nil {
		return ""
	}
	return *lease.Spec.HolderIdentity
}

func leaseRenewStr(lease *coordinationv1.Lease) string {
	if lease == nil || lease.Spec.RenewTime == nil {
		return ""
	}
	return lease.Spec.RenewTime.String()
}
