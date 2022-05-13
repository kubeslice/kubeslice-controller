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

// ValidateWorkerSliceGatewayUpdate is function to validate the updation of gateways
func ValidateWorkerSliceGatewayUpdate(ctx context.Context, workerSliceGateway *workerv1alpha1.WorkerSliceGateway) error {
	var allErrs field.ErrorList
	if err := preventUpdateWorkerSliceGateway(ctx, workerSliceGateway); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "worker.kubeslice.io", Kind: "WorkerSliceGateway"}, workerSliceGateway.Name, allErrs)
}

// preventUpdateWorkerSliceGateway is a function to check the gatewaynumber of workerslice
func preventUpdateWorkerSliceGateway(workerSliceGatewayCtx context.Context, sg *workerv1alpha1.WorkerSliceGateway) *field.Error {
	workerSliceGateway := workerv1alpha1.WorkerSliceGateway{}
	_, _ = util.GetResourceIfExist(workerSliceGatewayCtx, client.ObjectKey{Name: sg.Name, Namespace: sg.Namespace}, &workerSliceGateway)
	if workerSliceGateway.Spec.GatewayNumber != sg.Spec.GatewayNumber {
		return field.Invalid(field.NewPath("Spec").Child("GatewayNumber"), sg.Spec.GatewayNumber, "cannot be updated")
	}
	return nil
}
