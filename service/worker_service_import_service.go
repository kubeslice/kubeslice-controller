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
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"go.uber.org/zap"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IWorkerServiceImportService interface {
	ReconcileWorkerServiceImport(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	CreateMinimalWorkerServiceImport(ctx context.Context, clusters []string, namespace string, label map[string]string,
		serviceName string, serviceNamespace string, sliceName string, aliases []string) error
	DeleteWorkerServiceImportByLabel(ctx context.Context, label map[string]string, namespace string) error
	ListWorkerServiceImport(ctx context.Context, ownerLabel map[string]string, namespace string) ([]workerv1alpha1.WorkerServiceImport, error)
	ForceReconciliation(ctx context.Context, list []workerv1alpha1.WorkerServiceImport) error
	LookupServiceExportForService(ctx context.Context,
		serviceExportList *controllerv1alpha1.ServiceExportConfigList,
		namespace, serviceName, serviceNamespace, sliceName string) error
}

type WorkerServiceImportService struct {
}

// ReconcileWorkerServiceImport is a function to reconcile the service import for worker object
func (s *WorkerServiceImportService) ReconcileWorkerServiceImport(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get WorkerServiceImport resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Started Recoincilation of WorkerServiceImport %v", req.NamespacedName)
	workerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, workerServiceImport)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("workerServiceImport %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(req.Namespace)).
		WithNamespace(req.Namespace).
		WithSlice(workerServiceImport.Labels["original-slice-name"])
	//Step 1: Finalizers
	if workerServiceImport.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(workerServiceImport.GetFinalizers(), WorkerServiceImportFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, workerServiceImport, WorkerServiceImportFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debugf("starting delete for WorkerServiceImport %v", req.NamespacedName)
		result := RemoveWorkerFinalizers(ctx, workerServiceImport, WorkerServiceImportFinalizer)
		if result.Requeue {
			return result, nil
		}
		serviceExportList := &controllerv1alpha1.ServiceExportConfigList{}
		err = s.LookupServiceExportForService(ctx, serviceExportList, req.Namespace, workerServiceImport.Spec.ServiceName, workerServiceImport.Spec.ServiceNamespace, workerServiceImport.Spec.SliceName)
		if err != nil {
			return result, err
		}
		found = len(serviceExportList.Items) > 0
		if found && serviceExportList.Items[0].ObjectMeta.DeletionTimestamp.IsZero() {
			serviceExport := &serviceExportList.Items[0]
			slice := &controllerv1alpha1.SliceConfig{}
			exist, err := util.GetResourceIfExist(ctx, client.ObjectKey{
				Name:      serviceExport.Spec.SliceName,
				Namespace: req.Namespace,
			}, slice)
			if err != nil {
				return result, err
			}
			if exist && util.IsInSlice(slice.Spec.Clusters, workerServiceImport.Labels["worker-cluster"]) {
				//Register an event for worker service import deleted forcefully
				util.RecordEvent(ctx, eventRecorder, workerServiceImport, serviceExport, events.EventWorkerServiceImportDeletedForcefully)
				if serviceExport.Annotations == nil {
					serviceExport.Annotations = make(map[string]string)
				}
				serviceExport.Annotations["updatedTimestamp"] = time.Now().String()
				err = util.UpdateResource(ctx, serviceExport)
				if err != nil {
					//Register an event for worker service import recreation failure
					util.RecordEvent(ctx, eventRecorder, workerServiceImport, serviceExport, events.EventWorkerServiceImportRecreationFailed)
					return result, err
				}
				//Register an event for worker service import recreation success
				util.RecordEvent(ctx, eventRecorder, workerServiceImport, serviceExport, events.EventWorkerServiceImportRecreated)
			}
		}
		return result, nil
	}

	serviceExportList := &controllerv1alpha1.ServiceExportConfigList{}
	err = s.LookupServiceExportForService(ctx, serviceExportList, req.Namespace, workerServiceImport.Spec.ServiceName, workerServiceImport.Spec.ServiceNamespace, workerServiceImport.Spec.SliceName)
	if err != nil {
		logger.Errorf("failed to list resources of kind ServiceExportConfig for service import reconciliation %v", workerServiceImport)
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, err
	}
	found = len(serviceExportList.Items) > 0
	if !found {
		logger.Infof("serviceExport %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	//add more fields
	serviceImportSpec := s.copySpecFromServiceExportConfigToWorkerServiceImport(ctx, serviceExportList.Items)
	workerServiceImport.Spec = serviceImportSpec
	workerServiceImport.UID = ""
	err = util.UpdateResource(ctx, workerServiceImport)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, err
}

// CreateMinimalWorkerServiceImport is a function to create the service import on worker object/cluster
func (s *WorkerServiceImportService) CreateMinimalWorkerServiceImport(ctx context.Context, clusters []string,
	namespace string, label map[string]string, serviceName string, serviceNamespace string, sliceName string, aliases []string) error {
	logger := util.CtxLogger(ctx)
	err := s.cleanUpWorkerServiceImportsForRemovedClusters(ctx, label, namespace, clusters)
	if err != nil {
		return err
	}

	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	for _, cluster := range clusters {
		logger.Debugf("Cluster Object %s", cluster)
		label["worker-cluster"] = cluster
		label["project-namespace"] = namespace
		label["original-slice-name"] = sliceName
		label["kubeslice-manager"] = "controller"

		expectedWorkerServiceImport := workerv1alpha1.WorkerServiceImport{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%s-%s", serviceName, serviceNamespace, sliceName, cluster),
				Labels:    label,
				Namespace: namespace,
			},
			Spec: workerv1alpha1.WorkerServiceImportSpec{
				ServiceName:      serviceName,
				ServiceNamespace: serviceNamespace,
				SliceName:        sliceName,
				Aliases:          aliases,
			},
		}
		existingWorkerServiceImport := &workerv1alpha1.WorkerServiceImport{}
		found, err := util.GetResourceIfExist(ctx, client.ObjectKey{
			Name:      expectedWorkerServiceImport.Name,
			Namespace: namespace,
		}, existingWorkerServiceImport)
		if err != nil {
			return err
		}
		if !found {
			err = util.CreateResource(ctx, &expectedWorkerServiceImport)
			if err != nil {
				//Register an event for worker service import create failure
				util.RecordEvent(ctx, eventRecorder, &expectedWorkerServiceImport, nil, events.EventWorkerServiceImportCreationFailed)
				if !k8sErrors.IsAlreadyExists(err) { // ignores resource already exists error (for handling parallel calls to create same resource)
					logger.Debug("failed to create worker service import %s since it already exists, namespace - %s ",
						expectedWorkerServiceImport.Name, namespace)
					return err
				}
			}
			//Register an event for worker service import create success
			util.RecordEvent(ctx, eventRecorder, &expectedWorkerServiceImport, nil, events.EventWorkerServiceImportCreated)
		} else {
			existingWorkerServiceImport.UID = ""
			if existingWorkerServiceImport.Annotations == nil {
				existingWorkerServiceImport.Annotations = make(map[string]string)
			}
			existingWorkerServiceImport.Annotations["updatedTimestamp"] = time.Now().String()
			err = util.UpdateResource(ctx, existingWorkerServiceImport)
			if err != nil {
				//Register an event for worker service import update failure
				util.RecordEvent(ctx, eventRecorder, existingWorkerServiceImport, nil, events.EventWorkerServiceImportUpdateFailed)
				if !k8sErrors.IsAlreadyExists(err) { // ignores resource already exists error (for handling parallel calls to create same resource)
					logger.Debug("failed to create service import %s since it already exists, namespace - %s ",
						existingWorkerServiceImport.Name, namespace)
					return err
				}
			}
			//Register an event for worker service import update success
			util.RecordEvent(ctx, eventRecorder, existingWorkerServiceImport, nil, events.EventWorkerServiceImportUpdated)
		}
	}
	return nil
}

// DeleteWorkerServiceImportByLabel is function to delete the service import from worker cluster/object
func (s *WorkerServiceImportService) DeleteWorkerServiceImportByLabel(ctx context.Context, label map[string]string, namespace string) error {
	workerServiceImports, err := s.ListWorkerServiceImport(ctx, label, namespace)
	if err != nil {
		return err
	}
	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(label["original-slice-name"])
	for _, serviceImport := range workerServiceImports {
		err = util.DeleteResource(ctx, &serviceImport)
		if err != nil {
			//Register an event for worker service import delete failure
			util.RecordEvent(ctx, eventRecorder, &serviceImport, nil, events.EventWorkerServiceImportDeletionFailed)
			return err
		}
		//Register an event for worker service import delete success
		util.RecordEvent(ctx, eventRecorder, &serviceImport, nil, events.EventWorkerServiceImportDeleted)
	}
	return nil
}

// ListWorkerServiceImport is a function to list down the serviceImport
func (s *WorkerServiceImportService) ListWorkerServiceImport(ctx context.Context, ownerLabel map[string]string,
	namespace string) ([]workerv1alpha1.WorkerServiceImport, error) {
	workerServiceImports := &workerv1alpha1.WorkerServiceImportList{}
	err := util.ListResources(ctx, workerServiceImports, client.MatchingLabels(ownerLabel), client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return workerServiceImports.Items, nil
}

// cleanUpWorkerServiceImportsForRemovedClusters is a function to remove the service import for deleted clusters
func (s *WorkerServiceImportService) cleanUpWorkerServiceImportsForRemovedClusters(ctx context.Context, label map[string]string, namespace string, clusters []string) error {
	serviceImports, err := s.ListWorkerServiceImport(ctx, label, namespace)
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
	for _, serviceImport := range serviceImports {
		clusterName := serviceImport.Labels["worker-cluster"]
		if !clusterSet[clusterName] {
			err = util.DeleteResource(ctx, &serviceImport)
			if err != nil {
				//Register an event for worker service import delete failure
				util.RecordEvent(ctx, eventRecorder, &serviceImport, nil, events.EventWorkerServiceImportDeletionFailed)
				return err
			}
			//Register an event for worker service import delete success
			util.RecordEvent(ctx, eventRecorder, &serviceImport, nil, events.EventWorkerServiceImportDeleted)
		}
	}
	return nil
}

// copySpecFromServiceExportConfigToWorkerServiceImport is a function to copy the service export configuration from controller to worker
func (s *WorkerServiceImportService) copySpecFromServiceExportConfigToWorkerServiceImport(ctx context.Context,
	serviceExportConfig []controllerv1alpha1.ServiceExportConfig) workerv1alpha1.WorkerServiceImportSpec {
	serviceExport := serviceExportConfig[0]
	spec := workerv1alpha1.WorkerServiceImportSpec{
		ServiceName:      serviceExport.Spec.ServiceName,
		ServiceNamespace: serviceExport.Spec.ServiceNamespace,
		SliceName:        serviceExport.Spec.SliceName,
		Aliases:          serviceExport.Spec.Aliases,
	}
	sc := make([]string, 0)
	sde := make([]workerv1alpha1.ServiceDiscoveryEndpoint, 0)
	sdp := make([]workerv1alpha1.ServiceDiscoveryPort, 0)
	for _, config := range serviceExportConfig {
		sc = append(sc, config.Spec.SourceCluster)
		for _, ep := range config.Spec.ServiceDiscoveryEndpoints {
			sde = append(sde, workerv1alpha1.ServiceDiscoveryEndpoint{
				PodName: ep.PodName,
				Cluster: config.Spec.SourceCluster,
				NsmIp:   ep.NsmIp,
				DnsName: ep.DnsName,
				Port:    ep.Port,
			})
		}
	}
	for _, port := range serviceExport.Spec.ServiceDiscoveryPorts {
		sdp = append(sdp, workerv1alpha1.ServiceDiscoveryPort{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: port.Protocol,
		})
	}
	spec.SourceClusters = sc
	spec.ServiceDiscoveryEndpoints = sde
	spec.ServiceDiscoveryPorts = sdp
	return spec
}

// LookupServiceExportForService Returns a list of non-deleted ServiceExport for the service configuration
func (s *WorkerServiceImportService) LookupServiceExportForService(ctx context.Context,
	serviceExportList *controllerv1alpha1.ServiceExportConfigList,
	namespace, serviceName, serviceNamespace, sliceName string) error {
	sel := make([]controllerv1alpha1.ServiceExportConfig, 0)
	labels := map[string]string{
		"service-name":        serviceName,
		"service-namespace":   serviceNamespace,
		"original-slice-name": sliceName,
	}
	err := util.ListResources(ctx, serviceExportList, client.InNamespace(namespace), client.MatchingLabels(labels))
	for _, item := range serviceExportList.Items {
		if item.DeletionTimestamp.IsZero() {
			sel = append(sel, item)
		}
	}
	serviceExportList.Items = sel
	return err
}

// ForceReconciliation is a function to update the worker service import
func (s *WorkerServiceImportService) ForceReconciliation(ctx context.Context,
	list []workerv1alpha1.WorkerServiceImport) error {
	logger := util.CtxLogger(ctx)
	for _, serviceImport := range list {
		serviceImport.UID = ""
		if serviceImport.Annotations == nil {
			serviceImport.Annotations = make(map[string]string)
		}
		serviceImport.Annotations["updatedTimestamp"] = time.Now().String()
		err := util.UpdateResource(ctx, &serviceImport)
		if err != nil {
			logger.With(zap.Error(err)).Errorf("failed to update service import %s", serviceImport.Name)
			return err
		}
	}
	return nil
}
