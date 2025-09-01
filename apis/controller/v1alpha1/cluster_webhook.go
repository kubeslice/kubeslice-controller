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

type clusterValidation func(ctx context.Context, cluster *Cluster) (warnings admission.Warnings, err error)
type clusterUpdateValidation func(ctx context.Context, cluster *Cluster, old runtime.Object) (warnings admission.Warnings, err error)

var customClusterCreateValidation func(ctx context.Context, cluster *Cluster) (warnings admission.Warnings, err error) = nil
var customClusterUpdateValidation func(ctx context.Context, cluster *Cluster, old runtime.Object) (warnings admission.Warnings, err error) = nil
var customClusterDeleteValidation func(ctx context.Context, cluster *Cluster) (warnings admission.Warnings, err error) = nil

func (r *Cluster) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate clusterValidation, validateUpdate clusterUpdateValidation, validateDelete clusterValidation) error {
	customClusterCreateValidation = validateCreate
	customClusterUpdateValidation = validateUpdate
	customClusterDeleteValidation = validateDelete
	w := &clusterWebhook{Client: mgr.GetClient()}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

type clusterWebhook struct {
	client.Client
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=mcluster.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &clusterWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *clusterWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=clusters,verbs=create;update;delete,versions=v1alpha1,name=vcluster.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &clusterWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *clusterWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	clusterCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ClusterValidation", nil)
	return customClusterCreateValidation(clusterCtx, obj.(*Cluster))
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *clusterWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	clusterCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ClusterValidation", nil)
	return customClusterUpdateValidation(clusterCtx, newObj.(*Cluster), oldObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *clusterWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	clusterCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), r.Client, nil, "ClusterValidation", nil)

	return customClusterDeleteValidation(clusterCtx, obj.(*Cluster))
}
