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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"k8s.io/apimachinery/pkg/runtime"

	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWorkerSliceGatewayUpdate is function to validate the update of gateways
func ValidateWorkerSliceGatewayUpdate(ctx context.Context, workerSliceGateway *workerv1alpha1.WorkerSliceGateway, old runtime.Object) (admission.Warnings, error) {
	if err := preventUpdateWorkerSliceGateway(ctx, workerSliceGateway, old); err != nil {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceWorker, Kind: "WorkerSliceGateway"}, workerSliceGateway.Name, field.ErrorList{err})
	}
	return nil, nil
}

// preventUpdateWorkerSliceGateway is a function to check the GatewayNumber of WorkerSliceGateway
func preventUpdateWorkerSliceGateway(workerSliceGatewayCtx context.Context, sg *workerv1alpha1.WorkerSliceGateway, old runtime.Object) *field.Error {
	workerSliceGateway := old.(*workerv1alpha1.WorkerSliceGateway)
	if workerSliceGateway.Spec.GatewayNumber != sg.Spec.GatewayNumber {
		return field.Invalid(field.NewPath("Spec").Child("GatewayNumber"), sg.Spec.GatewayNumber, "cannot be updated")
	}
	return nil
}
