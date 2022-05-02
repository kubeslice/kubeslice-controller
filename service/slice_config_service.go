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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ISliceConfigService interface {
	ReconcileSliceConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	DeleteSliceConfigs(ctx context.Context, namespace string) (ctrl.Result, error)
}

// SliceConfigService implements different interfaces -
type SliceConfigService struct {
	ns  INamespaceService
	acs IAccessControlService
	sgs IWorkerSliceGatewayService
	ms  IWorkerSliceConfigService
	se  IServiceExportConfigService
}

// ReconcileSliceConfig is a function to reconcile the sliceconfig
func (s SliceConfigService) ReconcileSliceConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	if duplicate, value := util.CheckDuplicateInArray(sliceConfig.Spec.Clusters); duplicate {
		logger.Infof("Duplicate cluster name %v found in sliceConfig %v", value, req.NamespacedName)
		return ctrl.Result{}, nil
	}
	//Step 1: Finalizers
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
			return result, reconErr
		}
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

	// Step 3: Creation of worker slice Objects and Cluster Labels
	clusterMap, err := s.ms.CreateMinimalWorkerSliceConfig(ctx, sliceConfig.Spec.Clusters, req.Namespace, util.GetOwnerLabel(sliceConfig), sliceConfig.Name, sliceConfig.Spec.SliceSubnet)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Step 4: Create gateways with minimum specification
	_, err = s.sgs.CreateMinimumWorkerSliceGateways(ctx, sliceConfig.Name, sliceConfig.Spec.Clusters, req.Namespace, util.GetOwnerLabel(sliceConfig), clusterMap, sliceConfig.Spec.SliceSubnet)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Infof("sliceConfig %v reconciled", req.NamespacedName)
	return ctrl.Result{}, nil
}

// checkForProjectNamespace is a function to check the namespace is in proper format
func (s *SliceConfigService) checkForProjectNamespace(namespace *corev1.Namespace) bool {
	return namespace.Labels[util.LabelName] == fmt.Sprintf(util.LabelValue, "Project", namespace.Name)
}

// cleanUpSliceConfigResources is a function to delete the slice config resources
func (s *SliceConfigService) cleanUpSliceConfigResources(ctx context.Context,
	slice *v1alpha1.SliceConfig, namespace string) (ctrl.Result, error) {
	ownershipLabel := util.GetOwnerLabel(slice)
	err := s.se.DeleteServiceExportConfigByParticipatingSliceConfig(ctx, slice.Name, namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = s.sgs.DeleteWorkerSliceGatewaysByLabel(ctx, ownershipLabel, namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = s.ms.DeleteWorkerSliceConfigByLabel(ctx, ownershipLabel, namespace)
	if err != nil {
		return ctrl.Result{}, err
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
		err = util.DeleteResource(ctx, &sliceConfig)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}
