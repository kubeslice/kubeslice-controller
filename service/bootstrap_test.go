/*/*
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

//
//import (
//	"testing"
//
//	"github.com/dailymotion/allure-go"
//	"github.com/stretchr/testify/require"
//)
//
//func TestBootstrapSuite(t *testing.T) {
//	for k, v := range bootstrapTestbed {
//		t.Run(k, func(t *testing.T) {
//			allure.Test(t, allure.Name(k),
//				allure.Action(func() {
//					v(t)
//				}))
//		})
//	}
//}
//
//var bootstrapTestbed = map[string]func(*testing.T){
//	"allServices":                allServices,
//	"projectService":             projectService,
//	"clusterService":             clusterService,
//	"sliceConfigService":         sliceConfigService,
//	"serviceExportConfigService": serviceExportConfigService,
//	"namespaceService":           namespaceService,
//	"accessControlService":       accessControlService,
//	"secretService":              secretService,
//	"workerSliceGatewayService":  workerSliceGatewayService,
//	"jobService":                 jobService,
//	"workerSliceConfigService":   workerSliceConfigService,
//	"workerServiceImportService": workerServiceImportService,
//}
//
//var allServices = func(t *testing.T) {
//	service := WithServices()
//	require.NotNil(t, service)
//}
//
//var projectService = func(t *testing.T) {
//	service := WithProjectService()
//	require.NotNil(t, service)
//}
//
//var clusterService = func(t *testing.T) {
//	service := WithClusterService()
//	require.NotNil(t, service)
//}
//
//var sliceConfigService = func(t *testing.T) {
//	service := WithSliceConfigService()
//	require.NotNil(t, service)
//}
//
//var serviceExportConfigService = func(t *testing.T) {
//	service := WithServiceExportConfigService()
//	require.NotNil(t, service)
//}
//
//var namespaceService = func(t *testing.T) {
//	service := WithNameSpaceService()
//	require.NotNil(t, service)
//}
//
//var accessControlService = func(t *testing.T) {
//	service := WithAccessControlService()
//	require.NotNil(t, service)
//}
//
//var secretService = func(t *testing.T) {
//	service := WithSecretService()
//	require.NotNil(t, service)
//}
//
//var workerSliceGatewayService = func(t *testing.T) {
//	service := WithWorkerSliceGatewayService()
//	require.NotNil(t, service)
//}
//
//var jobService = func(t *testing.T) {
//	service := WithJobService()
//	require.NotNil(t, service)
//}
//
//var workerSliceConfigService = func(t *testing.T) {
//	service := WithWorkerSliceConfigService()
//	require.NotNil(t, service)
//}
//
//var workerServiceImportService = func(t *testing.T) {
//	service := WithWorkerServiceImportService()
//	require.NotNil(t, service)
//}
