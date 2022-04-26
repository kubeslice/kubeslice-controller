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
	"github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkerServiceImportReconciler reconciles a SliceConfig object
type WorkerServiceImportReconciler struct {
	client.Client
	Scheme                     *runtime.Scheme
	WorkerServiceImportService service.IWorkerServiceImportService
	Log                        logr.Logger
}

// Reconcile is a function to reconcile the workerServiceImport, WorkerServiceImportReconciler implements it
func (r *WorkerServiceImportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, r.Client, r.Scheme, "WorkerServiceImportController")
	return r.WorkerServiceImportService.ReconcileWorkerServiceImport(kubeSliceCtx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkerServiceImportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.WorkerServiceImport{}).
		Complete(r)
}
