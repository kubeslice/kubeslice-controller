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

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
)

// Default RemoteSyncer tunables, overridable through RemoteSyncerOptions.
const (
	DefaultSyncWorkers          = 4
	DefaultInformerResyncPeriod = 10 * time.Minute
	// DefaultInformerSetupRetryPeriod is how long Start waits between retries
	// when wiring up the remote informers fails (e.g. the Active hub is
	// briefly unreachable, or its RBAC from config/ha hasn't been applied
	// yet at the moment the Standby pod starts).
	DefaultInformerSetupRetryPeriod = 5 * time.Second
)

// namespaceMirrorSelector scopes the Namespace informer to only the
// namespaces this controller itself manages — every project namespace
// NamespaceService.ReconcileProjectNamespace creates carries this label (see
// util.LabelsKubeSliceController). Without it, the informer would watch
// every namespace on the Active hub cluster-wide (kube-system, kube-public,
// unrelated infra namespaces), and mirrorDelete (mirror.go) would delete any
// of those incidentally-mirrored namespaces — cascading to delete everything
// inside them on the Standby — the moment the Active hub deletes its copy
// for a completely unrelated reason.
func namespaceMirrorSelector() labels.Selector {
	return labels.SelectorFromSet(util.LabelsKubeSliceController)
}

// mirrorCacheByObject scopes the remote cache's informers server-side, so
// unrelated Active-hub objects never reach this process at all — the mirror
// set's Skip predicates enforce the same boundaries client-side, but for
// core types that exist cluster-wide (Secrets above all) not watching them
// in the first place is both the cheaper and the safer layer.
//
//   - Namespace, ServiceAccount, Role, RoleBinding: label-scoped to
//     util.LabelsKubeSliceController — stamped on every project namespace by
//     ReconcileProjectNamespace and on every credential object the
//     controller creates via util.GetOwnerLabel (which embeds the same
//     key/value pair).
//   - Secret: cannot be scoped either way. Not by label — gateway certificate
//     Secrets are created by the external cert-generator job, unlabeled. No
//     longer by field either: this cache previously excluded SA-token Secrets
//     with a "type" field selector, but the Standby now needs their shells in
//     order to hold a worker credential valid on itself (CredentialMirrorSet,
//     ADR Decision 6), and no single selector admits both those and the
//     unlabeled certificate Secrets. So Secrets are cached cluster-wide and
//     the project-namespace boundary stays client-side, in the row's
//     RequireMirroredNamespace gate — a Secret in kube-system is cached but
//     never written to the Standby.
//
//     What that widening does *not* do is widen access: the Standby's remote
//     identity already holds cluster-wide Secret read (config/ha, which says
//     so in as many words), because RBAC cannot scope Secrets by type or by
//     namespace label. The field selector narrowed what this process cached,
//     not what it was allowed to fetch. It does mean Active-side token bytes
//     would now pass through this process, so they are stripped on the way
//     into the cache by sanitizeCachedSecret below, before anything can read
//     them — the mirror's own Sanitize is the correctness layer, this is
//     defence in depth.
func mirrorCacheByObject() map[client.Object]cache.ByObject {
	controllerManaged := namespaceMirrorSelector()
	return map[client.Object]cache.ByObject{
		&corev1.Namespace{}:      {Label: controllerManaged},
		&corev1.ServiceAccount{}: {Label: controllerManaged},
		&rbacv1.Role{}:           {Label: controllerManaged},
		&rbacv1.RoleBinding{}:    {Label: controllerManaged},
		&corev1.Secret{}:         {Transform: sanitizeCachedSecret},
	}
}

// sanitizeCachedSecret drops the token bytes of every service-account-token
// Secret on their way into the remote cache, so an Active-minted token is
// never held in this process at all. Applied to both typed and unstructured
// informers (controller-runtime resolves cache.ByObject by GVK), which is what
// makes it a real boundary rather than a courtesy: the syncer reads through
// the unstructured path.
//
// Only .data is dropped here. The service-account.uid annotation has to
// survive into the cache — prune diffs against this view, and stripping
// identity from the cached copy would make the diff lie — so it is dropped in
// the payload instead, by sanitizeSecret. Anything that is not an SA-token
// Secret, and anything that is not a Secret at all, passes through untouched.
func sanitizeCachedSecret(obj any) (any, error) {
	switch secret := obj.(type) {
	case *corev1.Secret:
		if secret.Type != corev1.SecretTypeServiceAccountToken || secret.Data == nil {
			return obj, nil
		}
		stripped := secret.DeepCopy()
		stripped.Data = nil
		return stripped, nil
	case *unstructured.Unstructured:
		if !isServiceAccountTokenSecret(secret) {
			return obj, nil
		}
		if _, found, _ := unstructured.NestedFieldNoCopy(secret.Object, "data"); !found {
			return obj, nil
		}
		stripped := secret.DeepCopy()
		delete(stripped.Object, "data")
		return stripped, nil
	default:
		return obj, nil
	}
}

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
	// SetupRetryPeriod is how long Start waits between retries when wiring up
	// the remote informers fails.
	SetupRetryPeriod time.Duration
	// PruneInterval is how often the prune backstop diffs Standby mirrors
	// against the Active hub and removes orphans (see prune.go).
	PruneInterval time.Duration
	// EventRecorder, if set, gets an HAMirrorSyncFailed event on the first
	// failure of each mirror-failure episode, attached to the object that
	// failed to sync. Nil disables event emission (metrics and retries are
	// unaffected).
	EventRecorder events.EventRecorder
	Log           *zap.SugaredLogger
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

	workers          int
	setupRetryPeriod time.Duration
	pruneInterval    time.Duration
	// remoteList and waitForCacheSync back the prune loop (prune.go); like
	// remoteGet, they are fields so tests can drive pruning without a real
	// *rest.Config.
	remoteList       remoteListFunc
	waitForCacheSync func(ctx context.Context) bool
	queue            workqueue.TypedRateLimitingInterface[syncKey]
	// register performs one attempt at wiring informer event handlers for
	// every mirrored resource. Kept as a field (rather than calling
	// registerInformersOnce directly) so tests can exercise the retry loop
	// in Start without a real *rest.Config, the same reason remoteGet exists.
	register func(ctx context.Context) error
	// handlerRegistered tracks which GVKs already have their event handler
	// wired up, so a retried registerInformersOnce (after a later resource in
	// the list failed) skips the ones that already succeeded instead of
	// calling AddEventHandlerWithResyncPeriod on them again. Informer.
	// AddEventHandlerWithResyncPeriod adds an independent handler on every
	// call — it is not idempotent like GetInformer — so without this guard a
	// retry would double (or triple, ...) that resource's event and resync
	// load for the rest of the process's lifetime. Only ever touched from
	// Start's single call path, so it needs no locking.
	handlerRegistered map[schema.GroupVersionKind]bool

	// enqueuedAt tracks first-enqueue time per key, so update/delete lag
	// reflects total time since the triggering change even after a
	// coalescing queue collapses repeated events and retries into one item.
	mu         sync.Mutex
	enqueuedAt map[syncKey]time.Time

	eventRecorder events.EventRecorder
	log           *zap.SugaredLogger
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
	if opts.SetupRetryPeriod == 0 {
		opts.SetupRetryPeriod = DefaultInformerSetupRetryPeriod
	}
	if opts.PruneInterval == 0 {
		opts.PruneInterval = DefaultPruneInterval
	}
	if opts.Log == nil {
		opts.Log = util.NewLogger().With("name", "ha-remote-syncer")
	}

	byGVK := make(map[schema.GroupVersionKind]MirroredResource, len(opts.Resources))
	for _, res := range opts.Resources {
		byGVK[res.GVK] = res
	}

	s := &RemoteSyncer{
		mode:              mode,
		localClient:       localClient,
		resources:         opts.Resources,
		byGVK:             byGVK,
		workers:           opts.Workers,
		setupRetryPeriod:  opts.SetupRetryPeriod,
		pruneInterval:     opts.PruneInterval,
		queue:             workqueue.NewTypedRateLimitingQueue[syncKey](workqueue.DefaultTypedControllerRateLimiter[syncKey]()),
		handlerRegistered: make(map[schema.GroupVersionKind]bool, len(opts.Resources)),
		enqueuedAt:        make(map[syncKey]time.Time),
		eventRecorder:     opts.EventRecorder,
		log:               opts.Log,
	}
	s.register = s.registerInformersOnce

	if mode == ModeStandby {
		if remoteCfg == nil {
			return nil, fmt.Errorf("standby mode requires a remote config for the active hub")
		}
		remoteCache, err := cache.New(remoteCfg, cache.Options{
			Scheme:   scheme,
			ByObject: mirrorCacheByObject(),
		})
		if err != nil {
			return nil, fmt.Errorf("building remote cache: %w", err)
		}
		s.remoteCache = remoteCache
		s.remoteGet = s.getFromRemoteCache
		s.remoteList = s.listFromRemoteCache
		s.waitForCacheSync = remoteCache.WaitForCacheSync
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

	if !s.registerInformers(ctx) {
		return nil // ctx cancelled while retrying informer setup
	}

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runWorker(ctx)
		}()
	}
	// The prune loop only watches its context, unlike the workers (which exit
	// on queue shutdown) — cancel it explicitly so wg.Wait can't hang if the
	// cache ever stops with an error before ctx itself is cancelled.
	pruneCtx, cancelPrune := context.WithCancel(ctx)
	defer cancelPrune()
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.runPrune(pruneCtx)
	}()

	s.log.Infow("remote syncer started", "resources", len(s.resources), "workers", s.workers, "pruneInterval", s.pruneInterval)
	err := s.remoteCache.Start(ctx) // blocks until ctx.Done(); returns nil on graceful shutdown
	cancelPrune()
	s.queue.ShutDown()
	wg.Wait()
	return err
}

// registerInformers wires up an event handler for every mirrored resource,
// retrying with backoff via s.register on failure — e.g. the Active hub
// briefly unreachable at Standby startup, or its config/ha RBAC not yet
// applied — instead of giving up after one attempt. Without this, a single
// transient failure here would return an error from Start and permanently
// disable mirroring for the process's lifetime (main.go only logs that
// error; nothing restarts the goroutine), unlike StartLeaseRenewal and
// WatchRemoteLease, which retry every tick regardless of error. Returns
// false only if ctx is cancelled before setup succeeds.
func (s *RemoteSyncer) registerInformers(ctx context.Context) bool {
	for {
		err := s.register(ctx)
		if err == nil {
			return true
		}
		s.log.Warnw("remote syncer: informer setup failed; retrying",
			"error", err, "retryAfter", s.setupRetryPeriod)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(s.setupRetryPeriod):
		}
	}
}

// registerInformersOnce is register's real implementation: a single attempt
// at wiring an event handler for every mirrored resource onto the remote
// cache. Resources whose handler already got registered by an earlier,
// partially-failed attempt are skipped — see handlerRegistered's doc comment
// for why that matters.
func (s *RemoteSyncer) registerInformersOnce(ctx context.Context) error {
	for _, res := range s.resources {
		if s.handlerRegistered[res.GVK] {
			continue
		}
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(res.GVK)
		inf, err := s.remoteCache.GetInformer(ctx, u)
		if err != nil {
			return fmt.Errorf("remote syncer: getting informer for %s: %w", res.GVK, err)
		}
		if _, err := inf.AddEventHandlerWithResyncPeriod(s.handlersFor(res.GVK), DefaultInformerResyncPeriod); err != nil {
			return fmt.Errorf("remote syncer: adding handler for %s: %w", res.GVK, err)
		}
		s.handlerRegistered[res.GVK] = true
	}
	return nil
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
		s.enqueue(syncKey{GVK: objGVK, Namespace: u.GetNamespace(), Name: u.GetName()})
	}
	return toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { enqueue(obj) },
		UpdateFunc: func(_, newObj interface{}) { enqueue(newObj) },
		DeleteFunc: func(obj interface{}) { enqueue(obj) },
	}
}

// enqueue hands one key to the worker pool, stamping its first-enqueue time
// for the lag metric. Both the informer handlers and the prune loop go
// through here, so every mirror write shares one queue and one retry policy.
func (s *RemoteSyncer) enqueue(key syncKey) {
	s.markEnqueued(key)
	s.queue.Add(key)
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
		// One HAMirrorSyncFailed event per failure episode (NumRequeues is
		// still 0 here on the first failure; Forget resets it on success).
		// The retries that follow are milliseconds apart under early
		// backoff, and although the recorder aggregates repeats into one
		// Event's Count, every call is still an API-server write —
		// per-attempt emission would turn each outage into a write storm
		// while ha_sync_errors_total already counts every attempt.
		// The recorder is called directly rather than through
		// util.RecordEvent: that helper logs via util.CtxLogger, which
		// panics on any context that didn't pass through a reconciler's
		// PrepareKubeSliceControllersRequestContext — and Start's context
		// (main.go's signal-handler context) never does.
		if s.eventRecorder != nil && s.queue.NumRequeues(key) == 0 {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(key.GVK)
			obj.SetNamespace(key.Namespace)
			obj.SetName(key.Name)
			if recErr := s.eventRecorder.RecordEvent(ctx, &events.Event{
				Object:            obj,
				ReportingInstance: util.InstanceController,
				Name:              ossEvents.EventHAMirrorSyncFailed,
			}); recErr != nil {
				s.log.Warnw("failed to record mirror-sync-failed event",
					"kind", key.GVK.Kind, "namespace", key.Namespace,
					"name", key.Name, "error", recErr)
			}
		}
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
	if res.RequireMirroredNamespace {
		mirrored, err := s.namespaceIsMirrored(ctx, src.GetNamespace())
		if err != nil {
			return "", 0, fmt.Errorf("checking namespace of %s %s/%s: %w", key.GVK.Kind, key.Namespace, key.Name, err)
		}
		if !mirrored {
			return "", 0, nil
		}
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

// namespaceIsMirrored reports whether ns is one of the namespaces the syncer
// itself mirrors, by reading the remote cache's Namespace view — which is
// label-scoped to controller-managed project namespaces (see
// namespaceMirrorSelector), so any namespace outside that boundary reads as
// NotFound here no matter what it is named. This is the namespace gate
// behind MirroredResource.RequireMirroredNamespace.
//
// A skip verdict is terminal for this queue item (no retry), so an object
// racing its own namespace's informer delivery on cold start can be skipped
// once — the prune loop's reverse diff re-enqueues it within one
// --ha-sync-interval (see pruneOnce), rather than waiting for the informer's
// much longer resync period.
func (s *RemoteSyncer) namespaceIsMirrored(ctx context.Context, ns string) (bool, error) {
	if ns == "" {
		return true, nil // cluster-scoped objects have no namespace to gate on
	}
	nsKey := syncKey{GVK: schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, Name: ns}
	if _, err := s.remoteGet(ctx, nsKey); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
