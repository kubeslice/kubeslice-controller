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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Label and annotation keys the mirror engine stamps onto every object it
// writes to the Standby. LabelSyncedFromActive is the conflict guard: the
// engine only ever overwrites or deletes a Standby object that carries it, so
// it never touches anything the Standby's own reconcilers (or an operator)
// created directly — including a pre-existing Namespace such as kube-system
// or default, now that Namespace is an ordinary mirrored type (see
// CRDMirrorSet below) rather than a special-cased cold-start step.
const (
	LabelSyncedFromActive = "ha.kubeslice.io/synced-from"
	LabelValueActive      = "active"
	AnnotationSourceRV    = "ha.kubeslice.io/source-rv"
)

// MirroredResource describes one GroupVersionKind RemoteSyncer mirrors from
// the Active hub to the Standby.
type MirroredResource struct {
	GVK schema.GroupVersionKind
	// StripOwnerRefs strips metadata.ownerReferences from the mirrored copy.
	// Only VpnKeyRotation needs this: its ownerReference points at the
	// Active-side SliceConfig's UID, which the Standby's mirrored SliceConfig
	// does not share (a fresh UID is assigned on create) — left in place, the
	// Standby's garbage collector would see a dangling reference and delete
	// the object shortly after the mirror creates it.
	StripOwnerRefs bool
	// Skip, if set, excludes an object from mirroring based on its content —
	// e.g. filtering out kubernetes.io/service-account-token Secrets in the
	// credential mirror set.
	Skip func(u *unstructured.Unstructured) bool
	// RequireMirroredNamespace restricts mirroring to objects whose namespace
	// the syncer itself mirrors — i.e. the namespace is present in the remote
	// cache's label-scoped Namespace view (see namespaceMirrorSelector). Set
	// on every credential row: core types exist in every namespace, and no
	// name-based rule can draw this boundary safely — under the Helm chart's
	// real-world --project-namespace-prefix ("kubeslice-"), the controller's
	// own kubeslice-controller namespace matches the project-namespace naming
	// pattern, and a prefix test would have mirrored its webhook TLS key,
	// image-pull credentials, and Helm release Secrets onto the Standby
	// (found live against a Helm-installed Active hub). The label boundary is
	// the one ReconcileProjectNamespace actually maintains.
	RequireMirroredNamespace bool
}

const (
	groupController = "controller.kubeslice.io"
	groupWorker     = "worker.kubeslice.io"
	groupRBAC       = "rbac.authorization.k8s.io"
)

func gvk(group, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: group, Version: "v1alpha1", Kind: kind}
}

// CRDMirrorSet is the set of hub-side resources mirrored Active -> Standby.
//
// This intentionally does not match issue #295's own CRD table, which names
// Slice/SliceGateway/ServiceExport — none of those types exist in this repo.
// They are worker-cluster data-plane CRDs (group networking.kubeslice.io)
// owned by the separate worker-operator repo, irrelevant to hub-to-hub
// mirroring. Verified against apis/controller/v1alpha1 and apis/worker/v1alpha1.
var CRDMirrorSet = []MirroredResource{
	{GVK: schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}},
	{GVK: gvk(groupController, "Project")},
	{GVK: gvk(groupController, "Cluster")},
	{GVK: gvk(groupController, "SliceConfig")},
	{GVK: gvk(groupController, "ServiceExportConfig")},
	{GVK: gvk(groupController, "SliceQoSConfig")},
	{GVK: gvk(groupController, "VpnKeyRotation"), StripOwnerRefs: true},
	{GVK: gvk(groupWorker, "WorkerSliceConfig")},
	{GVK: gvk(groupWorker, "WorkerSliceGateway")},
	{GVK: gvk(groupWorker, "WorkerServiceImport")},
}

// skipServiceAccountTokenSecret excludes kubernetes.io/service-account-token
// Secrets from mirroring. SA tokens are signed by the issuing cluster's own
// service-account key, so an Active-minted token is cryptographically invalid
// on the Standby; mirroring the ServiceAccount itself is what matters — the
// Standby's token controller mints its own, locally-valid token for it.
func skipServiceAccountTokenSecret(u *unstructured.Unstructured) bool {
	secretType, _, _ := unstructured.NestedString(u.Object, "type")
	return secretType == string(corev1.SecretTypeServiceAccountToken)
}

// FullMirrorSet is everything a production Standby mirrors: CRDMirrorSet
// plus CredentialMirrorSet. Returned as a fresh slice so callers cannot
// mutate the package-level tables through it.
func FullMirrorSet() []MirroredResource {
	full := make([]MirroredResource, 0, len(CRDMirrorSet)+len(CredentialMirrorSet))
	full = append(full, CRDMirrorSet...)
	return append(full, CredentialMirrorSet...)
}

// CredentialMirrorSet is the set of credential resources mirrored Active ->
// Standby so a promoted Standby can serve its worker clusters without manual
// re-provisioning: worker-identity RBAC (Role/RoleBinding/ServiceAccount, the
// only RBAC kinds access_control_service.go ever creates — no ClusterRole or
// ClusterRoleBinding, despite ADR Decision 6's broader wording) and Secrets
// such as the gateway certificates the ovpn job generates.
//
// Every row sets RequireMirroredNamespace — see that field's doc comment for
// why the boundary is the mirrored-namespace set and not a name pattern —
// and StripOwnerRefs: ownerReferences resolve by UID, which never survives
// the cross-cluster copy, and unlike the CRD set (audited — only
// VpnKeyRotation ever gets a reference, from this repo's own code)
// credential objects are also written by actors outside this repo (the
// token controller, the cert-generator job), so no such audit can hold here.
var CredentialMirrorSet = []MirroredResource{
	{
		GVK:                      schema.GroupVersionKind{Version: "v1", Kind: "Secret"},
		StripOwnerRefs:           true,
		Skip:                     skipServiceAccountTokenSecret,
		RequireMirroredNamespace: true,
	},
	{
		GVK:                      schema.GroupVersionKind{Version: "v1", Kind: "ServiceAccount"},
		StripOwnerRefs:           true,
		RequireMirroredNamespace: true,
	},
	{
		GVK:                      schema.GroupVersionKind{Group: groupRBAC, Version: "v1", Kind: "Role"},
		StripOwnerRefs:           true,
		RequireMirroredNamespace: true,
	},
	{
		GVK:                      schema.GroupVersionKind{Group: groupRBAC, Version: "v1", Kind: "RoleBinding"},
		StripOwnerRefs:           true,
		RequireMirroredNamespace: true,
	},
}
