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

func TestPruneOnce_ReverseDiffEnqueuesActiveObjectsMissingLocally(t *testing.T) {
	ctx := context.Background()
	missing := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-missing"}

	// Active has an object the Standby has no mirror of — a create the
	// forward (orphan) pass can't see: a mirror deleted directly on the
	// Standby, a skip decided before the namespace informer synced, or a key
	// stuck deep in retry backoff.
	s := buildSyncer(t, newStubRemote())
	s.remoteList = stubRemoteList([]syncKey{missing}, nil)

	s.pruneOnce(ctx)

	require.Equal(t, 1, s.queue.Len())
	got, _ := s.queue.Get()
	assert.Equal(t, missing, got)
}

// TestPruneOnce_ReverseDiffSkipsNamespacesOutsideTheMirror pins the gate that
// keeps the reverse diff finite. A RequireMirroredNamespace row reads an
// unscoped remote cache — Secrets can be narrowed neither by label nor by field
// — so the Active-side listing carries the whole hub. Every out-of-scope key is
// permanently "missing locally", so without the gate each one is re-enqueued on
// every pass forever.
func TestPruneOnce_ReverseDiffSkipsNamespacesOutsideTheMirror(t *testing.T) {
	ctx := context.Background()
	inScope := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "wanted"}
	alsoInScope := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "also-wanted"}
	outOfScope := syncKey{GVK: testGVK, Namespace: "kube-system", Name: "unrelated"}

	remote := newStubRemote()
	projNS := syncKey{GVK: nsGVK, Name: "proj-a"}
	remote.objects[projNS] = newTestUnstructured(nsGVK, "", "proj-a")

	s := buildSyncer(t, remote)
	s.resources = []MirroredResource{{GVK: testGVK, RequireMirroredNamespace: true}}
	s.remoteList = stubRemoteList([]syncKey{inScope, alsoInScope, outOfScope}, nil)

	s.pruneOnce(ctx)

	assert.Equal(t, 2, s.queue.Len(), "only the two in-scope keys may be re-enqueued")
	got := map[syncKey]bool{}
	for s.queue.Len() > 0 {
		k, _ := s.queue.Get()
		got[k] = true
	}
	assert.True(t, got[inScope])
	assert.True(t, got[alsoInScope])
	assert.False(t, got[outOfScope], "a namespace the mirror does not cover must never be enqueued")

	assert.Equal(t, 1, remote.callCount(projNS),
		"the namespace verdict must be memoised for the pass, not re-read per key")
}

// TestPruneOnce_ReverseDiffStillEnqueuesWhenTheRowIsUnscoped guards the other
// side of the gate: a row without RequireMirroredNamespace has a label-scoped
// remote cache already, so the reverse diff must not start consulting the
// Namespace view for it.
func TestPruneOnce_ReverseDiffStillEnqueuesWhenTheRowIsUnscoped(t *testing.T) {
	ctx := context.Background()
	missing := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-missing"}

	remote := newStubRemote() // no Namespace object registered at all
	s := buildSyncer(t, remote)
	s.remoteList = stubRemoteList([]syncKey{missing}, nil)

	s.pruneOnce(ctx)

	require.Equal(t, 1, s.queue.Len())
	got, _ := s.queue.Get()
	assert.Equal(t, missing, got)
	assert.Zero(t, remote.callCount(syncKey{GVK: nsGVK, Name: "proj-a"}),
		"an unscoped row must not pay for a namespace lookup")
}

func TestPruneOnce_ReverseDiffCannotOverrideConflictGuard(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "hand-created"}

	// The object exists on both sides, but the Standby's copy is not
	// syncer-owned (no sync label) — so it is absent from the forward pass's
	// labeled listing and the reverse diff re-enqueues it every round. That
	// must stay harmless: the worker's conflict guard refuses the write.
	remote := newStubRemote()
	remote.objects[key] = newTestUnstructured(testGVK, key.Namespace, key.Name)
	s := buildSyncer(t, remote)
	require.NoError(t, s.localClient.Create(ctx, newTestUnstructured(testGVK, key.Namespace, key.Name)))
	s.remoteList = stubRemoteList([]syncKey{key}, nil)

	s.pruneOnce(ctx)
	k, shutdown := s.queue.Get()
	require.False(t, shutdown)
	require.Equal(t, key, k)
	s.processOnce(ctx, k)

	got := getUnstructured(t, s.localClient, key)
	assert.NotEqual(t, LabelValueActive, got.GetLabels()[LabelSyncedFromActive],
		"a hand-created Standby object must never be adopted by the mirror, even via the prune loop")
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
