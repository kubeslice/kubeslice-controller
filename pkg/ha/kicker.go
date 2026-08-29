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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/kubeslice/kubeslice-controller/util"
)

// The types that have a reconciler and therefore need waking after a
// promotion. This is CRDMirrorSet minus Namespace: the mirror copies namespaces
// so their contents have somewhere to land, but no reconciler owns them, so
// there is nothing to kick.
var (
	GVKProject             = gvk(groupController, "Project")
	GVKCluster             = gvk(groupController, "Cluster")
	GVKSliceConfig         = gvk(groupController, "SliceConfig")
	GVKServiceExportConfig = gvk(groupController, "ServiceExportConfig")
	GVKSliceQoSConfig      = gvk(groupController, "SliceQoSConfig")
	GVKVpnKeyRotation      = gvk(groupController, "VpnKeyRotation")
	GVKWorkerSliceConfig   = gvk(groupWorker, "WorkerSliceConfig")
	GVKWorkerSliceGateway  = gvk(groupWorker, "WorkerSliceGateway")
	GVKWorkerServiceImport = gvk(groupWorker, "WorkerServiceImport")
)

// ReconciledGVKs returns the nine types the controller reconciles, as a fresh
// slice so callers cannot mutate the package's own view.
func ReconciledGVKs() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		GVKProject, GVKCluster, GVKSliceConfig, GVKServiceExportConfig,
		GVKSliceQoSConfig, GVKVpnKeyRotation,
		GVKWorkerSliceConfig, GVKWorkerSliceGateway, GVKWorkerServiceImport,
	}
}

// DefaultKickChannelBuffer is the per-type channel depth. Sized so an ordinary
// hub's whole object set fits without the kick having to block on a consumer
// that has not started draining yet; see Kick for what happens when it does not.
const DefaultKickChannelBuffer = 256

// ReconcileKicker re-enqueues every object of every reconciled type after a
// promotion.
//
// It exists because flipping the write fence causes no reconcile at all. The
// fence returns without requeuing, so every request a Standby dropped is
// discarded rather than parked, and nothing fires again until an object changes
// or the informer resyncs — ten hours by default. A promoted hub would sit on
// state it believes it owns and never touch it. Most visibly, the mirror strips
// finalizers by design and only a running reconciler re-adds them, so until
// this runs a delete on the promoted hub skips its cleanup entirely and the
// object simply vanishes.
//
// Note for anyone reading the acceptance criteria: "a new Slice created on the
// Standby reconciles successfully" passes without any of this, because a new
// object generates its own event. It is the pre-existing mirrored state that
// stays frozen.
type ReconcileKicker struct {
	// channels is one channel per GVK, each wired into that type's controller
	// through source.Channel.
	//
	// One shared channel does not work, and the failure is quiet. Every
	// source.Channel starts its own goroutine reading the channel it was given
	// and fanning out to its own handler; nine sources over one Go channel means
	// nine goroutines competing for each value, so every event goes to exactly
	// one arbitrary controller and each type sees a random subset of its own
	// objects. In a small test with a couple of objects that looks like it
	// works.
	channels map[schema.GroupVersionKind]chan event.GenericEvent

	localClient client.Client
	log         *zap.SugaredLogger
}

// NewReconcileKicker builds a kicker with one channel per given GVK.
func NewReconcileKicker(localClient client.Client, gvks []schema.GroupVersionKind, log *zap.SugaredLogger) *ReconcileKicker {
	if log == nil {
		log = util.NewLogger().With("name", "ha-reconcile-kicker")
	}
	channels := make(map[schema.GroupVersionKind]chan event.GenericEvent, len(gvks))
	for _, gvk := range gvks {
		channels[gvk] = make(chan event.GenericEvent, DefaultKickChannelBuffer)
	}
	return &ReconcileKicker{channels: channels, localClient: localClient, log: log}
}

// Source returns the channel a controller should read, or nil if this kicker
// does not cover that type. A nil channel is safe to pass on: the caller simply
// registers no extra watch, which leaves that controller exactly as it is today.
func (k *ReconcileKicker) Source(gvk schema.GroupVersionKind) <-chan event.GenericEvent {
	if k == nil {
		return nil
	}
	ch, ok := k.channels[gvk]
	if !ok {
		return nil
	}
	return ch
}

// Kick lists every object of every registered type and pushes one event per
// object. It is one-shot and bounded by the size of the cluster's own state:
// there is no steady-state cost, and nothing here runs again until the next
// promotion.
//
// A type whose list fails is reported and the rest still run. Partial coverage
// beats none — the alternative is a promoted hub with nothing reconciled at all
// because one API call failed.
//
// Sends are non-blocking. The consumers are controller-runtime sources, which
// only start draining once the manager is running, and main.go starts the
// promotion path before mgr.Start; a blocking send in that window would hang
// promotion on a channel nobody is reading. A full channel is therefore counted
// and logged rather than waited on. Losing a kick costs a reconcile that would
// have happened anyway on the next change or resync, which is the same position
// the hub is in without this component at all.
func (k *ReconcileKicker) Kick(ctx context.Context) error {
	if k == nil || len(k.channels) == 0 {
		return nil
	}

	var (
		total   int
		dropped int
		failed  []string
	)
	for gvk, ch := range k.channels {
		// Checked explicitly rather than as a select case alongside the send
		// below. Both would be ready whenever the channel has room, and select
		// chooses randomly among ready cases, so cancellation would only be
		// honoured by chance — which is exactly how this was written first, and
		// what -shuffle caught.
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("kicking reconcilers: %w", err)
		}

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
		if err := k.localClient.List(ctx, list); err != nil {
			k.log.Errorw("kick: listing local objects failed; this type stays unreconciled until it changes",
				"kind", gvk.Kind, "error", err)
			failed = append(failed, gvk.Kind)
			continue
		}

		for i := range list.Items {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("kicking reconcilers: %w", err)
			}
			select {
			case ch <- event.GenericEvent{Object: &list.Items[i]}:
				total++
			default:
				dropped++
			}
		}
	}

	if dropped > 0 {
		k.log.Warnw("kick: some events were dropped because a type's channel was full",
			"dropped", dropped, "delivered", total, "buffer", DefaultKickChannelBuffer)
	}
	k.log.Infow("kick: re-enqueued objects after promotion",
		"objects", total, "types", len(k.channels), "dropped", dropped)

	if len(failed) > 0 {
		return fmt.Errorf("could not list these types to re-enqueue them: %v", failed)
	}
	return nil
}
