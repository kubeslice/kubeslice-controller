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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// stubRemoteList returns a fixed key set (or error) regardless of GVK,
// standing in for listFromRemoteCache the same way stubRemote stands in for
// getFromRemoteCache.
func stubRemoteList(keys []syncKey, err error) remoteListFunc {
	return func(_ context.Context, gvk schema.GroupVersionKind) (map[syncKey]struct{}, error) {
		if err != nil {
			return nil, err
		}
		set := make(map[syncKey]struct{}, len(keys))
		for _, k := range keys {
			if k.GVK == gvk {
				set[k] = struct{}{}
			}
		}
		return set, nil
	}
}

func labeledMirror(gvk schema.GroupVersionKind, namespace, name string) *unstructured.Unstructured {
	u := newTestUnstructured(gvk, namespace, name)
	u.SetLabels(map[string]string{LabelSyncedFromActive: LabelValueActive})
	return u
}

func TestPruneOnce_EnqueuesOnlyOrphanedMirrors(t *testing.T) {
	ctx := context.Background()
	kept := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-kept"}
	orphan := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-orphan"}

	s := buildSyncer(t, newStubRemote())
	require.NoError(t, s.localClient.Create(ctx, labeledMirror(testGVK, kept.Namespace, kept.Name)))
	require.NoError(t, s.localClient.Create(ctx, labeledMirror(testGVK, orphan.Namespace, orphan.Name)))
	s.remoteList = stubRemoteList([]syncKey{kept}, nil)

	s.pruneOnce(ctx)

	require.Equal(t, 1, s.queue.Len(), "only the mirror missing from Active should be enqueued")
	got, _ := s.queue.Get()
	assert.Equal(t, orphan, got)
}

func TestPruneOnce_ThenWorkerDeletesOrphan(t *testing.T) {
	ctx := context.Background()
	orphan := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-orphan"}

	// The stub remote holds nothing, so the worker's re-read reports NotFound
	// — the same path an informer-delivered delete takes.
	s := buildSyncer(t, newStubRemote())
	require.NoError(t, s.localClient.Create(ctx, labeledMirror(testGVK, orphan.Namespace, orphan.Name)))
	s.remoteList = stubRemoteList(nil, nil)

	s.pruneOnce(ctx)
	key, shutdown := s.queue.Get()
	require.False(t, shutdown)
	s.processOnce(ctx, key)

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(testGVK)
	err := s.localClient.Get(ctx, types.NamespacedName{Namespace: orphan.Namespace, Name: orphan.Name}, got)
	assert.True(t, apierrors.IsNotFound(err), "the orphaned mirror should be gone after the worker processes the pruned key")
}

func TestPruneOnce_LeavesUnlabeledObjectsAlone(t *testing.T) {
	ctx := context.Background()
	s := buildSyncer(t, newStubRemote())
	// An object the Standby's own users created — no sync label — must never
	// be pruned, even though Active has no such object.
	require.NoError(t, s.localClient.Create(ctx, newTestUnstructured(testGVK, "proj-a", "hand-created")))
	s.remoteList = stubRemoteList(nil, nil)

	s.pruneOnce(ctx)

	assert.Equal(t, 0, s.queue.Len(), "objects without the sync label are not the engine's to prune")
}

func TestPruneOnce_SkipsKindWhenRemoteListFails(t *testing.T) {
	ctx := context.Background()
	orphan := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-orphan"}

	s := buildSyncer(t, newStubRemote())
	require.NoError(t, s.localClient.Create(ctx, labeledMirror(testGVK, orphan.Namespace, orphan.Name)))
	s.remoteList = stubRemoteList(nil, fmt.Errorf("simulated transient list failure"))

	s.pruneOnce(ctx)

	assert.Equal(t, 0, s.queue.Len(), "a failed list must not be read as \"everything was deleted on Active\"")
}

func TestRunPrune_DoesNotPruneBeforeCacheSync(t *testing.T) {
	s := buildSyncer(t, newStubRemote())
	s.pruneInterval = time.Millisecond
	s.waitForCacheSync = func(ctx context.Context) bool { return false } // never syncs (ctx cancelled)

	var listCalls int32
	s.remoteList = func(_ context.Context, _ schema.GroupVersionKind) (map[syncKey]struct{}, error) {
		atomic.AddInt32(&listCalls, 1)
		return nil, nil
	}

	done := make(chan struct{})
	go func() { s.runPrune(context.Background()); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runPrune did not return after waitForCacheSync reported failure")
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(&listCalls),
		"pruning against an unsynced cache would delete every mirror; runPrune must bail out instead")
}

func TestRunPrune_TicksAndStopsOnContextCancel(t *testing.T) {
	s := buildSyncer(t, newStubRemote())
	s.pruneInterval = 5 * time.Millisecond
	s.waitForCacheSync = func(ctx context.Context) bool { return true }

	var listCalls int32
	s.remoteList = func(_ context.Context, _ schema.GroupVersionKind) (map[syncKey]struct{}, error) {
		atomic.AddInt32(&listCalls, 1)
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.runPrune(ctx); close(done) }()

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&listCalls) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.Greater(t, atomic.LoadInt32(&listCalls), int32(0), "the prune loop should tick once the cache is synced")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runPrune did not return after context cancellation")
	}
}
