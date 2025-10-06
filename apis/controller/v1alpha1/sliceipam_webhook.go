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
	"fmt"
	"net"

	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type sliceIpamValidation func(ctx context.Context, sliceIpam *SliceIpam) (admission.Warnings, error)
type sliceIpamUpdateValidation func(ctx context.Context, sliceIpam *SliceIpam, old runtime.Object) (admission.Warnings, error)

var customSliceIpamCreateValidation func(ctx context.Context, sliceIpam *SliceIpam) (admission.Warnings, error) = nil
var customSliceIpamUpdateValidation func(ctx context.Context, sliceIpam *SliceIpam, old runtime.Object) (admission.Warnings, error) = nil
var customSliceIpamDeleteValidation func(ctx context.Context, sliceIpam *SliceIpam) (admission.Warnings, error) = nil

func (r *SliceIpam) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate sliceIpamValidation, validateUpdate sliceIpamUpdateValidation, validateDelete sliceIpamValidation) error {
	w := &sliceIpamWebhook{Client: mgr.GetClient()}
	customSliceIpamCreateValidation = validateCreate
	customSliceIpamUpdateValidation = validateUpdate
	customSliceIpamDeleteValidation = validateDelete
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

type sliceIpamWebhook struct {
	client.Client
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-sliceipam,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceipams,verbs=create;update,versions=v1alpha1,name=msliceipam.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &sliceIpamWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (*sliceIpamWebhook) Default(ctx context.Context, obj runtime.Object) error {
	r := obj.(*SliceIpam)

	// Set default subnet size if not specified
	if r.Spec.SubnetSize == 0 {
		r.Spec.SubnetSize = 24
	}

	return nil
}

//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-sliceipam,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceipams,verbs=create;update;delete,versions=v1alpha1,name=vsliceipam.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &sliceIpamWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *sliceIpamWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	sliceIpam := obj.(*SliceIpam)

	// Validate slice subnet CIDR format
	if err := validateCIDR(sliceIpam.Spec.SliceSubnet); err != nil {
		return nil, fmt.Errorf("invalid slice subnet CIDR: %v", err)
	}

	// Validate subnet size constraints
	if sliceIpam.Spec.SubnetSize < 16 || sliceIpam.Spec.SubnetSize > 30 {
		return nil, fmt.Errorf("subnet size must be between 16 and 30, got %d", sliceIpam.Spec.SubnetSize)
	}

	// Validate that subnet size is larger than slice subnet
	_, sliceNet, err := net.ParseCIDR(sliceIpam.Spec.SliceSubnet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse slice subnet: %v", err)
	}

	sliceSize, _ := sliceNet.Mask.Size()
	if sliceIpam.Spec.SubnetSize <= sliceSize {
		return nil, fmt.Errorf("subnet size /%d must be larger than slice subnet size /%d", sliceIpam.Spec.SubnetSize, sliceSize)
	}

	if customSliceIpamCreateValidation != nil {
		sliceIpamCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceIpamValidation", nil)
		return customSliceIpamCreateValidation(sliceIpamCtx, sliceIpam)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *sliceIpamWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldSliceIpam := oldObj.(*SliceIpam)
	newSliceIpam := newObj.(*SliceIpam)

	// Prevent changes to immutable fields
	if oldSliceIpam.Spec.SliceName != newSliceIpam.Spec.SliceName {
		return nil, fmt.Errorf("slice name cannot be changed")
	}

	if oldSliceIpam.Spec.SliceSubnet != newSliceIpam.Spec.SliceSubnet {
		return nil, fmt.Errorf("slice subnet cannot be changed")
	}

	if oldSliceIpam.Spec.SubnetSize != newSliceIpam.Spec.SubnetSize {
		return nil, fmt.Errorf("subnet size cannot be changed")
	}

	if customSliceIpamUpdateValidation != nil {
		sliceIpamCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceIpamValidation", nil)
		return customSliceIpamUpdateValidation(sliceIpamCtx, newSliceIpam, oldObj)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *sliceIpamWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	if customSliceIpamDeleteValidation != nil {
		sliceIpamCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "SliceIpamValidation", nil)
		return customSliceIpamDeleteValidation(sliceIpamCtx, obj.(*SliceIpam))
	}

	return nil, nil
}

// validateCIDR validates that the given string is a valid CIDR notation
func validateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	return err
}
