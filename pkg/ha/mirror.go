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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// syncKey identifies one mirrored object by GVK and namespaced name. It is
// RemoteSyncer's workqueue item type (see remote_syncer.go); mirror.go only
// needs it to know what to read and where to write.
type syncKey struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

// mirrorOp reports which write mirrorCreateOrUpdate actually performed (or
// "" if the conflict guard skipped it), so callers can label metrics without
// re-deriving it.
type mirrorOp string

const (
	opCreate mirrorOp = "create"
	opUpdate mirrorOp = "update"
)

// mirrorCreateOrUpdate writes src onto the Standby via localClient: create if
// absent, update if present and syncer-owned. src is the informer's own
// cached object and is never mutated in place.
func mirrorCreateOrUpdate(ctx context.Context, localClient client.Client, key syncKey, res MirroredResource, src *unstructured.Unstructured) (mirrorOp, error) {
	payload := src.DeepCopy()
	payload.SetGroupVersionKind(key.GVK)
	payload.SetResourceVersion("")
	payload.SetUID("")
	payload.SetManagedFields(nil)
	payload.SetFinalizers(nil)
	// While an Active-side object is Terminating (deletionTimestamp set,
	// contents still being garbage-collected), the informer delivers it as
	// an ordinary Update, not a Delete yet — copying that onto a Standby
	// copy that isn't itself terminating fails immutable-field validation
	// on a real API server (confirmed live: "field is immutable" on both
	// deletionTimestamp and deletionGracePeriodSeconds). Its status is
	// dropped from payload too, explicitly rather than relying on a real
	// API server silently ignoring .status on the main resource endpoint:
	// for at least one type (confirmed live: Namespace), a stray
	// status.Phase="Terminating" is itself invalid once deletionTimestamp
	// is empty. The Standby converges correctly once Active reports
	// NotFound and mirrorDelete takes over; there's nothing useful to
	// reflect about the in-between Terminating state.
	if src.GetDeletionTimestamp() != nil {
		delete(payload.Object, "status")
	}
	payload.SetDeletionTimestamp(nil)
	payload.SetDeletionGracePeriodSeconds(nil)
	if res.StripOwnerRefs {
		payload.SetOwnerReferences(nil)
	}
	if res.Sanitize != nil {
		res.Sanitize(payload)
	}

	labels := payload.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[LabelSyncedFromActive] = LabelValueActive
	payload.SetLabels(labels)

	annotations := payload.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationSourceRV] = src.GetResourceVersion()
	payload.SetAnnotations(annotations)

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(key.GVK)
	err := localClient.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, existing)

	var op mirrorOp
	switch {
	case apierrors.IsNotFound(err):
		if err := localClient.Create(ctx, payload); err != nil {
			return "", fmt.Errorf("creating mirror of %s %s/%s: %w", key.GVK.Kind, key.Namespace, key.Name, err)
		}
		op = opCreate
	case err != nil:
		return "", fmt.Errorf("reading existing target %s %s/%s: %w", key.GVK.Kind, key.Namespace, key.Name, err)
	default:
		if existing.GetLabels()[LabelSyncedFromActive] != LabelValueActive {
			// Conflict guard: never overwrite an object the Standby didn't
			// create itself.
			return "", nil
		}
		if res.CreateOnly != nil && res.CreateOnly(src) {
			// Seeded from the Active once, owned locally afterwards. The
			// Standby's copy is deliberately allowed to diverge: for an
			// SA-token shell it is the local token controller's write that
			// makes the object useful, and re-applying the Active's version
			// would undo it. AnnotationSourceRV therefore stays at whatever
			// the source resourceVersion was at creation, which is harmless —
			// nothing reads it back, it exists for operators debugging drift.
			return "", nil
		}
		payload.SetResourceVersion(existing.GetResourceVersion())
		payload.SetUID(existing.GetUID())
		if err := localClient.Update(ctx, payload); err != nil {
			return "", fmt.Errorf("updating mirror of %s %s/%s: %w", key.GVK.Kind, key.Namespace, key.Name, err)
		}
		op = opUpdate
	}

	// Update() silently drops .status once a status subresource is
	// registered — true for every entry in CRDMirrorSet. Mirror it
	// explicitly, matching this repo's own UpdateStatus/CleanupUpdateStatus
	// convention (util/reconciliation_utility.go, util/cleanup_utility.go).
	//
	// Skipped while src is itself Terminating: its status is about to become
	// meaningless anyway, and for at least one type (confirmed live:
	// Namespace) copying a Terminating status onto a payload whose
	// deletionTimestamp was just stripped above fails a real API server
	// validation rule — status.Phase can only be "Terminating" if
	// deletionTimestamp is set. The Standby converges correctly once Active
	// reports NotFound and mirrorDelete takes over.
	if src.GetDeletionTimestamp() == nil {
		if status, ok, _ := unstructured.NestedFieldNoCopy(src.Object, "status"); ok {
			payload.Object["status"] = status
			if err := localClient.Status().Update(ctx, payload); err != nil {
				return op, fmt.Errorf("mirroring status of %s %s/%s: %w", key.GVK.Kind, key.Namespace, key.Name, err)
			}
		}
	}
	return op, nil
}

// mirrorDelete removes the Standby's mirror of key, if the engine created it.
// Deleting an already-absent object is treated as success (idempotent — a
// concurrent prune pass may already have removed it).
func mirrorDelete(ctx context.Context, localClient client.Client, key syncKey) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(key.GVK)
	err := localClient.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, existing)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading target %s %s/%s before delete: %w", key.GVK.Kind, key.Namespace, key.Name, err)
	}
	if existing.GetLabels()[LabelSyncedFromActive] != LabelValueActive {
		return nil
	}
	if err := localClient.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting mirror of %s %s/%s: %w", key.GVK.Kind, key.Namespace, key.Name, err)
	}
	return nil
}
