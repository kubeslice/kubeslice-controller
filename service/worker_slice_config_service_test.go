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

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sapimachinery "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestWorkerSliceSuite(t *testing.T) {
	for k, v := range WorkerSliceTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var WorkerSliceTestbed = map[string]func(*testing.T){
	"TestWorkerSliceReconciliation_Success":               testWorkerSliceGetsCreatedAndReturnsReconciliationSuccess,
	"TestWorkerSliceReconciliation_IfNotFound":            testWorkerSliceReconciliationIfNotFound,
	"TestWorkerSliceReconciliation_IfSliceConfigNotFound": testWorkerSliceReconciliationIfSliceConfigNotFound,
	"TestWorkerSliceReconciliation_DeleteSuccessfully":    testWorkerSliceReconciliationDeleteSuccessfully,
	"TestDeleteWorkerSliceConfigByLabel_Success":          testDeleteWorkerSliceConfigByLabelSuccess,
	"TestCreateWorkerSliceConfig_NewClusterSuccess":       testCreateWorkerSliceConfigNewClusterSuccess,
	"TestCreateWorkerSliceConfig_NewClusterFails":         testCreateWorkerSliceConfigNewClusterFails,
	"TestCreateWorkerSliceConfig_UpdateClusterSuccess":    testCreateWorkerSliceConfigUpdateClusterSuccess,
	"TestCreateWorkerSliceConfig_UpdateClusterFails":      testCreateWorkerSliceConfigUpdateClusterFails,
	"TestCreateWorkerSliceConfig_WithStandardQosProfile":  testCreateWorkerSliceConfigWithStandardQosProfile,
}

func testWorkerSliceGetsCreatedAndReturnsReconciliationSuccess(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerSlice).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceConfig)
		arg.Spec.SliceSubnet = "10.23.45.14/24"
		arg.Spec.SliceType = "sliceType"
		arg.Spec.SliceGatewayProvider = workerv1alpha1.WorkerSliceGatewayProvider{
			SliceGatewayType: "sliceGatewayType",
			SliceCaType:      "sliceCaType",
		}
		arg.Spec.SliceName = "red"
		arg.Spec.QosProfileDetails = workerv1alpha1.QOSProfile{
			QueueType:               "queueType",
			Priority:                1,
			TcType:                  "tcType",
			BandwidthCeilingKbps:    0,
			BandwidthGuaranteedKbps: 0,
			DscpClass:               "dscpClass",
		}
		arg.Spec.NamespaceIsolationProfile = workerv1alpha1.NamespaceIsolationProfile{
			IsolationEnabled:      false,
			ApplicationNamespaces: nil,
			AllowedNamespaces:     nil,
		}
		arg.Spec.ExternalGatewayConfig = workerv1alpha1.ExternalGatewayConfig{
			Ingress:     workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			Egress:      workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			NsIngress:   workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			GatewayType: "gatewayType",
		}
		arg.Spec.SliceIpamType = "sliceIpamType"
		arg.Labels = map[string]string{
			"worker-cluster": "cluster-1",
		}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
			{
				Ingress:     controllerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
				Egress:      controllerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
				NsIngress:   controllerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
				GatewayType: "gatewayType",
				Clusters:    []string{"*", "cluster-1"},
			},
		}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := WorkerSliceService.ReconcileWorkerSliceConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testWorkerSliceReconciliationIfNotFound(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerSlice).Return(notFoundError).Once()
	result, err := WorkerSliceService.ReconcileWorkerSliceConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testWorkerSliceReconciliationIfSliceConfigNotFound(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerSlice).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceConfig)
		arg.Spec.SliceSubnet = "10.23.45.14/24"
		arg.Spec.SliceType = "sliceType"
		arg.Spec.SliceGatewayProvider = workerv1alpha1.WorkerSliceGatewayProvider{
			SliceGatewayType: "sliceGatewayType",
			SliceCaType:      "sliceCaType",
		}
		arg.Spec.SliceName = "red"
		arg.Spec.QosProfileDetails = workerv1alpha1.QOSProfile{
			QueueType:               "queueType",
			Priority:                1,
			TcType:                  "tcType",
			BandwidthCeilingKbps:    0,
			BandwidthGuaranteedKbps: 0,
			DscpClass:               "dscpClass",
		}
		arg.Spec.NamespaceIsolationProfile = workerv1alpha1.NamespaceIsolationProfile{
			IsolationEnabled:      false,
			ApplicationNamespaces: nil,
			AllowedNamespaces:     nil,
		}
		arg.Spec.ExternalGatewayConfig = workerv1alpha1.ExternalGatewayConfig{
			Ingress:     workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			Egress:      workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			NsIngress:   workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			GatewayType: "gatewayType",
		}
		arg.Spec.SliceIpamType = "sliceIpamType"
		arg.ObjectMeta.Finalizers = []string{WorkerSliceConfigFinalizer}
	}).Once()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(notFoundError).Once()
	result, err := WorkerSliceService.ReconcileWorkerSliceConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testWorkerSliceReconciliationDeleteSuccessfully(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	time := k8sapimachinery.Now()
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerSlice).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceConfig)
		arg.ObjectMeta.DeletionTimestamp = &time
		arg.Labels = map[string]string{
			"worker-cluster": "cluster-1",
		}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.Clusters = []string{"cluster-1", "cluster-2"}
		arg.Annotations = map[string]string{}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := WorkerSliceService.ReconcileWorkerSliceConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testDeleteWorkerSliceConfigByLabelSuccess(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	label := map[string]string{
		"worker-cluster": "cluster-1",
	}
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSlices, client.MatchingLabels(label), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		arg.Items = []workerv1alpha1.WorkerSliceConfig{
			{
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
			},
			{
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
			},
		}
	}).Once()
	clientMock.On("Delete", ctx, workerSlice).Return(nil).Twice()
	err := WorkerSliceService.DeleteWorkerSliceConfigByLabel(ctx, label, requestObj.Namespace)
	require.NoError(t, nil)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testCreateWorkerSliceConfigNewClusterSuccess(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	label := map[string]string{
		"worker-cluster": "cluster-1",
	}
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSlices, client.MatchingLabels(label), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		octet := 0
		arg.Items = []workerv1alpha1.WorkerSliceConfig{
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: label,
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: map[string]string{
						"worker-cluster": "cluster-2",
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
		}
	}).Twice()
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), workerSlice).Return(notFoundError).Twice()
	clientMock.On("Create", ctx, mock.Anything).Return(nil).Twice()
	result, err := WorkerSliceService.CreateMinimalWorkerSliceConfig(ctx, []string{"cluster-1", "cluster-2"}, requestObj.Namespace, label, "red", "198.23.54.47/16", "/20")
	require.Equal(t, len(result), 2)
	require.NoError(t, nil)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testCreateWorkerSliceConfigNewClusterFails(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	label := map[string]string{
		"worker-cluster": "cluster-1",
	}
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSlices, client.MatchingLabels(label), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		octet := 0
		arg.Items = []workerv1alpha1.WorkerSliceConfig{
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: label,
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: map[string]string{
						"worker-cluster": "cluster-2",
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
		}
	}).Twice()
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), workerSlice).Return(notFoundError).Once()
	err1 := errors.New("internal_error")
	clientMock.On("Create", ctx, mock.Anything).Return(err1).Once()
	result, err := WorkerSliceService.CreateMinimalWorkerSliceConfig(ctx, []string{"cluster-1", "cluster-2"}, requestObj.Namespace, label, "red", "198.23.54.47/16", "/20")
	require.Error(t, err)
	require.Equal(t, len(result), 2)
	require.Equal(t, err, err1)
	clientMock.AssertExpectations(t)
}

func testCreateWorkerSliceConfigUpdateClusterSuccess(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	label := map[string]string{
		"worker-cluster": "cluster-1",
	}
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSlices, client.MatchingLabels(label), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		octet := 0
		arg.Items = []workerv1alpha1.WorkerSliceConfig{
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: label,
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: map[string]string{
						"worker-cluster": "cluster-2",
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
		}
	}).Twice()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), workerSlice).Return(nil).Twice()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Twice()

	result, err := WorkerSliceService.CreateMinimalWorkerSliceConfig(ctx, []string{"cluster-1", "cluster-2"}, requestObj.Namespace, label, "red", "198.23.54.47/16", "/20")
	require.Equal(t, len(result), 2)
	require.NoError(t, nil)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testCreateWorkerSliceConfigUpdateClusterFails(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	label := map[string]string{
		"worker-cluster": "cluster-1",
	}
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSlices, client.MatchingLabels(label), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		octet := 0
		arg.Items = []workerv1alpha1.WorkerSliceConfig{
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: label,
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
			{
				TypeMeta: k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{
					Labels: map[string]string{
						"worker-cluster": "cluster-2",
					},
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:                 "",
					SliceSubnet:               "",
					SliceType:                 "",
					SliceGatewayProvider:      workerv1alpha1.WorkerSliceGatewayProvider{},
					SliceIpamType:             "",
					QosProfileDetails:         workerv1alpha1.QOSProfile{},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{},
					Octet:                     &octet,
					ExternalGatewayConfig:     workerv1alpha1.ExternalGatewayConfig{},
				},
				Status: workerv1alpha1.WorkerSliceConfigStatus{},
			},
		}
	}).Twice()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), workerSlice).Return(nil).Once()
	err1 := errors.New("internal_error")
	clientMock.On("Update", ctx, mock.Anything).Return(err1).Once()
	result, err := WorkerSliceService.CreateMinimalWorkerSliceConfig(ctx, []string{"cluster-1", "cluster-2"}, requestObj.Namespace, label, "red", "198.23.54.47/16", "/20")
	require.Error(t, err)
	require.Equal(t, len(result), 2)
	require.Equal(t, err, err1)
	clientMock.AssertExpectations(t)
}

func setupWorkerSliceTest(name string, namespace string) (WorkerSliceConfigService, ctrl.Request, *utilMock.Client, *workerv1alpha1.WorkerSliceConfig, context.Context) {
	WorkerSliceService := WorkerSliceConfigService{}
	fmt.Println("WorkerSliceService", WorkerSliceService)
	workerSliceName := types.NamespacedName{
		Namespace: name,
		Name:      namespace,
	}
	requestObj := ctrl.Request{
		NamespacedName: workerSliceName,
	}
	clientMock := &utilMock.Client{}
	workerSlice := &workerv1alpha1.WorkerSliceConfig{}

	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "WorkerSliceConfigController", nil)
	return WorkerSliceService, requestObj, clientMock, workerSlice, ctx
}

func testCreateWorkerSliceConfigWithStandardQosProfile(t *testing.T) {
	WorkerSliceName := "red-cluster-worker-slice"
	namespace := "controller-manager-cisco"
	WorkerSliceService, requestObj, clientMock, workerSlice, ctx := setupWorkerSliceTest(WorkerSliceName, namespace)
	clientMock.On("Get", ctx, requestObj.NamespacedName, workerSlice).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceConfig)
		arg.Spec.SliceSubnet = "10.23.45.14/24"
		arg.Spec.SliceType = "sliceType"
		arg.Spec.SliceGatewayProvider = workerv1alpha1.WorkerSliceGatewayProvider{
			SliceGatewayType: "sliceGatewayType",
			SliceCaType:      "sliceCaType",
		}
		arg.Spec.SliceName = "red"
		arg.Spec.QosProfileDetails = workerv1alpha1.QOSProfile{
			QueueType:               "queueType",
			Priority:                1,
			TcType:                  "tcType",
			BandwidthCeilingKbps:    0,
			BandwidthGuaranteedKbps: 0,
			DscpClass:               "dscpClass",
		}
		arg.Spec.NamespaceIsolationProfile = workerv1alpha1.NamespaceIsolationProfile{
			IsolationEnabled:      false,
			ApplicationNamespaces: nil,
			AllowedNamespaces:     nil,
		}
		arg.Spec.ExternalGatewayConfig = workerv1alpha1.ExternalGatewayConfig{
			Ingress:     workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			Egress:      workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			NsIngress:   workerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
			GatewayType: "gatewayType",
		}
		arg.Spec.SliceIpamType = "sliceIpamType"
		arg.Labels = map[string]string{
			"worker-cluster": "cluster-1",
		}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.StandardQosProfileName = "profile-1"
		arg.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
			{
				Ingress:     controllerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
				Egress:      controllerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
				NsIngress:   controllerv1alpha1.ExternalGatewayConfigOptions{Enabled: true},
				GatewayType: "gatewayType",
				Clusters:    []string{"*", "cluster-1"},
			},
		}
	}).Once()
	qos := &controllerv1alpha1.SliceQoSConfig{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), qos).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceQoSConfig)
		arg.Spec.BandwidthCeilingKbps = 5000
		arg.Spec.BandwidthGuaranteedKbps = 2000
		arg.Spec.DscpClass = "AF42"
		arg.Spec.QueueType = "HTB"
		arg.Spec.Priority = 1
		arg.Spec.TcType = "BANDWIDTH_CONTROL"
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := WorkerSliceService.ReconcileWorkerSliceConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
