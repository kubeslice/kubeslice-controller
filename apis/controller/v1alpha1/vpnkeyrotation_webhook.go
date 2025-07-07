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

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	customVpnKeyRotationCreateValidation func(ctx context.Context, vpn *VpnKeyRotation) (admission.Warnings, error)
	customVpnKeyRotationDeleteValidation func(ctx context.Context, vpn *VpnKeyRotation) (admission.Warnings, error)
	eventRecorder                        events.EventRecorder
)

func (r *VpnKeyRotation) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate func(context.Context, *VpnKeyRotation) (admission.Warnings, error), validateDelete func(context.Context, *VpnKeyRotation) (admission.Warnings, error)) error {
	w := &vpnKeyRotationWebhook{Client: mgr.GetClient()}
	customVpnKeyRotationCreateValidation = validateCreate
	customVpnKeyRotationDeleteValidation = validateDelete
	eventRecorder = events.NewEventRecorder(mgr.GetClient(), mgr.GetScheme(), ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(w).
		Complete()
}

type vpnKeyRotationWebhook struct {
	client.Client
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-vpnkeyrotation,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=vpnkeyrotations,verbs=create;update;delete,versions=v1alpha1,name=vvpnkeyrotation.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &vpnKeyRotationWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *vpnKeyRotationWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "VpnKeyRotationConfigValidation", &eventRecorder)
	return customVpnKeyRotationCreateValidation(sliceConfigCtx, obj.(*VpnKeyRotation))
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *vpnKeyRotationWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *vpnKeyRotationWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "VpnKeyRotationConfigValidation", &eventRecorder)
	return customVpnKeyRotationDeleteValidation(sliceConfigCtx, obj.(*VpnKeyRotation))
}
