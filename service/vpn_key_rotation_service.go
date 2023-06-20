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
	"fmt"
	"reflect"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

type VpnKeyRotationService struct {
	wsgs IWorkerSliceGatewayService
	wscs IWorkerSliceConfigService
}

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
	// get slice config
	s, err := v.getSliceConfig(ctx, req.Name, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Step 1: Build map of clusterName: gateways
	clusterGatewayMapping, err := v.constructClusterGatewayMapping(ctx, s)
	if err != nil {
		return ctrl.Result{}, err
	}
	copyVpnConfig := vpnKeyRotationConfig.DeepCopy()

	if !reflect.DeepEqual(copyVpnConfig.Spec.ClusterGatewayMapping, clusterGatewayMapping) {
		copyVpnConfig.Spec.ClusterGatewayMapping = clusterGatewayMapping
		if err := util.UpdateResource(ctx, copyVpnConfig); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	// Step 2: TODO Update Certificate Creation TimeStamp and Expiry Timestamp if
	// a. The Creation TS and Expiry TS is empty
	// b. The Current TS is pass the expiry TS
	//TODO(): confirm if this is the right approach?
	if !v.verifyAllJobsAreCompleted(ctx, vpnKeyRotationConfig.Spec.SliceName) {
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	copyVpnConfig, err = v.reconcileVpnKeyRotationConfig(ctx, copyVpnConfig, s)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: (time.Duration(copyVpnConfig.Spec.RotationInterval) * 24 * time.Hour) - (time.Hour)}, nil
}

func (v *VpnKeyRotationService) reconcileVpnKeyRotationConfig(ctx context.Context, copyVpnConfig *controllerv1alpha1.VpnKeyRotation, s *controllerv1alpha1.SliceConfig) (*controllerv1alpha1.VpnKeyRotation, error) {
	now := metav1.Now()

	// Check if it's the first time creation
	if copyVpnConfig.Spec.CertificateCreationTime.IsZero() && copyVpnConfig.Spec.CertificateExpiryTime.IsZero() {
		copyVpnConfig.Spec.CertificateCreationTime = &now
		expiryTS := metav1.NewTime(now.AddDate(0, 0, copyVpnConfig.Spec.RotationInterval).Add(-1 * time.Hour))
		copyVpnConfig.Spec.CertificateExpiryTime = &expiryTS
		if err := util.UpdateResource(ctx, copyVpnConfig); err != nil {
			return nil, err
		}
	} else {
		if now.After(copyVpnConfig.Spec.CertificateExpiryTime.Time) {
			if err := v.triggerJobsForCertCreation(ctx, copyVpnConfig, s); err != nil {
				return nil, err
			}
			copyVpnConfig.Spec.CertificateCreationTime = &now
			expiryTS := metav1.NewTime(now.AddDate(0, 0, copyVpnConfig.Spec.RotationInterval).Add(-1 * time.Hour))
			copyVpnConfig.Spec.CertificateExpiryTime = &expiryTS
			if err := util.UpdateResource(ctx, copyVpnConfig); err != nil {
				return nil, err
			}
		}
	}

	return copyVpnConfig, nil
}

func (v *VpnKeyRotationService) constructClusterGatewayMapping(ctx context.Context, s *controllerv1alpha1.SliceConfig) (map[string][]string, error) {
	var clusterGatewayMapping = make(map[string][]string, 0)
	for _, cluster := range s.Spec.Clusters {
		// list workerslicegateways
		o := map[string]string{
			"worker-cluster":      cluster,
			"original-slice-name": s.Name,
		}
		workerSliceGatewaysList, err := v.listWorkerSliceGateways(ctx, o)
		if err != nil {
			return nil, err
		}
		vl := v.fetchGatewayNames(workerSliceGatewaysList)
		clusterGatewayMapping[cluster] = vl
	}
	return clusterGatewayMapping, nil
}

func (v *VpnKeyRotationService) triggerJobsForCertCreation(ctx context.Context, vpnKeyRotationConfig *controllerv1alpha1.VpnKeyRotation, s *controllerv1alpha1.SliceConfig) error {
	o := map[string]string{
		"original-slice-name": vpnKeyRotationConfig.Spec.SliceName,
	}
	workerSliceGatewaysList, err := v.listWorkerSliceGateways(ctx, o)
	if err != nil {
		return err
	}
	// fire certificate creation jobs for each gateway pair
	for _, gateway := range workerSliceGatewaysList.Items {
		if gateway.Spec.GatewayHostType == "Server" {
			cl, err := v.listClientPairGateway(workerSliceGatewaysList, gateway.Spec.RemoteGatewayConfig.GatewayName)
			if err != nil {
				return err
			}
			// construct clustermap
			clusterCidr := util.FindCIDRByMaxClusters(s.Spec.MaxClusters)
			completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(s), s.GetName())
			ownershipLabel := util.GetOwnerLabel(completeResourceName)
			workerSliceConfigs, err := v.wscs.ListWorkerSliceConfigs(ctx, ownershipLabel, s.Namespace)
			if err != nil {
				return err
			}
			clusterMap := v.wscs.ComputeClusterMap(s.Spec.Clusters, workerSliceConfigs)
			// contruct gw address
			gatewayAddresses := v.wsgs.BuildNetworkAddresses(s.Spec.SliceSubnet, gateway.Spec.LocalGatewayConfig.ClusterName, gateway.Spec.RemoteGatewayConfig.ClusterName, clusterMap, clusterCidr)
			// call GenerateCerts()
			if err := v.wsgs.GenerateCerts(ctx, s.Name, s.Namespace, &gateway, cl, gatewayAddresses); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *VpnKeyRotationService) listWorkerSliceGateways(ctx context.Context, labels map[string]string) (*workerv1alpha1.WorkerSliceGatewayList, error) {
	workerSliceGatewaysList := workerv1alpha1.WorkerSliceGatewayList{}
	// list workerslicegateways
	listOpts := []client.ListOption{
		client.MatchingLabels(
			labels,
		),
	}
	if err := util.ListResources(ctx, &workerSliceGatewaysList, listOpts...); err != nil {
		return nil, err
	}
	return &workerSliceGatewaysList, nil
}

// getSliceConfig
func (v *VpnKeyRotationService) getSliceConfig(ctx context.Context, name, namespace string) (*controllerv1alpha1.SliceConfig, error) {
	s := controllerv1alpha1.SliceConfig{}
	found, err := util.GetResourceIfExist(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, &s)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("sliceconfig %s not found", name)
	}
	return &s, nil
}

func (v *VpnKeyRotationService) listClientPairGateway(wl *workerv1alpha1.WorkerSliceGatewayList, clientGatewayName string) (*workerv1alpha1.WorkerSliceGateway, error) {
	for _, gateway := range wl.Items {
		if gateway.Name == clientGatewayName {
			return &gateway, nil
		}
	}
	return nil, fmt.Errorf("cannot find gateway %s", clientGatewayName)
}

// verifyAllJobsAreCompleted checks if all the jobs are in complete state
func (v *VpnKeyRotationService) verifyAllJobsAreCompleted(ctx context.Context, sliceName string) bool {
	jobs := batchv1.JobList{}
	o := map[string]string{
		"SLICE_NAME": sliceName,
	}
	listOpts := []client.ListOption{
		client.MatchingLabels(
			o,
		),
	}
	if err := util.ListResources(ctx, &jobs, listOpts...); err != nil {
		return false
	}
	for _, job := range jobs.Items {
		for _, condition := range job.Status.Conditions {
			if !(condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue) {
				return false
			}
		}
	}
	return true
}

// fetchGatewayNames fetches gateway names from the list of workerv1alpha1.WorkerSliceGatewayList
func (v *VpnKeyRotationService) fetchGatewayNames(gl *workerv1alpha1.WorkerSliceGatewayList) []string {
	var gatewayNames []string
	for _, g := range gl.Items {
		gatewayNames = append(gatewayNames, g.Name)
	}
	return gatewayNames
}
