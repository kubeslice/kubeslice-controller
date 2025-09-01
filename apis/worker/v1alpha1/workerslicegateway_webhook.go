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

package v1alpha1

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type customWorkerSliceGatewayValidation func(ctx context.Context, workerSliceGateway *WorkerSliceGateway, old runtime.Object) (admission.Warnings, error)

var customWorkerSliceGatewayUpdateValidation func(ctx context.Context, workerSliceGateway *WorkerSliceGateway, old runtime.Object) (admission.Warnings, error) = nil

func (r *WorkerSliceGateway) SetupWebhookWithManager(mgr ctrl.Manager, validateUpdate customWorkerSliceGatewayValidation) error {
	w := &workerSliceGatewayWebhook{Client: mgr.GetClient()}
	customWorkerSliceGatewayUpdateValidation = validateUpdate
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

type workerSliceGatewayWebhook struct {
	client.Client
}

//+kubebuilder:webhook:path=/mutate-worker-kubeslice-io-v1alpha1-workerslicegateway,mutating=true,failurePolicy=fail,sideEffects=None,groups=worker.kubeslice.io,resources=workerslicegateways,verbs=create;update,versions=v1alpha1,name=mworkerslicegateway.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &workerSliceGatewayWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *workerSliceGatewayWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-worker-kubeslice-io-v1alpha1-workerslicegateway,mutating=false,failurePolicy=fail,sideEffects=None,groups=worker.kubeslice.io,resources=workerslicegateways,verbs=create;update,versions=v1alpha1,name=vworkerslicegateway.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &workerSliceGatewayWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *workerSliceGatewayWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *workerSliceGatewayWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	workerSliceGatewayCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "WorkerSliceGatewayValidation", nil)
	return customWorkerSliceGatewayUpdateValidation(workerSliceGatewayCtx, newObj.(*WorkerSliceGateway), oldObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *workerSliceGatewayWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
