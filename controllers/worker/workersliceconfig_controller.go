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

package worker

import (
	"context"

	"github.com/go-logr/logr"
	util "github.com/kubeslice/apis/pkg/util"
	"github.com/kubeslice/kubeslice-controller/service"

	workerv1alpha1 "github.com/kubeslice/apis/pkg/worker/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkerSliceConfigReconciler reconciles a Cluster object
type WorkerSliceConfigReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	WorkerSliceService service.IWorkerSliceConfigService
	Log                logr.Logger
}

// SetupWithManager sets up the controller with the Manager.
func (c *WorkerSliceConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workerv1alpha1.WorkerSliceConfig{}).
		Complete(c)
}

// Reconcile is a function to reconcilation of WorkerSliceconfig, WorkerSliceConfigReconciler implements it
func (c *WorkerSliceConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, c.Client, c.Scheme, "WorkerSliceConfigController")
	return c.WorkerSliceService.ReconcileWorkerSliceConfig(kubeSliceCtx, req)
}
