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

import "github.com/kubeslice/kubeslice-monitoring/pkg/events"

type Services struct {
	ProjectService                    IProjectService
	ClusterService                    IClusterService
	SliceConfigService                ISliceConfigService
	ServiceExportConfigService        IServiceExportConfigService
	WorkerSliceConfigService          IWorkerSliceConfigService
	WorkerSliceGatewayService         IWorkerSliceGatewayService
	WorkerServiceImportService        IWorkerServiceImportService
	SliceQoSConfigService             ISliceQoSConfigService
	WorkerSliceGatewayRecyclerService IWorkerSliceGatewayRecyclerService
}

// bootstrapping Services
func WithServices(
	wscs IWorkerSliceConfigService,
	ps IProjectService,
	cs IClusterService,
	scs ISliceConfigService,
	secs IServiceExportConfigService,
	wsgs IWorkerSliceGatewayService,
	wsis IWorkerServiceImportService,
	sqcs ISliceQoSConfigService,
	wsgrs IWorkerSliceGatewayRecyclerService,
) *Services {
	return &Services{
		ProjectService:                    ps,
		ClusterService:                    cs,
		SliceConfigService:                scs,
		ServiceExportConfigService:        secs,
		WorkerSliceConfigService:          wscs,
		WorkerSliceGatewayService:         wsgs,
		WorkerServiceImportService:        wsis,
		SliceQoSConfigService:             sqcs,
		WorkerSliceGatewayRecyclerService: wsgrs,
	}
}

// bootstrapping Project services
func WithProjectService(
	ns INamespaceService,
	acs IAccessControlService,
	c IClusterService,
	sc ISliceConfigService,
	se IServiceExportConfigService,
	er *events.EventRecorder,
) IProjectService {
	return &ProjectService{
		ns:            ns,
		acs:           acs,
		c:             c,
		sc:            sc,
		se:            se,
		eventRecorder: er,
	}
}

// bootstrapping cluster service
func WithClusterService(
	ns INamespaceService,
	acs IAccessControlService,
	sgws IWorkerSliceGatewayService,
	er *events.EventRecorder,
) IClusterService {
	return &ClusterService{
		ns:            ns,
		acs:           acs,
		sgws:          sgws,
		eventRecorder: er,
	}
}

// bootstrapping slice config service
func WithSliceConfigService(
	ns INamespaceService,
	acs IAccessControlService,
	sgs IWorkerSliceGatewayService,
	ms IWorkerSliceConfigService,
	si IWorkerServiceImportService,
	se IServiceExportConfigService,
	wsgrs IWorkerSliceGatewayRecyclerService,
	er *events.EventRecorder,
) ISliceConfigService {
	return &SliceConfigService{
		ns:            ns,
		acs:           acs,
		sgs:           sgs,
		ms:            ms,
		si:            si,
		se:            se,
		wsgrs:         wsgrs,
		eventRecorder: er,
	}
}

// bootstrapping service export config service
func WithServiceExportConfigService(ses IWorkerServiceImportService, er *events.EventRecorder) IServiceExportConfigService {
	return &ServiceExportConfigService{
		ses:           ses,
		eventRecorder: er,
	}
}

// bootstrapping namespace service
func WithNameSpaceService(er *events.EventRecorder) INamespaceService {
	return &NamespaceService{
		eventRecorder: er,
	}
}

// bootstrapping accesscontrol service
func WithAccessControlService(ruleProvider IAccessControlRuleProvider, er *events.EventRecorder) IAccessControlService {
	return &AccessControlService{
		ruleProvider:  ruleProvider,
		eventRecorder: er,
	}
}

// bootstrapping secret service
func WithSecretService(er *events.EventRecorder) ISecretService {
	return &SecretService{
		eventRecorder: er,
	}
}

// bootstrapping slice gateway service
func WithWorkerSliceGatewayService(
	js IJobService,
	sscs IWorkerSliceConfigService,
	sc ISecretService,
) IWorkerSliceGatewayService {
	return &WorkerSliceGatewayService{
		js:   js,
		sscs: sscs,
		sc:   sc,
	}
}

// WithWorkerSliceGatewayRecyclerService bootstraps slice gateway_recycler service
func WithWorkerSliceGatewayRecyclerService() IWorkerSliceGatewayRecyclerService {
	return &WorkerSliceGatewayRecyclerService{}
}

// bootstrapping job service
func WithJobService() IJobService {
	return &JobService{}
}

func WithAccessControlRuleProvider() IAccessControlRuleProvider {
	return &AccessControlRuleProvider{}
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
func WithSliceQoSConfigService(wsc IWorkerSliceConfigService, er *events.EventRecorder) ISliceQoSConfigService {
	return &SliceQoSConfigService{
		wsc:           wsc,
		eventRecorder: er,
	}
}
