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
	Log          *zap.SugaredLogger
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

	endpoint string
	caBundle string
	interval time.Duration

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

	if err := p.PublishOnce(ctx); err != nil {
		p.log.Warnw("initial activeController publication failed; will retry", "error", err)
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.log.Infow("activeController publisher stopped", "reason", ctx.Err())
			return nil
		case <-ticker.C:
			if err := p.PublishOnce(ctx); err != nil {
				p.log.Warnw("activeController publication failed; will retry", "error", err)
			}
		}
	}
}

// PublishOnce runs a single pass over every Cluster CR on this hub. Promotion
// calls it synchronously so a failover does not wait for the next tick.
//
// It is a no-op unless this hub currently holds leadership: a Standby's copy of
// the field is owned by the state mirror, and a Standby writing its own identity
// there would break the very rule workers use to tell the two hubs apart.
func (p *ActivePublisher) PublishOnce(ctx context.Context) error {
	if !p.elector.IsLeader() {
		p.log.Debugw("not the leader; skipping activeController publication")
		return nil
	}
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
	for i := range clusters.Items {
		cluster := &clusters.Items[i]
		if activeControllerUpToDate(cluster.Status.ActiveController, desired) {
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
	}
	if updated > 0 {
		p.log.Infow("published activeController",
			"clusters", updated, "identity", desired.ActiveIdentity, "endpoint", desired.Endpoint)
	}
	return errors.Join(errs...)
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
// declaration anyway. Auto-demotion of a recovered hub is an explicit ADR
// non-goal (Decision 8); LastUpdated is what lets a consumer prefer the fresher
// of two claims if it ever does see both.
func activeControllerUpToDate(current *controllerv1alpha1.ActiveControllerInfo, desired controllerv1alpha1.ActiveControllerInfo) bool {
	return current != nil &&
		current.Endpoint == desired.Endpoint &&
		current.CABundle == desired.CABundle &&
		current.ActiveIdentity == desired.ActiveIdentity
}
