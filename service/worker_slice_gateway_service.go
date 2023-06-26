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
	"github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"os"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"

	"github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// gatewayName format string name of gateway
const gatewayName = "%s-%s-%s"

type IWorkerSliceGatewayService interface {
	ReconcileWorkerSliceGateways(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	CreateMinimumWorkerSliceGateways(ctx context.Context, sliceName string, clusterNames []string, namespace string,
		label map[string]string, clusterMap map[string]int, sliceSubnet string, clusterCidr string) (ctrl.Result, error)
	ListWorkerSliceGateways(ctx context.Context, ownerLabel map[string]string, namespace string) ([]v1alpha1.WorkerSliceGateway, error)
	DeleteWorkerSliceGatewaysByLabel(ctx context.Context, label map[string]string, namespace string) error
	NodeIpReconciliationOfWorkerSliceGateways(ctx context.Context, cluster *controllerv1alpha1.Cluster, namespace string) error
	GenerateCerts(ctx context.Context, sliceName string, namespace string,
		serverGateway *v1alpha1.WorkerSliceGateway, clientGateway *v1alpha1.WorkerSliceGateway,
		gatewayAddresses util.WorkerSliceGatewayNetworkAddresses) error
	BuildNetworkAddresses(sliceSubnet, sourceClusterName, destinationClusterName string,
		clusterMap map[string]int, clusterCidr string) util.WorkerSliceGatewayNetworkAddresses
}

// WorkerSliceGatewayService is a schema for interfaces JobService, WorkerSliceConfigService, SecretService
type WorkerSliceGatewayService struct {
	js   IJobService
	sscs IWorkerSliceConfigService
	sc   ISecretService
	mf   metrics.IMetricRecorder
}

// WorkerSliceGatewayNetworkAddresses is a schema for WorkerSlice gateway network parameters

// ReconcileWorkerSliceGateways is a function to reconcile/restore the worker slice gateways
func (s *WorkerSliceGatewayService) ReconcileWorkerSliceGateways(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Step 0: Get WorkerSliceGateway resource
	logger := util.CtxLogger(ctx)
	logger.Infof("Started Recoincilation of WorkerSliceGateway %v", req.NamespacedName)
	workerSliceGateway := &v1alpha1.WorkerSliceGateway{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, workerSliceGateway)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("workerSliceGateway %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(req.Namespace)).
		WithNamespace(req.Namespace).
		WithSlice(workerSliceGateway.Labels["original-slice-name"])

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(req.Namespace)).
		WithNamespace(req.Namespace).
		WithSlice(workerSliceGateway.Labels["original-slice-name"])

	//Step 1: Finalizers
	if workerSliceGateway.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(workerSliceGateway.GetFinalizers(), WorkerSliceGatewayFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, workerSliceGateway, WorkerSliceGatewayFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Infof("WorkerSliceGateway %v is being deleted", req.NamespacedName)
		result := RemoveWorkerFinalizers(ctx, workerSliceGateway, WorkerSliceGatewayFinalizer)
		if result.Requeue {
			return result, nil
		}
		secret := corev1.Secret{}
		serviceAccountSecretNamespacedName := types.NamespacedName{
			Namespace: req.Namespace,
			Name:      workerSliceGateway.Name,
		}
		found, err = util.GetResourceIfExist(ctx, serviceAccountSecretNamespacedName, &secret)
		if err != nil {
			return result, err
		}
		if found {
			_, err := s.sc.DeleteSecret(ctx, req.Namespace, workerSliceGateway.Name)
			if err != nil {
				return result, err
			}
		}
		slice := &controllerv1alpha1.SliceConfig{}
		found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
			Name:      workerSliceGateway.Spec.SliceName,
			Namespace: req.Namespace,
		}, slice)
		if err != nil {
			return result, err
		}
		if found && slice.ObjectMeta.DeletionTimestamp.IsZero() {
			clusters := slice.Spec.Clusters
			if util.IsInSlice(clusters, workerSliceGateway.Labels["worker-cluster"]) {
				logger.Debug("SliceGateway deleted forcefully from slice, removing gateway pair and secret", req.NamespacedName)
				//Register an event for worker slice gateway deleted forcefully
				util.RecordEvent(ctx, eventRecorder, workerSliceGateway, slice, events.EventWorkerSliceGatewayDeletedForcefully)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deleted_forcefully",
						"event":       string(events.EventWorkerSliceGatewayDeletedForcefully),
						"object_name": workerSliceGateway.Name,
						"object_kind": metricKindWorkerSliceGateway,
					},
				)
				completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(slice), slice.GetName())
				labels := util.GetOwnerLabel(completeResourceName)
				labels["worker-cluster"] = workerSliceGateway.Labels["remote-cluster"]
				labels["remote-cluster"] = workerSliceGateway.Labels["worker-cluster"]
				pairWorkerSliceGateway := &v1alpha1.WorkerSliceGatewayList{}
				err = util.ListResources(ctx, pairWorkerSliceGateway, client.MatchingLabels(labels), client.InNamespace(req.Namespace))
				if err != nil {
					return result, err
				}
				if pairWorkerSliceGateway.Items != nil && len(pairWorkerSliceGateway.Items) > 0 {
					err = util.DeleteResource(ctx, &pairWorkerSliceGateway.Items[0])
					if err != nil {
						return result, err
					}
					_, err := s.sc.DeleteSecret(ctx, req.Namespace, pairWorkerSliceGateway.Items[0].Name)
					if err != nil {
						return result, err
					}
				}
				if slice.Annotations == nil {
					slice.Annotations = make(map[string]string)
				}
				slice.Annotations["updatedTimestamp"] = time.Now().String()
				logger.Debug("Recreating sliceGateway", req.NamespacedName)
				err = util.UpdateResource(ctx, slice)
				if err != nil {
					//Register an event for worker slice gateway recreation failure
					util.RecordEvent(ctx, eventRecorder, workerSliceGateway, slice, events.EventWorkerSliceGatewayRecreationFailed)
					s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
						map[string]string{
							"action":      "recreation_failed",
							"event":       string(events.EventWorkerSliceGatewayRecreationFailed),
							"object_name": workerSliceGateway.Name,
							"object_kind": metricKindWorkerSliceGateway,
						},
					)
					return result, err
				}
				//Register an event for worker slice gateway recreation success
				util.RecordEvent(ctx, eventRecorder, workerSliceGateway, slice, events.EventWorkerSliceGatewayRecreated)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "recreated",
						"event":       string(events.EventWorkerSliceGatewayRecreated),
						"object_name": workerSliceGateway.Name,
						"object_kind": metricKindWorkerSliceGateway,
					},
				)
			}
		}
		return result, nil
	}
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
		Name:      workerSliceGateway.Spec.SliceName,
		Namespace: req.Namespace,
	}, sliceConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("sliceConfig %v not found, returning from  reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	workerSliceGateway.Spec.GatewayType = workerSliceGatewayType
	workerSliceGateway.UID = ""
	err = util.UpdateResource(ctx, workerSliceGateway)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = s.reconcileNodeIPAndNodePort(ctx, workerSliceGateway, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, err
}

// reconcileNodeIPAndNodePort is a function to reconcile NodeIp and NodePort of remoteGateway/server cluster
func (s *WorkerSliceGatewayService) reconcileNodeIPAndNodePort(ctx context.Context, localGateway *v1alpha1.WorkerSliceGateway, namespace string) error {
	remoteGateway := v1alpha1.WorkerSliceGateway{}
	found, err := util.GetResourceIfExist(ctx, client.ObjectKey{
		Name:      localGateway.Spec.RemoteGatewayConfig.GatewayName,
		Namespace: namespace,
	}, &remoteGateway)
	if err != nil {
		return err
	}
	if found {
		if !reflect.DeepEqual(localGateway.Spec.LocalGatewayConfig.NodeIps, remoteGateway.Spec.RemoteGatewayConfig.NodeIps) ||
			!reflect.DeepEqual(localGateway.Spec.LocalGatewayConfig.NodeIp, remoteGateway.Spec.RemoteGatewayConfig.NodeIp) ||
			localGateway.Spec.LocalGatewayConfig.NodePort != remoteGateway.Spec.RemoteGatewayConfig.NodePort ||
			!reflect.DeepEqual(localGateway.Spec.LocalGatewayConfig.NodePorts, remoteGateway.Spec.RemoteGatewayConfig.NodePorts) {
			remoteGateway.Spec.RemoteGatewayConfig.NodeIp = localGateway.Spec.LocalGatewayConfig.NodeIp
			remoteGateway.Spec.RemoteGatewayConfig.NodeIps = localGateway.Spec.LocalGatewayConfig.NodeIps
			remoteGateway.Spec.RemoteGatewayConfig.NodePort = localGateway.Spec.LocalGatewayConfig.NodePort
			remoteGateway.Spec.RemoteGatewayConfig.NodePorts = localGateway.Spec.LocalGatewayConfig.NodePorts
			err = util.UpdateResource(ctx, &remoteGateway)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteWorkerSliceGatewaysByLabel is a function to delete worker slice gateway by label
func (s *WorkerSliceGatewayService) DeleteWorkerSliceGatewaysByLabel(ctx context.Context, label map[string]string, namespace string) error {
	gateways, err := s.ListWorkerSliceGateways(ctx, label, namespace)
	if err != nil {
		return err
	}
	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(label["original-slice-name"])

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(label["original-slice-name"])

	for _, gateway := range gateways {
		err = util.DeleteResource(ctx, &gateway)
		if err != nil {
			//Register an event for worker slice gateway deletion failure
			util.RecordEvent(ctx, eventRecorder, &gateway, nil, events.EventWorkerSliceGatewayDeletionFailed)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deletion_failed",
					"event":       string(events.EventWorkerSliceGatewayDeletionFailed),
					"object_name": gateway.Name,
					"object_kind": metricKindWorkerSliceGateway,
				},
			)
			return err
		}
		//Register an event for worker slice gateway deletion success
		util.RecordEvent(ctx, eventRecorder, &gateway, nil, events.EventWorkerSliceGatewayDeleted)
		s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "deleted",
				"event":       string(events.EventWorkerSliceGatewayDeleted),
				"object_name": gateway.Name,
				"object_kind": metricKindWorkerSliceGateway,
			},
		)
	}
	return nil
}

type CertPairRequestMap struct {
	SliceName string                      `json:"sliceName,omitempty"`
	Pairs     []IndividualCertPairRequest `json:"pairs,omitempty"`
}

// IndividualCertPairRequest Parameters for individual certificate pair generations.
type IndividualCertPairRequest struct {
	VpnFqdn string `json:"vpnFqdn,omitempty"`
	// The NSM server network.
	NsmServerNetwork string `json:"nsmServerNetwork,omitempty"`
	// The NSM client network.
	NsmClientNetwork string `json:"nsmClientNetwork,omitempty"`
	// The NSM mask.
	NsmMask string `json:"nsmMask,omitempty"`
	// VPN's IP address to client.
	VpnIpToClient string `json:"vpnIpToClient,omitempty"`
	// VPN's network IP.
	VpnNetwork string `json:"vpnNetwork,omitempty"`
	// VPN's IP mask.
	VpnMask string `json:"vpnMask,omitempty"`
	// The client gateway ID.
	ClientId string `json:"clientId,omitempty"`
	// The server gateway ID.
	ServerId string `json:"serverId,omitempty"`
}

// CreateMinimumWorkerSliceGateways is a function to create gateways with minimum specification
func (s *WorkerSliceGatewayService) CreateMinimumWorkerSliceGateways(ctx context.Context, sliceName string,
	clusterNames []string, namespace string, label map[string]string, clusterMap map[string]int,
	sliceSubnet string, clusterCidr string) (ctrl.Result, error) {

	err := s.cleanupObsoleteGateways(ctx, namespace, label, clusterNames, clusterMap)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(clusterNames) < 2 {
		return ctrl.Result{}, nil
	}

	_, err = s.createMinimumGatewaysIfNotExists(ctx, sliceName, clusterNames, namespace, label, clusterMap, sliceSubnet, clusterCidr)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// ListWorkerSliceGateways is a function to list down the established gateways
func (s *WorkerSliceGatewayService) ListWorkerSliceGateways(ctx context.Context, ownerLabel map[string]string,
	namespace string) ([]v1alpha1.WorkerSliceGateway, error) {
	gateways := &v1alpha1.WorkerSliceGatewayList{}
	err := util.ListResources(ctx, gateways, client.MatchingLabels(ownerLabel), client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return gateways.Items, nil
}

// cleanupObsoleteGateways is a function delete outdated gateways
func (s *WorkerSliceGatewayService) cleanupObsoleteGateways(ctx context.Context, namespace string, ownerLabel map[string]string,
	clusters []string, clusterMap map[string]int) error {

	gateways, err := s.ListWorkerSliceGateways(ctx, ownerLabel, namespace)
	if err != nil {
		return err
	}

	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(ownerLabel["original-slice-name"])

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(ownerLabel["original-slice-name"])

	clusterExistMap := make(map[string]bool, len(clusters))
	for _, cluster := range clusters {
		clusterExistMap[cluster] = true
	}

	for _, gateway := range gateways {
		clusterSource := gateway.Spec.LocalGatewayConfig.ClusterName
		clusterDestination := gateway.Spec.RemoteGatewayConfig.ClusterName
		gatewayExpectedNumber := s.calculateGatewayNumber(clusterMap[clusterSource], clusterMap[clusterDestination])
		if !clusterExistMap[clusterSource] || !clusterExistMap[clusterDestination] || gatewayExpectedNumber != gateway.Spec.GatewayNumber {
			err = util.DeleteResource(ctx, &gateway)
			if err != nil {
				//Register an event for worker slice gateway deletion failure
				util.RecordEvent(ctx, eventRecorder, &gateway, nil, events.EventWorkerSliceGatewayDeletionFailed)
				s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deletion_failed",
						"event":       string(events.EventWorkerSliceGatewayDeletionFailed),
						"object_name": gateway.Name,
						"object_kind": metricKindWorkerSliceGateway,
					},
				)
				return err
			}
			//Register an event for worker slice gateway deletion success
			util.RecordEvent(ctx, eventRecorder, &gateway, nil, events.EventWorkerSliceGatewayDeleted)
			s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deleted",
					"event":       string(events.EventWorkerSliceGatewayDeleted),
					"object_name": gateway.Name,
					"object_kind": metricKindWorkerSliceGateway,
				},
			)
		}
	}
	return nil
}

// createMinimumGatewaysIfNotExists is a helper function to create the gateways between worker clusters if not exists
func (s *WorkerSliceGatewayService) createMinimumGatewaysIfNotExists(ctx context.Context, sliceName string,
	clusterNames []string, namespace string, ownerLabel map[string]string, clusterMap map[string]int,
	sliceSubnet string, clusterCidr string) (ctrl.Result, error) {
	noClusters := len(clusterNames)
	clusterMapping := map[string]*controllerv1alpha1.Cluster{}
	for _, clusterName := range clusterNames {
		cluster := controllerv1alpha1.Cluster{}
		found, err := util.GetResourceIfExist(ctx, client.ObjectKey{Name: clusterName, Namespace: namespace}, &cluster)
		if !found || err != nil {
			return ctrl.Result{}, err
		}
		clusterMapping[clusterName] = &cluster
	}
	for i := 0; i < noClusters; i++ {
		for j := i + 1; j < noClusters; j++ {
			sourceCluster, destinationCluster := clusterMapping[clusterNames[i]], clusterMapping[clusterNames[j]]
			gatewayNumber := s.calculateGatewayNumber(clusterMap[sourceCluster.Name], clusterMap[destinationCluster.Name])
			gatewayAddresses := s.BuildNetworkAddresses(sliceSubnet, sourceCluster.Name, destinationCluster.Name, clusterMap, clusterCidr)
			err := s.createMinimumGateWayPairIfNotExists(ctx, sourceCluster, destinationCluster, sliceName, namespace, ownerLabel, gatewayNumber, gatewayAddresses)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil

}

// createMinimumGateWayPairIfNotExists is a function to create the pair of gatways between 2 clusters if not exists
func (s *WorkerSliceGatewayService) createMinimumGateWayPairIfNotExists(ctx context.Context,
	sourceCluster *controllerv1alpha1.Cluster, destinationCluster *controllerv1alpha1.Cluster, sliceName string, namespace string,
	label map[string]string, gatewayNumber int, gatewayAddresses util.WorkerSliceGatewayNetworkAddresses) error {
	serverGatewayName := fmt.Sprintf(gatewayName, sliceName, sourceCluster.Name, destinationCluster.Name)
	clientGatewayName := fmt.Sprintf(gatewayName, sliceName, destinationCluster.Name, sourceCluster.Name)
	gateway := v1alpha1.WorkerSliceGateway{}
	found, err := util.GetResourceIfExist(ctx, client.ObjectKey{Name: serverGatewayName, Namespace: namespace}, &gateway)
	if err != nil {
		return err
	}
	if found {
		found, err = util.GetResourceIfExist(ctx, client.ObjectKey{
			Name:      clientGatewayName,
			Namespace: namespace,
		}, &gateway)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
	}
	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	serverGatewayObject := s.buildMinimumGateway(sourceCluster, destinationCluster, sliceName, namespace, label, serverGateway, gatewayNumber, gatewayAddresses.ServerSubnet, gatewayAddresses.ServerVpnAddress, clientGatewayName, gatewayAddresses.ClientSubnet, gatewayAddresses.ClientVpnAddress, serverGatewayName)
	err = util.CreateResource(ctx, serverGatewayObject)
	if err != nil {
		//Register an event for worker slice gateway creation failure
		util.RecordEvent(ctx, eventRecorder, serverGatewayObject, nil, events.EventWorkerSliceGatewayCreationFailed)
		s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "creation_failed",
				"event":       string(events.EventWorkerSliceGatewayCreationFailed),
				"object_name": serverGatewayObject.Name,
				"object_kind": metricKindWorkerSliceGateway,
			},
		)
		return err
	}
	//Register an event for worker slice gateway creation success
	util.RecordEvent(ctx, eventRecorder, serverGatewayObject, nil, events.EventWorkerSliceGatewayCreated)
	s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
		map[string]string{
			"action":      "created",
			"event":       string(events.EventWorkerSliceGatewayCreated),
			"object_name": serverGatewayObject.Name,
			"object_kind": metricKindWorkerSliceGateway,
		},
	)
	clientGatewayObject := s.buildMinimumGateway(destinationCluster, sourceCluster, sliceName, namespace, label, clientGateway, gatewayNumber, gatewayAddresses.ClientSubnet, gatewayAddresses.ClientVpnAddress, serverGatewayName, gatewayAddresses.ServerSubnet, gatewayAddresses.ServerVpnAddress, clientGatewayName)
	err = util.CreateResource(ctx, clientGatewayObject)
	if err != nil {
		//Register an event for worker slice gateway creation failure
		util.RecordEvent(ctx, eventRecorder, clientGatewayObject, nil, events.EventWorkerSliceGatewayCreationFailed)
		s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "creation_failed",
				"event":       string(events.EventWorkerSliceGatewayCreationFailed),
				"object_name": clientGatewayObject.Name,
				"object_kind": metricKindWorkerSliceGateway,
			},
		)
		return err
	}
	//Register an event for worker slice gateway creation success
	util.RecordEvent(ctx, eventRecorder, clientGatewayObject, nil, events.EventWorkerSliceGatewayCreated)
	s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
		map[string]string{
			"action":      "created",
			"event":       string(events.EventWorkerSliceGatewayCreated),
			"object_name": clientGatewayObject.Name,
			"object_kind": metricKindWorkerSliceGateway,
		},
	)

	err = s.GenerateCerts(ctx, sliceName, namespace, serverGatewayObject, clientGatewayObject, gatewayAddresses)
	if err != nil {
		return err
	}

	return nil
}

// buildNetworkAddresses - function generates the object of WorkerSliceGatewayNetworkAddresses
func (s *WorkerSliceGatewayService) BuildNetworkAddresses(sliceSubnet, sourceClusterName, destinationClusterName string,
	clusterMap map[string]int, clusterCidr string) util.WorkerSliceGatewayNetworkAddresses {
	gatewayAddresses := util.WorkerSliceGatewayNetworkAddresses{}
	ipr := strings.Split(sliceSubnet, ".")
	serverSubnet := fmt.Sprintf(util.GetClusterPrefixPool(sliceSubnet, clusterMap[sourceClusterName], clusterCidr))
	clientSubnet := fmt.Sprintf(util.GetClusterPrefixPool(sliceSubnet, clusterMap[destinationClusterName], clusterCidr))
	gatewayAddresses.ServerNetwork = strings.SplitN(serverSubnet, "/", -1)[0]
	gatewayAddresses.ClientNetwork = strings.SplitN(clientSubnet, "/", -1)[0]
	gatewayAddresses.ServerSubnet = serverSubnet
	gatewayAddresses.ClientSubnet = clientSubnet
	gatewayAddresses.ServerVpnNetwork = fmt.Sprintf("%s.%s.%d.%s", ipr[0], ipr[1], 255, "0")
	gatewayAddresses.ServerVpnAddress = fmt.Sprintf("%s.%s.%d.%s", ipr[0], ipr[1], 255, "1")
	gatewayAddresses.ClientVpnAddress = fmt.Sprintf("%s.%s.%d.%s", ipr[0], ipr[1], 255, "2")
	return gatewayAddresses
}

// buildMinimumGateway function returns the gateway object
func (s *WorkerSliceGatewayService) buildMinimumGateway(sourceCluster, destinationCluster *controllerv1alpha1.Cluster,
	sliceName, namespace string, labels map[string]string, gatewayHostType string, gatewayNumber int,
	gatewaySubnet, localVpnAddress, remoteGatewayName, remoteGatewaySubnet, remoteVpnAddress, localGatewayName string) *v1alpha1.WorkerSliceGateway {
	labels["worker-cluster"] = sourceCluster.Name
	labels["remote-cluster"] = destinationCluster.Name
	labels["kubeslice-manager"] = "controller"
	labels["project-namespace"] = namespace
	labels["original-slice-name"] = sliceName
	sourceClusterNodeIPs := sourceCluster.Spec.NodeIPs
	destinationClusterNodeIPs := destinationCluster.Spec.NodeIPs

	if len(sourceClusterNodeIPs) == 0 {
		sourceClusterNodeIPs = sourceCluster.Status.NodeIPs
	}
	if len(destinationClusterNodeIPs) == 0 {
		destinationClusterNodeIPs = destinationCluster.Status.NodeIPs
	}

	return &v1alpha1.WorkerSliceGateway{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(gatewayName, sliceName, sourceCluster.Name, destinationCluster.Name),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.WorkerSliceGatewaySpec{
			SliceName: sliceName,
			LocalGatewayConfig: v1alpha1.SliceGatewayConfig{
				ClusterName:   sourceCluster.Name,
				GatewayName:   localGatewayName,
				GatewaySubnet: gatewaySubnet,
				NodeIps:       sourceClusterNodeIPs,
				NodeIp:        sourceCluster.Spec.NodeIP,
				VpnIp:         localVpnAddress,
			},
			RemoteGatewayConfig: v1alpha1.SliceGatewayConfig{
				ClusterName:   destinationCluster.Name,
				GatewayName:   remoteGatewayName,
				GatewaySubnet: remoteGatewaySubnet,
				NodeIps:       destinationClusterNodeIPs,
				NodeIp:        destinationCluster.Spec.NodeIP,
				VpnIp:         remoteVpnAddress,
			},
			GatewayCredentials: v1alpha1.GatewayCredentials{
				SecretName: fmt.Sprintf(gatewayName, sliceName, sourceCluster.Name, destinationCluster.Name),
			},
			GatewayHostType: gatewayHostType,
			GatewayNumber:   gatewayNumber,
		},
	}
}

// generateCerts is a function to generate the certificates between serverGateway and clientGateway
func (s *WorkerSliceGatewayService) GenerateCerts(ctx context.Context, sliceName string, namespace string,
	serverGateway *v1alpha1.WorkerSliceGateway, clientGateway *v1alpha1.WorkerSliceGateway,
	gatewayAddresses util.WorkerSliceGatewayNetworkAddresses) error {
	cpr := s.buildCertPairRequest(sliceName, serverGateway, clientGateway, gatewayAddresses)

	//Load Event Recorder with project name, slice name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).
		WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	environment := make(map[string]string, 5)
	environment["NAMESPACE"] = namespace
	environment["SERVER_SLICEGATEWAY_NAME"] = serverGateway.Name
	environment["CLIENT_SLICEGATEWAY_NAME"] = clientGateway.Name
	environment["SLICE_NAME"] = sliceName
	environment["CERT_GEN_REQUESTS"], _ = util.EncodeToBase64(&cpr)
	util.CtxLogger(ctx).Info("jobNamespace", jobNamespace) //todo:remove
	_, err := s.js.CreateJob(ctx, os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE"), JobImage, environment)
	if err != nil {
		//Register an event for gateway job creation failure
		util.RecordEvent(ctx, eventRecorder, serverGateway, clientGateway, events.EventSliceGatewayJobCreationFailed)
		s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "job_creation_failed",
				"event":       string(events.EventSliceGatewayJobCreationFailed),
				"object_name": serverGateway.Name,
				"object_kind": metricKindWorkerSliceGateway,
			},
		)
		return err
	}
	//Register an event for gateway job creation success
	util.RecordEvent(ctx, eventRecorder, serverGateway, clientGateway, events.EventSliceGatewayJobCreated)
	s.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
		map[string]string{
			"action":      "job_created",
			"event":       string(events.EventSliceGatewayJobCreated),
			"object_name": serverGateway.Name,
			"object_kind": metricKindWorkerSliceGateway,
		},
	)
	return nil
}

// buildCertPairRequest is a function to generate the pair between server-cluster and client-cluster
func (s *WorkerSliceGatewayService) buildCertPairRequest(sliceName string,
	gateway1, gateway2 *v1alpha1.WorkerSliceGateway,
	gatewayAddresses util.WorkerSliceGatewayNetworkAddresses) CertPairRequestMap {
	clusterName := gateway1.Spec.LocalGatewayConfig.ClusterName
	serverNumber := gateway1.Spec.GatewayNumber
	serverId, clientId := gateway1.Name, gateway2.Name
	vpnFqdn := fmt.Sprintf("%s-%s-%d.vpn.aveshasystems.com", clusterName, sliceName, serverNumber)

	cpr := IndividualCertPairRequest{
		VpnFqdn:          vpnFqdn,
		NsmServerNetwork: gatewayAddresses.ServerNetwork,
		NsmClientNetwork: gatewayAddresses.ClientNetwork,
		NsmMask:          "255.255.255.0",
		VpnIpToClient:    gatewayAddresses.ClientVpnAddress,
		VpnNetwork:       gatewayAddresses.ServerVpnNetwork,
		VpnMask:          "255.255.255.0",
		ClientId:         clientId,
		ServerId:         serverId,
	}
	finalObject := CertPairRequestMap{
		SliceName: sliceName,
		Pairs:     []IndividualCertPairRequest{cpr}}

	return finalObject
}

// calculateGatewayNumber is a function to return the gateway number
func (s *WorkerSliceGatewayService) calculateGatewayNumber(cluster1Ins int, cluster2Ins int) int {
	maximum, minimum := cluster1Ins, cluster2Ins
	if cluster2Ins > cluster1Ins {
		maximum, minimum = cluster2Ins, cluster1Ins
	}
	return ((maximum-1)*(maximum-2))/2 + minimum
}

// NodeIpReconciliationOfWorkerSliceGateways is a function to update the NodeIP of local gateway
func (s *WorkerSliceGatewayService) NodeIpReconciliationOfWorkerSliceGateways(ctx context.Context, cluster *controllerv1alpha1.Cluster, namespace string) error {
	workerSliceGateways := &v1alpha1.WorkerSliceGatewayList{}
	label := map[string]string{}
	label["worker-cluster"] = cluster.Name
	err := util.ListResources(ctx, workerSliceGateways, client.MatchingLabels(label), client.InNamespace(namespace))
	if err != nil {
		return err
	}
	for _, gateway := range workerSliceGateways.Items {
		if len(gateway.Spec.LocalGatewayConfig.NodeIps) == 0 {
			gateway.Spec.LocalGatewayConfig.NodeIps = append(gateway.Spec.LocalGatewayConfig.NodeIps, gateway.Spec.LocalGatewayConfig.NodeIp)
			err = util.UpdateResource(ctx, &gateway)
			if err != nil {
				return err
			}
		}
		nodeIPs := cluster.Spec.NodeIPs
		if len(nodeIPs) == 0 {
			nodeIPs = cluster.Status.NodeIPs
		}
		if !reflect.DeepEqual(gateway.Spec.LocalGatewayConfig.NodeIps, nodeIPs) {
			gateway.Spec.LocalGatewayConfig.NodeIps = nodeIPs
			err = util.UpdateResource(ctx, &gateway)
			if err != nil {
				return err
			}
		}
		// For backward compatibility.
		if !reflect.DeepEqual(gateway.Spec.LocalGatewayConfig.NodeIp, cluster.Spec.NodeIP) {
			gateway.Spec.LocalGatewayConfig.NodeIp = cluster.Spec.NodeIP
			err = util.UpdateResource(ctx, &gateway)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
