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
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSliceQoSConfigSuite(t *testing.T) {
	for k, v := range SliceQoSConfigTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var SliceQoSConfigTestbed = map[string]func(*testing.T){
	"SliceQoSConfig_ReconciliationCompleteHappyCase": SliceQoSConfigReconciliationCompleteHappyCase,
	"SliceQoSConfig_GetObjectErrorOtherThanNotFound": SliceQoSConfigGetObjectErrorOtherThanNotFound,
	"SliceQoSConfig_GetObjectErrorNotFound":          SliceQoSConfigGetObjectErrorNotFound,
	"SliceQoSConfig_DeleteTheObjectHappyCase":        SliceQoSConfigDeleteTheObjectHappyCase,
	"SliceQoSConfig_ObjectNamespaceNotFound":         SliceQoSConfigObjectNamespaceNotFound,
	"SliceQoSConfig_ObjectNotInProjectNamespace":     SliceQoSConfigObjectNotInProjectNamespace,
}

func SliceQoSConfigReconciliationCompleteHappyCase(t *testing.T) {
	workerSliceConfigMock, requestObj, clientMock, sliceQosConfig, ctx, sliceQosConfigService := setupSliceQoSConfigTest("qos_profile_1", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, sliceQosConfig).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	namespace := corev1.Namespace{}
	clientMock.On("Get", ctx, mock.Anything, &namespace).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = requestObj.Namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
	}).Once()
	workerSliceConfigs := workerv1alpha1.WorkerSliceConfigList{
		Items: []workerv1alpha1.WorkerSliceConfig{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-slice-config-1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-slice-config-2",
				},
			},
		},
	}
	workerSliceConfigMock.On("ListWorkerSliceConfigs", ctx, mock.Anything, requestObj.Namespace).Return(workerSliceConfigs.Items, nil).Once()
	for i := 0; i < len(workerSliceConfigs.Items); i++ {
		clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	}
	result, err := sliceQosConfigService.ReconcileSliceQoSConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerSliceConfigMock.AssertExpectations(t)
}

func SliceQoSConfigGetObjectErrorOtherThanNotFound(t *testing.T) {
	_, requestObj, clientMock, sliceQosConfig, ctx, sliceQosConfigService := setupSliceQoSConfigTest("qos_profile_1", "namespace")
	err1 := errors.New("internal_error")
	clientMock.On("Get", ctx, requestObj.NamespacedName, sliceQosConfig).Return(err1).Once()
	result, err2 := sliceQosConfigService.ReconcileSliceQoSConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func SliceQoSConfigGetObjectErrorNotFound(t *testing.T) {
	_, requestObj, clientMock, sliceQosConfig, ctx, sliceQosConfigService := setupSliceQoSConfigTest("qos_profile_1", "namespace")
	notFoundError := k8sError.NewNotFound(util.Resource("SliceQoSConfigTest"), "isNotFound")
	clientMock.On("Get", ctx, requestObj.NamespacedName, sliceQosConfig).Return(notFoundError).Once()
	result, err2 := sliceQosConfigService.ReconcileSliceQoSConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err2)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func SliceQoSConfigDeleteTheObjectHappyCase(t *testing.T) {
	workerSliceConfigMock, requestObj, clientMock, sliceQosConfig, ctx, sliceQosConfigService := setupSliceQoSConfigTest("qos_profile_1", "namespace")
	time := metav1.Now()
	clientMock.On("Get", ctx, requestObj.NamespacedName, sliceQosConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceQoSConfig)
		arg.ObjectMeta.DeletionTimestamp = &time
	}).Once()
	//remove finalizer
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	result, err := sliceQosConfigService.ReconcileSliceQoSConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	workerSliceConfigMock.AssertExpectations(t)
}

func SliceQoSConfigObjectNamespaceNotFound(t *testing.T) {
	_, requestObj, clientMock, sliceQosConfig, ctx, sliceQosConfigService := setupSliceQoSConfigTest("qos_profile_1", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, sliceQosConfig).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	notFoundError := k8sError.NewNotFound(util.Resource("SliceQoSConfigTest"), "isNotFound")
	namespace := corev1.Namespace{}
	clientMock.On("Get", ctx, mock.Anything, &namespace).Return(notFoundError).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = requestObj.Namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
	}).Once()
	result, err := sliceQosConfigService.ReconcileSliceQoSConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func SliceQoSConfigObjectNotInProjectNamespace(t *testing.T) {
	_, requestObj, clientMock, sliceQosConfig, ctx, sliceQosConfigService := setupSliceQoSConfigTest("qos_profile_1", "namespace")
	clientMock.On("Get", ctx, requestObj.NamespacedName, sliceQosConfig).Return(nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	namespace := corev1.Namespace{}
	clientMock.On("Get", ctx, mock.Anything, &namespace).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = requestObj.Namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "SomeOther", requestObj.Namespace)
	}).Once()
	result, err := sliceQosConfigService.ReconcileSliceQoSConfig(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
}

func setupSliceQoSConfigTest(name string, namespace string) (*mocks.IWorkerSliceConfigService, ctrl.Request, *utilMock.Client, *controllerv1alpha1.SliceQoSConfig, context.Context, SliceQoSConfigService) {
	workerSliceConfigMock := &mocks.IWorkerSliceConfigService{}
	sliceQosConfigService := SliceQoSConfigService{
		wsc: workerSliceConfigMock,
	}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	requestObj := ctrl.Request{
		NamespacedName: namespacedName,
	}
	clientMock := &utilMock.Client{}
	sliceQosConfig := &controllerv1alpha1.SliceQoSConfig{}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "SliceQoSConfigServiceTest")
	return workerSliceConfigMock, requestObj, clientMock, sliceQosConfig, ctx, sliceQosConfigService
}
