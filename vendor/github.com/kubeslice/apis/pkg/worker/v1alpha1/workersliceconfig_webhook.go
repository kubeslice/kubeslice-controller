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
var workersliceconfiglog = logf.Log.WithName("workersliceconfig-resource")

type customWorkerSliceConfigValidation func(ctx context.Context, workerSliceConfig *WorkerSliceConfig) error

var customWorkerSliceConfigUpdateValidation func(ctx context.Context, workerSliceConfig *WorkerSliceConfig) error = nil
var workerSliceConfigWebhookClient client.Client

func (r *WorkerSliceConfig) SetupWebhookWithManager(mgr ctrl.Manager, validateUpdate customWorkerSliceConfigValidation) error {
	workerSliceConfigWebhookClient = mgr.GetClient()
	customWorkerSliceConfigUpdateValidation = validateUpdate
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-worker-kubeslice-io-v1alpha1-workersliceconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=worker.kubeslice.io,resources=workersliceconfigs,verbs=create;update,versions=v1alpha1,name=mworkersliceconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &WorkerSliceConfig{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *WorkerSliceConfig) Default() {
	workersliceconfiglog.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-worker-kubeslice-io-v1alpha1-workersliceconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=worker.kubeslice.io,resources=workersliceconfigs,verbs=create;update,versions=v1alpha1,name=vworkersliceconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &WorkerSliceConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *WorkerSliceConfig) ValidateCreate() error {
	workersliceconfiglog.Info("validate create", "name", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *WorkerSliceConfig) ValidateUpdate(old runtime.Object) error {
	workersliceconfiglog.Info("validate update", "name", r.Name)
	workerSliceConfigCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), workerSliceConfigWebhookClient, nil, "WorkerSliceConfigValidation")
	return customWorkerSliceConfigUpdateValidation(workerSliceConfigCtx, r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *WorkerSliceConfig) ValidateDelete() error {
	workersliceconfiglog.Info("validate delete", "name", r.Name)
	return nil
}
