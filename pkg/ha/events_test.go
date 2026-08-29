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
	"testing"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
)

// syncFailedEvents lists the HAMirrorSyncFailed events currently on c, in
// key.Namespace.
func syncFailedEvents(t *testing.T, c client.Client, namespace string) []corev1.Event {
	t.Helper()
	list := &corev1.EventList{}
	require.NoError(t, c.List(context.Background(), list, client.InNamespace(namespace)))
	var out []corev1.Event
	for _, ev := range list.Items {
		if ev.Reason == string(ossEvents.EventHAMirrorSyncFailed) {
			out = append(out, ev)
		}
	}
	return out
}

func testEventRecorder(t *testing.T, c client.Client, eventsMap map[events.EventName]*events.EventSchema) events.EventRecorder {
	t.Helper()
	return events.NewEventRecorder(c, testScheme(t), eventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   "test-cluster",
		Component: "controller",
	})
}

func TestProcessOnce_EmitsHAMirrorSyncFailedOncePerFailureEpisode(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	remote := newStubRemote()
	remote.errs[key] = fmt.Errorf("simulated transient read failure")

	s := buildSyncer(t, remote)
	eventsClient := fakeClient(t)
	s.eventRecorder = testEventRecorder(t, eventsClient, ossEvents.EventsMap)

	// First failure of the episode -> exactly one event.
	s.queue.Add(key)
	k, _ := s.queue.Get()
	s.processOnce(ctx, k)

	got := syncFailedEvents(t, eventsClient, key.Namespace)
	require.Len(t, got, 1, "the first mirror failure must surface as a Kubernetes event")
	assert.Equal(t, key.Name, got[0].InvolvedObject.Name, "the event must be attached to the object that failed to mirror")
	assert.Equal(t, int32(1), got[0].Count)

	// A retry failing within the same episode must not emit again — the
	// recorder aggregates by Count, so an ungated second RecordEvent call
	// would show up here as Count==2.
	k, _ = s.queue.Get() // AddRateLimited's redelivery
	s.processOnce(ctx, k)

	got = syncFailedEvents(t, eventsClient, key.Namespace)
	require.Len(t, got, 1)
	assert.Equal(t, int32(1), got[0].Count, "retries within one failure episode must not re-emit the event")

	// Recovery resets the episode (Forget zeroes NumRequeues)...
	remote.mu.Lock()
	delete(remote.errs, key)
	remote.objects[key] = newTestUnstructured(testGVK, key.Namespace, key.Name)
	remote.mu.Unlock()
	k, _ = s.queue.Get()
	s.processOnce(ctx, k)
	require.Len(t, syncFailedEvents(t, eventsClient, key.Namespace), 1, "a successful sync must not emit a failure event")

	// ...so a fresh failure afterwards is a new episode and emits again.
	remote.mu.Lock()
	remote.errs[key] = fmt.Errorf("simulated second outage")
	remote.mu.Unlock()
	s.queue.Add(key)
	k, _ = s.queue.Get()
	s.processOnce(ctx, k)

	got = syncFailedEvents(t, eventsClient, key.Namespace)
	require.Len(t, got, 1)
	assert.Equal(t, int32(2), got[0].Count, "a new failure episode after recovery must emit again")
}

func TestProcessOnce_NoRecorderMeansNoEventButStillRetries(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	remote := newStubRemote()
	remote.errs[key] = fmt.Errorf("simulated transient read failure")

	s := buildSyncer(t, remote) // eventRecorder deliberately left nil
	s.queue.Add(key)
	k, _ := s.queue.Get()
	s.processOnce(ctx, k)

	// The nil recorder must not panic, and the retry contract is unchanged:
	// the key comes back.
	k, shutdown := s.queue.Get()
	require.False(t, shutdown)
	assert.Equal(t, key, k)
}

// TestHAMirrorSyncFailedEvent_RegisteredInGeneratedMap guards the
// generate-events step this feature depends on: RecordEvent hard-fails for
// any EventName missing from the generated EventsMap, so if the
// config/events/controller.yaml entry (or the generated code) is ever
// reverted, this fails in `go test` rather than silently no-opping at
// runtime.
func TestHAMirrorSyncFailedEvent_RegisteredInGeneratedMap(t *testing.T) {
	ctx := context.Background()
	require.Contains(t, ossEvents.EventsMap, ossEvents.EventHAMirrorSyncFailed,
		"HAMirrorSyncFailed must be present in the generated EventsMap — re-run `make generate-events` if config/events/controller.yaml changed")

	obj := newTestUnstructured(testGVK, "proj-a", "sc-1")
	registered := testEventRecorder(t, fakeClient(t), ossEvents.EventsMap)
	assert.NoError(t, registered.RecordEvent(ctx, &events.Event{
		Object:            obj,
		ReportingInstance: "controller",
		Name:              ossEvents.EventHAMirrorSyncFailed,
	}), "recording HAMirrorSyncFailed against the real generated EventsMap must succeed")

	// And the failure mode being guarded against is loud, not silent:
	unregistered := testEventRecorder(t, fakeClient(t), map[events.EventName]*events.EventSchema{})
	assert.Error(t, unregistered.RecordEvent(ctx, &events.Event{
		Object:            obj,
		ReportingInstance: "controller",
		Name:              ossEvents.EventHAMirrorSyncFailed,
	}), "an EventName absent from EventsMap must error, proving a missing generated entry cannot no-op silently")
}
