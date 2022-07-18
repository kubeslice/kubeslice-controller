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

	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var sliceqosconfiglog = logf.Log.WithName("sliceqosconfig-resource")

type customSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) error

var customDeleteSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) error = nil
var customCreateSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) error = nil
var customUpdateSliceqosconfigValidation func(ctx context.Context, SliceQoSConfig *SliceQoSConfig) error = nil
var sliceqosconfigWebhookClient client.Client

func (r *SliceQoSConfig) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate customSliceqosconfigValidation, validateUpdate customSliceqosconfigValidation, validateDelete customSliceqosconfigValidation) error {
	sliceqosconfigWebhookClient = mgr.GetClient()
	customDeleteSliceqosconfigValidation = validateDelete
	customCreateSliceqosconfigValidation = validateCreate
	customUpdateSliceqosconfigValidation = validateUpdate
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-sliceqosconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceqosconfigs,verbs=create;update,versions=v1alpha1,name=msliceqosconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &SliceQoSConfig{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *SliceQoSConfig) Default() {
	sliceqosconfiglog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-sliceqosconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceqosconfigs,verbs=create;update;delete,versions=v1alpha1,name=vsliceqosconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &SliceQoSConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SliceQoSConfig) ValidateCreate() error {
	sliceqosconfiglog.Info("validate create", "name", r.Name)
	sliceqosConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), sliceqosconfigWebhookClient, nil, "SliceQoSConfigValidation")
	return customCreateSliceqosconfigValidation(sliceqosConfigCtx, r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SliceQoSConfig) ValidateUpdate(old runtime.Object) error {
	sliceqosconfiglog.Info("validate update", "name", r.Name)
	sliceqosConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), sliceqosconfigWebhookClient, nil, "SliceQoSConfigValidation")
	return customUpdateSliceqosconfigValidation(sliceqosConfigCtx, r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SliceQoSConfig) ValidateDelete() error {
	sliceqosconfiglog.Info("validate delete", "name", r.Name)
	sliceqosConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), sliceqosconfigWebhookClient, nil, "SliceQoSConfigValidation")
	return customDeleteSliceqosconfigValidation(sliceqosConfigCtx, r)
}
