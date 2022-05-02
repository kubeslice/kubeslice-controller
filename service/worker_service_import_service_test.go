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
	"fmt"
	"testing"

	"github.com/kubeslice/kubeslice-controller/util"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	utilmock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	kubemachine "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//	sigs.k8s.io/controller-runtime/pkg/client")
	//"k8s.io/apimachinery/pkg/api/errors"
)

func TestWorkerServiceImportServiceSuite(t *testing.T) {
	for k, v := range WorkerServiceImportServiceTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var WorkerServiceImportServiceTestbed = map[string]func(*testing.T){
	"TestReconcileWorkerServiceImportGetWorkerServiceImportResourceFail":        testReconcileWorkerServiceImportGetWorkerServiceImportResourceFail,
	"TestReconcileWorkerServiceDeleteTheobjectHappyCase":                        testReconcileWorkerServiceDeleteTheobjectHappyCase,
	"TestReconcileWorkerServiceImportGetServiceExportListFail":                  testReconcileWorkerServiceImportGetServiceExportListFail,
	"TestReconcileWorkerServiceImportGetServiceExportListEmpty":                 testReconcileWorkerServiceImportGetServiceExportListEmpty,
	"TestReconcileWorkerServiceImportHappyPath":                                 testReconcileWorkerServiceImportHappyPath,
	"TestListWorkerServiceImportFail":                                           testListWorkerServiceImportFail,
	"TestDeleteWorkerServiceImportByLabelPass":                                  testDeleteWorkerServiceImportByLabelPass,
	"TestCreateMinimalWorkerServiceImportGetexistingWorkerServiceImportFail":    testCreateMinimalWorkerServiceImportGetexistingWorkerServiceImportFail,
	"TestCreateMinimalWorkerServiceImportUpdateexistingWorkerServiceImportFail": testCreateMinimalWorkerServiceImportUpdateexistingWorkerServiceImportFail,
	"TestCreateMinimalWorkerServiceImportCreateexistingWorkerServiceImportFail": testCreateMinimalWorkerServiceImportCreateexistingWorkerServiceImportFail,
}

func testReconcileWorkerServiceImportGetWorkerServiceImportResourceFail(t *testing.T) {
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	workerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	workerServiceName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "mysql-1",
	}
	requestObj := ctrl.Request{
		workerServiceName,
	}
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerServiceImport).Return(kubeerrors.NewNotFound(util.Resource("WorkerServiceImportTest"), "workerServiceImport not found")).Once()
	result, err := WorkerServiceImportServiceStruct.ReconcileWorkerServiceImport(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)

}
func testReconcileWorkerServiceDeleteTheobjectHappyCase(t *testing.T) {
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	workerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	workerServiceName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "mysql-1",
	}
	requestObj := ctrl.Request{
		workerServiceName,
	}
	clientMock := &utilmock.Client{}
	serviceExportList := &controllerv1alpha1.ServiceExportConfigList{}
	slice := &controllerv1alpha1.SliceConfig{}

	timeStamp := kubemachine.Now()
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerServiceImport).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerServiceImport)
		arg.ObjectMeta.DeletionTimestamp = &timeStamp
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
			arg.Labels["worker-cluster"] = "random"
			fmt.Println("labels", arg.Labels)
		}
	}).Once()

	//remove
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("List", ctx, serviceExportList, client.InNamespace(requestObj.Namespace), mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			arg := args.Get(1).(*controllerv1alpha1.ServiceExportConfigList)
			if arg.Items == nil {
				arg.Items = make([]controllerv1alpha1.ServiceExportConfig, 1)
			}
			arg.Items[0].GenerateName = "random"
			if arg.Items[0].Annotations == nil {
				arg.Items[0].Annotations = make(map[string]string)
			}
		}).Once()
	clientMock.On("Get", ctx, mock.Anything, slice).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		if arg.Spec.Clusters == nil {
			arg.Spec.Clusters = append(arg.Spec.Clusters, "random")
		}
	})
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := WorkerServiceImportServiceStruct.ReconcileWorkerServiceImport(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)

}

func testReconcileWorkerServiceImportGetServiceExportListFail(t *testing.T) {
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	workerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	workerServiceName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "mysql-1",
	}
	requestObj := ctrl.Request{
		workerServiceName,
	}
	clientMock := &utilmock.Client{}
	serviceExportList := &controllerv1alpha1.ServiceExportConfigList{}
	labels := map[string]string{
		"service-name":        workerServiceImport.Spec.ServiceName,
		"service-namespace":   workerServiceImport.Spec.ServiceNamespace,
		"original-slice-name": workerServiceImport.Spec.SliceName,
	}

	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerServiceImport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()

	clientMock.On("List", ctx, serviceExportList, client.InNamespace(requestObj.Namespace), client.MatchingLabels(labels)).Return(kubeerrors.NewNotFound(util.Resource("WorkerServiceImportTest"), "workerServiceExport not found")).Once()
	result, err := WorkerServiceImportServiceStruct.ReconcileWorkerServiceImport(ctx, requestObj)
	require.True(t, result.Requeue)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func testReconcileWorkerServiceImportGetServiceExportListEmpty(t *testing.T) {
	//var errList errorList
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	workerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	workerServiceName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "mysql-1",
	}
	requestObj := ctrl.Request{
		workerServiceName,
	}
	clientMock := &utilmock.Client{}
	serviceExportList := &controllerv1alpha1.ServiceExportConfigList{}
	labels := map[string]string{
		"service-name":        workerServiceImport.Spec.ServiceName,
		"service-namespace":   workerServiceImport.Spec.ServiceNamespace,
		"original-slice-name": workerServiceImport.Spec.SliceName,
	}
	//	serviceImport := workerv1alpha1.WorkerServiceImport{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerServiceImport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()

	clientMock.On("List", ctx, serviceExportList, client.InNamespace(requestObj.Namespace), client.MatchingLabels(labels)).Return(nil).Once()
	result, err := WorkerServiceImportServiceStruct.ReconcileWorkerServiceImport(ctx, requestObj)
	require.False(t, result.Requeue) //serviceExportList.Items = empty
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func testReconcileWorkerServiceImportHappyPath(t *testing.T) {
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	workerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	workerServiceName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "mysql-1",
	}
	requestObj := ctrl.Request{
		workerServiceName,
	}
	clientMock := &utilmock.Client{}
	serviceExportList := &controllerv1alpha1.ServiceExportConfigList{}
	labels := map[string]string{
		"service-name":        workerServiceImport.Spec.ServiceName,
		"service-namespace":   workerServiceImport.Spec.ServiceNamespace,
		"original-slice-name": workerServiceImport.Spec.SliceName,
	}

	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerServiceImport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()

	clientMock.On("List", ctx, serviceExportList, client.InNamespace(requestObj.Namespace), client.MatchingLabels(labels)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.ServiceExportConfigList)
		if arg.Items == nil {
			arg.Items = make([]controllerv1alpha1.ServiceExportConfig, 1)
		}
		arg.Items[0].GenerateName = "random"
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := WorkerServiceImportServiceStruct.ReconcileWorkerServiceImport(ctx, requestObj)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func testListWorkerServiceImportFail(t *testing.T) {
	workerServiceImports := &workerv1alpha1.WorkerServiceImportList{}
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	labels := map[string]string{
		"service-name":        "random_service-name",
		"service-namespace":   "random_service-namespace",
		"original-slice-name": "random_original-slice-name",
	}
	workerServiceName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "mysql-1",
	}
	requestObj := ctrl.Request{
		workerServiceName,
	}
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	clientMock.On("List", ctx, workerServiceImports, client.MatchingLabels(labels), client.InNamespace(requestObj.Namespace)).Return(kubeerrors.NewNotFound(util.Resource("WorkerServiceImportTest"), "ListWorkerService not found")).Run(
		func(args mock.Arguments) {
			arg := args.Get(1).(*workerv1alpha1.WorkerServiceImportList)
			arg.Items = []workerv1alpha1.WorkerServiceImport{}
		}).Once()
	result, err := WorkerServiceImportServiceStruct.ListWorkerServiceImport(ctx, labels, requestObj.Namespace)
	require.Equal(t, len(result), 0)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}
func testDeleteWorkerServiceImportByLabelPass(t *testing.T) {
	workerServiceImports := &workerv1alpha1.WorkerServiceImportList{}
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	clientMock := &utilmock.Client{}
	labels := map[string]string{
		"service-name":        "random_service-name",
		"service-namespace":   "random_service-namespace",
		"original-slice-name": "random_original-slice-name",
	}
	workerServiceName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "mysql-1",
	}
	requestObj := ctrl.Request{
		workerServiceName,
	}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("List", ctx, workerServiceImports, client.MatchingLabels(labels), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerServiceImportList)
		if arg.Items == nil {
			arg.Items = make([]workerv1alpha1.WorkerServiceImport, 1)
			arg.Items[0].Name = "random"
		}
	})
	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Once()
	err := WorkerServiceImportServiceStruct.DeleteWorkerServiceImportByLabel(ctx, labels, requestObj.Namespace)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testCreateMinimalWorkerServiceImportGetexistingWorkerServiceImportFail(t *testing.T) {
	clientMock := &utilmock.Client{}
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	existingWorkerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	workerServiceImports := &workerv1alpha1.WorkerServiceImportList{}
	serviceName := "mysql"
	serviceNamespace := "alpha"
	sliceName := "red"
	clusters := []string{"cluster1", "cluster2"}
	namespace := "cisco"
	label := make(map[string]string)
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("List", ctx, workerServiceImports, mock.Anything, mock.Anything).Return(nil).Once()
	getError := errors.New("existingWorkerServiceImport not found")
	clientMock.On("Get", ctx, mock.Anything, existingWorkerServiceImport).Return(getError).Once()
	err := WorkerServiceImportServiceStruct.CreateMinimalWorkerServiceImport(ctx, clusters, namespace, label, serviceName, serviceNamespace, sliceName)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

//pass
func testCreateMinimalWorkerServiceImportUpdateexistingWorkerServiceImportFail(t *testing.T) {
	clientMock := &utilmock.Client{}
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}

	existingWorkerServiceImport := &workerv1alpha1.WorkerServiceImport{}
	workerServiceImports := &workerv1alpha1.WorkerServiceImportList{}
	serviceName := "mysql"
	serviceNamespace := "alpha"
	sliceName := "red"
	clusters := []string{"cluster1", "cluster2"}
	namespace := "cisco"
	label := make(map[string]string)
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("List", ctx, workerServiceImports, mock.Anything, mock.Anything).Return(nil).Once()
	updatetError := errors.New("existingWorkerServiceImport update failed")
	clientMock.On("Get", ctx, mock.Anything, existingWorkerServiceImport).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(updatetError).Once()
	err := WorkerServiceImportServiceStruct.CreateMinimalWorkerServiceImport(ctx, clusters, namespace, label, serviceName, serviceNamespace, sliceName)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func testCreateMinimalWorkerServiceImportCreateexistingWorkerServiceImportFail(t *testing.T) {
	clientMock := &utilmock.Client{}
	WorkerServiceImportServiceStruct := WorkerServiceImportService{}
	workerServiceImports := &workerv1alpha1.WorkerServiceImportList{}
	serviceName := "mysql"
	serviceNamespace := "alpha"
	sliceName := "red"
	clusters := []string{"cluster1", "cluster2"}
	namespace := "cisco"
	label := make(map[string]string)
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("List", ctx, workerServiceImports, mock.Anything, mock.Anything).Return(nil).Once()
	getError := kubeerrors.NewNotFound(util.Resource("WorkerServiceImportTest"), "existingWorkerServiceImport not found")
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(getError).Once()
	existserr := errors.New("IsAlreadyExists")
	clientMock.On("Create", ctx, mock.Anything).Return(existserr).Once()
	err := WorkerServiceImportServiceStruct.CreateMinimalWorkerServiceImport(ctx, clusters, namespace, label, serviceName, serviceNamespace, sliceName)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}
