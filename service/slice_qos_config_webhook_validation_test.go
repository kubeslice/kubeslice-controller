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
	"fmt"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	"SliceQoSConfig_ValidateSliceQoSConfigCreatePass":                                                  ValidateSliceQoSConfigCreatePass,
	"SliceQoSConfig_ValidateSliceQoSConfigCreateFailWithInvalidNamespace":                              ValidateSliceQoSConfigCreateFailWithInvalidNamespace,
	"SliceQoSConfig_ValidateSliceQoSConfigCreateFailWithBandwidthCeilingIsLessThanBandwidthGuaranteed": ValidateSliceQoSConfigCreateFailWithBandwidthCeilingIsLessThanBandwidthGuaranteed,
	"SliceQoSConfig_ValidateSliceQoSConfigUpdatePass":                                                  ValidateSliceQoSConfigUpdatePass,
	"SliceQoSConfig_ValidateSliceQoSConfigUpdateFailWithBandwidthCeilingIsLessThanBandwidthGuaranteed": ValidateSliceQoSConfigUpdateFailWithBandwidthCeilingIsLessThanBandwidthGuaranteed,
	"SliceQoSConfig_ValidateSliceQoSConfigDeleteFail":                                                  ValidateSliceQoSConfigDeleteFail,
	"SliceQoSConfig_ValidateSliceQoSConfigDeletePass":                                                  ValidateSliceQoSConfigDeletePass,
}

func ValidateSliceQoSConfigCreatePass(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	sliceQosConfig.Spec.BandwidthCeilingKbps = 1000
	sliceQosConfig.Spec.BandwidthGuaranteedKbps = 500
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &v1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
		arg.Name = "kubeslice-cisco"
	}).Once()
	err := ValidateSliceQosConfigCreate(ctx, sliceQosConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func ValidateSliceQoSConfigCreateFailWithInvalidNamespace(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	sliceQosConfig.Spec.BandwidthCeilingKbps = 1000
	sliceQosConfig.Spec.BandwidthGuaranteedKbps = 500
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &v1.Namespace{}).Return(nil).Once()
	err := ValidateSliceQosConfigCreate(ctx, sliceQosConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "SliceQosConfig must be applied on project namespace")
	clientMock.AssertExpectations(t)
}

func ValidateSliceQoSConfigCreateFailWithBandwidthCeilingIsLessThanBandwidthGuaranteed(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	sliceQosConfig.Spec.BandwidthCeilingKbps = 1000
	sliceQosConfig.Spec.BandwidthGuaranteedKbps = 1005
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &v1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
		arg.Name = "kubeslice-cisco"
	}).Once()
	err := ValidateSliceQosConfigCreate(ctx, sliceQosConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "BandwidthGuaranteedKbps cannot be greater than BandwidthCeilingKbps")
	clientMock.AssertExpectations(t)
}

func ValidateSliceQoSConfigUpdatePass(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	sliceQosConfig.Spec.BandwidthCeilingKbps = 1000
	sliceQosConfig.Spec.BandwidthGuaranteedKbps = 500
	err := ValidateSliceQosConfigUpdate(ctx, sliceQosConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func ValidateSliceQoSConfigUpdateFailWithBandwidthCeilingIsLessThanBandwidthGuaranteed(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	sliceQosConfig.Spec.BandwidthCeilingKbps = 1000
	sliceQosConfig.Spec.BandwidthGuaranteedKbps = 1005
	err := ValidateSliceQosConfigUpdate(ctx, sliceQosConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "BandwidthGuaranteedKbps cannot be greater than BandwidthCeilingKbps")
	clientMock.AssertExpectations(t)
}

func ValidateSliceQoSConfigDeleteFail(t *testing.T) {
	name := "profile1"
	namespace := "kubeslice-cisco"
	clientMock, sliceQosConfig, ctx := setupSliceqosConfigWebhookValidationTest(name, namespace)
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSliceConfig, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		if arg.Items == nil {
			arg.Items = make([]workerv1alpha1.WorkerSliceConfig, 1)
			arg.Items[0].Name = "red-slice-1"
			arg.Items[0].Spec.SliceName = "red"
		}
	}).Once()
	err := ValidateSliceQosConfigDelete(ctx, sliceQosConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func ValidateSliceQoSConfigDeletePass(t *testing.T) {
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
