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
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/kubeslice/kubeslice-controller/util"
)

// Secret .data values are base64 in the serialised form these tests build, so
// the fixtures are encoded from readable plaintext instead of being written as
// literals. A bare base64 blob in a file about credentials makes a reviewer
// stop and decode it to satisfy themselves it is not a real token, which is a
// poor thing to put in front of someone reading security-adjacent code.
func b64(plain string) string { return base64.StdEncoding.EncodeToString([]byte(plain)) }

var (
	activeSignedToken  = b64("active-signed-token")
	standbyMintedToken = b64("standby-minted-token")
	gatewayCertData    = b64("cert-data")
	oldGatewayCert     = b64("old-cert")
	newGatewayCert     = b64("new-cert")
	shortToken         = b64("token")
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
		"credential set is Secret/ServiceAccount/Role/RoleBinding only — access_control_service never creates ClusterRole/ClusterRoleBinding, despite ADR #293 Decision 6's broader wording")
}

func TestIsServiceAccountTokenSecret(t *testing.T) {
	saToken := newTestUnstructured(secretGVK, "proj", "sa-token")
	require.NoError(t, unstructured.SetNestedField(saToken.Object, string(corev1.SecretTypeServiceAccountToken), "type"))
	assert.True(t, isServiceAccountTokenSecret(saToken))

	opaque := newTestUnstructured(secretGVK, "proj", "gateway-cert")
	require.NoError(t, unstructured.SetNestedField(opaque.Object, string(corev1.SecretTypeOpaque), "type"))
	assert.False(t, isServiceAccountTokenSecret(opaque))

	untyped := newTestUnstructured(secretGVK, "proj", "no-type-field")
	assert.False(t, isServiceAccountTokenSecret(untyped))
}

// TestSanitizeSecret_ReducesTokenSecretToItsShell pins the payload-side half of
// the SA-token contract: the account name survives (it is what tells the
// Standby's token controller which account to mint for), the token bytes and
// the account UID do not.
func TestSanitizeSecret_ReducesTokenSecretToItsShell(t *testing.T) {
	saToken := newTestUnstructured(secretGVK, "proj", "kubeslice-rbac-worker-w1")
	require.NoError(t, unstructured.SetNestedField(saToken.Object, string(corev1.SecretTypeServiceAccountToken), "type"))
	require.NoError(t, unstructured.SetNestedField(saToken.Object, activeSignedToken, "data", corev1.ServiceAccountTokenKey))
	saToken.SetAnnotations(map[string]string{
		corev1.ServiceAccountNameKey: "kubeslice-rbac-worker-w1",
		corev1.ServiceAccountUIDKey:  "11111111-2222-3333-4444-555555555555",
	})

	sanitizeSecret(saToken)

	_, found, err := unstructured.NestedFieldNoCopy(saToken.Object, "data")
	require.NoError(t, err)
	assert.False(t, found,
		"an Active-signed token is invalid on the Standby; shipping it would mask the absence of a real credential")
	assert.NotContains(t, saToken.GetAnnotations(), corev1.ServiceAccountUIDKey,
		"a copied UID annotation never matches the mirrored ServiceAccount's fresh UID, and the Standby's token controller deletes the Secret on mismatch")
	assert.Equal(t, "kubeslice-rbac-worker-w1", saToken.GetAnnotations()[corev1.ServiceAccountNameKey],
		"the account name is the only link between the shell and the mirrored ServiceAccount — it must survive")

	// Other Secret types are none of this function's business.
	opaque := newTestUnstructured(secretGVK, "proj", "gateway-cert")
	require.NoError(t, unstructured.SetNestedField(opaque.Object, string(corev1.SecretTypeOpaque), "type"))
	require.NoError(t, unstructured.SetNestedField(opaque.Object, gatewayCertData, "data", "ovpn.crt"))
	sanitizeSecret(opaque)
	data, found, err := unstructured.NestedString(opaque.Object, "data", "ovpn.crt")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, gatewayCertData, data)
}

// TestSanitizeCachedSecret_StripsTokenBytesOnTheWayIntoTheCache covers the
// defence-in-depth layer: with the SA-token field selector gone, Active-minted
// tokens would otherwise be held in this process's memory. Both the typed and
// the unstructured path matter — the syncer reads through the latter.
func TestSanitizeCachedSecret_StripsTokenBytesOnTheWayIntoTheCache(t *testing.T) {
	typed := &corev1.Secret{
		Type: corev1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{corev1.ServiceAccountTokenKey: []byte("active-signed-token")},
	}
	out, err := sanitizeCachedSecret(typed)
	require.NoError(t, err)
	assert.Nil(t, out.(*corev1.Secret).Data)
	assert.NotNil(t, typed.Data, "the informer's own object must never be mutated in place")

	// The UID annotation deliberately survives into the cache: prune diffs
	// against this view, and sanitizeSecret drops it from the payload instead.
	typedWithAnnotations := &corev1.Secret{Type: corev1.SecretTypeServiceAccountToken}
	typedWithAnnotations.SetAnnotations(map[string]string{corev1.ServiceAccountUIDKey: "abc"})
	typedWithAnnotations.Data = map[string][]byte{corev1.ServiceAccountTokenKey: []byte("t")}
	out, err = sanitizeCachedSecret(typedWithAnnotations)
	require.NoError(t, err)
	assert.Equal(t, "abc", out.(*corev1.Secret).GetAnnotations()[corev1.ServiceAccountUIDKey])

	unstructuredToken := newTestUnstructured(secretGVK, "proj", "sa-token")
	require.NoError(t, unstructured.SetNestedField(unstructuredToken.Object, string(corev1.SecretTypeServiceAccountToken), "type"))
	require.NoError(t, unstructured.SetNestedField(unstructuredToken.Object, shortToken, "data", corev1.ServiceAccountTokenKey))
	out, err = sanitizeCachedSecret(unstructuredToken)
	require.NoError(t, err)
	_, found, err := unstructured.NestedFieldNoCopy(out.(*unstructured.Unstructured).Object, "data")
	require.NoError(t, err)
	assert.False(t, found)

	// Certificate Secrets and non-Secrets pass through untouched.
	opaque := &corev1.Secret{Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"ovpn.crt": []byte("cert")}}
	out, err = sanitizeCachedSecret(opaque)
	require.NoError(t, err)
	assert.Equal(t, []byte("cert"), out.(*corev1.Secret).Data["ovpn.crt"])

	sa := newTestUnstructured(saGVK, "proj", "kubeslice-rbac-worker-w1")
	out, err = sanitizeCachedSecret(sa)
	require.NoError(t, err)
	assert.Same(t, sa, out)
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
	require.NoError(t, unstructured.SetNestedField(src.Object, gatewayCertData, "data", "ovpn.crt"))

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
	assert.Equal(t, gatewayCertData, data, "the mirrored Secret must carry the source's data through unchanged")
}

// newActiveTokenSecret builds an SA-token Secret as it looks on the Active
// once that cluster's token controller has populated it.
func newActiveTokenSecret(t *testing.T, key syncKey) *unstructured.Unstructured {
	t.Helper()
	src := newTestUnstructured(secretGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, string(corev1.SecretTypeServiceAccountToken), "type"))
	require.NoError(t, unstructured.SetNestedField(src.Object, activeSignedToken, "data", corev1.ServiceAccountTokenKey))
	src.SetAnnotations(map[string]string{
		corev1.ServiceAccountNameKey: key.Name,
		corev1.ServiceAccountUIDKey:  "11111111-2222-3333-4444-555555555555",
	})
	return src
}

// TestReconcileKey_MirrorsServiceAccountTokenSecretAsShell covers the hub side
// of the worker's dual-hub credential: the Standby has to hold a worker
// credential valid on *itself* before any failover, and it cannot mint one from
// a fenced reconciler. Carrying the empty shell across lets its own token
// controller do it. What must not cross is the Active's token.
func TestReconcileKey_MirrorsServiceAccountTokenSecretAsShell(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: secretGVK, Namespace: "kubeslice-avesha", Name: "kubeslice-rbac-worker-w1"}

	remote := newStubRemote()
	remote.objects[key] = newActiveTokenSecret(t, key)
	registerMirroredNamespace(remote, key.Namespace)
	s := buildCredentialSyncer(t, remote)

	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, opCreate, op)

	got := getUnstructured(t, s.localClient, key)
	assert.Equal(t, LabelValueActive, got.GetLabels()[LabelSyncedFromActive])
	assert.Equal(t, string(corev1.SecretTypeServiceAccountToken), got.Object["type"])
	_, found, err := unstructured.NestedFieldNoCopy(got.Object, "data")
	require.NoError(t, err)
	assert.False(t, found, "an Active-signed token must never land on the Standby")
	assert.NotContains(t, got.GetAnnotations(), corev1.ServiceAccountUIDKey)
	assert.Equal(t, key.Name, got.GetAnnotations()[corev1.ServiceAccountNameKey],
		"without this annotation the Standby's token controller has nothing to mint against")
}

// TestReconcileKey_NeverOverwritesAMintedTokenSecret is the regression test for
// the failure mode that makes the shell approach non-trivial. The engine's
// update path is an unconditional full write and the remote informer resyncs
// every DefaultInformerResyncPeriod, so without CreateOnly every resync would
// clear the token the Standby's token controller minted, that controller would
// mint a fresh one, and any copy already handed to a worker would stop
// authenticating — on a timer, silently.
func TestReconcileKey_NeverOverwritesAMintedTokenSecret(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: secretGVK, Namespace: "kubeslice-avesha", Name: "kubeslice-rbac-worker-w1"}

	remote := newStubRemote()
	remote.objects[key] = newActiveTokenSecret(t, key)
	registerMirroredNamespace(remote, key.Namespace)
	s := buildCredentialSyncer(t, remote)

	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	require.Equal(t, opCreate, op)

	// Stand in for the Standby's token controller populating the shell.
	minted := getUnstructured(t, s.localClient, key)
	require.NoError(t, unstructured.SetNestedField(minted.Object, standbyMintedToken, "data", corev1.ServiceAccountTokenKey))
	require.NoError(t, s.localClient.Update(ctx, minted))

	// A resync delivers the Active's copy again.
	op, _, err = s.reconcileKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, mirrorOp(""), op, "a populated token Secret must not be rewritten")

	got := getUnstructured(t, s.localClient, key)
	token, found, err := unstructured.NestedString(got.Object, "data", corev1.ServiceAccountTokenKey)
	require.NoError(t, err)
	require.True(t, found, "the locally minted token must survive a resync")
	assert.Equal(t, standbyMintedToken, token)
}

// TestReconcileKey_StillUpdatesNonTokenSecretsOnResync pins CreateOnly as a
// per-object predicate rather than a property of the whole Secret row: gateway
// certificates must keep converging on the Active's content.
func TestReconcileKey_StillUpdatesNonTokenSecretsOnResync(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: secretGVK, Namespace: "kubeslice-avesha", Name: "gateway-cert"}

	src := newTestUnstructured(secretGVK, key.Namespace, key.Name)
	require.NoError(t, unstructured.SetNestedField(src.Object, string(corev1.SecretTypeOpaque), "type"))
	require.NoError(t, unstructured.SetNestedField(src.Object, oldGatewayCert, "data", "ovpn.crt"))

	remote := newStubRemote()
	remote.objects[key] = src
	registerMirroredNamespace(remote, key.Namespace)
	s := buildCredentialSyncer(t, remote)

	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	require.Equal(t, opCreate, op)

	rotated := src.DeepCopy()
	require.NoError(t, unstructured.SetNestedField(rotated.Object, newGatewayCert, "data", "ovpn.crt"))
	remote.objects[key] = rotated

	op, _, err = s.reconcileKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, opUpdate, op)

	got := getUnstructured(t, s.localClient, key)
	cert, found, err := unstructured.NestedString(got.Object, "data", "ovpn.crt")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, newGatewayCert, cert, "certificate rotation on the Active must still reach the Standby")
}

// TestPruneOnce_LeavesAMintedTokenSecretAlone closes the loop with the prune
// backstop, the other path that can write to a mirrored object. A shell that
// exists on both sides is neither an orphan nor missing, so neither diff
// direction touches it; and if some other round does re-enqueue it, the
// create-only guard still refuses to overwrite the minted token. Either way the
// worker's credential survives.
func TestPruneOnce_LeavesAMintedTokenSecretAlone(t *testing.T) {
	ctx := context.Background()
	key := syncKey{GVK: secretGVK, Namespace: "kubeslice-avesha", Name: "kubeslice-rbac-worker-w1"}

	remote := newStubRemote()
	remote.objects[key] = newActiveTokenSecret(t, key)
	registerMirroredNamespace(remote, key.Namespace)
	s := buildCredentialSyncer(t, remote)
	op, _, err := s.reconcileKey(ctx, key)
	require.NoError(t, err)
	require.Equal(t, opCreate, op)

	minted := getUnstructured(t, s.localClient, key)
	require.NoError(t, unstructured.SetNestedField(minted.Object, standbyMintedToken, "data", corev1.ServiceAccountTokenKey))
	require.NoError(t, s.localClient.Update(ctx, minted))

	s.remoteList = stubRemoteList([]syncKey{key}, nil)
	s.pruneOnce(ctx)
	assert.Equal(t, 0, s.queue.Len(), "a shell present on both sides is neither orphaned nor missing")

	got := getUnstructured(t, s.localClient, key)
	token, found, err := unstructured.NestedString(got.Object, "data", corev1.ServiceAccountTokenKey)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, standbyMintedToken, token)
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

	// Secret can be scoped neither way: cert Secrets come from the external
	// cert-generator job unlabeled, and the SA-token shells the Standby needs
	// rule out the "type" field selector that used to exclude them. It is
	// cached cluster-wide instead, with the project-namespace boundary held
	// client-side by RequireMirroredNamespace and the token bytes dropped on
	// the way in.
	var secretCfg cache.ByObject
	ok := false
	for obj, cfg := range byObject {
		if _, isSecret := obj.(*corev1.Secret); isSecret {
			secretCfg, ok = cfg, true
		}
	}
	require.True(t, ok, "Secret informer must have a ByObject entry")
	assert.Nil(t, secretCfg.Field,
		"a type-based field selector cannot admit both SA-token shells and unlabeled certificate Secrets")
	assert.Nil(t, secretCfg.Label)
	require.NotNil(t, secretCfg.Transform,
		"caching every Secret cluster-wide is only acceptable because Active-minted tokens are stripped on ingress")
}
