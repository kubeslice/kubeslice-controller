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
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubeslice/kubeslice-controller/util"
)

// Default RemoteSyncer tunables, overridable through RemoteSyncerOptions.
const (
	DefaultSyncWorkers          = 4
	DefaultInformerResyncPeriod = 10 * time.Minute
)

// opDelete extends mirror.go's opCreate/opUpdate for use in this file's
// metrics/logging; mirrorDelete itself has no ambiguity about which
// operation it performed, so it doesn't need to return one.
const opDelete mirrorOp = "delete"

// remoteGetFunc reads one object from the Active hub by key. The real
// implementation (getFromRemoteCache) reads controller-runtime's cache, which
// is itself a local-indexer read, not network I/O — so retrying via this
// function is cheap even under the workqueue's backoff. Overridable in tests
// so the retry engine is exercised without a real *rest.Config.
type remoteGetFunc func(ctx context.Context, key syncKey) (*unstructured.Unstructured, error)

// RemoteSyncerOptions configures a RemoteSyncer. Zero-valued fields fall back
// to the Default* constants, matching pkg/ha's existing Options pattern
// (see ClusterLeaderElector's Options in leader_elector.go).
type RemoteSyncerOptions struct {
	// Resources is the mirrored-resource table. Defaults to CRDMirrorSet.
	Resources []MirroredResource
	// Workers is the number of goroutines draining the mirror workqueue.
	Workers int
	Log     *zap.SugaredLogger
}

// RemoteSyncer mirrors a fixed set of resources from the Active hub onto the
// Standby's own cluster. It runs only in standby mode: informer event
// handlers enqueue a syncKey (no mirror logic in the callback itself), and a
// small worker pool dequeues, re-reads the object from the Active cache, and
// mirrors it — retrying with backoff via a rate-limited workqueue on any
// failure, the same primitive controller-runtime's own Controller uses
// internally. See ADR #293 and issue #295.
type RemoteSyncer struct {
	mode        HAMode
	localClient client.Client
	remoteCache cache.Cache
	remoteGet   remoteGetFunc

	resources []MirroredResource
	byGVK     map[schema.GroupVersionKind]MirroredResource

	workers int
	queue   workqueue.TypedRateLimitingInterface[syncKey]

	// enqueuedAt tracks first-enqueue time per key, so update/delete lag
	// reflects total time since the triggering change even after a
	// coalescing queue collapses repeated events and retries into one item.
	mu         sync.Mutex
	enqueuedAt map[syncKey]time.Time

	log *zap.SugaredLogger
}

// NewRemoteSyncer builds a RemoteSyncer. remoteCfg and scheme are only used
// (and required) in standby mode, mirroring how NewClusterLeaderElector
// accepts a possibly-nil remote client for non-standby modes.
func NewRemoteSyncer(localClient client.Client, remoteCfg *rest.Config, scheme *runtime.Scheme, mode HAMode, opts RemoteSyncerOptions) (*RemoteSyncer, error) {
	if len(opts.Resources) == 0 {
		opts.Resources = CRDMirrorSet
	}
	if opts.Workers == 0 {
		opts.Workers = DefaultSyncWorkers
	}
	if opts.Log == nil {
		opts.Log = util.NewLogger().With("name", "ha-remote-syncer")
	}

	byGVK := make(map[schema.GroupVersionKind]MirroredResource, len(opts.Resources))
	for _, res := range opts.Resources {
		byGVK[res.GVK] = res
	}

	s := &RemoteSyncer{
		mode:        mode,
		localClient: localClient,
		resources:   opts.Resources,
		byGVK:       byGVK,
		workers:     opts.Workers,
		queue:       workqueue.NewTypedRateLimitingQueue[syncKey](workqueue.DefaultTypedControllerRateLimiter[syncKey]()),
		enqueuedAt:  make(map[syncKey]time.Time),
		log:         opts.Log,
	}

	if mode == ModeStandby {
		if remoteCfg == nil {
			return nil, fmt.Errorf("standby mode requires a remote config for the active hub")
		}
		remoteCache, err := cache.New(remoteCfg, cache.Options{Scheme: scheme})
		if err != nil {
			return nil, fmt.Errorf("building remote cache: %w", err)
		}
		s.remoteCache = remoteCache
		s.remoteGet = s.getFromRemoteCache
	}

	return s, nil
}

// Start runs RemoteSyncer until ctx is cancelled. It is a no-op in any mode
// other than standby. It returns nil (not ctx.Err()) on graceful shutdown,
// the same contract StartLeaseRenewal/WatchRemoteLease use.
func (s *RemoteSyncer) Start(ctx context.Context) error {
	if s.mode != ModeStandby {
		s.log.Infow("remote syncer not started; not in standby mode", "mode", s.mode)
		return nil
	}

	for _, res := range s.resources {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(res.GVK)
		inf, err := s.remoteCache.GetInformer(ctx, u)
		if err != nil {
			return fmt.Errorf("remote syncer: getting informer for %s: %w", res.GVK, err)
		}
		if _, err := inf.AddEventHandlerWithResyncPeriod(s.handlersFor(res.GVK), DefaultInformerResyncPeriod); err != nil {
			return fmt.Errorf("remote syncer: adding handler for %s: %w", res.GVK, err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runWorker(ctx)
		}()
	}

	s.log.Infow("remote syncer started", "resources", len(s.resources), "workers", s.workers)
	err := s.remoteCache.Start(ctx) // blocks until ctx.Done(); returns nil on graceful shutdown
	s.queue.ShutDown()
	wg.Wait()
	return err
}

// handlersFor returns the informer callbacks for one GVK. They only enqueue a
// syncKey — no mirror logic runs on the informer's own goroutine. A burst of
// Update events for the same object coalesces into one queued item; the
// worker determines the real action at dequeue time by re-reading the Active
// cache (found -> mirror, NotFound -> delete), the same way a Reconcile call
// would.
func (s *RemoteSyncer) handlersFor(objGVK schema.GroupVersionKind) toolscache.ResourceEventHandlerFuncs {
	enqueue := func(obj interface{}) {
		if tomb, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
			obj = tomb.Obj
		}
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			s.log.Warnw("remote syncer: unexpected informer object type", "type", fmt.Sprintf("%T", obj))
			return
		}
		key := syncKey{GVK: objGVK, Namespace: u.GetNamespace(), Name: u.GetName()}
		s.markEnqueued(key)
		s.queue.Add(key)
	}
	return toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { enqueue(obj) },
		UpdateFunc: func(_, newObj interface{}) { enqueue(newObj) },
		DeleteFunc: func(obj interface{}) { enqueue(obj) },
	}
}

func (s *RemoteSyncer) runWorker(ctx context.Context) {
	for {
		key, shutdown := s.queue.Get()
		if shutdown {
			return
		}
		s.processOnce(ctx, key)
	}
}

// processOnce dequeues exactly one key and mirrors it. Any error — including
// a namespaced object created before its Namespace has synced, the concrete
// failure mode that motivated this design — is retried with backoff via
// queue.AddRateLimited rather than dropped, satisfying issue #295's own
// acceptance criterion that the syncer retries without crashing.
func (s *RemoteSyncer) processOnce(ctx context.Context, key syncKey) {
	defer s.queue.Done(key)

	op, lagSeconds, err := s.reconcileKey(ctx, key)
	if err != nil {
		label := string(op)
		if label == "" {
			label = "sync"
		}
		haSyncErrorsTotal.WithLabelValues(key.GVK.Kind, label).Inc()
		s.log.Warnw("mirror sync failed; will retry", "kind", key.GVK.Kind,
			"namespace", key.Namespace, "name", key.Name,
			"attempt", s.queue.NumRequeues(key), "error", err)
		s.queue.AddRateLimited(key)
		return
	}
	if op != "" {
		haSyncLagSeconds.WithLabelValues(key.GVK.Kind, string(op)).Observe(lagSeconds)
	}
	s.queue.Forget(key)
	s.clearEnqueued(key)
}

// reconcileKey reads the current state of key from the Active hub and
// mirrors it: NotFound -> delete, found -> create-or-update. It returns the
// operation performed ("" if the conflict guard or a Skip predicate
// suppressed it) and the lag to record for that operation.
func (s *RemoteSyncer) reconcileKey(ctx context.Context, key syncKey) (mirrorOp, float64, error) {
	res, ok := s.byGVK[key.GVK]
	if !ok {
		return "", 0, nil
	}

	src, err := s.remoteGet(ctx, key)
	switch {
	case apierrors.IsNotFound(err):
		if err := mirrorDelete(ctx, s.localClient, key); err != nil {
			return opDelete, 0, err
		}
		return opDelete, time.Since(s.enqueuedTime(key)).Seconds(), nil
	case err != nil:
		return "", 0, fmt.Errorf("reading from active cache: %w", err)
	}

	if res.Skip != nil && res.Skip(src) {
		return "", 0, nil
	}

	op, err := mirrorCreateOrUpdate(ctx, s.localClient, key, res, src)
	if err != nil {
		return op, 0, err
	}
	if op == "" {
		return "", 0, nil // conflict guard skipped it
	}
	if op == opCreate {
		return op, time.Since(src.GetCreationTimestamp().Time).Seconds(), nil
	}
	return op, time.Since(s.enqueuedTime(key)).Seconds(), nil
}

// getFromRemoteCache is remoteGetFunc's real implementation: a read from
// controller-runtime's cache, which serves Get from the informer's local
// indexer rather than the network, so it stays cheap even when a retry
// re-reads the same key minutes later under backoff.
func (s *RemoteSyncer) getFromRemoteCache(ctx context.Context, key syncKey) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(key.GVK)
	if err := s.remoteCache.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *RemoteSyncer) markEnqueued(key syncKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.enqueuedAt[key]; !exists {
		s.enqueuedAt[key] = time.Now()
	}
}

func (s *RemoteSyncer) clearEnqueued(key syncKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.enqueuedAt, key)
}

func (s *RemoteSyncer) enqueuedTime(key syncKey) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.enqueuedAt[key]; ok {
		return t
	}
	return time.Now()
}
