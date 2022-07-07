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
	"testing"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSliceqosConfigSuite(t *testing.T) {
	for k, v := range sliceqosConfigTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var sliceqosConfigTestBed = map[string]func(*testing.T){
	"TestValidateSliceqosConfigCreatepass": testValidateSliceqosConfigCreatepass,
	"TestValidateSliceqosConfigCreatefail": testValidateSliceqosConfigCreatefail,
	"TestValidateSliceqosConfigDeletepass": testValidateSliceqosConfigDeletepass,
	"TestValidateSliceqosConfigDeletefail": testValidateSliceqosConfigDeletefail,
}

func testValidateSliceqosConfigCreatepass(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	err := ValidateSliceQosConfigCreate(ctx, sliceQosConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}
func testValidateSliceqosConfigCreatefail(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	namespaceErr := errors.New("Project Namespace error")
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(namespaceErr).Once()
	err := ValidateSliceQosConfigCreate(ctx, sliceQosConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testValidateSliceqosConfigDeletefail(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSliceConfig, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		if arg.Items == nil {
			arg.Items = make([]workerv1alpha1.WorkerSliceConfig, 1)
			arg.Items[0].Name = "red-slice-1"
		}
	}).Once()
	err := ValidateSliceQosConfigDelete(ctx, sliceQosConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}
func testValidateSliceqosConfigDeletepass(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSliceConfig, mock.Anything, mock.Anything).Return(nil).Once()
	err := ValidateSliceQosConfigDelete(ctx, sliceQosConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func setupSliceqosConfigWebhookValidationTest(name string, namespace string) (*utilMock.Client, *controllerv1alpha1.SliceQoSConfig, context.Context) {
	clientMock := &utilMock.Client{}
	sliceQoSConfig := &controllerv1alpha1.SliceQoSConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "SliceqosConfigWebhookValidationTest")
	return clientMock, sliceQoSConfig, ctx
}
