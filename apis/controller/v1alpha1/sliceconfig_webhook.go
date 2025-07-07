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

type sliceConfigValidation func(ctx context.Context, sliceConfig *SliceConfig) (admission.Warnings, error)
type sliceConfigUpdateValidation func(ctx context.Context, sliceConfig *SliceConfig, old runtime.Object) (admission.Warnings, error)

var customSliceConfigCreateValidation func(ctx context.Context, sliceConfig *SliceConfig) (admission.Warnings, error) = nil
var customSliceConfigUpdateValidation func(ctx context.Context, sliceConfig *SliceConfig, old runtime.Object) (admission.Warnings, error) = nil
var customSliceConfigDeleteValidation func(ctx context.Context, sliceConfig *SliceConfig) (admission.Warnings, error) = nil

func (r *SliceConfig) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate sliceConfigValidation, validateUpdate sliceConfigUpdateValidation, validateDelete sliceConfigValidation) error {
	w := &sliceConfigWebhook{Client: mgr.GetClient()}
	customSliceConfigCreateValidation = validateCreate
	customSliceConfigUpdateValidation = validateUpdate
	customSliceConfigDeleteValidation = validateDelete
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

type sliceConfigWebhook struct {
	client.Client
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-sliceconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceconfigs,verbs=create;update,versions=v1alpha1,name=msliceconfig.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &sliceConfigWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (_ *sliceConfigWebhook) Default(ctx context.Context, obj runtime.Object) error {
	r := obj.(*SliceConfig)
	if r.Spec.OverlayNetworkDeploymentMode != NONET {
		if r.Spec.VPNConfig == nil {
			r.Spec.VPNConfig = &VPNConfiguration{
				Cipher: "AES-256-CBC",
			}
		}
	} else {
		r.Spec.VPNConfig = nil
	}
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-sliceconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceconfigs,verbs=create;update;delete,versions=v1alpha1,name=vsliceconfig.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &sliceConfigWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *sliceConfigWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	sliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceConfigValidation", nil)
	return customSliceConfigCreateValidation(sliceConfigCtx, obj.(*SliceConfig))
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *sliceConfigWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	sliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceConfigValidation", nil)
	return customSliceConfigUpdateValidation(sliceConfigCtx, newObj.(*SliceConfig), oldObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *sliceConfigWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	sliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceConfigValidation", nil)
	return customSliceConfigDeleteValidation(sliceConfigCtx, obj.(*SliceConfig))
}
