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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
)

const (
	// DefaultActivePublishInterval is how often the leader re-checks that every
	// Cluster CR carries its declaration. Convergence after a promotion does not
	// wait for this tick — promotion runs one synchronous PublishOnce.
	DefaultActivePublishInterval = 30 * time.Second

	// DefaultLeadershipPollInterval is how often a hub that is not (yet) the
	// leader re-checks whether it has become one. It is deliberately much shorter
	// than the publish interval: an Active acquires its Lease a second or two
	// after start-up, and waiting a full publish interval to notice would leave a
	// freshly started hub unadvertised for that whole window. Costs nothing while
	// idle — a non-leader returns before touching the API server.
	DefaultLeadershipPollInterval = 2 * time.Second

	// DefaultSelfCABundlePath is where a pod finds its own API server's CA.
	DefaultSelfCABundlePath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	// PlaceholderControllerEndpoint is the value service.ControllerEndpoint ships
	// with when --controller-end-point is not set. It is a documentation
	// placeholder that resolves to nothing, so publishing it as the failover
	// target would be worse than publishing nothing at all.
	//
	// It is duplicated here rather than imported because main.go overwrites
	// service.ControllerEndpoint with the flag value at startup, leaving no way
	// to recover the default afterwards. TestPlaceholderMatchesServiceDefault
	// pins the two together.
	PlaceholderControllerEndpoint = "https://controller.cisco.com:6443/"
)

// leadership is the subset of ClusterLeaderElector the publisher depends on.
// Narrowing it keeps the publisher testable without a live Lease.
type leadership interface {
	IsLeader() bool
	Identity() string
}

// ActivePublisherOptions configures an ActivePublisher. Zero-valued fields fall
// back to the Default* constants.
type ActivePublisherOptions struct {
	// Endpoint is this hub's own API server endpoint, as handed to workers at
	// registration (service.ControllerEndpoint / --controller-end-point). Reusing
	// that value keeps one source of truth for "where this hub is reachable".
	Endpoint string
	// CABundlePath is this hub's own API server CA, read once at startup.
	CABundlePath string
	Interval     time.Duration
	// LeadershipPollInterval is how often to re-check for leadership while this
	// hub does not hold it. Defaults to DefaultLeadershipPollInterval.
	LeadershipPollInterval time.Duration
	Log                    *zap.SugaredLogger
}

// ActivePublisher keeps status.activeController current on every Cluster CR on
// this hub, for as long as this hub holds leadership (ADR #293 Decision 7).
//
// It is a standalone loop rather than a step inside ClusterService.ReconcileCluster
// because it has to converge independently of reconciler traffic — and reconciler
// traffic is exactly what is absent immediately after a promotion, when the write
// fence has just opened but nothing has re-enqueued the pre-existing objects yet.
type ActivePublisher struct {
	localClient client.Client
	elector     leadership

	endpoint   string
	caBundle   string
	interval   time.Duration
	leaderPoll time.Duration

	// persistenceVerified records that a write of status.activeController has
	// been read back at least once and was really stored. Atomic because
	// promotion calls PublishOnce synchronously while the ticker loop is also
	// running.
	persistenceVerified atomic.Bool

	log *zap.SugaredLogger
}

// NewActivePublisher builds a publisher writing to local, gated on elector.
//
// A missing or unreadable CA bundle is not fatal: the endpoint and identity are
// what a worker needs to select a hub, and a worker that already pins the hub's
// CA does not need it republished. The failure is logged and publication
// continues without it.
func NewActivePublisher(local client.Client, elector leadership, opts ActivePublisherOptions) *ActivePublisher {
	if opts.Interval == 0 {
		opts.Interval = DefaultActivePublishInterval
	}
	if opts.LeadershipPollInterval == 0 {
		opts.LeadershipPollInterval = DefaultLeadershipPollInterval
	}
	if opts.CABundlePath == "" {
		opts.CABundlePath = DefaultSelfCABundlePath
	}
	if opts.Log == nil {
		opts.Log = util.NewLogger().With("name", "ha-active-publisher")
	}

	p := &ActivePublisher{
		localClient: local,
		elector:     elector,
		endpoint:    opts.Endpoint,
		interval:    opts.Interval,
		leaderPoll:  opts.LeadershipPollInterval,
		log:         opts.Log,
	}

	if ca, err := os.ReadFile(opts.CABundlePath); err != nil {
		p.log.Warnw("could not read own CA bundle; publishing activeController without it",
			"path", opts.CABundlePath, "error", err)
	} else {
		p.caBundle = base64.StdEncoding.EncodeToString(ca)
	}
	return p
}

// Start publishes immediately, then every interval, until ctx is cancelled.
// Returns nil on cancellation so a graceful shutdown is not logged as an error.
func (p *ActivePublisher) Start(ctx context.Context) error {
	p.log.Infow("starting activeController publisher",
		"endpoint", p.endpoint, "interval", p.interval, "haveCABundle", p.caBundle != "")

	for {
		// The wait is chosen from what this pass actually did, not from a second
		// IsLeader() read: leadership arriving between the two would otherwise
		// still cost a full publish interval.
		//
		// While this hub is not the leader there is nothing to publish, but
		// leadership can arrive at any moment — on an Active, a second or two
		// after start-up; on a Standby, at promotion. Re-checking on the short
		// interval is what makes a freshly started or freshly promoted hub
		// advertise itself promptly. Verified live: on the publish interval
		// alone a fresh Active took 31s to appear.
		wasLeader, err := p.publishOnce(ctx)
		if err != nil {
			// Counted here rather than inside publish(), which promotion also
			// calls — promotion increments it on its own failure path so that one
			// failed publication is one increment regardless of which caller made
			// it. Counting inside publish() would double-count promotion's.
			haActivePublishErrorsTotal.Inc()
			p.log.Warnw("activeController publication failed; will retry", "error", err)
		}
		wait := p.interval
		if !wasLeader {
			wait = p.leaderPoll
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			p.log.Infow("activeController publisher stopped", "reason", ctx.Err())
			return nil
		case <-timer.C:
		}
	}
}

// PublishOnce runs a single pass over every Cluster CR on this hub. Promotion
// calls it synchronously, at the point where it has already taken the Lease and
// switched to Active but has not yet opened the write fence, so a failover does
// not wait for the publisher's next tick.
//
// It deliberately does NOT gate on IsLeader(). That reads like a safety check
// and is in fact the opposite: promotion holds the write-fence latch across its
// whole sequence, so IsLeader() reports false for exactly the window in which
// this is called. Gating here made the promotion-time publication a silent
// no-op that still logged success, leaving status.activeController naming the
// dead hub until the periodic loop next ran.
//
// The caller is responsible for having established leadership first. The
// periodic loop must keep using publishOnce, which does gate — an Active that
// has lost its Lease must stop advertising itself.
func (p *ActivePublisher) PublishOnce(ctx context.Context) error {
	return p.publish(ctx)
}

// publishOnce is the leadership-gated pass the periodic loop runs. It also
// reports whether this hub held leadership, which Start uses to decide how long
// to wait next.
//
// A Standby must never write this field: its copy is owned by the state mirror
// and names the Active, which is the whole rule workers use to tell the two
// hubs apart.
func (p *ActivePublisher) publishOnce(ctx context.Context) (leader bool, err error) {
	if !p.elector.IsLeader() {
		p.log.Debugw("not the leader; skipping activeController publication")
		return false, nil
	}
	return true, p.publish(ctx)
}

// publish performs one ungated pass over every Cluster CR on this hub.
func (p *ActivePublisher) publish(ctx context.Context) error {
	if err := p.validEndpoint(); err != nil {
		// Deliberately not fatal. A hub that cannot describe itself should keep
		// reconciling; it just must not advertise an address nobody can reach.
		p.log.Errorw("refusing to publish activeController", "endpoint", p.endpoint, "error", err)
		return nil
	}

	clusters := &controllerv1alpha1.ClusterList{}
	if err := p.localClient.List(ctx, clusters); err != nil {
		return fmt.Errorf("listing clusters to publish activeController: %w", err)
	}

	desired := controllerv1alpha1.ActiveControllerInfo{
		Endpoint:       p.endpoint,
		CABundle:       p.caBundle,
		ActiveIdentity: p.elector.Identity(),
	}

	var errs []error
	updated := 0
	verifySkipped := false
	for i := range clusters.Items {
		cluster := &clusters.Items[i]
		if activeControllerUpToDate(cluster.Status.ActiveController, desired) {
			// Reading our own desired value back off the API server is itself
			// proof the field persists here, so this converged case counts as
			// verification and spares a read-back on some later write.
			p.persistenceVerified.Store(true)
			continue
		}
		payload := desired
		payload.LastUpdated = metav1.Now()
		cluster.Status.ActiveController = &payload
		if err := p.localClient.Status().Update(ctx, cluster); err != nil {
			errs = append(errs, fmt.Errorf("cluster %s/%s: %w", cluster.Namespace, cluster.Name, err))
			continue
		}
		updated++

		// An accepted Status().Update is not proof the field was stored. If the
		// Cluster CRD on this hub predates status.activeController — which is
		// what the published Helm chart installs — the API server prunes the
		// unknown field and still returns success. Nothing surfaces: the field
		// reads back nil, activeControllerUpToDate therefore never converges, so
		// every tick rewrites every Cluster CR while the hub logs that it
		// published, and no worker can discover the Active. A failover is
		// silently unfollowable. Confirmed live against a chart-installed hub.
		//
		// So read it back, until it is confirmed once. Once per process, not per
		// pass: this is a property of the CRD schema rather than of any single
		// write, and re-reading forever would double the read traffic to prove a
		// settled fact. Failures are returned, which routes them to
		// ha_active_publish_errors_total — the metric whose alert already means
		// "failover may work without any worker noticing".
		if !p.persistenceVerified.Load() && !verifySkipped {
			if err := p.verifyPersisted(ctx, cluster, desired); err != nil {
				errs = append(errs, err)
				// One report per pass, not one per Cluster CR: they all share the
				// same schema, so the rest would say the same thing.
				verifySkipped = true
				continue
			}
			p.persistenceVerified.Store(true)
			p.log.Infow("activeController persistence confirmed by read-back",
				"cluster", cluster.Name, "namespace", cluster.Namespace)
		}
	}
	if updated > 0 {
		p.log.Infow("published activeController",
			"clusters", updated, "identity", desired.ActiveIdentity, "endpoint", desired.Endpoint)
	}
	return errors.Join(errs...)
}

// verifyPersisted re-reads a Cluster CR and reports whether the activeController
// it was just given actually survived the write. See the call site for why an
// accepted write is not sufficient evidence.
func (p *ActivePublisher) verifyPersisted(ctx context.Context, cluster *controllerv1alpha1.Cluster, desired controllerv1alpha1.ActiveControllerInfo) error {
	fresh := &controllerv1alpha1.Cluster{}
	if err := p.localClient.Get(ctx, client.ObjectKeyFromObject(cluster), fresh); err != nil {
		return fmt.Errorf("verifying activeController persisted on %s/%s: %w",
			cluster.Namespace, cluster.Name, err)
	}
	if activeControllerUpToDate(fresh.Status.ActiveController, desired) {
		return nil
	}
	readBack := "a different value"
	if fresh.Status.ActiveController == nil {
		readBack = "empty"
	}
	return fmt.Errorf("status.activeController did not persist on %s/%s: the write was "+
		"accepted but the field read back %s. The Cluster CRD on this hub most likely "+
		"predates status.activeController, so the API server is pruning it. Apply the "+
		"updated Cluster CRD; until then no worker can discover this hub and a failover "+
		"cannot be followed", cluster.Namespace, cluster.Name, readBack)
}

// validEndpoint rejects the two values that must never reach a worker: nothing,
// and the shipped placeholder.
func (p *ActivePublisher) validEndpoint() error {
	switch p.endpoint {
	case "":
		return errors.New("controller endpoint is empty; set --controller-end-point on an HA deployment")
	case PlaceholderControllerEndpoint:
		return errors.New("controller endpoint is still the shipped placeholder; set --controller-end-point on an HA deployment")
	}
	return nil
}

// activeControllerUpToDate compares everything except LastUpdated. Including the
// timestamp would make every pass differ from itself, turning a convergence check
// into a write to every Cluster CR on every tick.
//
// Note what is deliberately absent: nothing ever clears this field. A hub only
// stops publishing by losing leadership, which in practice means it stopped
// renewing its Lease — it is unreachable, so a worker cannot read the stale
// declaration anyway. Auto-demotion of a recovered hub is an explicit ADR #293
// non-goal (Decision 8); LastUpdated is what lets a consumer prefer the fresher
// of two claims if it ever does see both.
func activeControllerUpToDate(current *controllerv1alpha1.ActiveControllerInfo, desired controllerv1alpha1.ActiveControllerInfo) bool {
	return current != nil &&
		current.Endpoint == desired.Endpoint &&
		current.CABundle == desired.CABundle &&
		current.ActiveIdentity == desired.ActiveIdentity
}
