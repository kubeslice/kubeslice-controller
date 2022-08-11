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
	"k8s.io/apimachinery/pkg/runtime"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateClusterCreate is a function to validate the creation of cluster
func ValidateClusterCreate(ctx context.Context, c *controllerv1alpha1.Cluster) error {
	if err := validateAppliedInProjectNamespace(ctx, c); err != nil {
		return err
	}
	if err := validateGeolocation(c); err != nil {
		return err
	}
	return nil
}

// ValidateClusterUpdate is a function to validate to the update of specification of cluster
func ValidateClusterUpdate(ctx context.Context, c *controllerv1alpha1.Cluster, old runtime.Object) error {
	//var allErrs field.ErrorList
	if err := validateClusterSpec(ctx, c, old); err != nil {
		return err
	}
	if err := validateGeolocation(c); err != nil {
		return err
	}
	//if len(allErrs) == 0 {
	return nil
	//}
	//return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "Cluster"}, c.Name, allErrs)
}

// ValidateClusterDelete is a function to validate the deletion of cluster
func ValidateClusterDelete(ctx context.Context, c *controllerv1alpha1.Cluster) error {
	var allErrs field.ErrorList
	if err := validateClusterInAnySlice(ctx, c); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "Cluster"}, c.Name, allErrs)
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

// validateClusterSpec is a function to validate the specification of cluster
func validateClusterSpec(ctx context.Context, c *controllerv1alpha1.Cluster, old runtime.Object) *field.Error {
	//cluster := controllerv1alpha1.Cluster{}
	cluster := old.(*controllerv1alpha1.Cluster)
	//_, _ = util.GetResourceIfExist(ctx, client.ObjectKey{Name: c.Name, Namespace: c.Namespace}, &cluster)
	if cluster.Spec.NetworkInterface != "" && cluster.Spec.NetworkInterface != c.Spec.NetworkInterface {
		return field.Invalid(field.NewPath("spec").Child("networkInterface"), c.Spec.NetworkInterface, "network interface can't be changed")
	}
	return nil
}

// validateClusterInAnySlice is a function to check if the cluster is in any slice
func validateClusterInAnySlice(ctx context.Context, c *controllerv1alpha1.Cluster) *field.Error {
	workerSlice := &workerv1alpha1.WorkerSliceConfigList{}
	label := map[string]string{"worker-cluster": c.Name}
	err := util.ListResources(ctx, workerSlice, client.MatchingLabels(label), client.InNamespace(c.Namespace))
	if err == nil {
		if len(workerSlice.Items) > 0 {
			return field.Invalid(field.NewPath("metadata").Child("name"), c.Name, "can't delete cluster which is participating in any slice")
		}
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
