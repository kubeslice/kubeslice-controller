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
var clusterlog = logf.Log.WithName("cluster-resource")

type customClusterValidation func(ctx context.Context, cluster *Cluster) error

var customClusterCreateValidation func(ctx context.Context, cluster *Cluster) error = nil
var customClusterUpdateValidation func(ctx context.Context, cluster *Cluster) error = nil
var customClusterDeleteValidation func(ctx context.Context, cluster *Cluster) error = nil
var clusterWebhookClient client.Client

func (r *Cluster) SetupWebhookWithManager(mgr ctrl.Manager, validateCreate customClusterValidation, validateUpdate customClusterValidation, validateDelete customClusterValidation) error {
	customClusterCreateValidation = validateCreate
	customClusterUpdateValidation = validateUpdate
	customClusterDeleteValidation = validateDelete
	clusterWebhookClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-controller-kubeslice-io-v1alpha1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=mcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Cluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Cluster) Default() {
	clusterlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=clusters,verbs=create;update;delete,versions=v1alpha1,name=vcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Cluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateCreate() error {
	projectlog.Info("validate create", "name", r.Name)
	clusterCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clusterWebhookClient, nil, "ClusterValidation")

	return customClusterCreateValidation(clusterCtx, r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateUpdate(old runtime.Object) error {
	projectlog.Info("validate update", "name", r.Name)
	clusterCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clusterWebhookClient, nil, "ClusterValidation")

	return customClusterUpdateValidation(clusterCtx, r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", r.Name)
	clusterCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clusterWebhookClient, nil, "ClusterValidation")

	return customClusterDeleteValidation(clusterCtx, r)
}
