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
	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type ISliceQoSConfigService interface {
	ReconcileSliceQoSConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

// SliceQoSConfigService implements different service interfaces
type SliceQoSConfigService struct {
	wsc IWorkerSliceConfigService
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
			return result, reconErr
		}
		//Register an event for slice qos config deletion
		util.RecordEvent(ctx, eventRecorder, sliceQosConfig, nil, events.EventSliceQoSConfigDeleted)
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
