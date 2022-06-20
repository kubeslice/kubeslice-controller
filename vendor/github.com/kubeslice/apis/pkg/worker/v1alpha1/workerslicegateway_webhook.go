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
var workerslicegatewaylog = logf.Log.WithName("workerslicegateway-resource")

type customWorkerSliceGatewayValidation func(ctx context.Context, workerSliceGateway *WorkerSliceGateway) error

var customWorkerSliceGatewayUpdateValidation func(ctx context.Context, workerSliceGateway *WorkerSliceGateway) error = nil
var workerSliceGatewayWebhookClient client.Client

func (r *WorkerSliceGateway) SetupWebhookWithManager(mgr ctrl.Manager, validateUpdate customWorkerSliceGatewayValidation) error {
	workerSliceGatewayWebhookClient = mgr.GetClient()
	customWorkerSliceGatewayUpdateValidation = validateUpdate
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-worker-kubeslice-io-v1alpha1-workerslicegateway,mutating=true,failurePolicy=fail,sideEffects=None,groups=worker.kubeslice.io,resources=workerslicegateways,verbs=create;update,versions=v1alpha1,name=mworkerslicegateway.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &WorkerSliceGateway{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *WorkerSliceGateway) Default() {
	workerslicegatewaylog.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-worker-kubeslice-io-v1alpha1-workerslicegateway,mutating=false,failurePolicy=fail,sideEffects=None,groups=worker.kubeslice.io,resources=workerslicegateways,verbs=create;update,versions=v1alpha1,name=vworkerslicegateway.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &WorkerSliceGateway{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *WorkerSliceGateway) ValidateCreate() error {
	workerslicegatewaylog.Info("validate create", "name", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *WorkerSliceGateway) ValidateUpdate(old runtime.Object) error {
	workerslicegatewaylog.Info("validate update", "name", r.Name)
	workerSliceGatewayCtx := util.PrepareKubeSliceControllersRequestContext(context.Background(), workerSliceGatewayWebhookClient, nil, "WorkerSliceGatewayValidation")
	return customWorkerSliceGatewayUpdateValidation(workerSliceGatewayCtx, r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *WorkerSliceGateway) ValidateDelete() error {
	workerslicegatewaylog.Info("validate delete", "name", r.Name)
	return nil
}
