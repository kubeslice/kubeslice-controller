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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultPruneInterval is how often the prune backstop diffs the Standby's
// mirrored objects against the Active hub. Overridable through
// RemoteSyncerOptions.PruneInterval (--ha-sync-interval in main.go).
const DefaultPruneInterval = 60 * time.Second

// opPrune labels ha_sync_errors_total increments coming from the prune
// loop's own list calls; failures of the deletes it triggers are labeled by
// the worker that performs them (opDelete), like any other queued item.
const opPrune mirrorOp = "prune"

// remoteListFunc lists, for one GVK, the keys of every object currently
// present on the Active hub. The real implementation (listFromRemoteCache)
// reads controller-runtime's cache — a local-indexer read, not a network
// call. Overridable in tests, the same seam pattern as remoteGetFunc.
type remoteListFunc func(ctx context.Context, gvk schema.GroupVersionKind) (map[syncKey]struct{}, error)

// runPrune periodically reconciles drift between the Standby's mirrors and
// the Active hub that the informers never reported. Forward direction: a
// mirrored object whose Active-side original was deleted while this process
// wasn't watching (e.g. between two Standby runs) never gets a Delete event
// — cold-start informers only deliver what currently exists — so its mirror
// would otherwise survive as an orphan forever. Reverse direction: an
// Active-side object with no Standby mirror is re-enqueued (see pruneOnce
// for the cases that produces). The workqueue still owns
// retry-on-transient-failure; this loop only feeds it.
//
// It blocks until the remote cache has synced before the first pass: an
// unsynced cache lists empty, and an empty "Active" view would read as
// "everything was deleted" and prune every mirror on the Standby.
func (s *RemoteSyncer) runPrune(ctx context.Context) {
	if !s.waitForCacheSync(ctx) {
		return // ctx cancelled before the remote cache ever synced
	}
	ticker := time.NewTicker(s.pruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pruneOnce(ctx)
		}
	}
}

// pruneOnce diffs one round: for each mirrored type, every Standby object
// carrying the sync label but no longer present on the Active hub is
// enqueued onto the ordinary mirror workqueue rather than deleted here
// directly — the worker re-reads Active at dequeue time (so an object that
// reappeared in the meantime is mirrored, not deleted), and the conflict
// guard and rate-limited retry apply unchanged, with no second write path
// racing the workers.
//
// A failed list skips that type for this round instead of pruning on partial
// information — deleting mirrors is the one operation where acting on an
// incomplete view is worse than doing nothing until the next tick.
func (s *RemoteSyncer) pruneOnce(ctx context.Context) {
	for _, res := range s.resources {
		activeKeys, err := s.remoteList(ctx, res.GVK)
		if err != nil {
			haSyncErrorsTotal.WithLabelValues(res.GVK.Kind, string(opPrune)).Inc()
			s.log.Warnw("prune: listing active hub failed; skipping kind this round",
				"kind", res.GVK.Kind, "error", err)
			continue
		}

		local := &unstructured.UnstructuredList{}
		local.SetGroupVersionKind(res.GVK.GroupVersion().WithKind(res.GVK.Kind + "List"))
		if err := s.localClient.List(ctx, local,
			client.MatchingLabels{LabelSyncedFromActive: LabelValueActive}); err != nil {
			haSyncErrorsTotal.WithLabelValues(res.GVK.Kind, string(opPrune)).Inc()
			s.log.Warnw("prune: listing local mirrors failed; skipping kind this round",
				"kind", res.GVK.Kind, "error", err)
			continue
		}

		localKeys := make(map[syncKey]struct{}, len(local.Items))
		for i := range local.Items {
			item := &local.Items[i]
			key := syncKey{GVK: res.GVK, Namespace: item.GetNamespace(), Name: item.GetName()}
			localKeys[key] = struct{}{}
			if _, onActive := activeKeys[key]; onActive {
				continue
			}
			s.log.Infow("prune: enqueuing orphaned mirror",
				"kind", key.GVK.Kind, "namespace", key.Namespace, "name", key.Name)
			s.enqueue(key)
		}

		// Reverse diff: an Active-side object with no mirror on the Standby.
		// Usually a create this loop's forward pass can't see — a mirror
		// someone deleted directly on the Standby, an object whose skip
		// verdict was decided before its namespace had synced (see
		// namespaceIsMirrored), or a key stuck deep in retry backoff.
		// Re-enqueueing is always safe: the worker re-reads Active and runs
		// the full Skip/namespace/conflict-guard chain, so objects that
		// should not mirror simply no-op again.
		for key := range activeKeys {
			if _, mirrored := localKeys[key]; !mirrored {
				haPruneResurrectedTotal.WithLabelValues(key.GVK.Kind).Inc()
				s.enqueue(key)
			}
		}
	}
	// After the loop, not inside it: one pass covers every kind, and a per-kind
	// timestamp would report the last kind processed rather than the last complete
	// pass. Set even when some kinds were skipped on a failed list — the pass did
	// run, and the skips are already counted on ha_sync_errors_total.
	haPruneLastRunTimestamp.WithLabelValues(string(s.mode)).Set(float64(time.Now().Unix()))
}

// listFromRemoteCache is remoteListFunc's real implementation: a List against
// controller-runtime's cache, served from the informer's local indexer. For
// Namespace this inherits the cache's label scoping (see
// namespaceMirrorSelector), so a namespace that loses the controller label on
// Active disappears from this view exactly as it does from the informer's.
func (s *RemoteSyncer) listFromRemoteCache(ctx context.Context, gvk schema.GroupVersionKind) (map[syncKey]struct{}, error) {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := s.remoteCache.List(ctx, ul); err != nil {
		return nil, err
	}
	keys := make(map[syncKey]struct{}, len(ul.Items))
	for i := range ul.Items {
		item := &ul.Items[i]
		keys[syncKey{GVK: gvk, Namespace: item.GetNamespace(), Name: item.GetName()}] = struct{}{}
	}
	return keys, nil
}
