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
	"testing"
	"time"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
)

// eventsWithReason lists the events on c carrying the given reason. Reason
// rather than event name, because reason is what an operator filters on with
// `kubectl get events --field-selector reason=...` and therefore what the
// runbook documents.
func eventsWithReason(t *testing.T, c client.Client, reason string) []corev1.Event {
	t.Helper()
	list := &corev1.EventList{}
	require.NoError(t, c.List(context.Background(), list))
	var out []corev1.Event
	for _, ev := range list.Items {
		if ev.Reason == reason {
			out = append(out, ev)
		}
	}
	return out
}

// standbyWithRecorder builds a Standby wired to a recorder backed by c.
func standbyWithRecorder(t *testing.T, c client.Client, mode HAMode) *ClusterLeaderElector {
	t.Helper()
	return NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode:          mode,
		Identity:      "hub-b",
		EventRecorder: testEventRecorder(t, c, ossEvents.EventsMap),
		Log:           testLog(),
	})
}

// TestHALifecycleEvents_RegisteredInGeneratedMap pins the generate-events step
// this feature depends on. RecordEvent hard-fails for a name that is not in
// EventsMap, so an entry added to config/events/controller.yaml without
// re-running `make generate-events` produces code that compiles, emits nothing,
// and logs a warning nobody reads.
func TestHALifecycleEvents_RegisteredInGeneratedMap(t *testing.T) {
	for _, name := range []events.EventName{
		ossEvents.EventHABecameActive,
		ossEvents.EventHABecameStandby,
		ossEvents.EventHALeadershipLost,
		ossEvents.EventHAPromotionAborted,
	} {
		require.Contains(t, ossEvents.EventsMap, name,
			"%s must be in the generated EventsMap — re-run `make generate-events`", name)
	}

	// The reasons are the operator-facing contract from issue #298's table, and
	// they deliberately differ from the internal event names.
	assert.Equal(t, "BecameActive", ossEvents.EventsMap[ossEvents.EventHABecameActive].Reason)
	assert.Equal(t, "BecameStandby", ossEvents.EventsMap[ossEvents.EventHABecameStandby].Reason)
	assert.Equal(t, "LeadershipLost", ossEvents.EventsMap[ossEvents.EventHALeadershipLost].Reason)
	assert.Equal(t, "PromotionAborted", ossEvents.EventsMap[ossEvents.EventHAPromotionAborted].Reason)

	// Severity matters: these three are the ones an operator should see without
	// going looking for them.
	assert.Equal(t, events.EventTypeWarning, ossEvents.EventsMap[ossEvents.EventHALeadershipLost].Type)
	assert.Equal(t, events.EventTypeWarning, ossEvents.EventsMap[ossEvents.EventHAPromotionAborted].Type)
	assert.Equal(t, events.EventTypeNormal, ossEvents.EventsMap[ossEvents.EventHABecameActive].Type)
}

func TestEmitStartupModeEvent_RecordsTheModeThisHubStartedIn(t *testing.T) {
	ctx := context.Background()

	standbyEvents := fakeClient(t)
	standbyWithRecorder(t, standbyEvents, ModeStandby).EmitStartupModeEvent(ctx)
	assert.Len(t, eventsWithReason(t, standbyEvents, "BecameStandby"), 1)
	assert.Empty(t, eventsWithReason(t, standbyEvents, "BecameActive"))

	activeEvents := fakeClient(t)
	standbyWithRecorder(t, activeEvents, ModeActive).EmitStartupModeEvent(ctx)
	assert.Len(t, eventsWithReason(t, activeEvents, "BecameActive"), 1)
	assert.Empty(t, eventsWithReason(t, activeEvents, "BecameStandby"))
}

// TestEmitStartupModeEvent_StandaloneIsSilent is the no-regression guarantee.
// Standalone is the default mode and the pre-HA behaviour; an Event announcing
// that HA is switched off would appear on every non-HA deployment there is.
func TestEmitStartupModeEvent_StandaloneIsSilent(t *testing.T) {
	c := fakeClient(t)
	standbyWithRecorder(t, c, ModeStandalone).EmitStartupModeEvent(context.Background())

	list := &corev1.EventList{}
	require.NoError(t, c.List(context.Background(), list))
	assert.Empty(t, list.Items, "standalone mode must record no HA lifecycle events at all")
}

// TestEmitLifecycleEvent_AttachesToTheLeaseInTheControllerNamespace pins where
// these land. The recorder derives an Event's namespace from its involved
// object, so naming the Lease is what puts the Event beside the controller that
// emitted it — and NOT in kubeslice-system, which is a worker namespace that
// does not exist on a hub at all.
func TestEmitLifecycleEvent_AttachesToTheLeaseInTheControllerNamespace(t *testing.T) {
	c := fakeClient(t)
	e := standbyWithRecorder(t, c, ModeStandby)
	e.EmitStartupModeEvent(context.Background())

	got := eventsWithReason(t, c, "BecameStandby")
	require.Len(t, got, 1)
	assert.Equal(t, DefaultLeaseNamespace, got[0].Namespace)
	assert.Equal(t, DefaultLeaseName, got[0].InvolvedObject.Name)
	assert.Equal(t, "Lease", got[0].InvolvedObject.Kind)
}

// TestEmitLifecycleEvent_WorksBeforeTheLeaseExists is why leaseReference builds
// a reference by name instead of reading the object. A Standby has no local
// Lease until the day it promotes, so requiring one would mean BecameStandby —
// the one mode event a Standby can emit — could never be recorded.
func TestEmitLifecycleEvent_WorksBeforeTheLeaseExists(t *testing.T) {
	c := fakeClient(t) // no Lease anywhere
	e := standbyWithRecorder(t, c, ModeStandby)

	e.EmitStartupModeEvent(context.Background())

	assert.Len(t, eventsWithReason(t, c, "BecameStandby"), 1,
		"the event must record against a Lease that does not exist yet")
}

func TestEmitLifecycleEvent_NilRecorderIsANoop(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	require.Nil(t, e.eventRecorder)

	// Must not panic, and must leave every other effect intact.
	assert.NotPanics(t, func() {
		e.EmitStartupModeEvent(context.Background())
		e.emitLifecycleEvent(context.Background(), ossEvents.EventHALeadershipLost)
	})
}

// TestEmitLifecycleEvent_RecorderFailureIsSwallowed keeps an observability gap
// from becoming an outage. Every caller is reporting something that has already
// happened, so a failed Event write must not fail the caller.
func TestEmitLifecycleEvent_RecorderFailureIsSwallowed(t *testing.T) {
	// An EventsMap without the HA entries makes RecordEvent return an error.
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode:          ModeStandby,
		Identity:      "hub-b",
		EventRecorder: testEventRecorder(t, fakeClient(t), map[events.EventName]*events.EventSchema{}),
		Log:           testLog(),
	})

	assert.NotPanics(t, func() { e.EmitStartupModeEvent(context.Background()) })
}

func TestAbortPromotion_CountsTheReasonAndEmitsOneEvent(t *testing.T) {
	haPromotionsAbortedTotal.Reset()
	c := fakeClient(t)
	e := standbyWithRecorder(t, c, ModeStandby)

	e.abortPromotion(context.Background(), abortSelfUnhealthy)

	assert.Equal(t, float64(1), testutil.ToFloat64(haPromotionsAbortedTotal.WithLabelValues(abortSelfUnhealthy)))
	got := eventsWithReason(t, c, "PromotionAborted")
	require.Len(t, got, 1, "a refusal to promote must be visible as an Event, not only as a metric")
	assert.Equal(t, string(corev1.EventTypeWarning), got[0].Type)
}

// TestGuardRefusal_EmitsPromotionAborted checks the wiring end to end: a live
// Active must produce both the metric and the Event through the real guard path,
// not just through a direct call to the helper.
func TestGuardRefusal_EmitsPromotionAborted(t *testing.T) {
	ctx := context.Background()
	haPromotionsAbortedTotal.Reset()

	remote := fakeClient(t)
	require.NoError(t, remote.Create(ctx, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now())))

	eventsClient := fakeClient(t)
	e := NewClusterLeaderElector(fakeClient(t), remote, Options{
		Mode:          ModeStandby,
		Identity:      "hub-b",
		EventRecorder: testEventRecorder(t, eventsClient, ossEvents.EventsMap),
		Log:           testLog(),
	})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))

	promoted, err := e.promote(ctx)
	require.NoError(t, err)
	require.False(t, promoted)

	assert.Len(t, eventsWithReason(t, eventsClient, "PromotionAborted"), 1)
	assert.Equal(t, float64(1), testutil.ToFloat64(haPromotionsAbortedTotal.WithLabelValues(abortLeaseLive)))
}

// TestRenewOnce_EmitsLeadershipLostOnlyPastTheRenewDeadline is the distinction
// issue #298's table draws: the Warning belongs to an Active that has actually
// given up leadership, not to every transient renewal failure. A hub still
// inside renewDeadline is expected to keep going quietly.
func TestRenewOnce_EmitsLeadershipLostOnlyPastTheRenewDeadline(t *testing.T) {
	ctx := context.Background()

	// Inside the deadline: leadership retained, no Event.
	quiet := fakeClient(t)
	within := NewClusterLeaderElector(failingWriteClient(t), nil, Options{
		Mode:          ModeActive,
		Identity:      "hub-a",
		RenewDeadline: time.Hour,
		EventRecorder: testEventRecorder(t, quiet, ossEvents.EventsMap),
		Log:           testLog(),
	})
	within.setLeader(true)
	within.lastRenew = time.Now()
	require.Error(t, within.renewOnce(ctx))
	assert.True(t, within.IsLeader(), "a failure inside the renew deadline keeps leadership")
	assert.Empty(t, eventsWithReason(t, quiet, "LeadershipLost"))

	// Past the deadline: leadership released, exactly one Event.
	loud := fakeClient(t)
	past := NewClusterLeaderElector(failingWriteClient(t), nil, Options{
		Mode:          ModeActive,
		Identity:      "hub-a",
		RenewDeadline: 10 * time.Millisecond,
		EventRecorder: testEventRecorder(t, loud, ossEvents.EventsMap),
		Log:           testLog(),
	})
	past.setLeader(true)
	past.lastRenew = time.Now().Add(-time.Hour)
	require.Error(t, past.renewOnce(ctx))
	assert.False(t, past.IsLeader(), "a failure past the renew deadline must release leadership")
	assert.Len(t, eventsWithReason(t, loud, "LeadershipLost"), 1)
}

// TestStartLeaseRenewal_ShutdownDoesNotEmitLeadershipLost is why the Event is
// emitted in renewOnce rather than in setLeader. Graceful shutdown also drops
// leadership, and a Warning Event on every rolling restart is noise — besides
// racing the pod's own termination for the write.
func TestStartLeaseRenewal_ShutdownDoesNotEmitLeadershipLost(t *testing.T) {
	c := fakeClient(t)
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{
		Mode:          ModeActive,
		Identity:      "hub-a",
		RetryPeriod:   time.Hour, // never ticks; only ctx cancellation ends the loop
		EventRecorder: testEventRecorder(t, c, ossEvents.EventsMap),
		Log:           testLog(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, e.StartLeaseRenewal(ctx))

	assert.False(t, e.IsLeader())
	assert.Empty(t, eventsWithReason(t, c, "LeadershipLost"),
		"a graceful shutdown must not report lost leadership as a warning")
}
