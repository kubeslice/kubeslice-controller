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

package controller

import (
	"context"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"go.uber.org/zap"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// SliceConfigReconciler reconciles a SliceConfig object
type SliceConfigReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	SliceConfigService service.ISliceConfigService
	Log                *zap.SugaredLogger
	EventRecorder      *events.EventRecorder
}

// Reconcile is a function to reconcile the slice config, SliceConfigReconciler implements it
func (r *SliceConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, r.Client, r.Scheme, "SliceConfigController", r.EventRecorder)
	return r.SliceConfigService.ReconcileSliceConfig(kubeSliceCtx, req)
}

// sliceConfigForGateway maps a WorkerSliceGateway to a reconcile request for the
// SliceConfig that owns it, so a change in a gateway's connectivity status
// re-triggers aggregation of the slice's TopologyConverged condition. The
// gateway carries its slice name in spec.SliceName and lives in the same
// (project) namespace as the SliceConfig.
func (r *SliceConfigReconciler) sliceConfigForGateway(ctx context.Context, obj client.Object) []ctrl.Request {
	gateway, ok := obj.(*workerv1alpha1.WorkerSliceGateway)
	if !ok || gateway.Spec.SliceName == "" {
		return nil
	}
	return []ctrl.Request{
		{NamespacedName: types.NamespacedName{Name: gateway.Spec.SliceName, Namespace: gateway.Namespace}},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SliceConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.SliceConfig{}).
		Watches(&workerv1alpha1.WorkerSliceGateway{}, handler.EnqueueRequestsFromMapFunc(r.sliceConfigForGateway)).
		Complete(r)
}
