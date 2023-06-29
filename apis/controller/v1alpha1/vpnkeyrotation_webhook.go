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
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	vpnKeyRotationLog                    = util.NewLogger().With("name", "vpnkeyrotation-resource")
	customVpnKeyRotationCreateValidation func(ctx context.Context, vpn *VpnKeyRotation) error
	customVpnKeyRotationDeleteValidation func(ctx context.Context, vpn *VpnKeyRotation) error
	vpnKeyRotationConfigWebhookClient    client.Client
)

func (r *VpnKeyRotation) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate func(context.Context, *VpnKeyRotation) error, validateDelete func(context.Context, *VpnKeyRotation) error) error {
	vpnKeyRotationConfigWebhookClient = mgr.GetClient()
	customVpnKeyRotationCreateValidation = validateCreate
	customVpnKeyRotationDeleteValidation = validateDelete
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-vpnkeyrotation,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=vpnkeyrotations,verbs=create;update;delete,versions=v1alpha1,name=vvpnkeyrotation.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &VpnKeyRotation{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *VpnKeyRotation) ValidateCreate() error {
	sliceconfigurationlog.Info("validate create", "name", r.Name)
	sliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), vpnKeyRotationConfigWebhookClient, nil, "VpnKeyRotationConfigValidation", nil)
	return customVpnKeyRotationCreateValidation(sliceConfigCtx, r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *VpnKeyRotation) ValidateUpdate(old runtime.Object) error {
	vpnKeyRotationLog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *VpnKeyRotation) ValidateDelete() error {
	vpnKeyRotationLog.Info("validate delete", "name", r.Name)

	sliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), vpnKeyRotationConfigWebhookClient, nil, "VpnKeyRotationConfigValidation", nil)
	return customVpnKeyRotationDeleteValidation(sliceConfigCtx, r)
}
