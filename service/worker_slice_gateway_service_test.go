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
	"github.com/kubeslice/kubeslice-controller/metrics"
	metricMock "github.com/kubeslice/kubeslice-controller/metrics/mocks"
	"testing"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"

	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sapimachinery "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestWorkerSliceGatewaySuite(t *testing.T) {
	for k, v := range WorkerSliceGatewayTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var WorkerSliceGatewayTestbed = map[string]func(*testing.T){
	"TestWorkerSliceGatewayReconciliation_Success":               testWorkerSliceGatewayReconciliationSuccess,
	"TestWorkerSliceGatewayReconciliation_IfSliceConfigNotFound": testWorkerSliceGatewayReconciliationIfSliceConfigNotFound,
	"TestWorkerSliceGatewayReconciliation_IfGatewayNotFound":     testWorkerSliceGatewayReconciliationIfGatewayNotFound,
	"TestWorkerSliceGatewayReconciliation_Delete":                testWorkerSliceGatewayReconciliationDelete,
	"TestWorkerSliceGatewayReconciliation_DeleteForcefully":      testWorkerSliceGatewayReconciliationDeleteForcefully,
	"TestCreateMinimumWorkerSliceGateways_IfAlreadyExists":       testCreateMinimumWorkerSliceGatewaysAlreadyExists,
	"TestCreateMinimumWorkerSliceGateways_IfNotExists":           testCreateMinimumWorkerSliceGatewaysNotExists,
	"TestDeleteWorkerSliceGatewaysByLabel_IfExists":              testDeleteWorkerSliceGatewaysByLabelExists,
	"TestNodeIpReconciliationOfWorkerSliceGateways_IfExists":     testNodeIpReconciliationOfWorkerSliceGatewaysExists,
}

func testWorkerSliceGatewayReconciliationSuccess(t *testing.T) {
	_, _, _, workerSliceGatewayService, requestObj, clientMock, WorkerSliceGateway, ctx, mMock := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, WorkerSliceGateway).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceGateway)
		arg.Spec.SliceName = "slice_gateway"
		arg.Spec.GatewayType = "gateway"
		arg.Spec.GatewayHostType = "host"
		arg.Spec.LocalGatewayConfig = workerv1alpha1.SliceGatewayConfig{
			NodeIps:       []string{"10.10.10.10"},
			NodePort:      0,
			GatewayName:   "1",
			ClusterName:   "cluster",
			VpnIp:         "45.45.2.5",
			GatewaySubnet: "10.10.10.10/16",
		}
		arg.Spec.RemoteGatewayConfig = workerv1alpha1.SliceGatewayConfig{
			NodeIps:       []string{""},
			NodePort:      0,
			GatewayName:   "",
			ClusterName:   "",
			VpnIp:         "",
			GatewaySubnet: "",
		}
		arg.Spec.GatewayNumber = 1
		arg.Spec.GatewayCredentials = workerv1alpha1.GatewayCredentials{SecretName: "secret"}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.SliceSubnet = "SliceSubnet"
		arg.Spec.SliceType = "SliceType"
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := workerSliceGatewayService.ReconcileWorkerSliceGateways(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testWorkerSliceGatewayReconciliationIfSliceConfigNotFound(t *testing.T) {
	_, _, _, workerSliceGatewayService, requestObj, clientMock, WorkerSliceGateway, ctx, mMock := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, WorkerSliceGateway).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceGateway)
		arg.Spec.SliceName = "slice_gateway"
		arg.Spec.GatewayType = "gateway"
		arg.Spec.GatewayHostType = "host"
		arg.Spec.LocalGatewayConfig = workerv1alpha1.SliceGatewayConfig{
			NodeIps:       []string{"10.10.10.10"},
			NodePort:      0,
			GatewayName:   "1",
			ClusterName:   "cluster",
			VpnIp:         "45.45.2.5",
			GatewaySubnet: "10.10.10.10/16",
		}
		arg.Spec.RemoteGatewayConfig = workerv1alpha1.SliceGatewayConfig{
			NodeIps:       []string{""},
			NodePort:      0,
			GatewayName:   "",
			ClusterName:   "",
			VpnIp:         "",
			GatewaySubnet: "",
		}
		arg.Spec.GatewayNumber = 1
		arg.Spec.GatewayCredentials = workerv1alpha1.GatewayCredentials{SecretName: "secret"}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(notFoundError).Once()
	result, err := workerSliceGatewayService.ReconcileWorkerSliceGateways(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testWorkerSliceGatewayReconciliationIfGatewayNotFound(t *testing.T) {
	_, _, _, workerSliceGatewayService, requestObj, clientMock, WorkerSliceGateway, ctx, _ := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, requestObj.NamespacedName, WorkerSliceGateway).Return(notFoundError).Once()
	result, err := workerSliceGatewayService.ReconcileWorkerSliceGateways(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testWorkerSliceGatewayReconciliationDelete(t *testing.T) {
	secretMock, _, _, workerSliceGatewayService, requestObj, clientMock, WorkerSliceGateway, ctx, mMock := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	time := k8sapimachinery.Now()
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, WorkerSliceGateway).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceGateway)
		arg.ObjectMeta.DeletionTimestamp = &time
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	secret := &corev1.Secret{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), secret).Return(nil).Once()
	secretMock.On("DeleteSecret", ctx, requestObj.Namespace, WorkerSliceGateway.Name).Return(ctrl.Result{}, nil).Once()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(notFoundError).Once()
	result, err := workerSliceGatewayService.ReconcileWorkerSliceGateways(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testWorkerSliceGatewayReconciliationDeleteForcefully(t *testing.T) {
	secretMock, _, _, workerSliceGatewayService, requestObj, clientMock, WorkerSliceGateway, ctx, mMock := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	time := k8sapimachinery.Now()
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, WorkerSliceGateway).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*workerv1alpha1.WorkerSliceGateway)
		arg.ObjectMeta.DeletionTimestamp = &time
		arg.Labels = map[string]string{
			"worker-cluster": "cluster-1",
			"remote-cluster": "cluster-2",
		}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	secret := &corev1.Secret{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), secret).Return(nil).Once()
	secretMock.On("DeleteSecret", ctx, requestObj.Namespace, WorkerSliceGateway.Name).Return(ctrl.Result{}, nil).Twice()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Twice()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Twice()
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.Clusters = []string{"cluster-1"}
		arg.Annotations = map[string]string{}
	}).Once()
	pairWorkerSliceGateway := &workerv1alpha1.WorkerSliceGatewayList{}
	clientMock.On("List", ctx, pairWorkerSliceGateway, mock.Anything, client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		arg.Items = []workerv1alpha1.WorkerSliceGateway{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:           "",
					GatewayType:         "",
					GatewayHostType:     "",
					GatewayCredentials:  workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig:  workerv1alpha1.SliceGatewayConfig{},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
		}
	}).Once()
	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := workerSliceGatewayService.ReconcileWorkerSliceGateways(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testCreateMinimumWorkerSliceGatewaysAlreadyExists(t *testing.T) {
	_, _, _, workerSliceGatewayService, requestObj, clientMock, _, ctx, mMock := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	label := map[string]string{
		"worker-cluster": "cluster-1",
		"remote-cluster": "cluster-2",
	}
	clusterNames := []string{"cluster-1", "cluster-2"}
	clusterMap := map[string]int{
		"cluster-1": 1,
		"cluster-2": 2,
	}
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	pairWorkerSliceGateway := &workerv1alpha1.WorkerSliceGatewayList{}
	clientMock.On("List", ctx, pairWorkerSliceGateway, mock.Anything, client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		arg.Items = []workerv1alpha1.WorkerSliceGateway{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{""},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-1",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{""},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-2",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
		}
	}).Once()
	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Twice()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	cluster := &controllerv1alpha1.Cluster{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), cluster).Return(nil).Twice()
	gateway := &workerv1alpha1.WorkerSliceGateway{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), gateway).Return(nil).Twice()
	//clientMock.On("Create", ctx, gateway).Return(nil).Times(4)
	//environment := make(map[string]string, 5)
	//jobMock.On("CreateJob", ctx, requestObj.Namespace, "image", environment).Return(ctrl.Result{}, nil).Once()

	result, err := workerSliceGatewayService.CreateMinimumWorkerSliceGateways(ctx, "red", clusterNames, requestObj.Namespace, label, clusterMap, "10.10.10.10/16", "/16")
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testCreateMinimumWorkerSliceGatewaysNotExists(t *testing.T) {
	_, _, jobMock, workerSliceGatewayService, requestObj, clientMock, _, ctx, mMock := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	label := map[string]string{
		"worker-cluster": "cluster-1",
		"remote-cluster": "cluster-2",
	}
	clusterNames := []string{"cluster-1", "cluster-2"}
	clusterMap := map[string]int{
		"cluster-1": 1,
		"cluster-2": 2,
	}
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Times(3)
	pairWorkerSliceGateway := &workerv1alpha1.WorkerSliceGatewayList{}
	clientMock.On("List", ctx, pairWorkerSliceGateway, mock.Anything, client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		arg.Items = []workerv1alpha1.WorkerSliceGateway{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{""},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-1",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{""},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-2",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
		}
	}).Once()
	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Twice()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Twice()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Twice()
	cluster := &controllerv1alpha1.Cluster{}
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), cluster).Return(nil).Twice()
	gateway := &workerv1alpha1.WorkerSliceGateway{}
	notFoundError := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceTest"}, "isNotFound")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), gateway).Return(notFoundError).Once()
	clientMock.On("Create", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	clientMock.On("Create", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	jobMock.On("CreateJob", ctx, mock.Anything, JobImage, mock.Anything).Return(ctrl.Result{}, nil).Once()
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := workerSliceGatewayService.CreateMinimumWorkerSliceGateways(ctx, "red", clusterNames, requestObj.Namespace, label, clusterMap, "10.10.10.10/16", "/16")
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testDeleteWorkerSliceGatewaysByLabelExists(t *testing.T) {
	_, _, _, workerSliceGatewayService, requestObj, clientMock, _, ctx, mMock := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	label := map[string]string{
		"worker-cluster": "cluster-1",
		"remote-cluster": "cluster-2",
	}
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	pairWorkerSliceGateway := &workerv1alpha1.WorkerSliceGatewayList{}
	clientMock.On("List", ctx, pairWorkerSliceGateway, mock.Anything, client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		arg.Items = []workerv1alpha1.WorkerSliceGateway{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{""},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-1",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{""},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-2",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
		}
	}).Once()
	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Twice()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	err := workerSliceGatewayService.DeleteWorkerSliceGatewaysByLabel(ctx, label, "namespace")
	require.NoError(t, nil)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testNodeIpReconciliationOfWorkerSliceGatewaysExists(t *testing.T) {
	_, _, _, workerSliceGatewayService, requestObj, clientMock, _, ctx, _ := setupWorkerSliceGatewayTest("slice_gateway", "namespace")
	pairWorkerSliceGateway := &workerv1alpha1.WorkerSliceGatewayList{}
	clientMock.On("List", ctx, pairWorkerSliceGateway, mock.Anything, client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		arg.Items = []workerv1alpha1.WorkerSliceGateway{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{"12"},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-1",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:          "",
					GatewayType:        "",
					GatewayHostType:    "",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{"11"},
						NodePort:      0,
						GatewayName:   "",
						ClusterName:   "cluster-2",
						VpnIp:         "",
						GatewaySubnet: "",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{},
					GatewayNumber:       0,
				},
				Status: workerv1alpha1.WorkerSliceGatewayStatus{},
			},
		}
	}).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Twice()
	cluster := controllerv1alpha1.Cluster{
		TypeMeta: k8sapimachinery.TypeMeta{},
		ObjectMeta: k8sapimachinery.ObjectMeta{
			Name:                       "cluster-1",
			GenerateName:               "",
			Namespace:                  "",
			SelfLink:                   "",
			UID:                        "",
			ResourceVersion:            "",
			Generation:                 0,
			CreationTimestamp:          k8sapimachinery.Time{},
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: nil,
			Labels:                     nil,
			Annotations:                nil,
			OwnerReferences:            nil,
			Finalizers:                 nil,
			ClusterName:                "",
			ManagedFields:              nil,
		},
		Spec:   controllerv1alpha1.ClusterSpec{},
		Status: controllerv1alpha1.ClusterStatus{},
	}
	err := workerSliceGatewayService.NodeIpReconciliationOfWorkerSliceGateways(ctx, &cluster, "namespace")
	require.NoError(t, nil)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func setupWorkerSliceGatewayTest(name string, namespace string) (*mocks.ISecretService, *mocks.IWorkerSliceConfigService, *mocks.IJobService, WorkerSliceGatewayService, ctrl.Request, *utilMock.Client, *workerv1alpha1.WorkerSliceGateway, context.Context, *metricMock.IMetricRecorder) {
	secretServiceMock := &mocks.ISecretService{}
	workerSliceConfigMock := &mocks.IWorkerSliceConfigService{}
	jobServiceMock := &mocks.IJobService{}
	mMock := &metricMock.IMetricRecorder{}
	workerSliceGatewayMock := WorkerSliceGatewayService{
		js:   jobServiceMock,
		sscs: workerSliceConfigMock,
		sc:   secretServiceMock,
		mf:   mMock,
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
	workerv1alpha1.AddToScheme(scheme)
	eventRecorder := events.NewEventRecorder(clientMock, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	workerSliceGateway := &workerv1alpha1.WorkerSliceGateway{}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, scheme, "WorkerSliceGatewayTest", &eventRecorder)
	return secretServiceMock, workerSliceConfigMock, jobServiceMock, workerSliceGatewayMock, requestObj, clientMock, workerSliceGateway, ctx, mMock
}
