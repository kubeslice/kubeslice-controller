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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubeslice/kubeslice-controller/util"
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
		handlerRegistered: map[schema.GroupVersionKind]bool{},
		enqueuedAt:        map[syncKey]time.Time{},
		log:               testLog(),
	}
}

// stubInformer is a minimal cache.Informer fake that only counts
// AddEventHandlerWithResyncPeriod calls; every other method panics via the
// embedded nil interface if exercised (registerInformersOnce never touches
// them).
type stubInformer struct {
	cache.Informer
	addCalls int
}

func (i *stubInformer) AddEventHandlerWithResyncPeriod(_ toolscache.ResourceEventHandler, _ time.Duration) (toolscache.ResourceEventHandlerRegistration, error) {
	i.addCalls++
	return nil, nil
}

// stubCache is a minimal cache.Cache fake that only implements GetInformer,
// letting tests drive registerInformersOnce's retry/dedup behaviour without a
// real *rest.Config.
type stubCache struct {
	cache.Cache
	informer         *stubInformer
	failGVKs         map[schema.GroupVersionKind]int // remaining failures before success, per GVK
	getInformerCalls map[schema.GroupVersionKind]int
}

func (c *stubCache) GetInformer(_ context.Context, obj client.Object, _ ...cache.InformerGetOption) (cache.Informer, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	c.getInformerCalls[gvk]++
	if c.failGVKs[gvk] > 0 {
		c.failGVKs[gvk]--
		return nil, fmt.Errorf("simulated GetInformer failure for %s", gvk)
	}
	return c.informer, nil
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

func TestNamespaceMirrorSelector_MatchesOnlyProjectNamespaces(t *testing.T) {
	sel := namespaceMirrorSelector()
	assert.True(t, sel.Matches(labels.Set(util.LabelsKubeSliceController)),
		"selector must match the labels NamespaceService.ReconcileProjectNamespace actually stamps on project namespaces")
	assert.False(t, sel.Matches(labels.Set{"kubernetes.io/metadata.name": "kube-system"}),
		"selector must not match an unrelated system namespace that happens to exist on the Active hub")
}

func TestRegisterInformers_RetriesUntilSuccess(t *testing.T) {
	s := buildSyncer(t, newStubRemote())
	s.setupRetryPeriod = time.Millisecond

	var calls int32
	s.register = func(_ context.Context) error {
		if atomic.AddInt32(&calls, 1) < 3 {
			return fmt.Errorf("simulated transient informer setup failure")
		}
		return nil
	}

	ok := s.registerInformers(context.Background())
	assert.True(t, ok, "registerInformers must keep retrying instead of giving up on the first failure")
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
}

func TestRegisterInformersOnce_SkipsAlreadyRegisteredHandlersOnRetry(t *testing.T) {
	resA := MirroredResource{GVK: schema.GroupVersionKind{Group: groupController, Version: "v1alpha1", Kind: "Cluster"}}
	resB := MirroredResource{GVK: schema.GroupVersionKind{Group: groupController, Version: "v1alpha1", Kind: "Project"}}

	informer := &stubInformer{}
	sc := &stubCache{
		informer:         informer,
		failGVKs:         map[schema.GroupVersionKind]int{resB.GVK: 1}, // fails once, then succeeds
		getInformerCalls: map[schema.GroupVersionKind]int{},
	}

	s := buildSyncer(t, newStubRemote())
	s.resources = []MirroredResource{resA, resB}
	s.remoteCache = sc

	// First attempt: resA succeeds and its handler gets registered; resB
	// fails at GetInformer, so registerInformersOnce returns an error before
	// reaching the end of the resource list.
	err := s.registerInformersOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, informer.addCalls, "resA's handler should be registered exactly once after the first (partial) attempt")

	// Second attempt (what registerInformers' retry loop would do): resA
	// must NOT be re-registered — AddEventHandlerWithResyncPeriod is not
	// idempotent, so a naive from-scratch retry would double resA's event
	// and resync load. resB now succeeds and gets registered for the first time.
	err = s.registerInformersOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, informer.addCalls, "retry must add resB's handler but must not double-register resA's")
}

func TestRegisterInformers_StopsRetryingOnContextCancel(t *testing.T) {
	s := buildSyncer(t, newStubRemote())
	s.setupRetryPeriod = 50 * time.Millisecond
	s.register = func(_ context.Context) error {
		return fmt.Errorf("always fails")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- s.registerInformers(ctx) }()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case ok := <-done:
		assert.False(t, ok, "registerInformers must report failure when ctx is cancelled mid-retry")
	case <-time.After(2 * time.Second):
		t.Fatal("registerInformers did not return after context cancellation")
	}
}
