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
	"time"

	"github.com/kubeslice/kubeslice-controller/metrics"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IClusterService interface {
	ReconcileCluster(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	DeleteClusters(ctx context.Context, namespace string) (ctrl.Result, error)
}

// ClusterService struct implements different service interfaces
type ClusterService struct {
	ns   INamespaceService
	acs  IAccessControlService
	sgws IWorkerSliceGatewayService
	mf   metrics.IMetricRecorder
}

// ReconcileCluster is function to reconcile cluster
func (c *ClusterService) ReconcileCluster(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get cluster resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Starting Reconcilation of Cluster %v", req.NamespacedName)
	cluster := &controllerv1alpha1.Cluster{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("cluster %v not found, returning from reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	// Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(cluster.Namespace)).WithNamespace(cluster.Namespace)

	// Load metrics with project name and namespace
	c.mf.WithProject(util.GetProjectName(cluster.Namespace)).
		WithNamespace(cluster.Namespace)

	// Step 0: check if cluster is in project namespace
	nsResource := &corev1.Namespace{}
	found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
		Name: req.Namespace,
	}, nsResource)
	if !found || !c.checkForProjectNamespace(nsResource) {
		logger.Infof("Created Cluster %v is not in project namespace. Returning from reconciliation loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	// Step 1: Finalizers
	if cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Debugf("Not deleting")
		if !util.ContainsString(cluster.GetFinalizers(), ClusterFinalizer) {
			if shouldRequeue, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, cluster, ClusterFinalizer)); shouldRequeue {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for cluster", req.NamespacedName)
		//  Check if ClusterDeregisterFinalizer is added by worker cluster.
		if !util.ContainsString(cluster.GetFinalizers(), ClusterDeregisterFinalizer) {
			if shouldRequeue, result, reconErr := util.IsReconciled(c.cleanUpClusterResources(ctx, req, cluster)); shouldRequeue {
				return result, reconErr
			}
			if shouldRequeue, result, reconErr := util.IsReconciled(DeregisterClusterFromDefaultSlice(ctx, req, logger, req.Name)); shouldRequeue {
				return result, reconErr
			}

			if shouldRequeue, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, cluster, ClusterFinalizer)); shouldRequeue {
				// Register an event for cluster deletion fail
				util.RecordEvent(ctx, eventRecorder, cluster, nil, events.EventClusterDeletionFailed)
				c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deletion_failed",
						"event":       string(events.EventClusterDeletionFailed),
						"object_name": cluster.Name,
						"object_kind": metricKindCluster,
					},
				)
				return result, reconErr
			}
			// Register an event for cluster deletion
			util.RecordEvent(ctx, eventRecorder, cluster, nil, events.EventClusterDeleted)
			c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deleted",
					"event":       string(events.EventClusterDeleted),
					"object_name": cluster.Name,
					"object_kind": metricKindCluster,
				},
			)
			return ctrl.Result{}, err
		} else {
			// If ClusterDeregisterFinalizer is added by worker cluster, then wait for 10 mins for worker cluster to remove it.
			// If not removed even after 10 mins, remove it.
			// This is to handle the case where worker cluster is not reachable.
			// For the condtion of Deregister success, the finalizer will be removed by the worker cluster after waiting for a few seconds.
			// This is to ensure an event is registered for successful deregistration.
			// For the condition of Deregister failure, the finalizer will be removed by the worker cluster after waiting for 10 mins.
			// An event will also be registered for deregistration failure.

			// Wait until ClusterDeregisterFinalizer is removed by the worker cluster. If not removed even after 10 mins, remove it.
			if cluster.ObjectMeta.DeletionTimestamp.Add(10 * time.Minute).Before(time.Now()) {
				if shouldRequeue, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, cluster, ClusterDeregisterFinalizer)); shouldRequeue {
					// Register an event for cluster deletion fail
					util.RecordEvent(ctx, eventRecorder, cluster, nil, events.EventClusterDeletionFailed)
					c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
						map[string]string{
							"action":      "deletion_failed",
							"event":       string(events.EventClusterDeletionFailed),
							"object_name": cluster.Name,
							"object_kind": metricKindCluster,
						},
					)
					return result, reconErr
				}
				logger.Info("Timed out waiting for worker-operator chart uninstallation")
				// Event for worker-operator chart uninstallation timeout [ClusterDeregisterTimeout]
				util.RecordEvent(ctx, eventRecorder, cluster, nil, events.EventClusterDeregisterTimeout)
				c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deregister_timeout",
						"event":       string(events.EventClusterDeregisterTimeout),
						"object_name": cluster.Name,
						"object_kind": metricKindCluster,
					},
				)
				return ctrl.Result{Requeue: true}, err
			} else {
				if cluster.Status.RegistrationStatus == v1alpha1.RegistrationStatusDeregisterFailed {
					logger.Info("Worker-operator charts failed to uninstall")
					// Event for worker-operator chart uninstallation failure [ClusterDeregisterFailed]
					util.RecordEvent(ctx, eventRecorder, cluster, nil, events.EventClusterDeregisterFailed)
					c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
						map[string]string{
							"action":      "deregister_failed",
							"event":       string(events.EventClusterDeregisterFailed),
							"object_name": cluster.Name,
							"object_kind": metricKindCluster,
						},
					)
					return ctrl.Result{}, nil
				} else if cluster.Status.RegistrationStatus == v1alpha1.RegistrationStatusDeregistered {
					logger.Info("Worker-operator charts uninstalled successfully")
					// Event for worker-operator chart uninstallation success [ClusterDeregistered]
					util.RecordEvent(ctx, eventRecorder, cluster, nil, events.EventClusterDeregistered)
					c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
						map[string]string{
							"action":      "deregistered",
							"event":       string(events.EventClusterDeregistered),
							"object_name": cluster.Name,
							"object_kind": metricKindCluster,
						},
					)
					return ctrl.Result{}, nil
				} else if !cluster.Status.IsDeregisterInProgress {
					logger.Info("Waiting for worker-operator charts to uninstall")
					// setting IsDeregisterInProgress to true to avoid requeuing
					cluster.Status.IsDeregisterInProgress = true
					util.UpdateStatus(ctx, cluster)
					// Event for worker-operator chart uninstallation in progress [ClusterDeregistrationInProgress]
					util.RecordEvent(ctx, eventRecorder, cluster, nil, events.EventClusterDeregistrationInProgress)
					c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
						map[string]string{
							"action":      "deregister_in_progress",
							"event":       string(events.EventClusterDeregistrationInProgress),
							"object_name": cluster.Name,
							"object_kind": metricKindCluster,
						},
					)
					// requeuing after ~10 mins
					return ctrl.Result{RequeueAfter: 610 * time.Second}, nil
				} else {
					return ctrl.Result{}, nil
				}
			}
		}
	}
	// Step 2: Get ServiceAccount
	serviceAccount := &corev1.ServiceAccount{}
	_, err = util.GetResourceIfExist(ctx, types.NamespacedName{Name: fmt.Sprintf(ServiceAccountWorkerCluster, cluster.Name), Namespace: req.Namespace}, serviceAccount)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Step 3: Create ServiceAccount & Reconcile
	if shouldReturn, result, reconErr := util.IsReconciled(c.acs.ReconcileWorkerClusterServiceAccountAndRoleBindings(ctx, req.Name, req.Namespace, cluster)); shouldReturn {
		return result, reconErr
	}

	if serviceAccount.Secrets == nil {
		logger.Infof("Service Account Token not populated. Requeuing")
		return ctrl.Result{Requeue: true, RequeueAfter: RequeueTime}, nil
	}
	// Step 4: Get Secret
	secret := corev1.Secret{}
	serviceAccountSecretNamespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      serviceAccount.Secrets[0].Name,
	}
	found, err = util.GetResourceIfExist(ctx, serviceAccountSecretNamespacedName, &secret)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		err = fmt.Errorf("could not find secret")
		logger.Errorf(err.Error())
		return ctrl.Result{}, err
	}

	// Step 5: Update Cluster with Secret
	cluster.Status.SecretName = secret.Name
	err = util.UpdateStatus(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data["controllerEndpoint"] = []byte(ControllerEndpoint)
	secret.Data["clusterName"] = []byte(cluster.Name)
	err = util.UpdateResource(ctx, &secret)
	if err != nil {
		return ctrl.Result{}, err
	}

	// This logic is to set NodeIPs to nil, if an empty string is set in the first index.
	if len(cluster.Spec.NodeIPs) > 0 && cluster.Spec.NodeIPs[0] == "" {
		cluster.Spec.NodeIPs = nil
		err = util.UpdateResource(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// this logic is for backward compatibility- check crds for more.
	if len(cluster.Spec.NodeIPs) < 2 && len(cluster.Spec.NodeIP) != 0 {
		if len(cluster.Spec.NodeIPs) == 0 {
			cluster.Spec.NodeIPs = make([]string, 1)
		}
		cluster.Spec.NodeIPs[0] = cluster.Spec.NodeIP

		err = util.UpdateResource(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Step 6: NodeIP Reconciliation to WorkerSliceGateways
	// Should be only done if Network componets are present
	if cluster.Status.NetworkPresent {
		err = c.sgws.NodeIpReconciliationOfWorkerSliceGateways(ctx, cluster, req.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if shouldReturn, result, reconErr := util.IsReconciled(DefaultSliceOperations(ctx,
		req, logger, cluster)); shouldReturn {
		return result, reconErr
	}

	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Infof("cluster %v reconciled", req.NamespacedName)
	return ctrl.Result{}, nil
}

// cleanUpClusterResources is function to clean/remove resources- servie account and role binding of clusters
func (c *ClusterService) cleanUpClusterResources(ctx context.Context, req ctrl.Request, owner client.Object) (ctrl.Result, error) {
	if shouldReturn, result, reconErr := util.IsReconciled(c.acs.RemoveWorkerClusterServiceAccountAndRoleBindings(ctx,
		req.Name, req.Namespace, owner)); shouldReturn {
		return result, reconErr
	}
	return ctrl.Result{}, nil
}

// checkForProjectNamespace is function to check the project if the namespace is in proper format
func (c *ClusterService) checkForProjectNamespace(namespace *corev1.Namespace) bool {
	return namespace.Labels[util.LabelName] == fmt.Sprintf(util.LabelValue, "Project", namespace.Name)
}

// DeleteClusters is function to delete the clusters
func (c *ClusterService) DeleteClusters(ctx context.Context, namespace string) (ctrl.Result, error) {
	clusters := &controllerv1alpha1.ClusterList{}
	err := util.ListResources(ctx, clusters, client.InNamespace(namespace))
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, cluster := range clusters.Items {
		err = util.DeleteResource(ctx, &cluster)
		// Load Event Recorder with project name and namespace
		eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(cluster.Namespace)).WithNamespace(cluster.Namespace)

		// Load metrics with project name and namespace
		c.mf.WithProject(util.GetProjectName(cluster.Namespace)).
			WithNamespace(cluster.Namespace)

		if err != nil {
			// Register an event for cluster deletion fail
			util.RecordEvent(ctx, eventRecorder, &cluster, nil, events.EventClusterDeletionFailed)
			c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventClusterDeletionFailed),
					"object_name": cluster.Name,
					"object_kind": metricKindCluster,
				},
			)
			return ctrl.Result{}, err
		}
		// Register an event for cluster deletion
		util.RecordEvent(ctx, eventRecorder, &cluster, nil, events.EventClusterDeleted)
		c.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventClusterDeleted),
				"object_name": cluster.Name,
				"object_kind": metricKindCluster,
			},
		)
	}
	return ctrl.Result{}, nil
}
