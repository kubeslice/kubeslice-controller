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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// stubRemote is a remoteGetFunc backend the retry-engine tests drive
// directly, so they exercise RemoteSyncer's workqueue/retry logic without a
// real *rest.Config or live cluster.
type stubRemote struct {
	mu      sync.Mutex
	objects map[syncKey]*unstructured.Unstructured
	errs    map[syncKey]error
	calls   map[syncKey]int
}

func newStubRemote() *stubRemote {
	return &stubRemote{
		objects: map[syncKey]*unstructured.Unstructured{},
		errs:    map[syncKey]error{},
		calls:   map[syncKey]int{},
	}
}

func (s *stubRemote) get(_ context.Context, key syncKey) (*unstructured.Unstructured, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[key]++
	if err, ok := s.errs[key]; ok {
		return nil, err
	}
	if obj, ok := s.objects[key]; ok {
		return obj, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: key.GVK.Group, Resource: key.GVK.Kind}, key.Name)
}

func (s *stubRemote) callCount(key syncKey) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[key]
}

// buildSyncer constructs a RemoteSyncer by struct literal (in-package test),
// backed by remote and a fast-backoff queue, bypassing NewRemoteSyncer's
// cache.New so no *rest.Config is needed.
func buildSyncer(t *testing.T, remote *stubRemote) *RemoteSyncer {
	t.Helper()
	return &RemoteSyncer{
		mode:        ModeStandby,
		localClient: mirrorFakeClient(t),
		remoteGet:   remote.get,
		resources:   []MirroredResource{{GVK: testGVK}},
		byGVK:       map[schema.GroupVersionKind]MirroredResource{testGVK: {GVK: testGVK}},
		workers:     1,
		queue: workqueue.NewTypedRateLimitingQueue[syncKey](
			workqueue.NewTypedItemExponentialFailureRateLimiter[syncKey](time.Millisecond, time.Second),
		),
		enqueuedAt: map[syncKey]time.Time{},
		log:        testLog(),
	}
}

func TestRemoteSyncer_ReconcileKey_FoundMirrorsCreate(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	remote := newStubRemote()
	remote.objects[key] = newTestUnstructured(testGVK, key.Namespace, key.Name)

	s := buildSyncer(t, remote)
	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, opCreate, op)

	got := getUnstructured(t, s.localClient, key)
	assert.Equal(t, LabelValueActive, got.GetLabels()[LabelSyncedFromActive])
}

func TestRemoteSyncer_ReconcileKey_NotFoundMirrorsDelete(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	remote := newStubRemote() // nothing registered -> NotFound

	s := buildSyncer(t, remote)
	existing := newTestUnstructured(testGVK, key.Namespace, key.Name)
	existing.SetLabels(map[string]string{LabelSyncedFromActive: LabelValueActive})
	require.NoError(t, s.localClient.Create(ctx, existing))

	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, opDelete, op)

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(key.GVK)
	err = s.localClient.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, got)
	assert.True(t, apierrors.IsNotFound(err), "the Standby mirror should be gone once Active reports NotFound")
}

func TestRemoteSyncer_ProcessOnce_RetriesOnErrorAndRedelivers(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	remote := newStubRemote()
	remote.errs[key] = fmt.Errorf("simulated transient read failure")

	s := buildSyncer(t, remote)
	s.queue.Add(key)

	got, shutdown := s.queue.Get()
	require.False(t, shutdown)
	require.Equal(t, key, got)
	s.processOnce(ctx, got)

	// AddRateLimited schedules redelivery asynchronously; poll briefly for it
	// rather than sleeping a fixed guess.
	deadline := time.Now().Add(2 * time.Second)
	redelivered := false
	for time.Now().Before(deadline) {
		if s.queue.Len() > 0 {
			redelivered = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.True(t, redelivered, "a failed sync must be retried, not dropped (issue #295's own acceptance criterion)")
	assert.GreaterOrEqual(t, remote.callCount(key), 1)
}

func TestRemoteSyncer_HandlersFor_UnwrapsDeletedFinalStateUnknown(t *testing.T) {
	s := buildSyncer(t, newStubRemote())
	handlers := s.handlersFor(testGVK)

	u := newTestUnstructured(testGVK, "proj-a", "sc-1")
	handlers.DeleteFunc(toolscache.DeletedFinalStateUnknown{Key: "proj-a/sc-1", Obj: u})

	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	require.Equal(t, 1, s.queue.Len())
	got, _ := s.queue.Get()
	assert.Equal(t, key, got)
}

func TestRemoteSyncer_HandlersFor_IgnoresUnexpectedType(t *testing.T) {
	s := buildSyncer(t, newStubRemote())
	handlers := s.handlersFor(testGVK)
	handlers.AddFunc("not-an-unstructured")
	assert.Equal(t, 0, s.queue.Len())
}

func TestRemoteSyncer_Start_NoopWhenNotStandby(t *testing.T) {
	c := mirrorFakeClient(t)
	s, err := NewRemoteSyncer(c, nil, nil, ModeStandalone, RemoteSyncerOptions{Log: testLog()})
	require.NoError(t, err)
	assert.NoError(t, s.Start(context.Background()))
}

func TestNewRemoteSyncer_StandbyRequiresRemoteConfig(t *testing.T) {
	c := mirrorFakeClient(t)
	_, err := NewRemoteSyncer(c, nil, testScheme(t), ModeStandby, RemoteSyncerOptions{Log: testLog()})
	assert.Error(t, err)
}
