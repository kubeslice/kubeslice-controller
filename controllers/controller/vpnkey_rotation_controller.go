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

package controller

import (
	"context"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// VpnKeyRotationReconciler reconciles a VpnKeyRotation object
type VpnKeyRotationReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	VpnKeyRotationService service.IVpnKeyRotationService
	Log                   *zap.SugaredLogger
	EventRecorder         *events.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpnKeyRotationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.VpnKeyRotation{}).
		Complete(r)
}

// Reconcile is a function to reconcile the VpnKeyRotation, VpnKeyRotationReconciler implements it
func (r *VpnKeyRotationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, r.Client, r.Scheme, "VpnKeyRotationController", r.EventRecorder)
	return r.VpnKeyRotationService.ReconcileVpnKeyRotation(kubeSliceCtx, req)
}
