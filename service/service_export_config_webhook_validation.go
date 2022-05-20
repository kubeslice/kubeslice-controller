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
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateServiceExportConfigCreate is a function to validate the create process of service export config
func ValidateServiceExportConfigCreate(ctx context.Context, serviceExportConfig *controllerv1alpha1.ServiceExportConfig) error {
	var allErrs field.ErrorList
	if err := validateServiceExportConfigNamespace(ctx, serviceExportConfig); err != nil {
		allErrs = append(allErrs, err)
		return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "ServiceExportConfig"}, serviceExportConfig.Name, allErrs)
	}
	if err := validateServiceExportClusterAndSlice(ctx, serviceExportConfig); err != nil {
		allErrs = append(allErrs, err...)
	}
	if err := validateServiceEndpoint(ctx, serviceExportConfig); err != nil {
		allErrs = append(allErrs, err...)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "ServiceExportConfig"}, serviceExportConfig.Name, allErrs)
}

func validateServiceExportClusterAndSlice(ctx context.Context, serviceExport *controllerv1alpha1.ServiceExportConfig) field.ErrorList {
	var allErrs field.ErrorList = nil
	cluster := &controllerv1alpha1.Cluster{}

	clusterExist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: serviceExport.Spec.SourceCluster, Namespace: serviceExport.Namespace}, cluster)
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	sliceExist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: serviceExport.Spec.SliceName, Namespace: serviceExport.Namespace}, sliceConfig)
	if !sliceExist {
		err := field.Invalid(field.NewPath("Spec").Child("SliceName"), serviceExport.Spec.SliceName, "There is no valid slice with this name")
		allErrs = append(allErrs, err)
	}
	if !clusterExist {
		err := field.Invalid(field.NewPath("Spec").Child("SourceCluster"), serviceExport.Spec.SourceCluster, "Cluster is not registered")
		allErrs = append(allErrs, err)
	}
	if clusterExist {
		clusterPresentInSlice := false
		for _, clusterInSlice := range sliceConfig.Spec.Clusters {
			util.CtxLogger(ctx).Info("clusterInSlice,sourcecluster ", clusterInSlice, serviceExport.Spec.SourceCluster)
			if clusterInSlice == serviceExport.Spec.SourceCluster {
				util.CtxLogger(ctx).Info("clusterPresentInSlice true")
				clusterPresentInSlice = true
			}
		}
		if !clusterPresentInSlice {
			err := field.Invalid(field.NewPath("Spec").Child("Cluster"), serviceExport.Spec.SourceCluster, fmt.Sprintf("Cluster %s is not a part of the slice %s", serviceExport.Spec.SourceCluster, serviceExport.Spec.SliceName))
			allErrs = append(allErrs, err)
		}
	}
	return allErrs
}
func validateServiceEndpoint(ctx context.Context, serviceExport *controllerv1alpha1.ServiceExportConfig) field.ErrorList {
	var allErrs field.ErrorList = nil
	sliceName := serviceExport.Spec.SliceName
	for _, serviceDiscoveryEndPoint := range serviceExport.Spec.ServiceDiscoveryEndpoints {
		clusterName := serviceDiscoveryEndPoint.Cluster
		cluster := &controllerv1alpha1.Cluster{}

		clusterExist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: clusterName, Namespace: serviceExport.Namespace}, cluster)
		sliceConfig := &controllerv1alpha1.SliceConfig{}
		sliceExist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: sliceName, Namespace: serviceExport.Namespace}, sliceConfig)
		if !sliceExist {
			err := field.Invalid(field.NewPath("Spec").Child("SliceName"), serviceExport.Spec.SliceName, "There is no valid slice with this name")
			allErrs = append(allErrs, err)
		}
		if !clusterExist {
			err := field.Invalid(field.NewPath("Spec").Child("ServiceDiscoveryEndpoints").Child("Cluster"), clusterName, "Cluster is not registered")
			allErrs = append(allErrs, err)
		}
		if clusterExist {
			clusterPresentInSlice := false
			for _, clusterInSlice := range sliceConfig.Spec.Clusters {
				util.CtxLogger(ctx).Info("clusterInSlice,sourcecluster ", clusterInSlice, clusterName)
				if clusterInSlice == clusterName {
					util.CtxLogger(ctx).Info("clusterPresentInSlice true")
					clusterPresentInSlice = true
				}
			}
			if !clusterPresentInSlice {
				err := field.Invalid(field.NewPath("Spec").Child("ServiceDiscoveryEndpoints").Child("Cluster"), clusterName, fmt.Sprintf("Service Discovery Endpoint Cluster %s is not a part of the slice %s", clusterName, sliceName))
				allErrs = append(allErrs, err)
			}
		}
	}
	return allErrs
}
func ValidateServiceExportConfigUpdate(ctx context.Context, serviceExportConfig *controllerv1alpha1.ServiceExportConfig) error {
	var allErrs field.ErrorList
	if err := validateServiceExportConfigNamespace(ctx, serviceExportConfig); err != nil {
		allErrs = append(allErrs, err)
		return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "ServiceExportConfig"}, serviceExportConfig.Name, allErrs)
	}
	if err := validateServiceExportClusterAndSlice(ctx, serviceExportConfig); err != nil {
		allErrs = append(allErrs, err...)
	}
	if err := validateServiceEndpoint(ctx, serviceExportConfig); err != nil {
		allErrs = append(allErrs, err...)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "ServiceExportConfig"}, serviceExportConfig.Name, allErrs)
}
func validateServiceExportConfigNamespace(ctx context.Context, serviceExport *controllerv1alpha1.ServiceExportConfig) *field.Error {
	namespace := &corev1.Namespace{}
	exist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: serviceExport.Namespace}, namespace)
	if !exist || !checkForProjectNamespace(namespace) {
		return field.Invalid(field.NewPath("metadata").Child("namespace"), serviceExport.Namespace, "ServiceExportConfig must be applied on project namespace")
	}
	return nil
}
