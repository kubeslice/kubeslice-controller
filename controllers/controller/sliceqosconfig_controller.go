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
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// SliceQoSConfigReconciler reconciles a SliceQoSConfig object
type SliceQoSConfigReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	SliceQoSConfigService service.ISliceQoSConfigService
	Log                   *zap.SugaredLogger
}

// SetupWithManager sets up the controller with the Manager.
func (r *SliceQoSConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.SliceQoSConfig{}).
		Complete(r)
}

// Reconcile is a function to reconcile the qos_profile, SliceQoSConfigReconciler implements it
func (r *SliceQoSConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, r.Client, r.Scheme, "SliceQoSConfigController")
	return r.SliceQoSConfigService.ReconcileSliceQoSConfig(kubeSliceCtx, req)
}
