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
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSliceConfigWebhookValidationSuite(t *testing.T) {
	for k, v := range SliceConfigWebhookValidationTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var SliceConfigWebhookValidationTestBed = map[string]func(*testing.T){
	"SliceConfigWebhookValidation_CreateValidateProjectNamespaceDoesNotExist":                                                  CreateValidateProjectNamespaceDoesNotExist,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigNotInProjectNamespace":                                              CreateValidateSliceConfigNotInProjectNamespace,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigSubnetIsNotPrivate":                                                 CreateValidateSliceConfigSubnetIsNotPrivate,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigSubnetHasPrefixOtherThan16":                                         CreateValidateSliceConfigSubnetHasPrefixOtherThan16,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigSubnetHasLastTwoOctetsOtherThanZero":                                CreateValidateSliceConfigSubnetHasLastTwoOctetsOtherThanZero,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithDuplicateClusters":                                              CreateValidateSliceConfigWithDuplicateClusters,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithClusterDoesNotExist":                                            CreateValidateSliceConfigWithClusterDoesNotExist,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithNetworkInterfaceEmptyInParticipatingCluster":                    CreateValidateSliceConfigWithNetworkInterfaceEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithNodeIPEmptyInParticipatingCluster":                              CreateValidateSliceConfigWithNodeIPEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster":                           CreateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster":      CreateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling":                 CreateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether": CreateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig":    CreateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace":            CreateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters":                      CreateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithNamespaceAlreadyAssignedToOtherSlice":                           CreateValidateSliceConfigWithNamespaceAlreadyAssignedToOtherSlice,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithoutErrors":                                                      CreateValidateSliceConfigWithoutErrors,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceSubnet":                                                UpdateValidateSliceConfigUpdatingSliceSubnet,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceType":                                                  UpdateValidateSliceConfigUpdatingSliceType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceGatewayType":                                           UpdateValidateSliceConfigUpdatingSliceGatewayType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceCaType":                                                UpdateValidateSliceConfigUpdatingSliceCaType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceIpamType":                                              UpdateValidateSliceConfigUpdatingSliceIpamType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithDuplicateClusters":                                              UpdateValidateSliceConfigWithDuplicateClusters,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithClusterDoesNotExist":                                            UpdateValidateSliceConfigWithClusterDoesNotExist,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNetworkInterfaceEmptyInParticipatingCluster":                    UpdateValidateSliceConfigWithNetworkInterfaceEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNodeIPEmptyInParticipatingCluster":                              UpdateValidateSliceConfigWithNodeIPEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster":                           UpdateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster":      UpdateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling":                 UpdateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether": UpdateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig":    UpdateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace":            UpdateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters":                      UpdateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNamespaceAlreadyAssignedToOtherSlice":                           UpdateValidateSliceConfigWithNamespaceAlreadyAssignedToOtherSlice,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithoutErrors":                                                      UpdateValidateSliceConfigWithoutErrors,
	"SliceConfigWebhookValidation_DeleteValidateSliceConfigWithApplicationNamespacesNotEmpty":                                  DeleteValidateSliceConfigWithApplicationNamespacesAndAllowedNamespacesNotEmpty,
	"SliceConfigWebhookValidation_DeleteValidateSliceConfigWithOnboardedAppNamespacesNotEmpty":                                 DeleteValidateSliceConfigWithOnboardedAppNamespacesNotEmpty,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSDuplicateNamespaces":                           ValidateNamespaceIsolationProfileApplicationNSDuplicateNamespaces,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileAllowedNSDuplicateNamespaces":                               ValidateNamespaceIsolationProfileAllowedNSDuplicateNamespaces,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSClusterIsNotParticipating":                     ValidateNamespaceIsolationProfileApplicationNSClusterIsNotParticipating,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileAllowedNSClusterIsNotParticipating":                         ValidateNamespaceIsolationProfileAllowedNSClusterIsNotParticipating,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSAsteriskAndOtherCluserPresent":                 ValidateNamespaceIsolationProfileApplicationNSAsteriskAndOtherCluserPresent,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileAllowedNSAsteriskAndOtherCluserPresent":                     ValidateNamespaceIsolationProfileAllowedNSAsteriskAndOtherCluserPresent,
}

func CreateValidateProjectNamespaceDoesNotExist(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(notFoundError).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "metadata.namespace: Invalid value:")
	require.Contains(t, err.Error(), namespace)
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigNotInProjectNamespace(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "metadata.namespace: Invalid value:")
	require.Contains(t, err.Error(), namespace)
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigSubnetIsNotPrivate(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "34.2.0.0/16"
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.sliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), sliceConfig.Spec.SliceSubnet)
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigSubnetHasPrefixOtherThan16(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/32"
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.sliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), sliceConfig.Spec.SliceSubnet)
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigSubnetHasLastTwoOctetsOtherThanZero(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.1.1/16"
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.sliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), sliceConfig.Spec.SliceSubnet)
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithDuplicateClusters(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-1"}
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), sliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithClusterDoesNotExist(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(notFoundError).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), sliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithNetworkInterfaceEmptyInParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = ""
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters.NetworkInterface: Required value:")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithNodeIPEmptyInParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = ""
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters.NodeIP: Required value:")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{}
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Status.CniSubnet: Not found:")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clusterCniSubnet := "192.168.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), clusterCniSubnet)
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps = 5120
	sliceConfig.Spec.QosProfileDetails.BandwidthCeilingKbps = 4096
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.QosProfileDetails.BandwidthGuaranteedKbps: Invalid value:")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*", "cluster-1"},
		},
	}
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1", "cluster-3"},
		},
	}
	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*"},
		},
		{
			Clusters: []string{"*"},
		},
	}
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-1", "cluster-2"},
		},
	}
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}
func CreateValidateSliceConfigWithNamespaceAlreadyAssignedToOtherSlice(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-2"},
		},
	}
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	sliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps = 4096
	sliceConfig.Spec.QosProfileDetails.BandwidthCeilingKbps = 5120
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		if arg.Status.Namespaces == nil {
			arg.Status.Namespaces = make([]controllerv1alpha1.NamespacesConfig, 1)
		}
		arg.Status.Namespaces[0].Name = "randomNamespace"
		arg.Status.Namespaces[0].SliceName = "randomSlice"
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	//t.Error(t, err)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}
func CreateValidateSliceConfigWithoutErrors(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
	sliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	sliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-2"},
		},
	}
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	sliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps = 4096
	sliceConfig.Spec.QosProfileDetails.BandwidthCeilingKbps = 5120
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		if arg.Status.Namespaces == nil {
			arg.Status.Namespaces = make([]controllerv1alpha1.NamespacesConfig, 1)
		}
		arg.Status.Namespaces[0].Name = "randomNamespace"
		arg.Status.Namespaces[0].SliceName = ""
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceSubnet(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.SliceSubnet = "10.1.0.0/16"
	}).Once()
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceSubnet: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceType(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.SliceType = "TYPE_1"
	}).Once()
	newSliceConfig.Spec.SliceType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceType: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceGatewayType(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.SliceGatewayProvider.SliceGatewayType = "TYPE_1"
	}).Once()
	newSliceConfig.Spec.SliceGatewayProvider.SliceGatewayType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayType: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceCaType(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.SliceGatewayProvider.SliceCaType = "TYPE_1"
	}).Once()
	newSliceConfig.Spec.SliceGatewayProvider.SliceCaType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceCaType: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceIpamType(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.SliceIpamType = "TYPE_1"
	}).Once()
	newSliceConfig.Spec.SliceIpamType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceIpamType: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithDuplicateClusters(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-1"}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithClusterDoesNotExist(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(notFoundError).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithNetworkInterfaceEmptyInParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = ""
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters.NetworkInterface: Required value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithNodeIPEmptyInParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = ""
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters.NodeIP: Required value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Status.CniSubnet: Not found:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Spec.SliceSubnet = "192.168.0.0/16"
	}).Once()
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clusterCniSubnet := "192.168.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), clusterCniSubnet)
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps = 5120
	newSliceConfig.Spec.QosProfileDetails.BandwidthCeilingKbps = 4096
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.QosProfileDetails.BandwidthGuaranteedKbps: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*", "cluster-1"},
		},
	}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1", "cluster-3"},
		},
	}
	newSliceConfig.Spec.Clusters = []string{"cluster-1"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*"},
		},
		{
			Clusters: []string{"*"},
		},
	}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-1", "cluster-2"},
		},
	}
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithNamespaceAlreadyAssignedToOtherSlice(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-2"},
		},
	}
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	newSliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps = 4096
	newSliceConfig.Spec.QosProfileDetails.BandwidthCeilingKbps = 5120

	if newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		if arg.Status.Namespaces == nil {
			arg.Status.Namespaces = make([]controllerv1alpha1.NamespacesConfig, 1)
		}
		arg.Status.Namespaces[0].Name = "randomNamespace"
		arg.Status.Namespaces[0].SliceName = "randomSlice"
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithoutErrors(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	existingSliceConfig := controllerv1alpha1.SliceConfig{}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &existingSliceConfig).Return(nil).Once()
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-2"},
		},
	}
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	newSliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps = 4096
	newSliceConfig.Spec.QosProfileDetails.BandwidthCeilingKbps = 5120
	if newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NetworkInterface = "eth0"
		arg.Spec.NodeIP = "10.10.1.1"
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		if arg.Status.Namespaces == nil {
			arg.Status.Namespaces = make([]controllerv1alpha1.NamespacesConfig, 1)
		}
		arg.Status.Namespaces[0].Name = "randomNamespace"
		arg.Status.Namespaces[0].SliceName = ""
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func setupSliceConfigWebhookValidationTest(name string, namespace string) (*utilMock.Client, *controllerv1alpha1.SliceConfig, context.Context) {
	clientMock := &utilMock.Client{}
	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "SliceConfigWebhookValidationServiceTest")
	return clientMock, sliceConfig, ctx
}

func DeleteValidateSliceConfigWithOnboardedAppNamespacesNotEmpty(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSliceConfig, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		if arg.Items == nil {
			arg.Items = make([]workerv1alpha1.WorkerSliceConfig, 1)
		}
		if arg.Items[0].Status.OnboardedAppNamespaces == nil {
			arg.Items[0].Status.OnboardedAppNamespaces = make([]workerv1alpha1.NamespaceConfig, 2)
		}
		arg.Items[0].Status.OnboardedAppNamespaces[0].Name = "random1"
		arg.Items[0].Status.OnboardedAppNamespaces[1].Name = "random2"

	}).Once()
	err := ValidateSliceConfigDelete(ctx, newSliceConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func DeleteValidateSliceConfigWithApplicationNamespacesAndAllowedNamespacesNotEmpty(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSliceConfig, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		if arg.Items == nil {
			arg.Items = make([]workerv1alpha1.WorkerSliceConfig, 1)
		}
		if arg.Items[0].Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
			arg.Items[0].Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]string, 2)
		}
		arg.Items[0].Spec.NamespaceIsolationProfile.ApplicationNamespaces[0] = "random1"
		arg.Items[0].Spec.NamespaceIsolationProfile.ApplicationNamespaces[1] = "random2"
		if arg.Items[0].Spec.NamespaceIsolationProfile.AllowedNamespaces == nil {
			arg.Items[0].Spec.NamespaceIsolationProfile.AllowedNamespaces = make([]string, 2)
		}
		arg.Items[0].Spec.NamespaceIsolationProfile.AllowedNamespaces[0] = "random1"
		arg.Items[0].Spec.NamespaceIsolationProfile.AllowedNamespaces[1] = "random2"
	}).Once()
	err := ValidateSliceConfigDelete(ctx, newSliceConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func ValidateNamespaceIsolationProfileApplicationNSDuplicateNamespaces(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		ApplicationNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-1"},
			},
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-2"},
			},
		},
	}

	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Duplicate namespace not allowed for application namespaces")
	clientMock.AssertExpectations(t)
}

func ValidateNamespaceIsolationProfileApplicationNSClusterIsNotParticipating(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		ApplicationNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-3"},
			},
		},
	}

	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Cluster is not participating in slice config")
	clientMock.AssertExpectations(t)
}

func ValidateNamespaceIsolationProfileApplicationNSAsteriskAndOtherCluserPresent(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		ApplicationNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"*", "cluster-1"},
			},
		},
	}

	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Other clusters are not allowed when * is present")
	clientMock.AssertExpectations(t)
}

func ValidateNamespaceIsolationProfileAllowedNSDuplicateNamespaces(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		AllowedNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-1"},
			},
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-2"},
			},
		},
	}

	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Duplicate namespace not allowed for allowed namespaces")
	clientMock.AssertExpectations(t)
}

func ValidateNamespaceIsolationProfileAllowedNSClusterIsNotParticipating(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		AllowedNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-3"},
			},
		},
	}

	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Cluster is not participating in slice config")
	clientMock.AssertExpectations(t)
}

func ValidateNamespaceIsolationProfileAllowedNSAsteriskAndOtherCluserPresent(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		AllowedNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"*", "cluster-1"},
			},
		},
	}

	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Other clusters are not allowed when * is present")
	clientMock.AssertExpectations(t)
}
