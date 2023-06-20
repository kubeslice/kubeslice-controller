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

	"github.com/kubeslice/kubeslice-controller/metrics"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ISliceQoSConfigService interface {
	ReconcileSliceQoSConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	DeleteSliceQoSConfig(ctx context.Context, namespace string) (ctrl.Result, error)
}

// SliceQoSConfigService implements different service interfaces
type SliceQoSConfigService struct {
	wsc IWorkerSliceConfigService
	mf  metrics.IMetricRecorder
}

// ReconcileSliceQoSConfig is a function to reconcile the qos_profile
func (q *SliceQoSConfigService) ReconcileSliceQoSConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get SliceQoSConfig resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Started Recoincilation of SliceQoSConfig %v", req.NamespacedName)
	sliceQosConfig := &v1alpha1.SliceQoSConfig{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, sliceQosConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("QoS Profile %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(sliceQosConfig.Namespace)).
		WithNamespace(sliceQosConfig.Namespace)

	// Load metrics with project name and namespace
	q.mf.WithProject(util.GetProjectName(sliceQosConfig.Namespace)).
		WithNamespace(sliceQosConfig.Namespace)

	//Step 1: Finalizers
	if sliceQosConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(sliceQosConfig.GetFinalizers(), SliceQoSConfigFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, sliceQosConfig, SliceQoSConfigFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for qos profile", req.NamespacedName)
		if shouldReturn, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, sliceQosConfig, SliceQoSConfigFinalizer)); shouldReturn {
			//Register an event for slice qos config deletion failure
			util.RecordEvent(ctx, eventRecorder, sliceQosConfig, nil, events.EventSliceQoSConfigDeletionFailed)
			q.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventSliceQoSConfigDeletionFailed),
					"object_name": sliceQosConfig.Name,
					"object_kind": metricKindSliceQoSConfig,
				},
			)
			return result, reconErr
		}
		//Register an event for slice qos config deletion
		util.RecordEvent(ctx, eventRecorder, sliceQosConfig, nil, events.EventSliceQoSConfigDeleted)
		q.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventSliceQoSConfigDeleted),
				"object_name": sliceQosConfig.Name,
				"object_kind": metricKindSliceQoSConfig,
			},
		)
		return ctrl.Result{}, err
	}

	// Step 2: check if SliceQoSConfig is in project namespace
	nsResource := &corev1.Namespace{}
	found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
		Name: req.Namespace,
	}, nsResource)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found || !q.checkForProjectNamespace(nsResource) {
		logger.Infof("Created QoS Profile %v is not in project namespace. Returning from reconciliation loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	err = q.updateWorkerSliceConfigs(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// checkForProjectNamespace is a function to check the namespace is in proper format
func (q *SliceQoSConfigService) checkForProjectNamespace(namespace *corev1.Namespace) bool {
	return namespace.Labels[util.LabelName] == fmt.Sprintf(util.LabelValue, "Project", namespace.Name)
}

// updateWorkerSliceConfigs is a function to trigger reconciliation of worker slice config whenever there is a change in qos profile
func (q *SliceQoSConfigService) updateWorkerSliceConfigs(ctx context.Context, namespacedName types.NamespacedName) error {
	logger := util.CtxLogger(ctx)
	label := map[string]string{
		StandardQoSProfileLabel: namespacedName.Name,
	}
	workerSlices, err := q.wsc.ListWorkerSliceConfigs(ctx, label, namespacedName.Namespace)
	if err != nil {
		return err
	}
	for _, workerSlice := range workerSlices {
		if workerSlice.Annotations == nil {
			workerSlice.Annotations = make(map[string]string)
		}
		workerSlice.Annotations["updatedTimestamp"] = time.Now().String()
		logger.Debug("Reconciling workerSliceConfig: ", workerSlice.Name)
		err = util.UpdateResource(ctx, &workerSlice)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteSliceQoSConfig is a function to delete the slice qos config
func (q *SliceQoSConfigService) DeleteSliceQoSConfig(ctx context.Context, namespace string) (ctrl.Result, error) {
	sliceQoSConfigs := &v1alpha1.SliceQoSConfigList{}
	err := util.ListResources(ctx, sliceQoSConfigs, client.InNamespace(namespace))
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, sliceQoSConfig := range sliceQoSConfigs.Items {
		//Load Event Recorder with project name, slice name and namespace
		eventRecorder := util.CtxEventRecorder(ctx).
			WithProject(util.GetProjectName(sliceQoSConfig.Namespace)).
			WithNamespace(sliceQoSConfig.Namespace)
		// Load metrics with project name and namespace
		q.mf.WithProject(util.GetProjectName(sliceQoSConfig.Namespace)).
			WithNamespace(sliceQoSConfig.Namespace)

		err = util.DeleteResource(ctx, &sliceQoSConfig)
		if err != nil {
			//Register an event for slice config deletion fail
			util.RecordEvent(ctx, eventRecorder, &sliceQoSConfig, nil, events.EventSliceQoSConfigDeletionFailed)
			q.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventSliceConfigDeletionFailed),
					"object_name": sliceQoSConfig.Name,
					"object_kind": metricKindSliceQoSConfig,
				},
			)
			return ctrl.Result{}, err
		}
		//Register an event for slice config deletion
		util.RecordEvent(ctx, eventRecorder, &sliceQoSConfig, nil, events.EventSliceQoSConfigDeleted)
		q.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventSliceConfigDeleted),
				"object_name": sliceQoSConfig.Name,
				"object_kind": metricKindSliceQoSConfig,
			},
		)
	}
	return ctrl.Result{}, nil
}
