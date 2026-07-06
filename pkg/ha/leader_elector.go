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

	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
)

// Options configures a ClusterLeaderElector. Zero-valued fields fall back to the
// Default* constants (or the OS hostname, for Identity).
type Options struct {
	Mode           HAMode
	Identity       string
	LeaseName      string
	LeaseNamespace string
	LeaseDuration  time.Duration
	RenewDeadline  time.Duration
	RetryPeriod    time.Duration
	PaddingSeconds time.Duration
	Log            *zap.SugaredLogger
}

// ClusterLeaderElector coordinates leadership between two hub clusters. Unlike
// controller-runtime's in-cluster --leader-elect (which coordinates pods sharing
// one API server), a Standby elector reads the Active's Lease across a cluster
// boundary through remoteClient. See ADR #293.
type ClusterLeaderElector struct {
	localClient  client.Client // own cluster — create and renew the Lease
	remoteClient client.Client // Standby only — read the Active's Lease (may be nil otherwise)

	mode      HAMode
	identity  string
	leaseName string
	leaseNS   string

	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
	padding       time.Duration

	// isLeader is the single source of truth read by IsLeader(). The background
	// renewal loop keeps it current, so readers never touch the API server.
	isLeader atomic.Bool
	// lastRenew is the time of the last successful Lease renewal. Only the
	// renewal goroutine reads and writes it.
	lastRenew time.Time

	log *zap.SugaredLogger
}

// NewClusterLeaderElector builds an elector. local is a client to this
// controller's own cluster; remote is a client to the Active hub and is required
// only in Standby mode (it may be nil otherwise).
//
// Note: issue #294 sketches a three-argument constructor, but the ADR lists the
// Lease timings as configurable, so configuration is passed through Options.
func NewClusterLeaderElector(local, remote client.Client, opts Options) *ClusterLeaderElector {
	if opts.Mode == "" {
		opts.Mode = ModeStandalone
	}
	if opts.LeaseName == "" {
		opts.LeaseName = DefaultLeaseName
	}
	if opts.LeaseNamespace == "" {
		opts.LeaseNamespace = DefaultLeaseNamespace
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

	e := &ClusterLeaderElector{
		localClient:   local,
		remoteClient:  remote,
		mode:          opts.Mode,
		identity:      opts.Identity,
		leaseName:     opts.LeaseName,
		leaseNS:       opts.LeaseNamespace,
		leaseDuration: opts.LeaseDuration,
		renewDeadline: opts.RenewDeadline,
		retryPeriod:   opts.RetryPeriod,
		padding:       opts.PaddingSeconds,
		log:           opts.Log,
	}
	// Standalone is always the leader: no Lease, no remote watch — identical to
	// the controller's behaviour before HA (the no-regression guarantee).
	if e.mode == ModeStandalone {
		e.isLeader.Store(true)
	}
	return e
}

// IsLeader reports whether this controller may perform mutating reconciles right
// now. It reads an in-memory flag kept current by the background loops, so it is
// cheap enough to call at the top of every Reconcile. The value reflects live
// leadership (refreshed every retryPeriod), never a value frozen at startup.
func (e *ClusterLeaderElector) IsLeader() bool {
	return e.isLeader.Load()
}

// Mode returns the configured HA mode.
func (e *ClusterLeaderElector) Mode() HAMode {
	return e.mode
}

// Identity returns this instance's Lease holder identity.
func (e *ClusterLeaderElector) Identity() string {
	return e.identity
}

// StartLeaseRenewal runs the Active's renewal loop until ctx is cancelled. It is
// a no-op in any other mode. It renews immediately, then every retryPeriod.
func (e *ClusterLeaderElector) StartLeaseRenewal(ctx context.Context) error {
	if e.mode != ModeActive {
		e.log.Infow("lease renewal not started; not in active mode", "mode", e.mode)
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
			return ctx.Err()
		case <-ticker.C:
			_ = e.renewOnce(ctx)
		}
	}
}

// renewOnce performs a single acquire/renew attempt and updates leadership state.
// On success it (re)acquires leadership. On failure it keeps leadership until
// renewDeadline elapses without any successful renewal, then releases it — the
// natural-fencing behaviour from the ADR: a dead API server means no renewal and
// therefore no writes.
func (e *ClusterLeaderElector) renewOnce(ctx context.Context) error {
	if _, err := acquireOrRenewLease(ctx, e.localClient, e.leaseName, e.leaseNS, e.identity, e.leaseDuration); err != nil {
		switch {
		case e.lastRenew.IsZero():
			e.log.Warnw("failed to acquire lease", "error", err)
		case time.Since(e.lastRenew) > e.renewDeadline:
			e.log.Warnw("failed to renew lease within renew deadline; releasing leadership",
				"error", err, "renewDeadline", e.renewDeadline, "sinceLastRenew", time.Since(e.lastRenew))
			e.setLeader(false)
		default:
			e.log.Warnw("failed to renew lease; still within renew deadline, keeping leadership", "error", err)
		}
		return err
	}
	e.lastRenew = time.Now()
	e.setLeader(true)
	return nil
}

// WatchRemoteLease runs the Standby's watch loop until ctx is cancelled. It reads
// the Active's Lease every retryPeriod and logs whether it is fresh or stale. In
// issue #294 it does not promote — detecting staleness and acquiring leadership
// is issue #297.
func (e *ClusterLeaderElector) WatchRemoteLease(ctx context.Context) error {
	if e.mode != ModeStandby {
		e.log.Infow("remote lease watch not started; not in standby mode", "mode", e.mode)
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
			return ctx.Err()
		case <-ticker.C:
			_, _ = e.checkRemoteLeaseOnce(ctx)
		}
	}
}

// checkRemoteLeaseOnce reads the Active's Lease once and reports whether it is
// stale. It never changes leadership in issue #294: a Standby stays a Standby.
func (e *ClusterLeaderElector) checkRemoteLeaseOnce(ctx context.Context) (stale bool, err error) {
	lease, err := getLease(ctx, e.remoteClient, e.leaseName, e.leaseNS)
	if err != nil {
		e.log.Warnw("could not read active hub lease", "error", err)
		return false, err
	}
	if isLeaseStale(lease, e.padding, time.Now()) {
		e.log.Warnw("active hub lease is STALE; promotion would trigger here (implemented in #297)",
			"lease", e.leaseName, "holder", leaseHolder(lease), "renewTime", leaseRenewStr(lease))
		return true, nil
	}
	e.log.Debugw("active hub lease is fresh",
		"lease", e.leaseName, "holder", leaseHolder(lease), "renewTime", leaseRenewStr(lease))
	return false, nil
}

// setLeader updates the leadership flag and logs LeadershipAcquired /
// LeadershipLost only on an actual transition.
func (e *ClusterLeaderElector) setLeader(leader bool) {
	if e.isLeader.Swap(leader) == leader {
		return
	}
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
