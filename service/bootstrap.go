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

import "github.com/kubeslice/kubeslice-controller/metrics"

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
	VpnKeyRotationService             IVpnKeyRotationService
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
	vpn IVpnKeyRotationService,
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
		VpnKeyRotationService:             vpn,
	}
}

// bootstrapping Project services
func WithProjectService(
	ns INamespaceService,
	acs IAccessControlService,
	c IClusterService,
	sc ISliceConfigService,
	se IServiceExportConfigService,
	q ISliceQoSConfigService,
	mf metrics.IMetricRecorder,
) IProjectService {
	return &ProjectService{
		ns:  ns,
		acs: acs,
		c:   c,
		sc:  sc,
		se:  se,
		q:   q,
		mf:  mf,
	}
}

// bootstrapping cluster service
func WithClusterService(
	ns INamespaceService,
	acs IAccessControlService,
	sgws IWorkerSliceGatewayService,
	mf metrics.IMetricRecorder,
) IClusterService {
	return &ClusterService{
		ns:   ns,
		acs:  acs,
		sgws: sgws,
		mf:   mf,
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
	mf metrics.IMetricRecorder,
	vpn IVpnKeyRotationService,
) ISliceConfigService {
	return &SliceConfigService{
		ns:    ns,
		acs:   acs,
		sgs:   sgs,
		ms:    ms,
		si:    si,
		se:    se,
		wsgrs: wsgrs,
		mf:    mf,
		vpn:   vpn,
	}
}

// bootstrapping service export config service
func WithServiceExportConfigService(ses IWorkerServiceImportService,
	mf metrics.IMetricRecorder) IServiceExportConfigService {
	return &ServiceExportConfigService{
		ses: ses,
		mf:  mf,
	}
}

// bootstrapping namespace service
func WithNameSpaceService(mf metrics.IMetricRecorder) INamespaceService {
	return &NamespaceService{
		mf: mf,
	}
}

// bootstrapping accesscontrol service
func WithAccessControlService(ruleProvider IAccessControlRuleProvider, mf metrics.IMetricRecorder) IAccessControlService {
	return &AccessControlService{
		ruleProvider: ruleProvider,
		mf:           mf,
	}
}

// bootstrapping secret service
func WithSecretService(mf metrics.IMetricRecorder) ISecretService {
	return &SecretService{
		mf: mf,
	}
}

// bootstrapping slice gateway service
func WithWorkerSliceGatewayService(
	js IJobService,
	sscs IWorkerSliceConfigService,
	sc ISecretService,
	mf metrics.IMetricRecorder,
) IWorkerSliceGatewayService {
	return &WorkerSliceGatewayService{
		js:   js,
		sscs: sscs,
		sc:   sc,
		mf:   mf,
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
func WithWorkerSliceConfigService(mf metrics.IMetricRecorder) IWorkerSliceConfigService {
	return &WorkerSliceConfigService{
		mf: mf,
	}
}

// bootstrapping worker service import service
func WithWorkerServiceImportService(mf metrics.IMetricRecorder) IWorkerServiceImportService {
	return &WorkerServiceImportService{
		mf: mf,
	}
}

// bootstrapping slice qos config service
func WithSliceQoSConfigService(wsc IWorkerSliceConfigService, mf metrics.IMetricRecorder) ISliceQoSConfigService {
	return &SliceQoSConfigService{
		wsc: wsc,
		mf:  mf,
	}
}

func WithMetricsRecorder() metrics.IMetricRecorder {
	return &metrics.MetricRecorder{}
}

// bootstrapping Vpn Key Rotation service
func WithVpnKeyRotationService(w IWorkerSliceGatewayService, ws IWorkerSliceConfigService) IVpnKeyRotationService {
	return &VpnKeyRotationService{
		wsgs: w,
		wscs: ws,
	}
}
