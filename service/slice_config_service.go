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

	"github.com/kubeslice/kubeslice-controller/metrics"
	"go.uber.org/zap"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ISliceConfigService interface {
	ReconcileSliceConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	DeleteSliceConfigs(ctx context.Context, namespace string) (ctrl.Result, error)
}

// SliceConfigService implements different interfaces -
type SliceConfigService struct {
	ns    INamespaceService
	acs   IAccessControlService
	sgs   IWorkerSliceGatewayService
	ms    IWorkerSliceConfigService
	si    IWorkerServiceImportService
	se    IServiceExportConfigService
	wsgrs IWorkerSliceGatewayRecyclerService
	mf    metrics.IMetricRecorder
	vpn   IVpnKeyRotationService
	sipam ISliceIpamService
}

const NamespaceAndClusterFormat = "namespace=%s&cluster=%s"

// ReconcileSliceConfig is a function to reconcile the sliceconfig
func (s *SliceConfigService) ReconcileSliceConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get SliceConfig resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Started Recoincilation of SliceConfig %v", req.NamespacedName)
	sliceConfig := &v1alpha1.SliceConfig{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, sliceConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("sliceConfig %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	// Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(sliceConfig.Namespace)).
		WithNamespace(sliceConfig.Namespace).
		WithSlice(sliceConfig.Name)

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(sliceConfig.Namespace)).
		WithNamespace(sliceConfig.Namespace).
		WithSlice(sliceConfig.Name)

	if duplicate, value := util.CheckDuplicateInArray(sliceConfig.Spec.Clusters); duplicate {
		logger.Infof("Duplicate cluster name %v found in sliceConfig %v", value, req.NamespacedName)
		return ctrl.Result{}, nil
	}
	// Step 1: Finalizers
	if sliceConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(sliceConfig.GetFinalizers(), SliceConfigFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, sliceConfig, SliceConfigFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for slice config", req.NamespacedName)
		if shouldReturn, result, reconErr := util.IsReconciled(s.cleanUpSliceConfigResources(ctx, sliceConfig, req.Namespace)); shouldReturn {
			return result, reconErr
		}
		if shouldReturn, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, sliceConfig, SliceConfigFinalizer)); shouldReturn {
			// Register an event for slice config deletion fail
			util.RecordEvent(ctx, eventRecorder, sliceConfig, nil, events.EventSliceConfigDeletionFailed)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventSliceConfigDeletionFailed),
					"object_name": sliceConfig.Name,
					"object_kind": metricKindSliceConfig,
				},
			)
			return result, reconErr
		}
		// Register an event for slice config deletion
		util.RecordEvent(ctx, eventRecorder, sliceConfig, nil, events.EventSliceConfigDeleted)
		s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventSliceConfigDeleted),
				"object_name": sliceConfig.Name,
				"object_kind": metricKindSliceConfig,
			},
		)
		return ctrl.Result{}, err
	}

	// Step 2: check if SliceConfig is in project namespace
	nsResource := &corev1.Namespace{}
	found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
		Name: req.Namespace,
	}, nsResource)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found || !s.checkForProjectNamespace(nsResource) {
		logger.Infof("Created SliceConfig %v is not in project namespace. Returning from reconciliation loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Step 3: Before creation or update of worker slice config, handle default slice appns removal if project has defaultSliceCreation enabled
	projectName := util.GetProjectName(req.Namespace)
	project := &v1alpha1.Project{}
	foundProject, err := util.GetResourceIfExist(ctx, types.NamespacedName{
		Name:      projectName,
		Namespace: ControllerNamespace,
	}, project)
	if err != nil {
		return ctrl.Result{}, err
	}

	if foundProject {
		// add missing project label to sliceConfig
		additionalLabels := util.FilterLabelsAndAnnotations(project.Labels)
		additionalAnnotations := util.FilterLabelsAndAnnotations(project.Annotations)
		if sliceConfig.Labels == nil {
			sliceConfig.Labels = make(map[string]string)
		}
		// check if project label is already present in sliceConfig
		isUpadataRequired := false
		for key, value := range additionalLabels {
			if val, ok := sliceConfig.Labels[key]; !ok && val != value {
				sliceConfig.Labels[key] = value
				isUpadataRequired = true
			}
		}
		if sliceConfig.Annotations == nil {
			sliceConfig.Annotations = make(map[string]string)
		}
		for key, value := range additionalAnnotations {
			if val, ok := sliceConfig.Annotations[key]; !ok && val != value {
				sliceConfig.Annotations[key] = value
				isUpadataRequired = true
			}
		}
		if isUpadataRequired {
			if err := util.UpdateResource(ctx, sliceConfig); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if foundProject && project.Spec.DefaultSliceCreation {
		logger.Info("found project and defaultslicecreation is enable")
		if shouldReturn, result, reconErr := util.IsReconciled(s.handleDefaultSliceConfigAppns(ctx, req, logger, projectName, sliceConfig)); shouldReturn {
			return result, reconErr
		}
	}

	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(sliceConfig), sliceConfig.GetName())
	ownershipLabel := util.GetOwnerLabel(completeResourceName)

	if sliceConfig.Spec.OverlayNetworkDeploymentMode == v1alpha1.NONET {
		err = s.ms.CreateMinimalWorkerSliceConfigForNoNetworkSlice(ctx, sliceConfig.Spec.Clusters, req.Namespace, ownershipLabel, sliceConfig.Name)
		return ctrl.Result{}, err
	}

	// Step 3.5: Handle Dynamic IPAM if enabled
	if sliceConfig.Spec.SliceIpamType == "Dynamic" {
		logger.Infof("Dynamic IPAM enabled for slice %s", sliceConfig.Name)

		// Create SliceIpam resource if service is available
		if s.sipam != nil {
			if err := s.sipam.CreateSliceIpam(ctx, sliceConfig); err != nil {
				logger.Errorf("Failed to create SliceIpam for slice %s: %v", sliceConfig.Name, err)
				return ctrl.Result{}, err
			}
			logger.Infof("SliceIpam successfully created/updated for slice %s", sliceConfig.Name)
		} else {
			logger.Warnf("SliceIpam service not available for slice %s", sliceConfig.Name)
		}
	}

	// Step 4: Creation of worker slice Objects and Cluster Labels
	// get cluster cidr from maxClusters of slice config
	clusterCidr := ""
	clusterCidr = util.FindCIDRByMaxClusters(sliceConfig.Spec.MaxClusters)

	// collect slice gw svc info for given clusters
	sliceGwSvcTypeMap := getSliceGwSvcTypes(sliceConfig)

	clusterMap, err := s.ms.CreateMinimalWorkerSliceConfig(ctx, sliceConfig.Spec.Clusters, req.Namespace, ownershipLabel, sliceConfig.Name, sliceConfig.Spec.SliceSubnet, clusterCidr, sliceGwSvcTypeMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Step 5: Create gateways with minimum specification
	_, err = s.sgs.CreateMinimumWorkerSliceGateways(ctx, sliceConfig.Name, sliceConfig.Spec.Clusters, req.Namespace, ownershipLabel, clusterMap, sliceConfig.Spec.SliceSubnet, clusterCidr, sliceGwSvcTypeMap)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Infof("sliceConfig %v reconciled", req.NamespacedName)

	// Step 6: Create VPNKeyRotation CR
	// TODO(rahul): handle change in rotation interval
	if err := s.vpn.CreateMinimalVpnKeyRotationConfig(ctx, sliceConfig.Name, sliceConfig.Namespace, sliceConfig.Spec.RotationInterval); err != nil {
		// register an event
		util.RecordEvent(ctx, eventRecorder, sliceConfig, nil, events.EventVPNKeyRotationConfigCreationFailed)
		return ctrl.Result{}, err
	}
	// Step 7: update cluster info into vpnkeyrotation Cconfig
	if _, err := s.vpn.ReconcileClusters(ctx, sliceConfig.Name, sliceConfig.Namespace, sliceConfig.Spec.Clusters); err != nil {
		return ctrl.Result{}, err
	}

	// Step 8: Create ServiceImport Objects
	serviceExports := &v1alpha1.ServiceExportConfigList{}
	_, err = s.getServiceExportBySliceName(ctx, req.Namespace, sliceConfig.Name, serviceExports)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(serviceExports.Items) > 0 {
		// iterate service export configs
		for _, serviceExport := range serviceExports.Items {
			err = s.si.CreateMinimalWorkerServiceImport(ctx, sliceConfig.Spec.Clusters, req.Namespace, s.getOwnerLabelsForServiceExport(&serviceExport), serviceExport.Spec.ServiceName, serviceExport.Spec.ServiceNamespace, serviceExport.Spec.SliceName, serviceExport.Spec.Aliases)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// checkForProjectNamespace is a function to check the namespace is in proper format
func (s *SliceConfigService) checkForProjectNamespace(namespace *corev1.Namespace) bool {
	return namespace.Labels[util.LabelName] == fmt.Sprintf(util.LabelValue, "Project", namespace.Name)
}

// cleanUpSliceConfigResources is a function to delete the slice config resources
func (s *SliceConfigService) cleanUpSliceConfigResources(ctx context.Context,
	slice *v1alpha1.SliceConfig, namespace string,
) (ctrl.Result, error) {
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(slice), slice.GetName())
	ownershipLabel := util.GetOwnerLabel(completeResourceName)
	err := s.sgs.DeleteWorkerSliceGatewaysByLabel(ctx, ownershipLabel, namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = s.ms.DeleteWorkerSliceConfigByLabel(ctx, ownershipLabel, namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	recyclerLabel := map[string]string{
		"slice_name": slice.Name,
	}
	err = s.wsgrs.DeleteWorkerSliceGatewayRecyclersByLabel(ctx, recyclerLabel, namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Clean up SliceIpam resource if it exists
	if slice.Spec.SliceIpamType == "Dynamic" {
		logger := util.CtxLogger(ctx)
		logger.Infof("Cleaning up SliceIpam for slice %s", slice.Name)

		// Delete SliceIpam resource if service is available
		if s.sipam != nil {
			if err := s.sipam.DeleteSliceIpam(ctx, slice.Name, namespace); err != nil {
				logger.Errorf("Failed to delete SliceIpam for slice %s: %v", slice.Name, err)
				return ctrl.Result{}, err
			}
			logger.Infof("SliceIpam successfully deleted for slice %s", slice.Name)
		} else {
			logger.Warnf("SliceIpam service not available for slice %s", slice.Name)
		}
	}

	return ctrl.Result{}, nil
}

// DeleteSliceConfigs is a function to delete the sliceconfigs
func (s *SliceConfigService) DeleteSliceConfigs(ctx context.Context, namespace string) (ctrl.Result, error) {
	sliceConfigs := &v1alpha1.SliceConfigList{}
	err := util.ListResources(ctx, sliceConfigs, client.InNamespace(namespace))
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, sliceConfig := range sliceConfigs.Items {
		// Load Event Recorder with project name, slice name and namespace
		eventRecorder := util.CtxEventRecorder(ctx).
			WithProject(util.GetProjectName(sliceConfig.Namespace)).
			WithNamespace(sliceConfig.Namespace).
			WithSlice(sliceConfig.Name)

		// Load metrics with project name and namespace
		s.mf.WithProject(util.GetProjectName(sliceConfig.Namespace)).
			WithNamespace(sliceConfig.Namespace).
			WithSlice(sliceConfig.Name)

		err = util.DeleteResource(ctx, &sliceConfig)
		if err != nil {
			// Register an event for slice config deletion fail
			util.RecordEvent(ctx, eventRecorder, &sliceConfig, nil, events.EventSliceConfigDeletionFailed)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventSliceConfigDeletionFailed),
					"object_name": sliceConfig.Name,
					"object_kind": metricKindSliceConfig,
				},
			)
			return ctrl.Result{}, err
		}
		// Register an event for slice config deletion
		util.RecordEvent(ctx, eventRecorder, &sliceConfig, nil, events.EventSliceConfigDeleted)
		s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventSliceConfigDeleted),
				"object_name": sliceConfig.Name,
				"object_kind": metricKindSliceConfig,
			},
		)
	}
	return ctrl.Result{}, nil
}

// getServiceExportBySliceName is a function to get the service export configs by slice name
func (s *SliceConfigService) getServiceExportBySliceName(ctx context.Context, namespace string, sliceName string, serviceExports *v1alpha1.ServiceExportConfigList) (ctrl.Result, error) {
	label := map[string]string{
		"original-slice-name": sliceName,
	}
	err := util.ListResources(ctx, serviceExports, client.InNamespace(namespace), client.MatchingLabels(label))
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// getOwnerLabelsForServiceExport is a function to get the owner labels for service export
func (s *SliceConfigService) getOwnerLabelsForServiceExport(serviceExportConfig *v1alpha1.ServiceExportConfig) map[string]string {
	ownerLabels := make(map[string]string)
	resourceName := fmt.Sprintf("%s-%s-%s", serviceExportConfig.Spec.ServiceName, serviceExportConfig.Spec.ServiceNamespace, serviceExportConfig.Spec.SliceName)
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(serviceExportConfig), resourceName)
	ownerLabels = util.GetOwnerLabel(completeResourceName)
	return ownerLabels
}

func (s *SliceConfigService) constructApplicationNamespaceMap(registeredClusters []string, sliceConfigApplicationNamespaces []v1alpha1.SliceNamespaceSelection) map[string]struct{} {
	nsMap := make(map[string]struct{})
	for _, appns := range sliceConfigApplicationNamespaces {
		if len(appns.Clusters) > 0 && appns.Clusters[0] == "*" {
			// add all cluster and namespace combnination to the map
			for _, cluster := range registeredClusters {
				namespaceToClusterMapKey := fmt.Sprintf(NamespaceAndClusterFormat, appns.Namespace, cluster)
				nsMap[namespaceToClusterMapKey] = struct{}{}
			}
		} else {
			for _, cluster := range appns.Clusters {
				namespaceToClusterMapKey := fmt.Sprintf(NamespaceAndClusterFormat, appns.Namespace, cluster)
				nsMap[namespaceToClusterMapKey] = struct{}{}
			}
		}
	}
	return nsMap
}

func (s *SliceConfigService) removeSliceApplicationNamespaces(namespaceWithCluster map[string]struct{}, defaultSliceConfigApplicationNamespaces []v1alpha1.SliceNamespaceSelection) []v1alpha1.SliceNamespaceSelection {
	filteredApplicaitonNamespaces := []v1alpha1.SliceNamespaceSelection{}

	for _, appns := range defaultSliceConfigApplicationNamespaces {
		if len(appns.Clusters) > 1 {
			for _, cluster := range appns.Clusters {
				mapKey := fmt.Sprintf(NamespaceAndClusterFormat, appns.Namespace, cluster)
				if _, ok := namespaceWithCluster[mapKey]; ok {
					appns.Clusters = util.RemoveElementFromArray(appns.Clusters, cluster)
				}
			}
			filteredApplicaitonNamespaces = append(filteredApplicaitonNamespaces, appns)
		} else {
			mapKey := fmt.Sprintf(NamespaceAndClusterFormat, appns.Namespace, appns.Clusters[0])
			if _, ok := namespaceWithCluster[mapKey]; ok {
				continue
			}

			filteredApplicaitonNamespaces = append(filteredApplicaitonNamespaces, appns)
		}
	}
	return filteredApplicaitonNamespaces
}

func (s *SliceConfigService) handleDefaultSliceConfigAppns(ctx context.Context, req ctrl.Request, logger *zap.SugaredLogger, projectName string, sliceConfig *v1alpha1.SliceConfig) (ctrl.Result, error) {
	defaultSliceName := fmt.Sprintf(util.DefaultProjectSliceName, projectName)
	defaultProjectSlice := &v1alpha1.SliceConfig{}
	defaultSliceNamespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      defaultSliceName,
	}
	foundDefaultSlice, err := util.GetResourceIfExist(ctx, defaultSliceNamespacedName, defaultProjectSlice)
	if err != nil {
		logger.Error("error while getting default slice %v", defaultSliceName)
		return ctrl.Result{}, err
	}
	if foundDefaultSlice {

		logger.Info("found default slice", defaultProjectSlice.Name)
		if defaultProjectSlice.Name == sliceConfig.Name {
			// reconciling for default-slice so no need to remove ns
			return ctrl.Result{}, nil
		}
		// remove all namespaces from default slice that are present in this slice config
		defaultApplicationNamespaces := defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces
		sliceConfigApplicationNamespaces := sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces

		sliceAppnsMap := s.constructApplicationNamespaceMap(sliceConfig.Spec.Clusters, sliceConfigApplicationNamespaces)
		defaultAppnsMap := s.constructApplicationNamespaceMap(defaultProjectSlice.Spec.Clusters, defaultApplicationNamespaces)
		appnsMapToRemove := make(map[string]struct{})

		for key := range sliceAppnsMap {
			// if same namespace and cluster is present in default slice, then  add it to appnsMapToRemove
			if namespaceWithCluster, ok := defaultAppnsMap[key]; ok {
				appnsMapToRemove[key] = namespaceWithCluster
			}
		}

		filteredDefaultApplicationNamespaces := s.removeSliceApplicationNamespaces(appnsMapToRemove, defaultApplicationNamespaces)

		if !reflect.DeepEqual(filteredDefaultApplicationNamespaces, defaultApplicationNamespaces) {
			logger.Info("updating default slice config from %s to %s", defaultApplicationNamespaces, filteredDefaultApplicationNamespaces)
			defaultProjectSlice.Spec.NamespaceIsolationProfile.ApplicationNamespaces = filteredDefaultApplicationNamespaces
			err := util.UpdateResource(ctx, defaultProjectSlice)
			if err != nil {
				return ctrl.Result{}, err
			}
			logger.Info("successfully updated default slice config")

			return ctrl.Result{}, nil
		}
	}
	return ctrl.Result{}, nil
}
