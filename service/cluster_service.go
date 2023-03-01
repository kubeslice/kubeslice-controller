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
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/schema"
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
	ns            INamespaceService
	acs           IAccessControlService
	sgws          IWorkerSliceGatewayService
	eventRecorder events.EventRecorder
}

func (c *ClusterService) loadEventRecorder(ctx context.Context, project, cluster, namespace string) {
	c.eventRecorder = events.EventRecorder{
		Client:    util.CtxClient(ctx),
		Logger:    util.CtxLogger(ctx),
		Scheme:    util.CtxScheme(ctx),
		Project:   project,
		Cluster:   cluster,
		Namespace: namespace,
		Component: "controller",
	}
	return
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
	//Load Event Recorder with project name and namespace
	c.loadEventRecorder(ctx, util.GetProjectName(cluster.Namespace), cluster.Name, cluster.Namespace)
	// Step 0: check if cluster is in project namespace
	nsResource := &corev1.Namespace{}
	found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
		Name: req.Namespace,
	}, nsResource)
	if !found || !c.checkForProjectNamespace(nsResource) {
		logger.Infof("Created Cluster %v is not in project namespace. Returning from reconciliation loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	//Step 1: Finalizers
	if cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Debugf("Not deleting")
		if !util.ContainsString(cluster.GetFinalizers(), ClusterFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, cluster, ClusterFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("starting delete for cluster", req.NamespacedName)
		if shouldReturn, result, reconErr := util.IsReconciled(c.cleanUpClusterResources(ctx, req, cluster)); shouldReturn {
			return result, reconErr
		}
		if shouldReturn, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, cluster, ClusterFinalizer)); shouldReturn {
			return result, reconErr
		}
		//Register an event for cluster deletion
		c.recordEvent(ctx, cluster, schema.EventClusterDeleted)
		return ctrl.Result{}, err
	}
	//Step 2: Get ServiceAccount
	serviceAccount := &corev1.ServiceAccount{}
	_, err = util.GetResourceIfExist(ctx, types.NamespacedName{Name: fmt.Sprintf(ServiceAccountWorkerCluster, cluster.Name), Namespace: req.Namespace}, serviceAccount)
	if err != nil {
		return ctrl.Result{}, err
	}
	//Step 3: Create ServiceAccount & Reconcile
	if shouldReturn, result, reconErr := util.IsReconciled(c.acs.ReconcileWorkerClusterServiceAccountAndRoleBindings(ctx, req.Name, req.Namespace, cluster)); shouldReturn {
		return result, reconErr
	}

	if serviceAccount.Secrets == nil {
		logger.Infof("Service Account Token not populated. Requeuing")
		return ctrl.Result{Requeue: true, RequeueAfter: RequeueTime}, nil
	}
	//Step 4: Get Secret
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

	//Register an event for cluster creation
	if cluster.Generation == 1 {
		c.recordEvent(ctx, cluster, schema.EventClusterCreated)
	}

	//Step 5: Update Cluster with Secret
	cluster.Status.SecretName = secret.Name
	err = util.UpdateStatus(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}
	secret.Data["controllerEndpoint"] = []byte(ControllerEndpoint)
	secret.Data["clusterName"] = []byte(cluster.Name)
	err = util.UpdateResource(ctx, &secret)
	if err != nil {
		return ctrl.Result{}, err
	}
	// this logic is for backward compatibility- check crds for more.
	if len(cluster.Spec.NodeIPs) < 2 && len(cluster.Spec.NodeIP) != 0 {
		if len(cluster.Spec.NodeIPs) == 0 {
			cluster.Spec.NodeIPs = make([]string, 1)
		}
		cluster.Spec.NodeIPs[0] = cluster.Spec.NodeIP

		//Register an event for cluster update
		if cluster.Generation > 1 {
			c.recordEvent(ctx, cluster, schema.EventClusterUpdated)
		}

		err = util.UpdateResource(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	//Step 6: NodeIP Reconciliation to WorkerSliceGateways
	err = c.sgws.NodeIpReconciliationOfWorkerSliceGateways(ctx, cluster, req.Namespace)
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
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (c *ClusterService) recordEvent(ctx context.Context, cluster *controllerv1alpha1.Cluster, name string) {
	c.eventRecorder.RecordEvent(ctx, &events.Event{
		Object:            cluster,
		ReportingInstance: "controller",
		Name:              name,
	})
}
