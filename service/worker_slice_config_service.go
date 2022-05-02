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
	"time"

	"go.uber.org/zap"

	"github.com/jinzhu/copier"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)


type IWorkerSliceConfigService interface {
	ReconcileWorkerSliceConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	DeleteWorkerSliceConfigByLabel(ctx context.Context, label map[string]string, namespace string) error
	ListWorkerSliceConfigs(ctx context.Context, ownerLabel map[string]string, namespace string) ([]workerv1alpha1.WorkerSliceConfig, error)
	ComputeClusterMap(clusterNames []string, meshSlices []workerv1alpha1.WorkerSliceConfig) map[string]int
	CreateWorkerSliceConfig(ctx context.Context, clusters []string, namespace string, label map[string]string, name, sliceSubnet string) (map[string]int, error)
}

// WorkerSliceConfigService implements the IWorkerSliceConfigService interface
type WorkerSliceConfigService struct {
}

// ReconcileWorkerSliceConfig is a function to reconcile the config of worker slice
func (s *WorkerSliceConfigService) ReconcileWorkerSliceConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get WorkerSliceConfig resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Starting Recoincilation of WorkerSliceConfig with name %s in namespace %s",
		req.Name, req.Namespace)
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfig{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, workerSliceConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("Worker slice config %v not found, returning from reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Step 2: add Finalizers to the resource
	if workerSliceConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(workerSliceConfig.GetFinalizers(), WorkerSliceConfigFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, workerSliceConfig, WorkerSliceConfigFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for WorkerSliceConfig", req.NamespacedName)
		result := RemoveWorkerFinalizers(ctx, workerSliceConfig, WorkerSliceConfigFinalizer)
		if result.Requeue {
			return result, nil
		}
		slice := &controllerv1alpha1.SliceConfig{}
		found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
			Name:      workerSliceConfig.Spec.SliceName,
			Namespace: req.Namespace,
		}, slice)
		if err != nil {
			return result, err
		}
		if found && slice.ObjectMeta.DeletionTimestamp.IsZero() {
			clusters := slice.Spec.Clusters
			if util.IsInSlice(clusters, workerSliceConfig.Labels["worker-cluster"]) {
				logger.Debug("workerSliceConfig deleted forcefully from slice", req.NamespacedName)
				if slice.Annotations == nil {
					slice.Annotations = make(map[string]string)
				}
				slice.Annotations["updatedTimestamp"] = time.Now().String()
				logger.Debug("Recreating workerSliceConfig", req.NamespacedName)
				err = util.UpdateResource(ctx, slice)
				if err != nil {
					return result, err
				}
			}
		}
		return result, nil
	}

	// Step 3: get the slice config
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
		Name:      workerSliceConfig.Spec.SliceName,
		Namespace: req.Namespace,
	}, sliceConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("sliceConfig %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	ipamClusterOctet := workerSliceConfig.Spec.IpamClusterOctet
	slice := s.copySpecFromSliceConfigToWorkerSlice(ctx, *sliceConfig)
	workerSliceConfig.Spec = slice.Spec

	// Reconcile External Gateway Configuration
	externalGatewayConfig := workerv1alpha1.ExternalGatewayConfig{}
	var externalGatewayControllersConfig controllerv1alpha1.ExternalGatewayConfig
outer:
	for _, config := range sliceConfig.Spec.ExternalGatewayConfig {
		for _, cluster := range config.Clusters {
			if cluster == "*" {
				externalGatewayControllersConfig = config
			} else if cluster == workerSliceConfig.Labels["worker-cluster"] {
				externalGatewayControllersConfig = config
				break outer
			}
		}
	}
	err = copier.CopyWithOption(&externalGatewayConfig, &externalGatewayControllersConfig, copier.Option{
		DeepCopy: true,
	})
	if err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to deep copy external gateway configuration")
	}

	// Reconcile the Namespace Isolation Profile
	controllerIsolationProfile := sliceConfig.Spec.NamespaceIsolationProfile
	workerIsolationProfile := workerv1alpha1.NamespaceIsolationProfile{
		IsolationEnabled:      controllerIsolationProfile.IsolationEnabled,
		ApplicationNamespaces: make([]string, 0),
		AllowedNamespaces:     make([]string, 0),
	}

	for _, namespace := range controllerIsolationProfile.ApplicationNamespaces {
		for _, cluster := range namespace.Clusters {
			if cluster == "*" || cluster == workerSliceConfig.Labels["worker-cluster"] {
				workerIsolationProfile.ApplicationNamespaces = append(workerIsolationProfile.ApplicationNamespaces, namespace.Namespace)
			}
		}
	}

	for _, namespace := range controllerIsolationProfile.AllowedNamespaces {
		for _, cluster := range namespace.Clusters {
			if cluster == "*" || cluster == workerSliceConfig.Labels["worker-cluster"] {
				workerIsolationProfile.AllowedNamespaces = append(workerIsolationProfile.AllowedNamespaces, namespace.Namespace)
			}
		}
	}

	workerSliceConfig.Spec.ExternalGatewayConfig = externalGatewayConfig
	workerSliceConfig.Spec.NamespaceIsolationProfile = workerIsolationProfile
	workerSliceConfig.Spec.SliceName = sliceConfig.Name
	workerSliceConfig.Spec.IpamClusterOctet = ipamClusterOctet
	err = util.UpdateResource(ctx, workerSliceConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, err
}

// CreateWorkerSliceConfig is a function to create the worker slice config
func (s *WorkerSliceConfigService) CreateWorkerSliceConfig(ctx context.Context, clusters []string, namespace string, label map[string]string, name, sliceSubnet string) (map[string]int, error) {
	logger := util.CtxLogger(ctx)
	err := s.cleanUpSlices(ctx, label, namespace, clusters)
	if err != nil {
		return nil, err
	}
	meshSlices, err := s.ListWorkerSliceConfigs(ctx, label, namespace)
	if err != nil {
		return nil, err
	}
	clusterMap := s.ComputeClusterMap(clusters, meshSlices)
	for _, cluster := range clusters {
		logger.Debugf("Cluster Object %s", cluster)
		workerSliceConfigName := fmt.Sprintf("%s-%s", name, cluster)
		existingSlice := &workerv1alpha1.WorkerSliceConfig{}
		found, err := util.GetResourceIfExist(ctx, client.ObjectKey{
			Name:      workerSliceConfigName,
			Namespace: namespace,
		}, existingSlice)

		if err != nil {
			return clusterMap, err
		}
		if !found {
			label["project-namespace"] = namespace
			label["original-slice-name"] = name
			label["worker-cluster"] = cluster
			label["kubeslice-manager"] = "controller"

			expectedSlice := workerv1alpha1.WorkerSliceConfig{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      workerSliceConfigName,
					Labels:    label,
					Namespace: namespace,
				},
			}
			expectedSlice.Spec.SliceName = name
			expectedSlice.Spec.IpamClusterOctet = clusterMap[cluster]
			expectedSlice.Spec.SliceSubnet = sliceSubnet
			err = util.CreateResource(ctx, &expectedSlice)
			if err != nil {
				if !k8sErrors.IsAlreadyExists(err) {// ignores resource already exists error(for handling parallel calls to create same resource)
					logger.Debug("failed to create mesh slice %s since it already exists, namespace - %s ",
						expectedSlice.Name, namespace)
					return clusterMap, err
				}
			}
		} else {
			existingSlice.UID = ""

			existingSlice.Spec.IpamClusterOctet = clusterMap[cluster]
			logger.Debug("updating slice with new ipam cluster octet", existingSlice)
			if existingSlice.Annotations == nil {
				existingSlice.Annotations = make(map[string]string)
			}
			existingSlice.Annotations["updatedTimestamp"] = time.Now().String()
			err = util.UpdateResource(ctx, existingSlice)
			if err != nil {
				if !k8sErrors.IsAlreadyExists(err) {// ignores resource already exists error(for handling parallel calls to create same resource)
					logger.Debug("failed to create mesh slice %s since it already exists, namespace - %s ",
						workerSliceConfigName, namespace)
					return clusterMap, err
				}
			}
		}
	}
	return clusterMap, nil
}

// DeleteWorkerSliceConfigByLabel is a function to delete configs of workerslice by label
func (s *WorkerSliceConfigService) DeleteWorkerSliceConfigByLabel(ctx context.Context, label map[string]string, namespace string) error {
	slices, err := s.ListWorkerSliceConfigs(ctx, label, namespace)
	if err != nil {
		return err
	}
	for _, slice := range slices {
		err = util.DeleteResource(ctx, &slice)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListWorkerSliceConfigs
func (s *WorkerSliceConfigService) ListWorkerSliceConfigs(ctx context.Context, ownerLabel map[string]string,
	namespace string) ([]workerv1alpha1.WorkerSliceConfig, error) {
	slices := &workerv1alpha1.WorkerSliceConfigList{}
	err := util.ListResources(ctx, slices, client.MatchingLabels(ownerLabel), client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return slices.Items, nil
}

// ComputeClusterMap - function returns map of the cluster and the index
func (s *WorkerSliceConfigService) ComputeClusterMap(clusterNames []string, meshSlices []workerv1alpha1.WorkerSliceConfig) map[string]int {
	clusterMapping := make(map[string]int, len(clusterNames))
	usedIndexes := make(map[int]bool, 0)
	for _, meshSlice := range meshSlices {
		clusterMapping[meshSlice.Labels["worker-cluster"]] = meshSlice.Spec.IpamClusterOctet
		usedIndexes[meshSlice.Spec.IpamClusterOctet] = true
	}
	unUsedIndexes := make([]int, 0)
	for index := 1; index <= len(clusterNames); index++ {
		if !usedIndexes[index] {
			unUsedIndexes = append(unUsedIndexes, index)
		}
	}

	currentIndex := 0
	for _, clusterName := range clusterNames {
		if clusterMapping[clusterName] == 0 {
			clusterMapping[clusterName] = unUsedIndexes[currentIndex]
			currentIndex++
		}
	}
	return clusterMapping
	//gatewaynumber = no_clusters*serverClusterNumber+ clientClusterNumber
}

// cleanUpSlices is a function
func (s *WorkerSliceConfigService) cleanUpSlices(ctx context.Context, label map[string]string, namespace string, clusters []string) error {
	slices, err := s.ListWorkerSliceConfigs(ctx, label, namespace)
	if err != nil {
		return err
	}
	clusterSet := map[string]bool{}
	for _, cluster := range clusters {
		clusterSet[cluster] = true
	}
	for _, slice := range slices {
		clusterName := slice.Labels["worker-cluster"]
		if !clusterSet[clusterName] {
			err = util.DeleteResource(ctx, &slice)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// copySpecFromSliceConfigToWorkerSlice is a function to copy configuration from slice to worker slice
func (s *WorkerSliceConfigService) copySpecFromSliceConfigToWorkerSlice(ctx context.Context, sliceConfig controllerv1alpha1.SliceConfig) workerv1alpha1.WorkerSliceConfig {
	slice := workerv1alpha1.WorkerSliceConfig{}
	err := copier.Copy(&slice.Spec, &sliceConfig.Spec)
	if err != nil {
		return workerv1alpha1.WorkerSliceConfig{}
	}
	return slice
}
