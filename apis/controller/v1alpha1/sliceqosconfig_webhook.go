/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

type customSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) (admission.Warnings, error)

var customDeleteSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) (admission.Warnings, error) = nil
var customCreateSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) (admission.Warnings, error) = nil
var customUpdateSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) (admission.Warnings, error) = nil

func (r *SliceQoSConfig) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate customSliceqosconfigValidation, validateUpdate customSliceqosconfigValidation, validateDelete customSliceqosconfigValidation) error {
	w := &sliceQoSConfigWebhook{Client: mgr.GetClient()}
	customDeleteSliceqosconfigValidation = validateDelete
	customCreateSliceqosconfigValidation = validateCreate
	customUpdateSliceqosconfigValidation = validateUpdate
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

type sliceQoSConfigWebhook struct {
	client.Client
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-sliceqosconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceqosconfigs,verbs=create;update,versions=v1alpha1,name=msliceqosconfig.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &sliceQoSConfigWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *sliceQoSConfigWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-sliceqosconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceqosconfigs,verbs=create;update;delete,versions=v1alpha1,name=vsliceqosconfig.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &sliceQoSConfigWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *sliceQoSConfigWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sliceqosConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceQoSConfigValidation", nil)
	return customCreateSliceqosconfigValidation(sliceqosConfigCtx, obj.(*SliceQoSConfig))
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *sliceQoSConfigWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	sliceqosConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceQoSConfigValidation", nil)
	return customUpdateSliceqosconfigValidation(sliceqosConfigCtx, newObj.(*SliceQoSConfig))
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *sliceQoSConfigWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sliceqosConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceQoSConfigValidation", nil)
	return customDeleteSliceqosconfigValidation(sliceqosConfigCtx, obj.(*SliceQoSConfig))
}
