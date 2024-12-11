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

package service

import (
	"context"
	"fmt"

	"github.com/kubeslice/kubeslice-controller/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	ctrl "sigs.k8s.io/controller-runtime"
)

type IProjectService interface {
	ReconcileProject(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

// ProjectService implements different service interfaces
type ProjectService struct {
	ns  INamespaceService
	acs IAccessControlService
	c   IClusterService
	sc  ISliceConfigService
	se  IServiceExportConfigService
	q   ISliceQoSConfigService
	mf  metrics.IMetricRecorder
}

// ReconcileProject is a function to reconcile the projects includes reconciliation of roles, clusters, project namespaces etc.
func (t *ProjectService) ReconcileProject(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get project resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Starting Recoincilation of Project with name %s in namespace %s",
		req.Name, req.Namespace)
	project := &controllerv1alpha1.Project{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, project)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("project %v not found, returning from reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	// Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(project.Name).WithNamespace(ControllerNamespace)

	// Load metrics with project name and namespace
	t.mf.WithProject(project.Name).
		WithNamespace(ControllerNamespace)

	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.GetName())
	// Finalizers
	if project.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(project.GetFinalizers(), ProjectFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, project, ProjectFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for project", req.NamespacedName)
		if shouldReturn, result, reconErr := util.IsReconciled(t.CleanUpProjectResources(ctx, projectNamespace)); shouldReturn {
			return result, reconErr
		}
		if shouldReturn, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, project, ProjectFinalizer)); shouldReturn {
			// Register an event for project deletion fail
			util.RecordEvent(ctx, eventRecorder, project, nil, events.EventProjectDeletionFailed)
			t.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventProjectDeletionFailed),
					"object_name": project.Name,
					"object_kind": metricKindProject,
				},
			)
			return result, reconErr
		}
		// Register an event for project deletion
		util.RecordEvent(ctx, eventRecorder, project, nil, events.EventProjectDeleted)
		t.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventProjectDeleted),
				"object_name": project.Name,
				"object_kind": metricKindProject,
			},
		)
		return ctrl.Result{}, nil
	}

	// Step 1: Namespace Reconciliation
	if shouldReturn, result, reconErr := util.IsReconciled(t.ns.ReconcileProjectNamespace(ctx, projectNamespace, project)); shouldReturn {
		return result, reconErr
	}

	// Step 2: Worker-Cluster Role reconciliation
	if shouldReturn, result, reconErr := util.IsReconciled(t.acs.ReconcileWorkerClusterRole(ctx, projectNamespace, project)); shouldReturn {
		return result, reconErr
	}
	// Step 3: Create shared Read-Only and Read-Write Roles for end-users
	// 3.1 Read-Only Shared Role
	if shouldReturn, result, reconErr := util.IsReconciled(t.acs.ReconcileReadOnlyRole(ctx, projectNamespace, project)); shouldReturn {
		return result, reconErr
	}

	// 3.2 Read-Write Shared Role
	if shouldReturn, result, reconErr := util.IsReconciled(t.acs.ReconcileReadWriteRole(ctx, projectNamespace, project)); shouldReturn {
		return result, reconErr
	}

	// Step 4: Reconciliation for Read-Only Users
	if shouldReturn, result, reconErr := util.IsReconciled(t.acs.ReconcileReadOnlyUserServiceAccountAndRoleBindings(ctx,
		projectNamespace, project.Spec.ServiceAccount.ReadOnly, project)); shouldReturn {
		return result, reconErr
	}

	// Step 5: Reconciliation for Read-Write Users
	if shouldReturn, result, reconErr := util.IsReconciled(t.acs.ReconcileReadWriteUserServiceAccountAndRoleBindings(ctx,
		projectNamespace, project.Spec.ServiceAccount.ReadWrite, project)); shouldReturn {
		return result, reconErr
	}

	// Step 6: adding ProjectNamespace in labels
	labels := make(map[string]string)
	labels["kubeslice-project-namespace"] = projectNamespace
	project.Labels = labels

	err = util.UpdateResource(ctx, project)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Step 7: Create default SliceQOSConfig (10 gbps ceiling, 1 gbps guaranteed)
	defaultSliceQOSConfig := &controllerv1alpha1.SliceQoSConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.DefaultSliceQOSConfigName,
			Namespace: projectNamespace,
		},
		Spec: controllerv1alpha1.SliceQoSConfigSpec{
			QueueType:               "HTB",
			Priority:                0,
			TcType:                  "BANDWIDTH_CONTROL",
			BandwidthCeilingKbps:    10000000,
			BandwidthGuaranteedKbps: 1000000,
			DscpClass:               "AF11",
		},
	}
	// check if default SliceQOSConfig exists
	found, err = util.GetResourceIfExist(
		ctx,
		types.NamespacedName{
			Name:      util.DefaultSliceQOSConfigName,
			Namespace: projectNamespace,
		},
		defaultSliceQOSConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		err = util.CreateResource(ctx, defaultSliceQOSConfig)
		if err != nil {
			return ctrl.Result{}, err
		}
		eventRecorder.WithNamespace(projectNamespace)
		logger.Infof("default SliceQOSConfig %s created", util.DefaultSliceQOSConfigName)
		util.RecordEvent(ctx, eventRecorder, defaultSliceQOSConfig, nil, events.EventDefaultSliceQoSConfigCreated)
		t.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "created",
				"event":       string(events.EventDefaultSliceQoSConfigCreated),
				"object_name": defaultSliceQOSConfig.Name,
				"object_kind": metricKindSliceQoSConfig,
			},
		)
	}

	logger.Infof("project %s reconciled", req.Name)
	return ctrl.Result{}, nil
}

func (t *ProjectService) CleanUpProjectResources(ctx context.Context, namespace string) (ctrl.Result, error) {
	if shouldReturn, result, reconErr := util.IsReconciled(t.se.DeleteServiceExportConfigs(ctx, namespace)); shouldReturn {
		return result, reconErr
	}
	if shouldReturn, result, reconErr := util.IsReconciled(t.sc.DeleteSliceConfigs(ctx, namespace)); shouldReturn {
		return result, reconErr
	}
	if shouldReturn, result, reconErr := util.IsReconciled(t.c.DeleteClusters(ctx, namespace)); shouldReturn {
		return result, reconErr
	}
	if shouldReturn, result, reconErr := util.IsReconciled(t.q.DeleteSliceQoSConfig(ctx, namespace)); shouldReturn {
		return result, reconErr
	}
	if shouldReturn, result, reconErr := util.IsReconciled(t.ns.DeleteNamespace(ctx, namespace)); shouldReturn {
		return result, reconErr
	}
	return ctrl.Result{}, nil
}
