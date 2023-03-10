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
	"errors"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sapimachinery "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceExportConfigSuite(t *testing.T) {
	for k, v := range ServiceExportConfigTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ServiceExportConfigTestBed = map[string]func(*testing.T){
	"ServiceExport_ReconciliationCompleteHappyCase":                                      ServiceExportReconciliationCompleteHappyCase,
	"ServiceExport_ServiceExportGetObjectErrorOtherThanNotFound":                         ServiceExportGetObjectErrorOtherThanNotFound,
	"ServiceExport_ServiceExportGetObjectErrorNotFound":                                  ServiceExportGetObjectErrorNotFound,
	"ServiceExport_ServiceExportDeleteTheObjectHappyCase":                                ServiceExportDeleteTheObjectHappyCase,
	"ServiceExport_ServiceExportErrorOnUpdatingTheFinalizer":                             ServiceExportErrorOnUpdatingTheFinalizer,
	"ServiceExport_ServiceExportRemoveFinalizerErrorOnUpdate":                            ServiceExportRemoveFinalizerErrorOnUpdate,
	"ServiceExport_ServiceExportRemoveFinalizerErrorOnGetAfterUpdate":                    ServiceExportRemoveFinalizerErrorOnGetAfterUpdate,
	"ServiceExport_ServiceExportErrorOnDeleteWorkerServiceImportByLabel":                 ServiceExportErrorOnDeleteWorkerServiceImportByLabel,
	"ServiceExport_ServiceExportErrorOnGettingSliceConfigOtherThanNotFound":              ServiceExportErrorOnGettingSliceConfigOtherThanNotFound,
	"ServiceExport_ServiceExportErrorNotFoundOnGettingSliceConfig":                       ServiceExportErrorNotFoundOnGettingSliceConfig,
	"ServiceExport_ServiceExportErrorOnCreateWorkerServiceImport":                        ServiceExportErrorOnCreateWorkerServiceImport,
	"ServiceExport_ServiceExportUpdateHappyCase":                                         ServiceExportUpdateHappyCase,
	"ServiceExport_ServiceExportErrorOnUpdate":                                           ServiceExportErrorOnUpdate,
	"ServiceExport_ServiceExportLabelsNullOnUpdate":                                      ServiceExportLabelsNullOnUpdate,
	"ServiceExport_ServiceExportDeleteByListSuccess":                                     ServiceExportDeleteByListSuccess,
	"ServiceExport_ServiceExportDeleteByListFailedToDelete":                              ServiceExportDeleteByListFailedToDelete,
	"ServiceExport_ServiceExportDeleteByListFailedToReturnList":                          ServiceExportDeleteByListFailToReturnList,
	"ServiceExport_ServiceExportDeleteServiceExportConfigByParticipatingSliceConfigFail": ServiceExportDeleteServiceExportConfigByParticipatingSliceConfigFail,
	"ServiceExport_ServiceExportListServiceExportConfigsFail":                            ServiceExportListServiceExportConfigsFail,
}

func ServiceExportReconciliationCompleteHappyCase(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Twice()
	workerServiceImportMock.On("CreateMinimalWorkerServiceImport", ctx, mock.Anything, requestObj.Namespace, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	result, err := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportGetObjectErrorOtherThanNotFound(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	err1 := errors.New("internal_error")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(err1).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportGetObjectErrorNotFound(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	notFoundError := k8sError.NewNotFound(util.Resource("ServiceExportConfigTest"), "isNotFound")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(notFoundError).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportDeleteTheObjectHappyCase(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	time := k8sapimachinery.Now()
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.ServiceExportConfig)
		arg.ObjectMeta.DeletionTimestamp = &time
	}).Once()
	workerServiceImportMock.On("DeleteWorkerServiceImportByLabel", ctx, mock.Anything, requestObj.Namespace).Return(nil).Once()
	workerServiceImportMock.On("LookupServiceExportForService", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	//remove finalizer
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportErrorOnUpdatingTheFinalizer(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Once()
	err1 := errors.New("internal_error")
	clientMock.On("Update", ctx, mock.Anything).Return(err1).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}
func ServiceExportRemoveFinalizerErrorOnUpdate(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	time := k8sapimachinery.Now()
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.ServiceExportConfig)
		arg.ObjectMeta.DeletionTimestamp = &time
	}).Once()
	workerServiceImportMock.On("DeleteWorkerServiceImportByLabel", ctx, mock.Anything, requestObj.Namespace).Return(nil).Once()
	workerServiceImportMock.On("LookupServiceExportForService", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	err1 := errors.New("internal_error")
	clientMock.On("Update", ctx, mock.Anything).Return(err1).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportRemoveFinalizerErrorOnGetAfterUpdate(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	time := k8sapimachinery.Now()
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.ServiceExportConfig)
		arg.ObjectMeta.DeletionTimestamp = &time
		arg.Spec.ServiceName = "service_name"
		arg.Spec.ServiceNamespace = "service_namespace"
		arg.Spec.SourceCluster = "SourceCluster"
		arg.Spec.SliceName = "slice_name"
		arg.Spec.ServiceDiscoveryEndpoints = []controllerv1alpha1.ServiceDiscoveryEndpoint{}
		arg.Spec.ServiceDiscoveryPorts = []controllerv1alpha1.ServiceDiscoveryPort{}
		arg.Namespace = "namespace"
	}).Once()
	workerServiceImportMock.On("DeleteWorkerServiceImportByLabel", ctx, mock.Anything, requestObj.Namespace).Return(nil).Once()
	workerServiceImportMock.On("LookupServiceExportForService", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	//remove finalizer
	err1 := errors.New("internal_error")
	clientMock.On("Update", ctx, mock.Anything).Return(err1).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportErrorOnDeleteWorkerServiceImportByLabel(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	time := k8sapimachinery.Now()
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.ServiceExportConfig)
		arg.ObjectMeta.DeletionTimestamp = &time
	}).Once()
	err1 := errors.New("internal_error")
	workerServiceImportMock.On("DeleteWorkerServiceImportByLabel", ctx, mock.Anything, requestObj.Namespace).Return(err1).Once()
	workerServiceImportMock.On("LookupServiceExportForService", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportErrorOnGettingSliceConfigOtherThanNotFound(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	err1 := errors.New("internal_error")
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(err1).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportErrorNotFoundOnGettingSliceConfig(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	notFoundError := k8sError.NewNotFound(util.Resource("ServiceExportConfigTest"), "isNotFound")
	clientMock.On("Get", ctx, mock.Anything, &controllerv1alpha1.SliceConfig{}).Return(notFoundError).Once()
	result, err := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportErrorOnCreateWorkerServiceImport(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Twice()
	err1 := errors.New("internal_error")
	workerServiceImportMock.On("CreateMinimalWorkerServiceImport", ctx, mock.Anything, requestObj.Namespace, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(err1).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportUpdateHappyCase(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.ServiceExportConfig)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels["original-slice-name"] = "slice-1"
		arg.Spec.SliceName = "slice-2"
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Twice()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	workerServiceImportMock.On("CreateMinimalWorkerServiceImport", ctx, mock.Anything, requestObj.Namespace, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	result, err := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportErrorOnUpdate(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.ServiceExportConfig)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels["original-slice-name"] = "slice-1"
		arg.Spec.SliceName = "slice-2"
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Twice()
	err1 := errors.New("internal_error")
	clientMock.On("Update", ctx, mock.Anything).Return(err1).Once()
	result, err2 := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportLabelsNullOnUpdate(t *testing.T) {
	workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx := setupServiceExportTest("service_export", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, serviceExport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.ServiceExportConfig)
		arg.Spec.SliceName = "slice-2"
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Twice()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	workerServiceImportMock.On("CreateMinimalWorkerServiceImport", ctx, mock.Anything, requestObj.Namespace, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	result, err := serviceExportConfigService.ReconcileServiceExportConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerServiceImportMock.AssertExpectations(t)
}

func ServiceExportDeleteByListSuccess(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, _, ctx := setupServiceExportTest("service_export", "namespace")
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	clientMock.On("List", ctx, serviceExports, client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.ServiceExportConfigList)
		arg.Items = []controllerv1alpha1.ServiceExportConfig{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: controllerv1alpha1.ServiceExportConfigSpec{
					ServiceName:               "",
					ServiceNamespace:          "",
					SourceCluster:             "",
					SliceName:                 "",
					ServiceDiscoveryEndpoints: nil,
					ServiceDiscoveryPorts:     nil,
				},
				Status: controllerv1alpha1.ServiceExportConfigStatus{},
			},
		}
	}).Once()
	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Once()
	result, err := serviceExportConfigService.DeleteServiceExportConfigs(ctx, requestObj.Namespace)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportDeleteByListFailToReturnList(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, _, ctx := setupServiceExportTest("service_export", "namespace")
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	err1 := errors.New("internal_error")
	clientMock.On("List", ctx, serviceExports, client.InNamespace(requestObj.Namespace)).Return(err1).Once()
	result, err := serviceExportConfigService.DeleteServiceExportConfigs(ctx, requestObj.Namespace)
	expectedResult := ctrl.Result{}
	require.Equal(t, err1, err)
	require.Equal(t, expectedResult, result)
	require.Error(t, err1)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportDeleteByListFailedToDelete(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, _, ctx := setupServiceExportTest("service_export", "namespace")
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	clientMock.On("List", ctx, serviceExports, client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.ServiceExportConfigList)
		arg.Items = []controllerv1alpha1.ServiceExportConfig{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: controllerv1alpha1.ServiceExportConfigSpec{
					ServiceName:               "",
					ServiceNamespace:          "",
					SourceCluster:             "",
					SliceName:                 "",
					ServiceDiscoveryEndpoints: nil,
					ServiceDiscoveryPorts:     nil,
				},
				Status: controllerv1alpha1.ServiceExportConfigStatus{},
			},
		}
	}).Once()
	err1 := errors.New("internal_error")
	clientMock.On("Delete", ctx, mock.Anything).Return(err1).Once()
	result, err := serviceExportConfigService.DeleteServiceExportConfigs(ctx, requestObj.Namespace)
	expectedResult := ctrl.Result{}
	require.Equal(t, err1, err)
	require.Equal(t, expectedResult, result)
	require.Error(t, err1)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func ServiceExportListServiceExportConfigsFail(t *testing.T) {
	_, serviceExportConfigService, requestObj, clientMock, _, ctx := setupServiceExportTest("service_export", "namespace")
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	listError := errors.New("list error")
	clientMock.On("List", ctx, serviceExports, mock.Anything).Return(listError).Once()
	result, err := serviceExportConfigService.ListServiceExportConfigs(ctx, requestObj.Namespace)
	require.NotNil(t, err)
	require.Empty(t, result)
	clientMock.AssertExpectations(t)
}

func ServiceExportDeleteServiceExportConfigByParticipatingSliceConfigFail(t *testing.T) {
	_, serviceExportConfigService, _, clientMock, requestObj, ctx := setupServiceExportTest("service_export", "namespace")
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	clientMock.On("List", ctx, serviceExports, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.ServiceExportConfigList)
		if arg.Items == nil {
			arg.Items = make([]controllerv1alpha1.ServiceExportConfig, 1)
			arg.Items[0].Name = "random"
			if arg.Items[0].Labels == nil {
				arg.Items[0].Labels = make(map[string]string)
				arg.Items[0].Labels["original-slice-name"] = "random"
			}
		}
	}).Once()
	deleteError := errors.New("delete failed")
	clientMock.On("Delete", ctx, mock.Anything).Return(deleteError).Once()
	err := serviceExportConfigService.DeleteServiceExportConfigByParticipatingSliceConfig(ctx, "random", requestObj.Namespace)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func setupServiceExportTest(name string, namespace string) (*mocks.IWorkerServiceImportService, ServiceExportConfigService, ctrl.Request, *utilMock.Client, *controllerv1alpha1.ServiceExportConfig, context.Context) {
	workerServiceImportMock := &mocks.IWorkerServiceImportService{}
	serviceExportConfigService := ServiceExportConfigService{
		ses: workerServiceImportMock,
	}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	requestObj := ctrl.Request{
		NamespacedName: namespacedName,
	}
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	serviceExport := &controllerv1alpha1.ServiceExportConfig{}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, scheme, "ServiceExportConfigServiceTest")
	return workerServiceImportMock, serviceExportConfigService, requestObj, clientMock, serviceExport, ctx
}
