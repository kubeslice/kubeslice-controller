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

	"github.com/kubeslice/apis/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var serviceexportlog = logf.Log.WithName("serviceexport-resource")

type customValidationServiceExport func(ctx context.Context, serviceExportConfig *ServiceExportConfig) error

var customCreateValidationServiceExport func(ctx context.Context, serviceExportConfig *ServiceExportConfig) error = nil
var customUpdateValidationServiceExport func(ctx context.Context, serviceExportConfig *ServiceExportConfig) error = nil
var serviceExportConfigWebhookClient client.Client

func (r *ServiceExportConfig) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate customValidationServiceExport, updateUpdate customValidationServiceExport) error {
	serviceExportConfigWebhookClient = mgr.GetClient()
	customCreateValidationServiceExport = validateCreate
	customUpdateValidationServiceExport = updateUpdate
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-serviceexportconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=serviceexportconfigs,verbs=create;update,versions=v1alpha1,name=mserviceexportconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ServiceExportConfig{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ServiceExportConfig) Default() {
	serviceexportlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-serviceexportconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=serviceexportconfigs,verbs=create;update,versions=v1alpha1,name=vserviceexportconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ServiceExportConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceExportConfig) ValidateCreate() error {
	serviceexportlog.Info("validate create", "name", r.Name)
	serviceExportConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), serviceExportConfigWebhookClient, nil, "ServiceExportConfigValidation")
	return customCreateValidationServiceExport(serviceExportConfigCtx, r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceExportConfig) ValidateUpdate(old runtime.Object) error {
	serviceexportlog.Info("validate update", "name", r.Name)
	serviceExportConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), serviceExportConfigWebhookClient, nil, "ServiceExportConfigValidation")
	return customUpdateValidationServiceExport(serviceExportConfigCtx, r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceExportConfig) ValidateDelete() error {
	serviceexportlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
