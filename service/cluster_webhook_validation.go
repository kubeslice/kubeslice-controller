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

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// c is instance of cluster schema
var c *controllerv1alpha1.Cluster = nil
var clusterCtx context.Context = nil

// ValidateClusterCreate is a function to validate the creation of cluster
func ValidateClusterCreate(ctx context.Context, cluster *controllerv1alpha1.Cluster) error {
	clusterCtx = ctx
	c = cluster
	var allErrs field.ErrorList
	if err := validateAppliedInProjectNamespace(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "Cluster"}, c.Name, allErrs)
}

// ValidateClusterUpdate is a function to validate to the update of specification of cluster
func ValidateClusterUpdate(ctx context.Context, cluster *controllerv1alpha1.Cluster) error {
	clusterCtx = ctx
	c = cluster
	var allErrs field.ErrorList
	if err := validateClusterSpec(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "Cluster"}, c.Name, allErrs)
}

// ValidateClusterDelete is a function to validate the deletion of cluster
func ValidateClusterDelete(ctx context.Context, cluster *controllerv1alpha1.Cluster) error {
	clusterCtx = ctx
	c = cluster
	var allErrs field.ErrorList
	if err := validateClusterInAnySlice(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "Cluster"}, c.Name, allErrs)
}

// validateAppliedInProjectNamespace is a function to validate the if the cluster is applied in project namespace or not
func validateAppliedInProjectNamespace() *field.Error {
	actualNamespace := corev1.Namespace{}
	exist, _ := util.GetResourceIfExist(clusterCtx, client.ObjectKey{Name: c.Namespace}, &actualNamespace)
	if exist {
		if actualNamespace.Labels[util.LabelName] == "" {
			return field.Invalid(field.NewPath("metadata").Child("namespace"), c.Name, "cluster must be applied on project namespace")
		}
	}
	return nil
}

// validateClusterSpec is a function to to validate the specification of cluster
func validateClusterSpec() *field.Error {
	cluster := controllerv1alpha1.Cluster{}
	_, _ = util.GetResourceIfExist(clusterCtx, client.ObjectKey{Name: c.Name, Namespace: c.Namespace}, &cluster)
	if cluster.Spec.NetworkInterface != "" && cluster.Spec.NetworkInterface != c.Spec.NetworkInterface {
		return field.Invalid(field.NewPath("spec").Child("networkInterface"), c.Spec.NetworkInterface, "network interface can't be changed")
	}
	return nil
}

// validateClusterInAnySlice is a function to check if the cluster is in any slice
func validateClusterInAnySlice() *field.Error {
	workerSlice := &workerv1alpha1.WorkerSliceConfigList{}
	label := map[string]string{"worker-cluster": c.Name}
	err := util.ListResources(clusterCtx, workerSlice, client.MatchingLabels(label), client.InNamespace(c.Namespace))
	if err == nil {
		if len(workerSlice.Items) > 0 {
			return field.Invalid(field.NewPath("metadata").Child("name"), c.Name, "can't delete cluster which is participating in any slice")
		}
	}
	return nil
}
