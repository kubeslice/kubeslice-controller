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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var testGVK = schema.GroupVersionKind{Group: groupController, Version: "v1alpha1", Kind: "SliceConfig"}

func newTestUnstructured(gvk schema.GroupVersionKind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

// mirrorFakeClient builds a fake client that emulates the status-subresource
// split real clusters apply to every CRDMirrorSet entry: Update() must not
// alter .status, only Status().Update() may.
func mirrorFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	statusGVKObj := &unstructured.Unstructured{}
	statusGVKObj.SetGroupVersionKind(testGVK)
	return fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithStatusSubresource(statusGVKObj).
		WithObjects(objs...).
		Build()
}

func getUnstructured(t *testing.T, c client.Client, key syncKey) *unstructured.Unstructured {
	t.Helper()
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(key.GVK)
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, got))
	return got
}

func TestMirrorCreateOrUpdate_CreatesWithLabelAndAnnotation(t *testing.T) {
	ctx := context.Background()
	c := mirrorFakeClient(t)
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	src := newTestUnstructured(testGVK, key.Namespace, key.Name)
	src.SetResourceVersion("999")

	op, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK}, src)
	require.NoError(t, err)
	assert.Equal(t, opCreate, op)

	got := getUnstructured(t, c, key)
	assert.Equal(t, LabelValueActive, got.GetLabels()[LabelSyncedFromActive])
	assert.Equal(t, "999", got.GetAnnotations()[AnnotationSourceRV])
}

func TestMirrorCreateOrUpdate_UpdatesExistingSyncedObject(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	existing := newTestUnstructured(testGVK, key.Namespace, key.Name)
	existing.SetLabels(map[string]string{LabelSyncedFromActive: LabelValueActive})
	existing.SetResourceVersion("1")
	existing.SetUID("standby-uid-1")
	c := mirrorFakeClient(t, existing)

	src := newTestUnstructured(testGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, "updated-value", "spec", "field"))
	src.SetResourceVersion("active-rv-2")

	op, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK}, src)
	require.NoError(t, err)
	assert.Equal(t, opUpdate, op)

	got := getUnstructured(t, c, key)
	val, _, _ := unstructured.NestedString(got.Object, "spec", "field")
	assert.Equal(t, "updated-value", val)
	assert.Equal(t, "active-rv-2", got.GetAnnotations()[AnnotationSourceRV],
		"source-rv annotation should reflect the Active object's resourceVersion")
}

func TestMirrorCreateOrUpdate_ConflictGuardSkipsUnlabeledExisting(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	existing := newTestUnstructured(testGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(existing.Object, "hand-applied", "spec", "field"))
	c := mirrorFakeClient(t, existing)

	src := newTestUnstructured(testGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, "from-active", "spec", "field"))

	op, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK}, src)
	require.NoError(t, err)
	assert.Equal(t, mirrorOp(""), op, "conflict guard should report no-op")

	got := getUnstructured(t, c, key)
	val, _, _ := unstructured.NestedString(got.Object, "spec", "field")
	assert.Equal(t, "hand-applied", val, "an unlabeled existing object must never be overwritten")
}

func TestMirrorCreateOrUpdate_StripOwnerRefsOnlyWhenConfigured(t *testing.T) {
	ctx := context.Background()
	ownerRef := metav1.OwnerReference{APIVersion: "v1alpha1", Kind: "SliceConfig", Name: "owner", UID: "owner-uid"}

	stripKey := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "vpn-1"}
	src := newTestUnstructured(testGVK, stripKey.Namespace, stripKey.Name)
	src.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
	c := mirrorFakeClient(t)

	_, err := mirrorCreateOrUpdate(ctx, c, stripKey, MirroredResource{GVK: testGVK, StripOwnerRefs: true}, src)
	require.NoError(t, err)
	got := getUnstructured(t, c, stripKey)
	assert.Empty(t, got.GetOwnerReferences(), "StripOwnerRefs=true must strip ownerReferences")

	keepKey := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "vpn-2"}
	src2 := newTestUnstructured(testGVK, keepKey.Namespace, keepKey.Name)
	src2.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	_, err = mirrorCreateOrUpdate(ctx, c, keepKey, MirroredResource{GVK: testGVK, StripOwnerRefs: false}, src2)
	require.NoError(t, err)
	got2 := getUnstructured(t, c, keepKey)
	assert.Len(t, got2.GetOwnerReferences(), 1, "StripOwnerRefs=false must leave ownerReferences untouched")
}

func TestMirrorCreateOrUpdate_StripsDeletionTimestampFromTerminatingSource(t *testing.T) {
	ctx := context.Background()
	gracePeriod := int64(0)
	terminating := func(u *unstructured.Unstructured) {
		now := metav1.Now()
		u.SetDeletionTimestamp(&now)
		u.SetDeletionGracePeriodSeconds(&gracePeriod)
	}

	t.Run("update path", func(t *testing.T) {
		key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
		existing := newTestUnstructured(testGVK, key.Namespace, key.Name)
		existing.SetLabels(map[string]string{LabelSyncedFromActive: LabelValueActive})
		c := mirrorFakeClient(t, existing)

		src := newTestUnstructured(testGVK, key.Namespace, key.Name)
		terminating(src)

		op, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK}, src)
		require.NoError(t, err, "an Active-side object mid-Terminating must not fail mirroring")
		assert.Equal(t, opUpdate, op)

		got := getUnstructured(t, c, key)
		assert.Nil(t, got.GetDeletionTimestamp(), "the Standby mirror must never carry a deletionTimestamp copied from a Terminating Active-side object")
		assert.Nil(t, got.GetDeletionGracePeriodSeconds())
	})

	t.Run("create path", func(t *testing.T) {
		key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-2"}
		c := mirrorFakeClient(t)

		src := newTestUnstructured(testGVK, key.Namespace, key.Name)
		terminating(src)

		op, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK}, src)
		require.NoError(t, err, "creating a mirror from an already-Terminating source must not fail")
		assert.Equal(t, opCreate, op)

		got := getUnstructured(t, c, key)
		assert.Nil(t, got.GetDeletionTimestamp())
	})
}

func TestMirrorCreateOrUpdate_MirrorsStatusExplicitly(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	src := newTestUnstructured(testGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, "Ready", "status", "phase"))
	c := mirrorFakeClient(t)

	_, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK}, src)
	require.NoError(t, err)

	got := getUnstructured(t, c, key)
	phase, ok, _ := unstructured.NestedString(got.Object, "status", "phase")
	require.True(t, ok, "status.phase must be present after mirroring — a plain Update() alone would have dropped it")
	assert.Equal(t, "Ready", phase)
}

func TestMirrorCreateOrUpdate_SkipsStatusMirrorWhenSourceIsTerminating(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	c := mirrorFakeClient(t)

	src := newTestUnstructured(testGVK, key.Namespace, key.Name)
	now := metav1.Now()
	src.SetDeletionTimestamp(&now)
	require.NoError(t, unstructured.SetNestedField(src.Object, "Terminating", "status", "phase"))

	op, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK}, src)
	require.NoError(t, err, "mirroring a Terminating source must not fail trying to write an invalid status combination")
	assert.Equal(t, opCreate, op)

	got := getUnstructured(t, c, key)
	_, ok, _ := unstructured.NestedString(got.Object, "status", "phase")
	assert.False(t, ok, "status must not be mirrored while the source is Terminating — see mirror.go for the real API-server validation rule this avoids")
}

func TestMirrorCreateOrUpdate_SkipsStatusMirrorWhenSkipStatusSet(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	c := mirrorFakeClient(t)

	// A live (non-Terminating) source that carries a status, which is exactly
	// the shape a Namespace always has. Without SkipStatus the engine would
	// attempt the status write, which is what failed Forbidden forever on a
	// cluster whose RBAC withheld namespaces/status.
	src := newTestUnstructured(testGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, "Active", "status", "phase"))

	op, err := mirrorCreateOrUpdate(ctx, c, key, MirroredResource{GVK: testGVK, SkipStatus: true}, src)
	require.NoError(t, err)
	assert.Equal(t, opCreate, op)

	got := getUnstructured(t, c, key)
	_, ok, _ := unstructured.NestedString(got.Object, "status", "phase")
	assert.False(t, ok, "SkipStatus must suppress the status write for types whose status the API server owns")
}

func TestCRDMirrorSet_NamespaceSkipsStatusAndNothingElseDoes(t *testing.T) {
	// The defect lived in the mirror set, not the engine: Namespace was listed
	// like any CRD, so its ever-present status.phase made the engine attempt a
	// namespaces/status write on every pass. Pin the row here — the engine test
	// above cannot catch a regression in this table.
	var skipped []string
	for _, res := range FullMirrorSet() {
		if res.SkipStatus {
			skipped = append(skipped, res.GVK.Kind)
		}
	}
	assert.Equal(t, []string{"Namespace"}, skipped,
		"Namespace must skip the status write, and it must be the only type that does; "+
			"any other type gaining SkipStatus needs the same justification written down")
}

func TestMirrorDelete_IdempotentOnNotFound(t *testing.T) {
	c := mirrorFakeClient(t)
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "missing"}
	assert.NoError(t, mirrorDelete(context.Background(), c, key))
}

func TestMirrorDelete_DeletesSyncedObject(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	existing := newTestUnstructured(testGVK, key.Namespace, key.Name)
	existing.SetLabels(map[string]string{LabelSyncedFromActive: LabelValueActive})
	c := mirrorFakeClient(t, existing)

	require.NoError(t, mirrorDelete(ctx, c, key))

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(key.GVK)
	err := c.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, got)
	assert.True(t, apierrors.IsNotFound(err), "object should be gone after delete")
}

func TestMirrorDelete_ConflictGuardSkipsUnlabeledExisting(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-1"}
	existing := newTestUnstructured(testGVK, key.Namespace, key.Name)
	c := mirrorFakeClient(t, existing)

	require.NoError(t, mirrorDelete(ctx, c, key))

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(key.GVK)
	err := c.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, got)
	assert.NoError(t, err, "an unlabeled existing object must never be deleted")
}
