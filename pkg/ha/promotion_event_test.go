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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
)

func promotedEvents(t *testing.T, c client.Client, namespace string) []corev1.Event {
	t.Helper()
	list := &corev1.EventList{}
	require.NoError(t, c.List(context.Background(), list, client.InNamespace(namespace)))
	var out []corev1.Event
	for _, ev := range list.Items {
		if ev.Reason == "PromotedToActive" {
			out = append(out, ev)
		}
	}
	return out
}

// TestPromotedToActiveEvent_LandsInTheControllersOwnNamespace is the reason the
// Lease is the involved object. Issue #297 asks for this Event on
// kubeslice-system, which does not exist on a hub — ADR #293 Decision 1 is
// explicit that it is a worker-cluster namespace. The recorder derives the
// Event's namespace from the involved object, so attaching it to the Lease is
// what puts it beside the controller that emitted it.
func TestPromotedToActiveEvent_LandsInTheControllersOwnNamespace(t *testing.T) {
	eventsClient := fakeClient(t)
	recorder := testEventRecorder(t, eventsClient, ossEvents.EventsMap)
	emit := PromotedToActiveEmitter(recorder)
	require.NotNil(t, emit)

	lease := newLease(DefaultLeaseName, "kubeslice-controller", "hub-b", time.Now())
	require.NoError(t, emit(context.Background(), lease))

	got := promotedEvents(t, eventsClient, "kubeslice-controller")
	require.Len(t, got, 1, "promotion must surface as a Kubernetes event")
	assert.Equal(t, DefaultLeaseName, got[0].InvolvedObject.Name,
		"the Lease is the leadership record, so it is what the event describes")
	assert.Equal(t, "Lease", got[0].InvolvedObject.Kind)
	assert.Equal(t, corev1.EventTypeNormal, got[0].Type, "a successful failover is not a warning")

	// And nothing landed in the worker namespace the issue names.
	assert.Empty(t, promotedEvents(t, eventsClient, "kubeslice-system"),
		"kubeslice-system is a worker-cluster namespace and does not exist on a hub")
}

// TestPromotedToActiveEvent_FollowsTheLeaseNamespace: the namespace is not
// hardcoded anywhere, it comes from wherever the Lease actually lives — which
// is the downward-API-derived namespace the controller is deployed into.
func TestPromotedToActiveEvent_FollowsTheLeaseNamespace(t *testing.T) {
	eventsClient := fakeClient(t)
	emit := PromotedToActiveEmitter(testEventRecorder(t, eventsClient, ossEvents.EventsMap))

	lease := newLease(DefaultLeaseName, "kubeslice-avesha", "hub-b", time.Now())
	require.NoError(t, emit(context.Background(), lease))

	assert.Len(t, promotedEvents(t, eventsClient, "kubeslice-avesha"), 1,
		"a controller deployed into a non-default namespace must emit there, not into a fixed one")
}

// TestPromotedToActiveEvent_RequiresGeneratedMapEntry guards the generation
// step. RecordEvent hard-fails on an event name that is not in EventsMap, so a
// hand-written config/events/controller.yaml entry without a `make
// generate-events` run would fail at the worst possible moment — during a real
// failover.
func TestPromotedToActiveEvent_RequiresGeneratedMapEntry(t *testing.T) {
	schema, ok := ossEvents.EventsMap[ossEvents.EventHAPromotedToActive]
	require.True(t, ok,
		"EventHAPromotedToActive is missing from the generated EventsMap — run `make generate-events`")
	assert.Equal(t, "PromotedToActive", schema.Reason)
	assert.Equal(t, events.EventTypeNormal, schema.Type)

	// And the failure being guarded against is loud rather than silent.
	unregistered := testEventRecorder(t, fakeClient(t), map[events.EventName]*events.EventSchema{})
	emit := PromotedToActiveEmitter(unregistered)
	assert.Error(t, emit(context.Background(), newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-b", time.Now())),
		"an unregistered event name must surface as an error rather than being dropped")
}

func TestPromotedToActiveEmitter_NilRecorderDisablesEmission(t *testing.T) {
	assert.Nil(t, PromotedToActiveEmitter(nil),
		"a nil recorder must yield a nil hook, which promote() skips")
}

func TestPromotedToActiveEmitter_NilLeaseIsNotFatal(t *testing.T) {
	emit := PromotedToActiveEmitter(testEventRecorder(t, fakeClient(t), ossEvents.EventsMap))
	assert.NoError(t, emit(context.Background(), nil),
		"promotion has already succeeded by this point; a missing lease must not be reported as failure")
}
