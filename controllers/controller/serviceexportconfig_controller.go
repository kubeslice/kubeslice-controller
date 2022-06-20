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

	"github.com/go-logr/logr"
	controllerv1alpha1 "github.com/kubeslice/apis/pkg/controller/v1alpha1"
	util "github.com/kubeslice/apis/pkg/util"
	"github.com/kubeslice/kubeslice-controller/service"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceExportConfigReconciler reconciles a ServiceExportConfig object
type ServiceExportConfigReconciler struct {
	client.Client
	Scheme                     *runtime.Scheme
	ServiceExportConfigService service.IServiceExportConfigService
	Log                        logr.Logger
}

// Reconcile is a function to reconcile the ServiceExportConfig, ServiceExportConfigReconciler implements it
func (r *ServiceExportConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, r.Client, r.Scheme, "ServiceExportConfigController")
	return r.ServiceExportConfigService.ReconcileServiceExportConfig(kubeSliceCtx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceExportConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.ServiceExportConfig{}).
		Complete(r)
}
