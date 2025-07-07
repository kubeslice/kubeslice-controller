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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"k8s.io/apimachinery/pkg/runtime"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateClusterCreate is a function to validate the creation of cluster
func ValidateClusterCreate(ctx context.Context, c *controllerv1alpha1.Cluster) (warnings admission.Warnings, err error) {
	if err := validateAppliedInProjectNamespace(ctx, c); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Cluster"}, c.Name, field.ErrorList{err})
	}
	if err := validateGeolocation(c); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Cluster"}, c.Name, field.ErrorList{err})
	}
	if errs := validateNodeIPs(c); len(errs) != 0 {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Cluster"}, c.Name, errs)
	}
	return nil, nil
}

// ValidateClusterUpdate is a function to validate to the update of specification of cluster
func ValidateClusterUpdate(ctx context.Context, c *controllerv1alpha1.Cluster, old runtime.Object) (warnings admission.Warnings, err error) {
	if err := validateGeolocation(c); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Cluster"}, c.Name, field.ErrorList{err})
	}
	if errs := validateNodeIPs(c); len(errs) != 0 {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Cluster"}, c.Name, errs)
	}
	return nil, nil
}

// ValidateClusterDelete is a function to validate the deletion of cluster
func ValidateClusterDelete(ctx context.Context, c *controllerv1alpha1.Cluster) (warnings admission.Warnings, err error) {
	if err := validateClusterInAnySlice(ctx, c); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Cluster"}, c.Name, field.ErrorList{err})
	}
	return nil, nil
}

// validateAppliedInProjectNamespace is a function to validate the if the cluster is applied in project namespace or not
func validateAppliedInProjectNamespace(ctx context.Context, c *controllerv1alpha1.Cluster) *field.Error {
	namespace := &corev1.Namespace{}
	exist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: c.Namespace}, namespace)
	if !exist || !util.CheckForProjectNamespace(namespace) {
		return field.Invalid(field.NewPath("metadata").Child("namespace"), c.Namespace, "cluster must be applied on project namespace")
	}
	return nil
}

// validateClusterInAnySlice is a function to check if the cluster is in any slice
func validateClusterInAnySlice(ctx context.Context, c *controllerv1alpha1.Cluster) *field.Error {
	workerSlice := &workerv1alpha1.WorkerSliceConfigList{}
	label := map[string]string{"worker-cluster": c.Name}
	err := util.ListResources(ctx, workerSlice, client.MatchingLabels(label), client.InNamespace(c.Namespace))

	workerSliceCount := len(workerSlice.Items)
	defaultWorkerSliceCount := 0
	for _, slice := range workerSlice.Items {
		projectNamespace := slice.Labels["project-namespace"]
		originalSliceName := slice.Labels["original-slice-name"]
		projectName := util.GetProjectName(projectNamespace)
		defaultSliceName := fmt.Sprintf(util.DefaultProjectSliceName, projectName)
		if defaultSliceName == originalSliceName {
			defaultWorkerSliceCount++
		}
	}
	// if all the workerslice are default workeslice, then allow cluster deletion
	if err == nil && workerSliceCount == defaultWorkerSliceCount {
		return nil
	}

	if err == nil && len(workerSlice.Items) > 0 {
		return field.Forbidden(field.NewPath("Cluster"), "The cluster cannot be deleted which is participating in slice config")
	}
	return nil
}

func validateGeolocation(c *controllerv1alpha1.Cluster) *field.Error {
	if len(c.Spec.ClusterProperty.GeoLocation.Latitude) == 0 && len(c.Spec.ClusterProperty.GeoLocation.Longitude) == 0 {
		return nil
	}
	latitude := c.Spec.ClusterProperty.GeoLocation.Latitude
	longitude := c.Spec.ClusterProperty.GeoLocation.Longitude
	err := util.ValidateCoOrdinates(latitude, longitude)
	if err == true {
		return field.Invalid(field.NewPath("spec").Child("clusterProperty.geoLocation"), util.ArrayToString([]string{latitude, longitude}), "Latitude and longitude are not valid")
	}
	return nil
}

func validateNodeIPs(c *controllerv1alpha1.Cluster) field.ErrorList {
	if len(c.Spec.NodeIPs) == 0 {
		return nil
	}
	var errors field.ErrorList
	for _, ip := range c.Spec.NodeIPs {
		ipErrs := validation.IsValidIP(field.NewPath("spec").Child("nodeIPs"), ip)
		if len(ipErrs) > 0 {
			errors = append(errors, ipErrs...)
		}
	}
	return errors
}
