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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type IQoSProfileService interface {
	ReconcileQoSProfile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

// QoSProfileService implements different service interfaces
type QoSProfileService struct {
	wsc IWorkerSliceConfigService
}

// ReconcileQoSProfile is a function to reconcile the qos_profile
func (q *QoSProfileService) ReconcileQoSProfile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get QoSProfile resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Started Recoincilation of QoSProfile %v", req.NamespacedName)
	qosProfile := &v1alpha1.QoSProfile{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, qosProfile)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("QoS Profile %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	//Step 1: Finalizers
	if qosProfile.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(qosProfile.GetFinalizers(), QoSProfileFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, qosProfile, QoSProfileFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for qos profile", req.NamespacedName)
		if shouldReturn, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, qosProfile, QoSProfileFinalizer)); shouldReturn {
			return result, reconErr
		}
		return ctrl.Result{}, err
	}

	// Step 2: check if QoSProfile is in project namespace
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
func (q *QoSProfileService) checkForProjectNamespace(namespace *corev1.Namespace) bool {
	return namespace.Labels[util.LabelName] == fmt.Sprintf(util.LabelValue, "Project", namespace.Name)
}

// updateWorkerSliceConfigs is a function to trigger reconciliation of worker slice config whenever there is a change in qos profile
func (q *QoSProfileService) updateWorkerSliceConfigs(ctx context.Context, namespacedName types.NamespacedName) error {
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
