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

type customValidationServiceExport func(ctx context.Context, serviceExportConfig *ServiceExportConfig) (admission.Warnings, error)

var customCreateValidationServiceExport func(ctx context.Context, serviceExportConfig *ServiceExportConfig) (admission.Warnings, error) = nil
var customUpdateValidationServiceExport func(ctx context.Context, serviceExportConfig *ServiceExportConfig) (admission.Warnings, error) = nil
var customDeleteValidationServiceExport func(ctx context.Context, serviceExportConfig *ServiceExportConfig) (admission.Warnings, error) = nil

func (r *ServiceExportConfig) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate customValidationServiceExport, validateUpdate customValidationServiceExport, validateDelete customValidationServiceExport) error {
	w := &serviceExportConfigWebhook{Client: mgr.GetClient()}
	customCreateValidationServiceExport = validateCreate
	customUpdateValidationServiceExport = validateUpdate
	customDeleteValidationServiceExport = validateDelete
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

type serviceExportConfigWebhook struct {
	client.Client
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-serviceexportconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=serviceexportconfigs,verbs=create;update,versions=v1alpha1,name=mserviceexportconfig.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &serviceExportConfigWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *serviceExportConfigWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-serviceexportconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=serviceexportconfigs,verbs=create;update;delete,versions=v1alpha1,name=vserviceexportconfig.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &serviceExportConfigWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *serviceExportConfigWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	serviceExportConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ServiceExportConfigValidation", nil)
	return customCreateValidationServiceExport(serviceExportConfigCtx, obj.(*ServiceExportConfig))
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *serviceExportConfigWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	serviceExportConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ServiceExportConfigValidation", nil)
	return customUpdateValidationServiceExport(serviceExportConfigCtx, newObj.(*ServiceExportConfig))
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *serviceExportConfigWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	serviceExportConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ServiceExportConfigValidation", nil)
	return customDeleteValidationServiceExport(serviceExportConfigCtx, obj.(*ServiceExportConfig))
}
