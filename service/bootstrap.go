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

type Services struct {
	ProjectService             IProjectService
	ClusterService             IClusterService
	SliceConfigService         ISliceConfigService
	ServiceExportConfigService IServiceExportConfigService
	WorkerSliceConfigService   IWorkerSliceConfigService
	WorkerSliceGatewayService  IWorkerSliceGatewayService
	WorkerServiceImportService IWorkerServiceImportService
	SliceQoSConfigService      ISliceQoSConfigService
}

// bootstrapping Services
func WithServices() *Services {
	return &Services{
		ProjectService:             WithProjectService(),
		ClusterService:             WithClusterService(),
		SliceConfigService:         WithSliceConfigService(),
		ServiceExportConfigService: WithServiceExportConfigService(),
		WorkerSliceConfigService:   WithWorkerSliceConfigService(),
		WorkerSliceGatewayService:  WithWorkerSliceGatewayService(),
		WorkerServiceImportService: WithWorkerServiceImportService(),
		SliceQoSConfigService:      WithSliceQoSConfigService(),
	}
}

// bootstrapping Project services
func WithProjectService() IProjectService {
	return &ProjectService{
		ns:  WithNameSpaceService(),
		acs: WithAccessControlService(),
		c:   WithClusterService(),
		sc:  WithSliceConfigService(),
		se:  WithServiceExportConfigService(),
	}
}

// bootstrapping cluster service
func WithClusterService() IClusterService {
	return &ClusterService{
		ns:   WithNameSpaceService(),
		acs:  WithAccessControlService(),
		sgws: WithWorkerSliceGatewayService(),
	}
}

// bootstrapping slice config service
func WithSliceConfigService() ISliceConfigService {
	return &SliceConfigService{
		ns:  WithNameSpaceService(),
		acs: WithAccessControlService(),
		sgs: WithWorkerSliceGatewayService(),
		ms:  WithWorkerSliceConfigService(),
		si:  WithWorkerServiceImportService(),
		se:  WithServiceExportConfigService(),
	}
}

// bootstrapping service export config service
func WithServiceExportConfigService() IServiceExportConfigService {
	return &ServiceExportConfigService{
		ses: WithWorkerServiceImportService(),
	}
}

// bootstrapping namespace service
func WithNameSpaceService() INamespaceService {
	return &NamespaceService{}
}

// bootstrapping accesscontrol service
func WithAccessControlService() IAccessControlService {
	return &AccessControlService{}
}

// bootstrapping secret service
func WithSecretService() ISecretService {
	return &SecretService{}
}

// bootstrapping slice gateway service
func WithWorkerSliceGatewayService() IWorkerSliceGatewayService {
	return &WorkerSliceGatewayService{
		js:   WithJobService(),
		sscs: WithWorkerSliceConfigService(),
		sc:   WithSecretService(),
	}
}

// bootstrapping job service
func WithJobService() IJobService {
	return &JobService{}
}

// bootstrapping worker slice config service
func WithWorkerSliceConfigService() IWorkerSliceConfigService {
	return &WorkerSliceConfigService{}
}

// bootstrapping worker service import service
func WithWorkerServiceImportService() IWorkerServiceImportService {
	return &WorkerServiceImportService{}
}

// bootstrapping slice qos config service
func WithSliceQoSConfigService() ISliceQoSConfigService {
	return &SliceQoSConfigService{
		wsc: WithWorkerSliceConfigService(),
	}
}
