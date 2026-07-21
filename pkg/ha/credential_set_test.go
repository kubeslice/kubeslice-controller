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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/kubeslice/kubeslice-controller/util"
)

var (
	nsGVK     = schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	secretGVK = schema.GroupVersionKind{Version: "v1", Kind: "Secret"}
	saGVK     = schema.GroupVersionKind{Version: "v1", Kind: "ServiceAccount"}
	roleGVK   = schema.GroupVersionKind{Group: groupRBAC, Version: "v1", Kind: "Role"}
	rbGVK     = schema.GroupVersionKind{Group: groupRBAC, Version: "v1", Kind: "RoleBinding"}
)

// buildCredentialSyncer is buildSyncer's sibling for the credential set.
func buildCredentialSyncer(t *testing.T, remote *stubRemote) *RemoteSyncer {
	t.Helper()
	byGVK := make(map[schema.GroupVersionKind]MirroredResource, len(CredentialMirrorSet))
	for _, res := range CredentialMirrorSet {
		byGVK[res.GVK] = res
	}
	return &RemoteSyncer{
		mode:        ModeStandby,
		localClient: fakeClient(t),
		remoteGet:   remote.get,
		resources:   CredentialMirrorSet,
		byGVK:       byGVK,
		workers:     1,
		queue: workqueue.NewTypedRateLimitingQueue[syncKey](
			workqueue.NewTypedItemExponentialFailureRateLimiter[syncKey](time.Millisecond, time.Second),
		),
		handlerRegistered: map[schema.GroupVersionKind]bool{},
		enqueuedAt:        map[syncKey]time.Time{},
		log:               testLog(),
	}
}

// registerMirroredNamespace makes ns visible in the stub's Namespace view,
// standing in for a project namespace the label-scoped remote cache mirrors.
func registerMirroredNamespace(remote *stubRemote, ns string) {
	key := syncKey{GVK: nsGVK, Name: ns}
	remote.objects[key] = newTestUnstructured(nsGVK, "", ns)
}

func TestCredentialMirrorSet_ShapeAndDefenses(t *testing.T) {
	var gvks []schema.GroupVersionKind
	for _, res := range CredentialMirrorSet {
		gvks = append(gvks, res.GVK)
		assert.True(t, res.StripOwnerRefs,
			"%s: UID-based ownerReferences never survive a cross-cluster copy, and credential objects are written by actors outside this repo — every row must strip them", res.GVK.Kind)
		assert.True(t, res.RequireMirroredNamespace,
			"%s: core types exist cluster-wide; every row must be gated on the mirrored-namespace boundary", res.GVK.Kind)
	}
	assert.ElementsMatch(t, []schema.GroupVersionKind{secretGVK, saGVK, roleGVK, rbGVK}, gvks,
		"credential set is Secret/ServiceAccount/Role/RoleBinding only — access_control_service never creates ClusterRole/ClusterRoleBinding, despite the ADR's broader wording")
}

func TestSkipServiceAccountTokenSecret(t *testing.T) {
	saToken := newTestUnstructured(secretGVK, "proj", "sa-token")
	require.NoError(t, unstructured.SetNestedField(saToken.Object, string(corev1.SecretTypeServiceAccountToken), "type"))
	assert.True(t, skipServiceAccountTokenSecret(saToken),
		"SA tokens are signed by the issuing cluster's key and are invalid on the Standby")

	opaque := newTestUnstructured(secretGVK, "proj", "gateway-cert")
	require.NoError(t, unstructured.SetNestedField(opaque.Object, string(corev1.SecretTypeOpaque), "type"))
	assert.False(t, skipServiceAccountTokenSecret(opaque))

	untyped := newTestUnstructured(secretGVK, "proj", "no-type-field")
	assert.False(t, skipServiceAccountTokenSecret(untyped))
}

func TestFullMirrorSet_CombinesBothSetsWithoutCollisions(t *testing.T) {
	full := FullMirrorSet()
	assert.Len(t, full, len(CRDMirrorSet)+len(CredentialMirrorSet))

	seen := map[schema.GroupVersionKind]bool{}
	for _, res := range full {
		assert.False(t, seen[res.GVK], "duplicate mirror row for %s — byGVK keying would silently drop one", res.GVK)
		seen[res.GVK] = true
	}
	assert.True(t, seen[nsGVK])
	assert.True(t, seen[secretGVK])
}

func TestReconcileKey_MirrorsOpaqueSecretWithData(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: secretGVK, Namespace: "kubeslice-avesha", Name: "gateway-cert"}

	src := newTestUnstructured(secretGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, string(corev1.SecretTypeOpaque), "type"))
	require.NoError(t, unstructured.SetNestedField(src.Object, "Y2VydC1kYXRh", "data", "ovpn.crt"))

	remote := newStubRemote()
	remote.objects[key] = src
	registerMirroredNamespace(remote, key.Namespace)
	s := buildCredentialSyncer(t, remote)

	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, opCreate, op)

	got := getUnstructured(t, s.localClient, key)
	assert.Equal(t, LabelValueActive, got.GetLabels()[LabelSyncedFromActive])
	data, found, err := unstructured.NestedString(got.Object, "data", "ovpn.crt")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Y2VydC1kYXRh", data, "the mirrored Secret must carry the source's data through unchanged")
}

func TestReconcileKey_SkipsServiceAccountTokenSecret(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: secretGVK, Namespace: "kubeslice-avesha", Name: "kubeslice-rbac-worker-w1"}

	src := newTestUnstructured(secretGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, string(corev1.SecretTypeServiceAccountToken), "type"))

	remote := newStubRemote()
	remote.objects[key] = src
	registerMirroredNamespace(remote, key.Namespace)
	s := buildCredentialSyncer(t, remote)

	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, mirrorOp(""), op)

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(secretGVK)
	err = s.localClient.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, existing)
	assert.Error(t, err, "an SA-token Secret must never be written onto the Standby")
}

// TestReconcileKey_SkipsCredentialsInUnmirroredNamespaces pins the boundary
// that matters most: a namespace that is not label-mirrored is out of bounds
// no matter what it is named. The concrete case that motivated this (found
// live against a Helm-installed Active hub): under the chart's
// --project-namespace-prefix ("kubeslice-"), the controller's own
// kubeslice-controller namespace looks like a project namespace by name, and
// a name-based rule would have mirrored its webhook TLS key and image-pull
// Secrets onto the Standby.
func TestReconcileKey_SkipsCredentialsInUnmirroredNamespaces(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		gvk  schema.GroupVersionKind
		ns   string
		name string
	}{
		{secretGVK, "kubeslice-controller", "webhook-server-cert-secret"},
		{secretGVK, "kube-system", "bootstrap-token"},
		{saGVK, "kube-system", "hand-labeled-sa"},
		{roleGVK, "default", "some-role"},
		{rbGVK, "default", "some-rolebinding"},
	} {
		key := syncKey{GVK: tc.gvk, Namespace: tc.ns, Name: tc.name}
		src := newTestUnstructured(tc.gvk, tc.ns, tc.name)
		if tc.gvk == secretGVK {
			require.NoError(t, unstructured.SetNestedField(src.Object, string(corev1.SecretTypeOpaque), "type"))
		}

		remote := newStubRemote()
		remote.objects[key] = src // object visible, namespace deliberately NOT mirrored
		s := buildCredentialSyncer(t, remote)

		op, _, err := s.reconcileKey(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, mirrorOp(""), op, "%s %s/%s: unmirrored namespace must mean skip", tc.gvk.Kind, tc.ns, tc.name)

		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(tc.gvk)
		err = s.localClient.Get(ctx, types.NamespacedName{Namespace: tc.ns, Name: tc.name}, existing)
		assert.Error(t, err, "%s %s/%s must not exist on the Standby", tc.gvk.Kind, tc.ns, tc.name)
	}
}

func TestReconcileKey_NamespaceCheckErrorIsRetryable(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: secretGVK, Namespace: "kubeslice-avesha", Name: "gateway-cert"}

	src := newTestUnstructured(secretGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, string(corev1.SecretTypeOpaque), "type"))

	remote := newStubRemote()
	remote.objects[key] = src
	remote.errs[syncKey{GVK: nsGVK, Name: key.Namespace}] = fmt.Errorf("simulated transient cache failure")
	s := buildCredentialSyncer(t, remote)

	_, _, err := s.reconcileKey(ctx, key)
	assert.Error(t, err,
		"a transient failure reading the namespace must surface as an error (workqueue retry), not as a silent skip")
}

func TestMirrorCacheByObject_ScopesCredentialInformers(t *testing.T) {
	byObject := mirrorCacheByObject()

	// The label-scoped types must match exactly what the controller stamps
	// (via ReconcileProjectNamespace and util.GetOwnerLabel) and nothing else.
	labeled := labels.Set(util.LabelsKubeSliceController)
	for obj, cfg := range byObject {
		if _, isSecret := obj.(*corev1.Secret); isSecret {
			continue
		}
		require.NotNil(t, cfg.Label, "%T informer must be label-scoped", obj)
		assert.True(t, cfg.Label.Matches(labeled), "%T: selector must match controller-stamped labels", obj)
		assert.False(t, cfg.Label.Matches(labels.Set{}), "%T: selector must not match unlabeled objects", obj)
	}

	// Secret can't be label-scoped (cert Secrets come from the external
	// cert-generator job, unlabeled) — it must be field-scoped to exclude
	// SA-token Secrets at the watch itself.
	var secretCfg cache.ByObject
	ok := false
	for obj, cfg := range byObject {
		if _, isSecret := obj.(*corev1.Secret); isSecret {
			secretCfg, ok = cfg, true
		}
	}
	require.True(t, ok, "Secret informer must have a ByObject entry")
	require.NotNil(t, secretCfg.Field)
	assert.False(t, secretCfg.Field.Matches(fields.Set{"type": string(corev1.SecretTypeServiceAccountToken)}),
		"the Secret watch itself must exclude SA-token Secrets")
	assert.True(t, secretCfg.Field.Matches(fields.Set{"type": string(corev1.SecretTypeOpaque)}))
}
