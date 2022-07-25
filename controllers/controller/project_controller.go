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
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProjectReconciler reconciles a Project object
type ProjectReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	ProjectService         service.IProjectService
	Log                    logr.Logger
	WorkerClusterRoleRules []rbacv1.PolicyRule
	ReadOnlyRoleRules      []rbacv1.PolicyRule
	ReadWriteRoleRules     []rbacv1.PolicyRule
}

// SetupWithManager sets up the controller with the Manager.
func (t *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.Project{}).
		Complete(t)
}

// Reconcile is a function to reconcile the project, ProjectReconciler implements it
func (t *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, t.Client, t.Scheme, "ProjectController")
	return t.ProjectService.ReconcileProject(kubeSliceCtx, req, t.WorkerClusterRoleRules, t.ReadOnlyRoleRules, t.ReadWriteRoleRules)
}
