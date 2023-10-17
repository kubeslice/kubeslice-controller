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

	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWorkerSliceConfigUpdate is a function to verify the update of config of workerslice
func ValidateWorkerSliceConfigUpdate(ctx context.Context, workerSliceConfig *workerv1alpha1.WorkerSliceConfig, old runtime.Object) error {
	if err := preventUpdateWorkerSliceConfig(ctx, workerSliceConfig, old); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceWorker, Kind: "WorkerSliceConfig"}, workerSliceConfig.Name, field.ErrorList{err})
	}
	return nil
}

// preventUpdateWorkerSliceConfig is a function to prevent the update of workersliceconfig
func preventUpdateWorkerSliceConfig(ctx context.Context, ss *workerv1alpha1.WorkerSliceConfig, old runtime.Object) *field.Error {
	workerSliceConfig := old.(*workerv1alpha1.WorkerSliceConfig)
	if workerSliceConfig.Spec.Octet != nil && *workerSliceConfig.Spec.Octet != *ss.Spec.Octet {
		return field.Invalid(field.NewPath("Spec").Child("Octet"), *ss.Spec.Octet, "cannot be updated")
	}
	if workerSliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType != ss.Spec.SliceGatewayProvider.SliceGatewayType {
		return field.Forbidden(field.NewPath("Spec").Child("SliceGatewayProvider").Child("SliceGatewayServiceType"), "update not allowed")
	}
	return nil
}
