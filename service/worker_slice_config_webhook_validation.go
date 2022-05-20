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

	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateWorkerSliceConfigUpdate is a function to verify the update of config of workerslice
func ValidateWorkerSliceConfigUpdate(ctx context.Context, workerSliceConfig *workerv1alpha1.WorkerSliceConfig) error {
	var allErrs field.ErrorList
	if err := preventUpdateWorkerSliceConfig(ctx, workerSliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "worker.kubeslice.io", Kind: "WorkerSliceConfig"}, workerSliceConfig.Name, allErrs)
}

// preventUpdateWorkerSliceConfig is a function to prevent the update of workersliceconfig
func preventUpdateWorkerSliceConfig(ctx context.Context, ss *workerv1alpha1.WorkerSliceConfig) *field.Error {
	workerSliceConfig := workerv1alpha1.WorkerSliceConfig{}
	_, _ = util.GetResourceIfExist(ctx, client.ObjectKey{Name: ss.Name, Namespace: ss.Namespace}, &workerSliceConfig)
	if workerSliceConfig.Spec.IpamClusterOctet != ss.Spec.IpamClusterOctet {
		return field.Invalid(field.NewPath("Spec").Child("IpamClusterOctet"), ss.Spec.IpamClusterOctet, "cannot be updated")
	}
	return nil
}
