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
)

// Api Groups
const (
	apiGroupKubeSliceControllers = "controller.kubeslice.io"
	apiGroupKubeSliceWorker      = "worker.kubeslice.io"
)

// Resources
const (
	resourceProjects             = "projects"
	resourceCluster              = "clusters"
	resourceSliceConfig          = "sliceconfigs"
	resourceWorkerSliceConfig    = "workersliceconfigs"
	resourceWorkerSliceGateways  = "workerslicegateways"
	resourceServiceExportConfigs = "serviceexportconfigs"
	resourceWorkerServiceImport  = "workerserviceimports"
	resourceSecrets              = "secrets"
	resourceEvents               = "events"
	resourceStatusSuffix         = "/status"
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
	ProjectFinalizer             = "controller.kubeslice.io/project-finalizer"
	ClusterFinalizer             = "controller.kubeslice.io/cluster-finalizer"
	SliceConfigFinalizer         = "controller.kubeslice.io/slice-configuration-finalizer"
	serviceExportConfigFinalizer = "controller.kubeslice.io/service-export-finalizer"
	WorkerSliceConfigFinalizer   = "worker.kubeslice.io/worker-slice-configuration-finalizer"
	WorkerSliceGatewayFinalizer  = "worker.kubeslice.io/worker-slice-gateway-finalizer"
	WorkerServiceImportFinalizer = "worker.kubeslice.io/worker-service-import-finalizer"
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
	serverGateway          = "Server"
	clientGateway          = "Client"
	workerSliceGatewayType = "OpenVPN"
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
