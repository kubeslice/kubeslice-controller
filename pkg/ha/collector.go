/*
 * 	Copyright (c) 2026 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
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

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// kubeSliceCollector lists the core KubeSlice control-plane CRDs that define
// the desired multi-cluster state. These are the resources a promoted Standby
// must already be aware of to continue reconciliation without regression.
type kubeSliceCollector struct {
	client client.Client
}

// NewKubeSliceCollector returns a ResourceCollector backed by a
// controller-runtime client that enumerates KubeSlice control-plane CRDs.
func NewKubeSliceCollector(c client.Client) ResourceCollector {
	return &kubeSliceCollector{client: c}
}

// Collect lists SliceConfig, Cluster, VpnKeyRotation, and ServiceExportConfig
// resources across all namespaces and returns their lightweight refs.
func (k *kubeSliceCollector) Collect(ctx context.Context) ([]ResourceRef, error) {
	var refs []ResourceRef

	slices := &controllerv1alpha1.SliceConfigList{}
	if err := k.client.List(ctx, slices); err != nil {
		return nil, fmt.Errorf("listing SliceConfigs: %w", err)
	}
	for i := range slices.Items {
		refs = append(refs, refOf("SliceConfig", &slices.Items[i].ObjectMeta))
	}

	clusters := &controllerv1alpha1.ClusterList{}
	if err := k.client.List(ctx, clusters); err != nil {
		return nil, fmt.Errorf("listing Clusters: %w", err)
	}
	for i := range clusters.Items {
		refs = append(refs, refOf("Cluster", &clusters.Items[i].ObjectMeta))
	}

	rotations := &controllerv1alpha1.VpnKeyRotationList{}
	if err := k.client.List(ctx, rotations); err != nil {
		return nil, fmt.Errorf("listing VpnKeyRotations: %w", err)
	}
	for i := range rotations.Items {
		refs = append(refs, refOf("VpnKeyRotation", &rotations.Items[i].ObjectMeta))
	}

	exports := &controllerv1alpha1.ServiceExportConfigList{}
	if err := k.client.List(ctx, exports); err != nil {
		return nil, fmt.Errorf("listing ServiceExportConfigs: %w", err)
	}
	for i := range exports.Items {
		refs = append(refs, refOf("ServiceExportConfig", &exports.Items[i].ObjectMeta))
	}

	return refs, nil
}

func refOf(kind string, m *metav1.ObjectMeta) ResourceRef {
	return ResourceRef{
		Kind:            kind,
		Namespace:       m.Namespace,
		Name:            m.Name,
		ResourceVersion: m.ResourceVersion,
		UID:             string(m.UID),
	}
}
