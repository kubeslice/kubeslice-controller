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
var projectlog = logf.Log.WithName("project-resource")

type customProjectValidation func(ctx context.Context, project *Project) error

var customProjectCreateValidation func(ctx context.Context, project *Project) error = nil
var customProjectUpdateValidation func(ctx context.Context, project *Project) error = nil
var projectWebhookClient client.Client

func (r *Project) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate customProjectValidation, validateUpdate customProjectValidation) error {
	projectWebhookClient = mgr.GetClient()
	customProjectCreateValidation = validateCreate
	customProjectUpdateValidation = validateUpdate
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-project,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=projects,verbs=create;update,versions=v1alpha1,name=mproject.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Project{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Project) Default() {
	projectlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-project,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=projects,verbs=create;update,versions=v1alpha1,name=vproject.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Project{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Project) ValidateCreate() error {
	projectlog.Info("validate create", "name", r.Name)
	projectCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), projectWebhookClient, nil, "ProjectValidation")
	return customProjectCreateValidation(projectCtx, r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Project) ValidateUpdate(old runtime.Object) error {
	projectlog.Info("validate update", "name", r.Name)
	projectCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), projectWebhookClient, nil, "ProjectValidation")
	return customProjectUpdateValidation(projectCtx, r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Project) ValidateDelete() error {
	projectlog.Info("validate delete", "name", r.Name)
	return nil
}
