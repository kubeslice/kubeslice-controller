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
	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type customProjectValidation func(ctx context.Context, project *Project) (admission.Warnings, error)

var customProjectCreateValidation func(ctx context.Context, project *Project) (admission.Warnings, error) = nil
var customProjectUpdateValidation func(ctx context.Context, project *Project) (admission.Warnings, error) = nil
var customProjectDeleteValidation func(ctx context.Context, project *Project) (admission.Warnings, error) = nil

func (r *Project) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate customProjectValidation, validateUpdate customProjectValidation, validateDelete customProjectValidation) error {
	w := &projectWebhook{Client: mgr.GetClient()}
	customProjectCreateValidation = validateCreate
	customProjectUpdateValidation = validateUpdate
	customProjectDeleteValidation = validateDelete
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

type projectWebhook struct {
	client.Client
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-project,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=projects,verbs=create;update,versions=v1alpha1,name=mproject.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &projectWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *projectWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-project,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=projects,verbs=create;update;delete,versions=v1alpha1,name=vproject.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &projectWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *projectWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	projectCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ProjectValidation", nil)
	return customProjectCreateValidation(projectCtx, obj.(*Project))
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *projectWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	projectCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ProjectValidation", nil)
	return customProjectUpdateValidation(projectCtx, newObj.(*Project))
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *projectWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	projectCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ProjectValidation", nil)
	return customProjectDeleteValidation(projectCtx, obj.(*Project))
}
