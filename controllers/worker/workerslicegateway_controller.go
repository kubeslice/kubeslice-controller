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
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"go.uber.org/zap"

	"github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkerSliceGatewayReconciler reconciles a SliceConfig object
type WorkerSliceGatewayReconciler struct {
	client.Client
	Scheme                    *runtime.Scheme
	WorkerSliceGatewayService service.IWorkerSliceGatewayService
	Log                       *zap.SugaredLogger
	EventRecorder             *events.EventRecorder
}

// Reconcile is a function, WorkerSliceGatewayReconciler implements it
func (r *WorkerSliceGatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, r.Client, r.Scheme, "WorkerSliceGatewayController", r.EventRecorder)
	return r.WorkerSliceGatewayService.ReconcileWorkerSliceGateways(kubeSliceCtx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkerSliceGatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.WorkerSliceGateway{}).
		Complete(r)
}
