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
	"github.com/kubeslice/kubeslice-controller/metrics"
	"time"

	"github.com/kubeslice/kubeslice-controller/events"

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
	ComputeClusterMap(clusterNames []string, workerSliceConfigs []workerv1alpha1.WorkerSliceConfig) map[string]int
	CreateMinimalWorkerSliceConfig(ctx context.Context, clusters []string, namespace string, label map[string]string, name, sliceSubnet string, clusterCidr string) (map[string]int, error)
}

// WorkerSliceConfigService implements the IWorkerSliceConfigService interface
type WorkerSliceConfigService struct {
	mf metrics.MetricRecorder
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

	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(req.Namespace)).
		WithNamespace(req.Namespace).
		WithSlice(workerSliceConfig.Labels["original-slice-name"])

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(req.Namespace)).
		WithNamespace(req.Namespace).
		WithSlice(workerSliceConfig.Labels["original-slice-name"])

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
				//Register an event for worker slice config deleted forcefully
				util.RecordEvent(ctx, eventRecorder, workerSliceConfig, slice, events.EventWorkerSliceConfigDeletedForcefully)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deleted_forcefully",
						"event":       string(events.EventWorkerSliceConfigDeletedForcefully),
						"object_name": workerSliceConfig.Name,
						"object_kind": metricKindWorkerSliceConfig,
					},
				)
				if slice.Annotations == nil {
					slice.Annotations = make(map[string]string)
				}
				slice.Annotations["updatedTimestamp"] = time.Now().String()
				logger.Debug("Recreating workerSliceConfig", req.NamespacedName)
				err = util.UpdateResource(ctx, slice)
				if err != nil {
					//Register an event for worker slice config recreation failure
					util.RecordEvent(ctx, eventRecorder, workerSliceConfig, slice, events.EventWorkerSliceConfigRecreationFailed)
					s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
						map[string]string{
							"action":      "recreation_failed",
							"event":       string(events.EventWorkerSliceConfigRecreationFailed),
							"object_name": workerSliceConfig.Name,
							"object_kind": metricKindWorkerSliceConfig,
						},
					)
					return result, err
				}
				//Register an event for worker slice config recreation success
				util.RecordEvent(ctx, eventRecorder, workerSliceConfig, slice, events.EventWorkerSliceConfigRecreated)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "recreated",
						"event":       string(events.EventWorkerSliceConfigRecreated),
						"object_name": workerSliceConfig.Name,
						"object_kind": metricKindWorkerSliceConfig,
					},
				)
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
	octet := workerSliceConfig.Spec.Octet
	clusterSubnetCIDR := workerSliceConfig.Spec.ClusterSubnetCIDR
	slice := s.copySpecFromSliceConfigToWorkerSlice(ctx, *sliceConfig)
	workerSliceConfig.Spec = slice.Spec

	// if standardQos Found update the workerSliceConfig
	if sliceConfig.Spec.StandardQosProfileName != "" {
		qos := &controllerv1alpha1.SliceQoSConfig{}
		found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
			Name:      sliceConfig.Spec.StandardQosProfileName,
			Namespace: req.Namespace,
		}, qos)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !found {
			logger.Infof("QOS profile %v not found, returning from  reconciler loop.", sliceConfig.Spec.StandardQosProfileName)
			return ctrl.Result{}, nil
		}
		workerSliceConfig.Spec.QosProfileDetails = workerv1alpha1.QOSProfile{
			QueueType:               qos.Spec.QueueType,
			Priority:                qos.Spec.Priority,
			TcType:                  qos.Spec.TcType,
			BandwidthCeilingKbps:    qos.Spec.BandwidthCeilingKbps,
			BandwidthGuaranteedKbps: qos.Spec.BandwidthGuaranteedKbps,
			DscpClass:               qos.Spec.DscpClass,
		}
		workerSliceConfig.Labels["standard-qos-profile"] = sliceConfig.Spec.StandardQosProfileName
	} else {
		workerSliceConfig.Labels["standard-qos-profile"] = ""
	}

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
		nonDuplicateClusters := util.RemoveDuplicatesFromArray(namespace.Clusters)
		for _, cluster := range nonDuplicateClusters {
			if cluster == "*" || cluster == workerSliceConfig.Labels["worker-cluster"] {
				workerIsolationProfile.ApplicationNamespaces = append(workerIsolationProfile.ApplicationNamespaces, namespace.Namespace)
			}
		}
	}
	for _, namespace := range controllerIsolationProfile.AllowedNamespaces {
		nonDuplicateClusters := util.RemoveDuplicatesFromArray(namespace.Clusters)
		for _, cluster := range nonDuplicateClusters {
			if cluster == "*" || cluster == workerSliceConfig.Labels["worker-cluster"] {
				workerIsolationProfile.AllowedNamespaces = append(workerIsolationProfile.AllowedNamespaces, namespace.Namespace)
			}
		}
	}

	workerSliceConfig.Spec.ExternalGatewayConfig = externalGatewayConfig
	workerSliceConfig.Spec.NamespaceIsolationProfile = workerIsolationProfile
	workerSliceConfig.Spec.SliceName = sliceConfig.Name
	workerSliceConfig.Spec.Octet = octet
	workerSliceConfig.Spec.ClusterSubnetCIDR = clusterSubnetCIDR
	err = util.UpdateResource(ctx, workerSliceConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, err
}

// CreateMinimalWorkerSliceConfig CreateWorkerSliceConfig is a function to create the worker slice configs with minimum number of fields.
// More fields are added in reconciliation loop.
func (s *WorkerSliceConfigService) CreateMinimalWorkerSliceConfig(ctx context.Context, clusters []string, namespace string, label map[string]string, name, sliceSubnet string, clusterCidr string) (map[string]int, error) {
	logger := util.CtxLogger(ctx)

	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(name)

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(name)

	err := s.cleanUpSlices(ctx, label, namespace, clusters)
	if err != nil {
		return nil, err
	}
	workerSliceConfigs, err := s.ListWorkerSliceConfigs(ctx, label, namespace)
	if err != nil {
		return nil, err
	}
	clusterMap := s.ComputeClusterMap(clusters, workerSliceConfigs)
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
		ipamOctet := clusterMap[cluster]
		clusterSubnetCIDR := fmt.Sprintf(util.GetClusterPrefixPool(sliceSubnet, ipamOctet, clusterCidr))
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
			expectedSlice.Spec.Octet = &ipamOctet
			expectedSlice.Spec.ClusterSubnetCIDR = clusterSubnetCIDR
			expectedSlice.Spec.SliceSubnet = sliceSubnet
			err = util.CreateResource(ctx, &expectedSlice)
			if err != nil {
				//Register an event for worker slice config creation failure
				util.RecordEvent(ctx, eventRecorder, &expectedSlice, nil, events.EventWorkerSliceConfigCreationFailed)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "creation_failed",
						"event":       string(events.EventWorkerSliceConfigCreationFailed),
						"object_name": expectedSlice.Name,
						"object_kind": metricKindWorkerSliceConfig,
					},
				)
				if !k8sErrors.IsAlreadyExists(err) { // ignores resource already exists error(for handling parallel calls to create same resource)
					logger.Debug("failed to create worker slice %s since it already exists, namespace - %s ",
						expectedSlice.Name, namespace)
					return clusterMap, err
				}
			}
			//Register an event for worker slice config creation success
			util.RecordEvent(ctx, eventRecorder, &expectedSlice, nil, events.EventWorkerSliceConfigCreated)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "created",
					"event":       string(events.EventWorkerSliceConfigCreated),
					"object_name": expectedSlice.Name,
					"object_kind": metricKindWorkerSliceConfig,
				},
			)
		} else {
			existingSlice.UID = ""
			existingSlice.Spec.Octet = &ipamOctet
			existingSlice.Spec.ClusterSubnetCIDR = clusterSubnetCIDR
			logger.Debug("updating slice with new octet", existingSlice)
			if existingSlice.Annotations == nil {
				existingSlice.Annotations = make(map[string]string)
			}
			existingSlice.Annotations["updatedTimestamp"] = time.Now().String()
			err = util.UpdateResource(ctx, existingSlice)
			if err != nil {
				//Register an event for worker slice config update failure
				util.RecordEvent(ctx, eventRecorder, existingSlice, nil, events.EventWorkerSliceConfigUpdateFailed)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "update_failed",
						"event":       string(events.EventWorkerSliceConfigUpdateFailed),
						"object_name": existingSlice.Name,
						"object_kind": metricKindWorkerSliceConfig,
					},
				)
				if !k8sErrors.IsAlreadyExists(err) { // ignores resource already exists error(for handling parallel calls to create same resource)
					logger.Debug("failed to create worker slice %s since it already exists, namespace - %s ",
						workerSliceConfigName, namespace)
					return clusterMap, err
				}
			}
			//Register an event for worker slice config update success
			util.RecordEvent(ctx, eventRecorder, existingSlice, nil, events.EventWorkerSliceConfigUpdated)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "updated",
					"event":       string(events.EventWorkerSliceConfigUpdated),
					"object_name": existingSlice.Name,
					"object_kind": metricKindWorkerSliceConfig,
				},
			)
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
	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(label["original-slice-name"])

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(label["original-slice-name"])

	for _, slice := range slices {
		err = util.DeleteResource(ctx, &slice)
		if err != nil {
			//Register an event for worker slice config deletion failure
			util.RecordEvent(ctx, eventRecorder, &slice, nil, events.EventWorkerSliceConfigDeletionFailed)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventWorkerSliceConfigDeletionFailed),
					"object_name": slice.Name,
					"object_kind": metricKindWorkerSliceConfig,
				},
			)
			return err
		}
		//Register an event for worker slice config deletion success
		util.RecordEvent(ctx, eventRecorder, &slice, nil, events.EventWorkerSliceConfigDeleted)
		s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventWorkerSliceConfigDeleted),
				"object_name": slice.Name,
				"object_kind": metricKindWorkerSliceConfig,
			},
		)
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

// ComputeClusterMap - function assigns a numerical value to the cluster. The value will be from 1 to n, where n is the number of clusters in the slice.
func (s *WorkerSliceConfigService) ComputeClusterMap(clusterNames []string, workerSliceConfigs []workerv1alpha1.WorkerSliceConfig) map[string]int {
	clusterMapping := make(map[string]int, len(clusterNames))
	tempClusterMapping := make(map[string]string, len(clusterNames))
	usedIndexes := make(map[int]bool, 0)
	for _, WorkerSliceConfig := range workerSliceConfigs {
		if WorkerSliceConfig.Spec.Octet != nil {
			clusterMapping[WorkerSliceConfig.Labels["worker-cluster"]] = *WorkerSliceConfig.Spec.Octet
			tempClusterMapping[WorkerSliceConfig.Labels["worker-cluster"]] = fmt.Sprintf("%d", *WorkerSliceConfig.Spec.Octet)
			usedIndexes[*WorkerSliceConfig.Spec.Octet] = true
		} else {
			tempClusterMapping[WorkerSliceConfig.Labels["worker-cluster"]] = ""
		}
	}
	unUsedIndexes := make([]int, 0)
	for index := 0; index < len(clusterNames); index++ {
		if !usedIndexes[index] {
			unUsedIndexes = append(unUsedIndexes, index)
		}
	}

	currentIndex := 0
	for _, clusterName := range clusterNames {
		if tempClusterMapping[clusterName] == "" {
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
	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(label["original-slice-name"])

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(label["original-slice-name"])

	for _, slice := range slices {
		clusterName := slice.Labels["worker-cluster"]
		if !clusterSet[clusterName] {
			err = util.DeleteResource(ctx, &slice)
			if err != nil {
				//Register an event for worker slice config deletion failure
				util.RecordEvent(ctx, eventRecorder, &slice, nil, events.EventWorkerSliceConfigDeletionFailed)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deletion_failed",
						"event":       string(events.EventWorkerSliceConfigDeletionFailed),
						"object_name": slice.Name,
						"object_kind": metricKindWorkerSliceConfig,
					},
				)
				return err
			}
			//Register an event for worker slice config deletion success
			util.RecordEvent(ctx, eventRecorder, &slice, nil, events.EventWorkerSliceConfigDeleted)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deleted",
					"event":       string(events.EventWorkerSliceConfigDeleted),
					"object_name": slice.Name,
					"object_kind": metricKindWorkerSliceConfig,
				},
			)
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
