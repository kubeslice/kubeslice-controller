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

package service

import (
	"context"
	"reflect"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IVpnKeyRotationService interface {
	CreateMinimalVpnKeyRotationConfig(ctx context.Context, sliceName, namespace string, r int) error
	ReconcileClusters(ctx context.Context, sliceName, namespace string, clusters []string) (*controllerv1alpha1.VpnKeyRotation, error)
	ReconcileVpnKeyRotation(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

type VpnKeyRotationService struct{}

// CreateMinimalVpnKeyRotationConfig creates minimal VPNKeyRotationCR if not found
func (v *VpnKeyRotationService) CreateMinimalVpnKeyRotationConfig(ctx context.Context, sliceName, namespace string, r int) error {
	vpnKeyRotationConfig := controllerv1alpha1.VpnKeyRotation{}
	found, err := util.GetResourceIfExist(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      sliceName,
	}, &vpnKeyRotationConfig)
	if err != nil {
		return err
	}
	if !found {
		vpnKeyRotationConfig = controllerv1alpha1.VpnKeyRotation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: namespace,
				Labels: map[string]string{
					"kubeslice-slice": sliceName,
				},
			},
			Spec: controllerv1alpha1.VpnKeyRotationSpec{
				RotationInterval: r,
				SliceName:        sliceName,
			},
		}
		if err := util.CreateResource(ctx, &vpnKeyRotationConfig); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileClusters checks whether any cluster is added/removed and updates it in vpnkeyrotation config
// the first arg is returned for testing purposes
func (v *VpnKeyRotationService) ReconcileClusters(ctx context.Context, sliceName, namespace string, clusters []string) (*controllerv1alpha1.VpnKeyRotation, error) {
	vpnKeyRotationConfig := controllerv1alpha1.VpnKeyRotation{}
	found, err := util.GetResourceIfExist(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      sliceName,
	}, &vpnKeyRotationConfig)
	if err != nil {
		return nil, err
	}
	if found {
		if !reflect.DeepEqual(vpnKeyRotationConfig.Spec.Clusters, clusters) {
			vpnKeyRotationConfig.Spec.Clusters = clusters
			return &vpnKeyRotationConfig, util.UpdateResource(ctx, &vpnKeyRotationConfig)
		}
	}
	return &vpnKeyRotationConfig, nil
}

func (v *VpnKeyRotationService) ReconcileVpnKeyRotation(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get VpnKeyRotation resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Starting Recoincilation of VpnKeyRotation with name %s in namespace %s",
		req.Name, req.Namespace)
	vpnKeyRotationConfig := &controllerv1alpha1.VpnKeyRotation{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, vpnKeyRotationConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("Vpn Key Rotation Config %v not found, returning from reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	// Step 1: Build map of clusterName: gateways
	var clusterGatewayMapping = make(map[string][]string, 0)
	workerSliceGatewaysList := workerv1alpha1.WorkerSliceGatewayList{}
	for _, cluster := range vpnKeyRotationConfig.Spec.Clusters {
		// list workerslicegateways
		o := map[string]string{
			"worker-cluster": cluster,
		}
		listOpts := []client.ListOption{
			client.MatchingLabels(
				o,
			),
		}
		if err := util.ListResources(ctx, &workerSliceGatewaysList, listOpts...); err != nil {
			return ctrl.Result{}, err
		}
		vl := v.fetchGatewayNames(workerSliceGatewaysList)
		clusterGatewayMapping[cluster] = vl
	}
	if !reflect.DeepEqual(vpnKeyRotationConfig.Spec.ClusterGatewayMapping, clusterGatewayMapping) {
		vpnKeyRotationConfig.Spec.ClusterGatewayMapping = clusterGatewayMapping
		if err := util.UpdateResource(ctx, vpnKeyRotationConfig); err != nil {
			return ctrl.Result{}, err
		}
	}
	// Step 2: TODO Update Certificate Creation TimeStamp and Expiry Timestamp
	return ctrl.Result{}, nil
}

// fetchGatewayNames fetches gateway names from the list of workerv1alpha1.WorkerSliceGatewayList
func (v *VpnKeyRotationService) fetchGatewayNames(gl workerv1alpha1.WorkerSliceGatewayList) []string {
	var gatewayNames []string
	for _, g := range gl.Items {
		gatewayNames = append(gatewayNames, g.Name)
	}
	return gatewayNames
}
