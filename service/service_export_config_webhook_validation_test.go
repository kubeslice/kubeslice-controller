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
	"testing"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceExportConfigWebhookValidationSuite(t *testing.T) {
	for k, v := range ServiceExportConfigWebhookValidationTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ServiceExportConfigWebhookValidationTestBed = map[string]func(*testing.T){
	"TestValidateServiceExportConfigCreate_DoesNotExist":                  testValidateServiceExportConfigCreateDoesNotExist,
	"TestValidateServiceExportConfigCreate_NotInProjectNamespace":         testValidateServiceExportConfigCreateNotInProjectNamespace,
	"TestValidateServiceExportConfigCreate_IfSliceNotExist":               testValidateServiceExportConfigCreateIfSliceNotExist,
	"TestValidateServiceExportConfigCreate_IfClusterNotExist":             testValidateServiceExportConfigCreateIfClusterNotExist,
	"TestValidateServiceExportConfigCreate_IfClusterNotPresentInSlice":    testValidateServiceExportConfigCreateIfClusterNotPresentInSlice,
	"TestValidateServiceExportConfigCreate_ServiceEndpointInvalidSlice":   testValidateServiceExportConfigCreateServiceEndpointInvalidSlice,
	"TestValidateServiceExportConfigCreate_ServiceEndpointInvalidCluster": testValidateServiceExportConfigCreateServiceEndpointInvalidCluster,
	"TestValidateServiceExportConfigUpdate_ServiceEndpointInvalidCluster": testValidateServiceExportConfigUpdateServiceEndpointInvalidCluster,
	"TestValidateServiceExportConfigUpdate_DoesNotExist":                  testValidateServiceExportConfigUpdateDoesNotExist,
}

func testValidateServiceExportConfigCreateDoesNotExist(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	notFoundError := k8sError.NewNotFound(util.Resource("ServiceExportConfigWebhookValidationServiceTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(notFoundError).Once()
	err := ValidateServiceExportConfigCreate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "metadata.namespace: Invalid value:")
	require.Contains(t, err.Error(), namespace)
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigCreateNotInProjectNamespace(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "SomeOther", namespace)
	}).Once()
	err := ValidateServiceExportConfigCreate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "metadata.namespace: Invalid value:")
	require.Contains(t, err.Error(), namespace)
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigCreateIfSliceNotExist(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
	}).Once()
	cluster := &controllerv1alpha1.Cluster{}
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, sliceConfig).Return(notFoundError).Once()
	err := ValidateServiceExportConfigCreate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceName: Invalid value:")
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigCreateIfClusterNotExist(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
	}).Once()
	cluster := &controllerv1alpha1.Cluster{}
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(notFoundError).Once()
	clientMock.On("Get", ctx, mock.Anything, sliceConfig).Return(nil).Once()
	err := ValidateServiceExportConfigCreate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SourceCluster: Invalid value:")
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigCreateIfClusterNotPresentInSlice(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	serviceExportConfig.Spec.SourceCluster = "cluster"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
	}).Once()
	cluster := &controllerv1alpha1.Cluster{}
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		agr := args.Get(2).(*controllerv1alpha1.SliceConfig)
		agr.Spec.Clusters = []string{"cluster1", "cluster2"}
	}).Once()
	err := ValidateServiceExportConfigCreate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Cluster: Invalid value:")
	require.Contains(t, err.Error(), serviceExportConfig.Spec.SourceCluster)
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigCreateServiceEndpointInvalidSlice(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	serviceExportConfig.Spec.SourceCluster = "cluster"
	serviceExportConfig.Spec.ServiceDiscoveryEndpoints = []controllerv1alpha1.ServiceDiscoveryEndpoint{
		{
			PodName: "pod1",
			Cluster: "cluster1",
			NsmIp:   "10.10.12.23",
			DnsName: "nsm-service-dns-name",
			Port:    0,
		},
	}
	serviceExportConfig.Spec.SourceCluster = "cluster"
	serviceExportConfig.Spec.SliceName = "slice"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
	}).Once()
	cluster := &controllerv1alpha1.Cluster{}
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(nil).Twice()
	clientMock.On("Get", ctx, mock.Anything, sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		agr := args.Get(2).(*controllerv1alpha1.SliceConfig)
		agr.Spec.Clusters = []string{"cluster1", "cluster2"}
	}).Once()
	clientMock.On("Get", ctx, mock.Anything, sliceConfig).Return(notFoundError).Once()
	err := ValidateServiceExportConfigCreate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceName: Invalid value:")
	require.Contains(t, err.Error(), serviceExportConfig.Spec.SliceName)
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigCreateServiceEndpointInvalidCluster(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	serviceExportConfig.Spec.SourceCluster = "cluster"
	serviceExportConfig.Spec.ServiceDiscoveryEndpoints = []controllerv1alpha1.ServiceDiscoveryEndpoint{
		{
			PodName: "pod1",
			Cluster: "cluster1",
			NsmIp:   "10.10.12.23",
			DnsName: "nsm-service-dns-name",
			Port:    0,
		},
	}
	serviceExportConfig.Spec.SourceCluster = "cluster"
	serviceExportConfig.Spec.SliceName = "slice"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
	}).Once()
	cluster := &controllerv1alpha1.Cluster{}
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		agr := args.Get(2).(*controllerv1alpha1.SliceConfig)
		agr.Spec.Clusters = []string{"cluster1", "cluster2"}
	}).Twice()
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(notFoundError).Once()
	err := ValidateServiceExportConfigCreate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ServiceDiscoveryEndpoints.Cluster: Invalid value:")
	require.Contains(t, err.Error(), serviceExportConfig.Spec.ServiceDiscoveryEndpoints[0].Cluster)
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigUpdateServiceEndpointInvalidCluster(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	serviceExportConfig.Spec.SourceCluster = "cluster"
	serviceExportConfig.Spec.ServiceDiscoveryEndpoints = []controllerv1alpha1.ServiceDiscoveryEndpoint{
		{
			PodName: "pod1",
			Cluster: "cluster1",
			NsmIp:   "10.10.12.23",
			DnsName: "nsm-service-dns-name",
			Port:    0,
		},
	}
	serviceExportConfig.Spec.SourceCluster = "cluster"
	serviceExportConfig.Spec.SliceName = "slice"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Name = namespace
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", namespace)
	}).Once()
	cluster := &controllerv1alpha1.Cluster{}
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, sliceConfig).Return(nil).Run(func(args mock.Arguments) {
		agr := args.Get(2).(*controllerv1alpha1.SliceConfig)
		agr.Spec.Clusters = []string{"cluster1", "cluster2"}
	}).Twice()
	clientMock.On("Get", ctx, mock.Anything, cluster).Return(notFoundError).Once()
	err := ValidateServiceExportConfigUpdate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ServiceDiscoveryEndpoints.Cluster: Invalid value:")
	require.Contains(t, err.Error(), serviceExportConfig.Spec.ServiceDiscoveryEndpoints[0].Cluster)
	clientMock.AssertExpectations(t)
}

func testValidateServiceExportConfigUpdateDoesNotExist(t *testing.T) {
	name := "service_export_config"
	namespace := "namespace"
	clientMock, serviceExportConfig, ctx := setupServiceExportConfigWebhookValidationTest(name, namespace)
	notFoundError := k8sError.NewNotFound(util.Resource("ServiceExportConfigWebhookValidationServiceTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(notFoundError).Once()
	err := ValidateServiceExportConfigUpdate(ctx, serviceExportConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "metadata.namespace: Invalid value:")
	require.Contains(t, err.Error(), namespace)
	clientMock.AssertExpectations(t)
}

func setupServiceExportConfigWebhookValidationTest(name string, namespace string) (*utilMock.Client, *controllerv1alpha1.ServiceExportConfig, context.Context) {
	clientMock := &utilMock.Client{}
	serviceExportConfig := &controllerv1alpha1.ServiceExportConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "ServiceExportConfigWebhookValidationServiceTest")
	return clientMock, serviceExportConfig, ctx
}
