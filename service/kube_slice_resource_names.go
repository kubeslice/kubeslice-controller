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
	"os"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
)

// Api Groups
const (
	apiGroupKubeSliceControllers = "controller.kubeslice.io"
	apiGroupKubeSliceWorker      = "worker.kubeslice.io"
)

// ControllerNamespace Controller Namespace
const (
	ControllerNamespace = "kubeslice-controller"
)

// Resources
const (
	resourceProjects              = "projects"
	resourceCluster               = "clusters"
	resourceSliceConfig           = "sliceconfigs"
	resourceSliceQoSConfig        = "sliceqosconfigs"
	resourceWorkerSliceConfig     = "workersliceconfigs"
	resourceWorkerSliceGateways   = "workerslicegateways"
	resourceServiceExportConfigs  = "serviceexportconfigs"
	resourceWorkerServiceImport   = "workerserviceimports"
	resourceWorkerSliceGwRecycler = "workerslicegwrecyclers"
	resourceSecrets               = "secrets"
	resourceEvents                = "events"
	ResourceStatusSuffix          = "/status"
	resourceVpnKeyRotationConfigs = "vpnkeyrotations"
)

// metric kind
const (
	metricKindRole                = "role"
	metricKindServiceAccount      = "service_account"
	metricKindSecret              = "secret"
	metricKindRoleBinding         = "role_binding"
	metricKindCluster             = "cluster"
	metricKindNamespace           = "namespace"
	metricKindProject             = "project"
	metricKindServiceExportConfig = "service_export_config"
	metricKindSliceConfig         = "slice_config"
	metricKindSliceQoSConfig      = "slice_qos_config"
	metricKindWorkerServiceImport = "worker_service_import"
	metricKindWorkerSliceConfig   = "worker_slice_config"
	metricKindWorkerSliceGateway  = "worker_slice_gateway"
)

var (
	MetricPort = "18080"
)

// Verbs
const (
	verbCreate = "create"
	verbDelete = "delete"
	verbUpdate = "update"
	verbPatch  = "patch"
	verbGet    = "get"
	verbList   = "list"
	verbWatch  = "watch"
)

// Annotation Prefix
const (
	annotationKubeSliceControllers = "controller.kubeslice.io"
)

// Role Names
const (
	roleWorkerCluster   = "kubeslice-worker-cluster"
	roleSharedReadOnly  = "kubeslice-read-only"
	roleSharedReadWrite = "kubeslice-read-write"
)

// rbacResourcePrefix

var RbacResourcePrefix = "kubeslice"

// RoleBinding Names
var (
	RoleBindingWorkerCluster = "kubeslice-worker-%s"
	RoleBindingReadOnlyUser  = "kubeslice-ro-%s"
	RoleBindingReadWriteUser = "kubeslice-rw-%s"
)

// ServiceAccount Names
var (
	ServiceAccountWorkerCluster = "kubeslice-worker-%s"
	ServiceAccountReadOnlyUser  = "kubeslice-ro-%s"
	ServiceAccountReadWriteUser = "kubeslice-rw-%s"
)

// Access Types
const (
	AccessTypeAnnotationLabel  = "access-type"
	AccessTypeClusterReadWrite = "cluster-read-write"
	AccessTypeReadOnly         = "read-only"
	AccessTypeReadWrite        = "read-write"
)

// Request Timeout
const (
	RequeueTime = time.Duration(30000000000)
)

// Finalizers
const (
	ProjectFinalizer              = "controller.kubeslice.io/project-finalizer"
	ClusterFinalizer              = "controller.kubeslice.io/cluster-finalizer"
	ClusterDeregisterFinalizer    = "worker.kubeslice.io/cluster-deregister-finalizer"
	SliceConfigFinalizer          = "controller.kubeslice.io/slice-configuration-finalizer"
	serviceExportConfigFinalizer  = "controller.kubeslice.io/service-export-finalizer"
	WorkerSliceConfigFinalizer    = "worker.kubeslice.io/worker-slice-configuration-finalizer"
	WorkerSliceGatewayFinalizer   = "worker.kubeslice.io/worker-slice-gateway-finalizer"
	WorkerServiceImportFinalizer  = "worker.kubeslice.io/worker-service-import-finalizer"
	SliceQoSConfigFinalizer       = "controller.kubeslice.io/slice-qos-config-finalizer"
	VPNKeyRotationConfigFinalizer = "controller.kubeslice.io/vpn-key-rotation-config-finalizer"
)

// ControllerEndpoint
var (
	ControllerEndpoint = "https://controller.cisco.com:6443/"
)

// Project Namespace prefix. Customer can over ride this.
var (
	ProjectNamespacePrefix = "kubeslice-controller-project-"
)

const (
	serverGateway                      = "Server"
	clientGateway                      = "Client"
	workerSliceGatewayType             = "OpenVPN"
	defaultSliceGatewayServiceType     = "NodePort"
	defaultSliceGatewayServiceProtocol = "UDP"
)

var (
	// Job namespace
	jobNamespace = os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE")

	// Job Image
	JobImage          = "aveshasystems/gateway-certs-generator:latest"
	JobCredential     = ""
	JobServiceAccount = "kubeslice-controller-ovpn-manager"
)

const (
	KubesliceWorkerDeleteRequeueTime = 3
)

// StandardQoSProfileLabel name
const (
	StandardQoSProfileLabel = "standard-qos-profile"
)

type IAccessControlRuleProvider interface {
	WorkerClusterRoleRules() []rbacv1.PolicyRule
	ReadOnlyRoleRules() []rbacv1.PolicyRule
	ReadWriteRoleRules() []rbacv1.PolicyRule
}

type AccessControlRuleProvider struct {
}

func (k *AccessControlRuleProvider) WorkerClusterRoleRules() []rbacv1.PolicyRule {
	return workerClusterRoleRules
}

func (k *AccessControlRuleProvider) ReadOnlyRoleRules() []rbacv1.PolicyRule {
	return readOnlyRoleRules
}

func (k *AccessControlRuleProvider) ReadWriteRoleRules() []rbacv1.PolicyRule {
	return readWriteRoleRules
}

// Rules

var (
	workerClusterRoleRules = []rbacv1.PolicyRule{
		{
			Verbs:     []string{verbCreate, verbDelete, verbUpdate, verbPatch, verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceServiceExportConfigs},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceCluster},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch, verbCreate, verbDelete},
			APIGroups: []string{apiGroupKubeSliceWorker},
			Resources: []string{resourceWorkerSliceGwRecycler},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceVpnKeyRotationConfigs},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceWorker},
			Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceCluster + ResourceStatusSuffix, resourceVpnKeyRotationConfigs + ResourceStatusSuffix},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet},
			APIGroups: []string{apiGroupKubeSliceWorker},
			Resources: []string{resourceWorkerSliceConfig + ResourceStatusSuffix, resourceWorkerSliceGateways + ResourceStatusSuffix, resourceWorkerServiceImport + ResourceStatusSuffix, resourceWorkerSliceGwRecycler + ResourceStatusSuffix},
		},
		{
			Verbs:     []string{verbGet, verbList, verbWatch, verbCreate, verbUpdate, verbPatch},
			APIGroups: []string{""},
			Resources: []string{resourceSecrets},
		},
		{
			Verbs:     []string{verbCreate, verbPatch},
			APIGroups: []string{""},
			Resources: []string{resourceEvents},
		},
	}
)

var (
	readOnlyRoleRules = []rbacv1.PolicyRule{
		{
			Verbs:     []string{verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceCluster, resourceSliceConfig, resourceSliceQoSConfig, resourceServiceExportConfigs},
		},
		{
			Verbs:     []string{verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceWorker},
			Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
		},
		{
			Verbs:     []string{verbGet},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceCluster + ResourceStatusSuffix, resourceSliceConfig + ResourceStatusSuffix, resourceServiceExportConfigs + ResourceStatusSuffix},
		},
		{
			Verbs:     []string{verbGet},
			APIGroups: []string{apiGroupKubeSliceWorker},
			Resources: []string{resourceWorkerSliceConfig + ResourceStatusSuffix, resourceWorkerSliceGateways + ResourceStatusSuffix, resourceWorkerServiceImport + ResourceStatusSuffix},
		},
		{
			Verbs:     []string{verbGet, verbList, verbWatch},
			APIGroups: []string{""},
			Resources: []string{resourceSecrets, resourceEvents},
		},
	}
)

var (
	readWriteRoleRules = []rbacv1.PolicyRule{
		{
			Verbs:     []string{verbCreate, verbDelete, verbUpdate, verbPatch, verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceCluster, resourceSliceConfig, resourceSliceQoSConfig, resourceServiceExportConfigs},
		},
		{
			Verbs:     []string{verbGet, verbList, verbWatch},
			APIGroups: []string{apiGroupKubeSliceWorker},
			Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet},
			APIGroups: []string{apiGroupKubeSliceControllers},
			Resources: []string{resourceCluster + ResourceStatusSuffix, resourceSliceConfig + ResourceStatusSuffix, resourceServiceExportConfigs + ResourceStatusSuffix},
		},
		{
			Verbs:     []string{verbUpdate, verbPatch, verbGet},
			APIGroups: []string{apiGroupKubeSliceWorker},
			Resources: []string{resourceWorkerSliceConfig + ResourceStatusSuffix, resourceWorkerSliceGateways + ResourceStatusSuffix, resourceWorkerServiceImport + ResourceStatusSuffix},
		},
		{
			Verbs:     []string{verbGet, verbList, verbWatch},
			APIGroups: []string{""},
			Resources: []string{resourceSecrets, resourceEvents},
		},
	}
)
