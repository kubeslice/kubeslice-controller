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

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IServiceExportConfigService interface {
	ReconcileServiceExportConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	DeleteServiceExportConfigs(ctx context.Context, namespace string) (ctrl.Result, error)
	DeleteServiceExportConfigByParticipatingSliceConfig(ctx context.Context, sliceName string, namespace string) error
}

type ServiceExportConfigService struct {
	ses IWorkerServiceImportService
}

// ReconcileServiceExportConfig is a function to reconcile the service export config
func (s *ServiceExportConfigService) ReconcileServiceExportConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get project resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Starting Recoincilation of ServiceExportConfig with name %s in namespace %s",
		req.Name, req.Namespace)
	serviceExportConfig := &controllerv1alpha1.ServiceExportConfig{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, serviceExportConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("ServiceExportConfig %v not found, returning from reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(serviceExportConfig.Namespace)).
		WithNamespace(serviceExportConfig.Namespace).
		WithSlice(serviceExportConfig.Labels["original-slice-name"])

	//Step 1: Finalizers
	if serviceExportConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Debugf("Not deleting")
		if !util.ContainsString(serviceExportConfig.GetFinalizers(), serviceExportConfigFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, serviceExportConfig, serviceExportConfigFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for serviceExportConfig", req.NamespacedName)
		// todo: delete logic
		if shouldReturn, result, reconErr := util.IsReconciled(s.cleanUpServiceExportConfigResources(ctx, serviceExportConfig, req.Namespace)); shouldReturn {
			return result, reconErr
		}
		if shouldReturn, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, serviceExportConfig, serviceExportConfigFinalizer)); shouldReturn {
			//Register an event for service export config deletion failure
			util.RecordEvent(ctx, eventRecorder, serviceExportConfig, nil, events.EventServiceExportConfigDeletionFailed)
			return result, reconErr
		}
		//Register an event for service export config deletion
		util.RecordEvent(ctx, eventRecorder, serviceExportConfig, nil, events.EventServiceExportConfigDeleted)
		return ctrl.Result{}, err
	}
	//Step 2: Get the slice based upon the sliceName and sliceNamespace
	slice := controllerv1alpha1.SliceConfig{}
	exist, err := util.GetResourceIfExist(ctx, client.ObjectKey{
		Name:      serviceExportConfig.Spec.SliceName,
		Namespace: req.Namespace,
	}, &slice)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exist {
		logger.With(zap.Error(err)).Errorf("Slice %s doesn't exist in namespace %s",
			serviceExportConfig.Spec.SliceName, req.Namespace)
		return ctrl.Result{}, nil
	}
	// Step 3: Add labels to ServiceExportConfig (if not present)
	if serviceExportConfig.Labels["original-slice-name"] != serviceExportConfig.Spec.SliceName ||
		serviceExportConfig.Labels["worker-cluster"] != serviceExportConfig.Spec.SourceCluster ||
		serviceExportConfig.Labels["service-name"] != serviceExportConfig.Spec.ServiceName ||
		serviceExportConfig.Labels["service-namespace"] != serviceExportConfig.Spec.ServiceNamespace {

		if serviceExportConfig.Labels == nil {
			serviceExportConfig.Labels = make(map[string]string)
		}
		serviceExportConfig.Labels["original-slice-name"] = serviceExportConfig.Spec.SliceName
		serviceExportConfig.Labels["worker-cluster"] = serviceExportConfig.Spec.SourceCluster
		serviceExportConfig.Labels["service-name"] = serviceExportConfig.Spec.ServiceName
		serviceExportConfig.Labels["service-namespace"] = serviceExportConfig.Spec.ServiceNamespace
		err = util.UpdateResource(ctx, serviceExportConfig)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	ownerLabels := s.getOwnerLabelsForServiceExport(serviceExportConfig)
	err = s.ses.CreateMinimalWorkerServiceImport(ctx, slice.Spec.Clusters, req.Namespace, ownerLabels, serviceExportConfig.Spec.ServiceName, serviceExportConfig.Spec.ServiceNamespace, serviceExportConfig.Spec.SliceName, serviceExportConfig.Spec.Aliases)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (s *ServiceExportConfigService) cleanUpServiceExportConfigResources(ctx context.Context,
	serviceExportConfig *controllerv1alpha1.ServiceExportConfig, namespace string) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	seList := controllerv1alpha1.ServiceExportConfigList{}
	if err := s.ses.LookupServiceExportForService(ctx, &seList, serviceExportConfig.Namespace, serviceExportConfig.Spec.ServiceName, serviceExportConfig.Spec.ServiceNamespace, serviceExportConfig.Spec.SliceName); err != nil {
		return ctrl.Result{}, err
	}
	ownershipLabel := s.getOwnerLabelsForServiceExport(serviceExportConfig)
	// If no ServiceExportConfig are found: delete all service imports
	// If exactly 1 ServiceExportConfig is found, and it belongs to same cluster for which the service export is being deleted for (race condition): delete all service imports
	if len(seList.Items) == 0 || (len(seList.Items) == 1 && seList.Items[0].Spec.SourceCluster == serviceExportConfig.Spec.SourceCluster) {
		err := s.ses.DeleteWorkerServiceImportByLabel(ctx, ownershipLabel, namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		list, err := s.ses.ListWorkerServiceImport(ctx, ownershipLabel, serviceExportConfig.Namespace)
		if err != nil {
			logger.With(zap.Error(err)).Errorf("failed to list worker service imports with labels %v", ownershipLabel)
		}
		err = s.ses.ForceReconciliation(ctx, list)
		if err != nil {
			logger.With(zap.Error(err)).Errorf("failed to queue worker service imports for reconciliation with labels %v", ownershipLabel)
		}
	}
	return ctrl.Result{}, nil
}

// DeleteServiceExportConfigs is a function to delete the export configs
func (s *ServiceExportConfigService) DeleteServiceExportConfigs(ctx context.Context, namespace string) (ctrl.Result, error) {
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	err := util.ListResources(ctx, serviceExports, client.InNamespace(namespace))
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, serviceExport := range serviceExports.Items {
		//Load Event Recorder with project name, slice name and namespace
		eventRecorder := util.CtxEventRecorder(ctx).
			WithProject(util.GetProjectName(serviceExport.Namespace)).
			WithNamespace(serviceExport.Namespace).
			WithSlice(serviceExport.Labels["original-slice-name"])
		err = util.DeleteResource(ctx, &serviceExport)
		if err != nil {
			//Register an event for service export config deletion
			util.RecordEvent(ctx, eventRecorder, &serviceExport, nil, events.EventServiceExportConfigDeletionFailed)
			return ctrl.Result{}, err
		}
		//Register an event for service export config deletion
		util.RecordEvent(ctx, eventRecorder, &serviceExport, nil, events.EventServiceExportConfigDeleted)
	}
	return ctrl.Result{}, nil
}

// DeleteServiceExportConfigByParticipatingSliceConfig is a function to delete the export config which are in slice
func (s *ServiceExportConfigService) DeleteServiceExportConfigByParticipatingSliceConfig(ctx context.Context, sliceName string, namespace string) error {
	serviceExports, err := s.ListServiceExportConfigs(ctx, namespace)
	if err != nil {
		return err
	}
	for _, serviceExport := range serviceExports {
		if serviceExport.Labels["original-slice-name"] == sliceName {
			//Load Event Recorder with project name, slice name and namespace
			eventRecorder := util.CtxEventRecorder(ctx).
				WithProject(util.GetProjectName(serviceExport.Namespace)).
				WithNamespace(serviceExport.Namespace).
				WithSlice(serviceExport.Labels["original-slice-name"])
			err = util.DeleteResource(ctx, &serviceExport)
			if err != nil {
				//Register an event for service export config deletion
				util.RecordEvent(ctx, eventRecorder, &serviceExport, nil, events.EventServiceExportConfigDeletionFailed)
				return err
			}
			//Register an event for service export config deletion
			util.RecordEvent(ctx, eventRecorder, &serviceExport, nil, events.EventServiceExportConfigDeleted)
		}
	}
	return nil
}

func (s *ServiceExportConfigService) ListServiceExportConfigs(ctx context.Context, namespace string) ([]controllerv1alpha1.ServiceExportConfig, error) {
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	err := util.ListResources(ctx, serviceExports, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return serviceExports.Items, nil
}

func (s *ServiceExportConfigService) getOwnerLabelsForServiceExport(serviceExportConfig *controllerv1alpha1.ServiceExportConfig) map[string]string {
	ownerLabels := make(map[string]string)
	resourceName := fmt.Sprintf("%s-%s-%s", serviceExportConfig.Spec.ServiceName, serviceExportConfig.Spec.ServiceNamespace, serviceExportConfig.Spec.SliceName)
	// validating the length of label
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(serviceExportConfig), resourceName)
	ownerLabels = util.GetOwnerLabel(completeResourceName)
	return ownerLabels
}
