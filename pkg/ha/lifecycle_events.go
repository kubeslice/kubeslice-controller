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
	"time"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
)

// The HA lifecycle Events of issue #298, alongside PromotedToActive which lives
// in promotion_event.go because only promotion holds the object it attaches to.
//
// All four hang off the leader-election Lease, for the reason set out in
// PromotedToActiveEmitter: the recorder derives an Event's namespace from its
// involved object, so a namespaced object is required to land the Event beside
// the controller that emitted it, and the Lease is the one object that both
// exists for this purpose and *is* the leadership record. That makes
// `kubectl -n kubeslice-controller get events` a single place to read the whole
// HA history of a hub.
//
// leaseReference builds that reference by name rather than reading the Lease,
// which is what lets BecameStandby work at all: a Standby has no local Lease
// until the day it promotes, and an Event whose involvedObject names an object
// that does not exist yet is well-formed — the reference carries kind, namespace
// and name, and only the UID is empty.

// emitLifecycleEvent records one HA lifecycle Event against this hub's Lease.
//
// Failures are logged and swallowed. Every caller is on a path where the Event
// is a report of something that has already happened — leadership already lost,
// promotion already abandoned — so failing the caller because the report failed
// would turn an observability gap into an outage.
//
// recorder.RecordEvent is called directly, never util.RecordEvent, for the
// reason promotion_event.go documents at length: that helper starts with
// util.CtxLogger(ctx), which nil-panics on any context that has not been through
// PrepareKubeSliceControllersRequestContext, and every context here comes from
// main.go's signal handler rather than from a reconciler.
func (e *ClusterLeaderElector) emitLifecycleEvent(ctx context.Context, name events.EventName) {
	if e.eventRecorder == nil {
		return
	}
	if err := e.eventRecorder.RecordEvent(ctx, &events.Event{
		Object:            leaseReference(e.leaseName, e.leaseNS),
		ReportingInstance: util.InstanceController,
		Name:              name,
	}); err != nil {
		e.log.Warnw("failed to record HA lifecycle event",
			"event", name, "lease", e.leaseName, "namespace", e.leaseNS, "error", err)
	}
}

// EmitStartupModeEvent records BecameActive or BecameStandby for the mode this
// hub started in. Standalone records nothing: it is the pre-HA behaviour, and an
// Event announcing that HA is switched off would appear on every non-HA
// deployment in existence.
//
// Exported and called from main.go rather than fired inside the constructor, so
// that construction stays free of API-server writes — every existing test builds
// an elector, and none of them should start recording Events by doing so.
func (e *ClusterLeaderElector) EmitStartupModeEvent(ctx context.Context) {
	switch e.Mode() {
	case ModeActive:
		e.emitLifecycleEvent(ctx, ossEvents.EventHABecameActive)
	case ModeStandby:
		e.emitLifecycleEvent(ctx, ossEvents.EventHABecameStandby)
	}
}

// abortPromotion records a refusal to promote, in both surfaces at once: the
// counter labelled with which guard fired, and one PromotionAborted Event.
//
// Single helper rather than a metric increment at each site, because the two
// must not drift — a new abort path that increments the counter and forgets the
// Event (or the reverse) is the kind of gap nobody notices until the one
// failover that needed it. The reason lives only on the metric label and in the
// logs: EventSchema fixes an Event's Message at generation time, so a
// per-reason Event would mean six schema entries for one condition.
func (e *ClusterLeaderElector) abortPromotion(ctx context.Context, reason string) {
	haPromotionsAbortedTotal.WithLabelValues(reason).Inc()
	e.emitLifecycleEvent(ctx, ossEvents.EventHAPromotionAborted)
}

// leaseReference is a Lease valued only for its name and namespace — enough for
// an Event's involvedObject, and never read or written as an object.
func leaseReference(name, namespace string) *coordinationv1.Lease {
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// remoteLeaseAge reports how long ago the given Lease was renewed. The bool is
// false when there is nothing to measure — no Lease read yet, or a Lease with no
// renewTime — which callers must treat as "do not publish", not as zero: an age
// of zero means "renewed just now", the exact opposite of "unknown".
//
// A negative age is clamped to zero rather than reported. It means the Active's
// clock is ahead of this hub's, and a gauge that dips below zero during skew
// would make an age-based alert flap for a reason unrelated to the Active's
// health. The staleness verdict has its own tolerance for this in padding.
func remoteLeaseAge(lease *coordinationv1.Lease, now time.Time) (time.Duration, bool) {
	if lease == nil || lease.Spec.RenewTime == nil {
		return 0, false
	}
	age := now.Sub(lease.Spec.RenewTime.Time)
	if age < 0 {
		return 0, true
	}
	return age, true
}
