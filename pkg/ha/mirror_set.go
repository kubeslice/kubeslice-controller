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
}

const (
	groupController = "controller.kubeslice.io"
	groupWorker     = "worker.kubeslice.io"
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
