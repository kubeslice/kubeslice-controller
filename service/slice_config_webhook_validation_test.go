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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

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
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithNodeIPsEmptyInParticipatingCluster":                             CreateValidateSliceConfigWithNodeIPsEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster":                           CreateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster":      CreateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling":                 CreateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether": CreateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig":    CreateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace":            CreateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters":                      CreateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithoutErrors":                                                      CreateValidateSliceConfigWithoutErrors,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceSubnet":                                                UpdateValidateSliceConfigUpdatingSliceSubnet,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceType":                                                  UpdateValidateSliceConfigUpdatingSliceType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceGatewayType":                                           UpdateValidateSliceConfigUpdatingSliceGatewayType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceCaType":                                                UpdateValidateSliceConfigUpdatingSliceCaType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingSliceIpamType":                                              UpdateValidateSliceConfigUpdatingSliceIpamType,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithDuplicateClusters":                                              UpdateValidateSliceConfigWithDuplicateClusters,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithClusterDoesNotExist":                                            UpdateValidateSliceConfigWithClusterDoesNotExist,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNodeIPsEmptyInSpecAndStatusForParticipatingCluster":             UpdateValidateSliceConfigWithNodeIPsEmptyInSpecAndStatusForParticipatingCluster,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNodeIPsInSpecIsSuccess":                                         UpdateValidateSliceConfigWithNodeIPsInSpecIsSuccess,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNodeIPsInStatusIsSuccess":                                       UpdateValidateSliceConfigWithNodeIPsInStatusIsSuccess,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster":                           UpdateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster":      UpdateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling":                 UpdateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether": UpdateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig":    UpdateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace":            UpdateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters":                      UpdateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithoutErrors":                                                      UpdateValidateSliceConfigWithoutErrors,
	"SliceConfigWebhookValidation_DeleteValidateSliceConfigWithApplicationNamespacesNotEmpty":                                  DeleteValidateSliceConfigWithApplicationNamespacesAndAllowedNamespacesNotEmpty,
	"SliceConfigWebhookValidation_DeleteValidateSliceConfigWithOnboardedAppNamespacesNotEmpty":                                 DeleteValidateSliceConfigWithOnboardedAppNamespacesNotEmpty,
	"SliceConfigWebhookValidation_validateAllowedNamespacesWithDuplicateClusters":                                              ValidateAllowedNamespacesWithDuplicateClusters,
	"SliceConfigWebhookValidation_validateApplicationNamespacesWithNamespaceAlreadyAcquiredByotherSlice":                       ValidateApplicationNamespacesWithNamespaceAlreadyAcquiredByotherSlice,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSDuplicateNamespaces":                           ValidateNamespaceIsolationProfileApplicationNSDuplicateNamespaces,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileAllowedNSDuplicateNamespaces":                               ValidateNamespaceIsolationProfileAllowedNSDuplicateNamespaces,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSClusterIsNotParticipating":                     ValidateNamespaceIsolationProfileApplicationNSClusterIsNotParticipating,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileAllowedNSClusterIsNotParticipating":                         ValidateNamespaceIsolationProfileAllowedNSClusterIsNotParticipating,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSAsteriskAndOtherCluserPresent":                 ValidateNamespaceIsolationProfileApplicationNSAsteriskAndOtherCluserPresent,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileAllowedNSAsteriskAndOtherCluserPresent":                     ValidateNamespaceIsolationProfileAllowedNSAsteriskAndOtherCluserPresent,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSNamespaceEmpty":                                ValidateNamespaceIsolationProfileAllowedNSNamespaceEmpty,
	"SliceConfigWebhookValidation_ValidateNamespaceIsolationProfileApplicationNSNamespaceSpecialCharacter":                     ValidateNamespaceIsolationProfileApplicationNSNamespaceSpecialCharacter,
	"SliceConfigWebhookValidationValidateSliceConfigCreateWithErrorInNSIsolationProfile":                                       ValidateSliceConfigCreateWithErrorInNSIsolationProfile,
	"SliceConfigWebhookValidationValidateSliceConfigUpdateWithErrorInNSIsolationProfile":                                       ValidateSliceConfigUpdateWithErrorInNSIsolationProfile,
	"SliceConfigWebhookValidation_DeleteValidateSliceConfigWithServiceExportsNotEmpty":                                         DeleteValidateSliceConfigWithServiceExportsNotEmpty,
	"SliceConfigWebhookValidation_ValidateQosProfileBothStandardQosProfileNameAndQosProfileDetailsPresent":                     ValidateQosProfileBothStandardQosProfileNameAndQosProfileDetailsPresent,
	"SliceConfigWebhookValidation_ValidateQosProfileBothStandardQosProfileNameAndQosProfileDetailsNotPresent":                  ValidateQosProfileBothStandardQosProfileNameAndQosProfileDetailsNotPresent,
	"SliceConfigWebhookValidation_ValidateQosProfileStandardQosProfileNameDoesNotExist":                                        ValidateQosProfileStandardQosProfileNameDoesNotExist,
	"SliceConfigWebhookValidation_ValidateMaxCluster":                                                                          ValidateMaxCluster,
	"SliceConfigWebhookValidation_ValidateMaxClusterForParticipatingCluster":                                                   ValidateMaxClusterForParticipatingCluster,
	"TestValidateCertsRotationInterval_Postive":                                                                               TestValidateCertsRotationInterval_Postive,
	"TestValidateCertsRotationInterval_Negative":                                                                               TestValidateCertsRotationInterval_Negative,
	"TestValidateCertsRotationInterval_inProgressClusterStatus":                                                                TestValidateCertsRotationInterval_NegativeClusterStatus,
	"TestValidateCertsRotationInterval_PositiveClusterStatus":                                                                 TestValidateCertsRotationInterval_PositiveClusterStatus,
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
	require.Contains(t, err.Error(), "Spec.Clusters: Duplicate value:")
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

func CreateValidateSliceConfigWithNodeIPsEmptyInParticipatingCluster(t *testing.T) {
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
		arg.Spec.NodeIPs = []string{}
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters.NodeIPs: Required value:")
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
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
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
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
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
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType:               "SomeType",
		BandwidthGuaranteedKbps: 5120,
		BandwidthCeilingKbps:    4096,
	}
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
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
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
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
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
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
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
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Duplicate value:")
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
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType:               "SomeType",
		BandwidthGuaranteedKbps: 4096,
		BandwidthCeilingKbps:    5120,
	}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	sliceConfig.Spec.MaxClusters = 2
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
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
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.SliceSubnet = "192.168.1.0/16"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.SliceType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceType: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceGatewayType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.SliceGatewayProvider.SliceGatewayType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceGatewayProvider.SliceGatewayType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayType: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceCaType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.SliceGatewayProvider.SliceCaType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceGatewayProvider.SliceCaType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceCaType: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceIpamType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.SliceIpamType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceIpamType = "TYPE_2"
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceIpamType: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithDuplicateClusters(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-1"}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Duplicate value:")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithClusterDoesNotExist(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(notFoundError).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	t.Log(err.Error())
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), "cluster is not registered")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithNodeIPsEmptyInSpecAndStatusForParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	clusterCniSubnet := "10.10.1.1/16"
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters.NodeIPs: Required value:")
	clientMock.AssertExpectations(t)
}
func UpdateValidateSliceConfigWithNodeIPsInSpecIsSuccess(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	clusterCniSubnet := "10.10.1.1/16"
	newSliceConfig.Spec.Clusters = []string{"cluster-1"}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType:               "SomeType",
		BandwidthGuaranteedKbps: 5120,
		BandwidthCeilingKbps:    5122,
	}
	newSliceConfig.Spec.MaxClusters = 2
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"11.11.14.114"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func UpdateValidateSliceConfigWithNodeIPsInStatusIsSuccess(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	clusterCniSubnet := "10.10.1.1/16"
	newSliceConfig.Spec.Clusters = []string{"cluster-1"}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType:               "SomeType",
		BandwidthGuaranteedKbps: 5120,
		BandwidthCeilingKbps:    5122,
	}
	newSliceConfig.Spec.MaxClusters = 2
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Status.NodeIPs = []string{"11.11.14.114"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithCniSubnetEmptyInParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Status.CniSubnet: Not found:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithOverlappingSliceSubnetWithCniSubnetOfParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clusterCniSubnet := "192.168.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), "must not overlap with CniSubnet")
	require.Contains(t, err.Error(), clusterCniSubnet)
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithBandwidthGuaranteedGreaterThanBandwidthCeiling(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType:               "SomeType",
		BandwidthGuaranteedKbps: 5120,
		BandwidthCeilingKbps:    4096,
	}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.QosProfileDetails.BandwidthGuaranteedKbps: Invalid value:")
	require.Contains(t, err.Error(), "BandwidthGuaranteedKbps cannot be greater than BandwidthCeilingKbps")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*", "cluster-1"},
		},
	}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	require.Contains(t, err.Error(), "other clusters are not allowed when * is present")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigClusterIsNotParticipatingInSliceConfig(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
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
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	require.Contains(t, err.Error(), "cluster is not participating in slice config")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigHasAsterisksInMoreThanOnePlace(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*"},
		},
		{
			Clusters: []string{"*"},
		},
	}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Invalid value:")
	require.Contains(t, err.Error(), "* is not allowed in more than one external gateways")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigHasDuplicateClusters(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-1", "cluster-2"},
		},
	}
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.ExternalGatewayConfig.Clusters: Duplicate value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithoutErrors(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"cluster-1"},
		},
		{
			Clusters: []string{"cluster-2"},
		},
	}
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType:               "SomeType",
		BandwidthGuaranteedKbps: 4096,
		BandwidthCeilingKbps:    5120,
	}
	if newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	newSliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	newSliceConfig.Spec.MaxClusters = 16
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
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
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
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
	require.Contains(t, err.Error(), "Deboarding of namespaces is in progress, please try after some time.")
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
	require.Contains(t, err.Error(), "Please deboard the namespaces before deletion of slice.")
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func DeleteValidateSliceConfigWithServiceExportsNotEmpty(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfigList{}
	clientMock.On("List", ctx, workerSliceConfig, mock.Anything, mock.Anything).Return(nil).Once()
	clientMock.On("List", ctx, &controllerv1alpha1.ServiceExportConfigList{}, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.ServiceExportConfigList)
		if arg.Items == nil {
			arg.Items = make([]controllerv1alpha1.ServiceExportConfig, 1)
		}
		arg.Items[0].Spec.SliceName = name
		if arg.Items[0].Labels == nil {
			arg.Items[0].Labels = make(map[string]string, 1)
		}
		arg.Items[0].Labels["original-slice-name"] = name
	}).Once()
	err := ValidateSliceConfigDelete(ctx, newSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "The SliceConfig can only be deleted after all the service export configs are deleted for the slice.")
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
	require.Contains(t, err.Error(), "NamespaceIsolationProfile.ApplicationNamespaces.Namespace: Duplicate value:")
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
	require.Contains(t, err.Error(), "NamespaceIsolationProfile.AllowedNamespaces.Namespace: Duplicate value:")
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

func ValidateNamespaceIsolationProfileAllowedNSNamespaceEmpty(t *testing.T) {
	name := "slice_config"
	namespace := ""
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		AllowedNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-1"},
			},
		},
	}
	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func ValidateNamespaceIsolationProfileApplicationNSNamespaceSpecialCharacter(t *testing.T) {
	name := "slice_config"
	namespace := "namespace&"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		ApplicationNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-1"},
			},
		},
	}
	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	err := validateNamespaceIsolationProfile(sliceConfig)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func ValidateSliceConfigCreateWithErrorInNSIsolationProfile(t *testing.T) {
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
	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		AllowedNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"*", "cluster-1"},
			},
		},
	}
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Other clusters are not allowed when * is present")
	clientMock.AssertExpectations(t)
}

func ValidateSliceConfigUpdateWithErrorInNSIsolationProfile(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	clusterCniSubnet := "10.10.1.1/16"
	newSliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		AllowedNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"*", "cluster-1"},
			},
		},
	}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
	}).Once()
	err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Other clusters are not allowed when * is present")
	clientMock.AssertExpectations(t)
}

func ValidateAllowedNamespacesWithDuplicateClusters(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		AllowedNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-3", "cluster-3", "cluster-4", "cluster-4"},
			},
		},
	}
	err := validateAllowedNamespaces(sliceConfig)
	require.Contains(t, err.Error(), "Spec.NamespaceIsolationProfile.AllowedNamespaces.Clusters: Duplicate value")
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func ValidateApplicationNamespacesWithNamespaceAlreadyAcquiredByotherSlice(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.NamespaceIsolationProfile = controllerv1alpha1.NamespaceIsolationProfile{
		ApplicationNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
			{
				Namespace: namespace,
				Clusters:  []string{"cluster-3"},
			},
		},
	}
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
	err := validateApplicationNamespaces(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "The given namespace: randomNamespace in cluster")
	require.Contains(t, err.Error(), "is already acquired by other slice")
	clientMock.AssertExpectations(t)
}

func ValidateQosProfileBothStandardQosProfileNameAndQosProfileDetailsPresent(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.StandardQosProfileName = "testQos"
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "someType",
	}
	err := validateQosProfile(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "StandardQosProfileName cannot be set when QosProfileDetails is set")
	clientMock.AssertExpectations(t)
}

func ValidateQosProfileBothStandardQosProfileNameAndQosProfileDetailsNotPresent(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	err := validateQosProfile(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Either StandardQosProfileName or QosProfileDetails is required")
	clientMock.AssertExpectations(t)
}

func ValidateQosProfileStandardQosProfileNameDoesNotExist(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.StandardQosProfileName = "testQos"
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Once()
	err := validateQosProfile(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "SliceQoSConfig not found.")
	clientMock.AssertExpectations(t)
}

func ValidateMaxCluster(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.StandardQosProfileName = "testQos"
	sliceConfig.Spec.MaxClusters = 1
	err := validateMaxClusterCount(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "MaxClusterCount cannot be less than 2 or greater than 32.")
	clientMock.AssertExpectations(t)
}

func ValidateMaxClusterForParticipatingCluster(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, _ := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.StandardQosProfileName = "testQos"
	sliceConfig.Spec.MaxClusters = 2
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2", "cluster-3"}
	err := validateMaxClusterCount(sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "participating clusters cannot be greater than MaxClusterCount")
	clientMock.AssertExpectations(t)
}
func TestValidateCertsRotationInterval_Postive(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.RenewBefore = time.Now()
	expiry := metav1.Now().Add(30)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: metav1.Now(),
			CertificateExpiryTime:   metav1.Time{expiry},
		}
	}).Once()
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.Nil(t, err)
}

func TestValidateCertsRotationInterval_Negative(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	// RenewBefore is 1 hour after, decline
	sliceConfig.Spec.RenewBefore = time.Now().Add(time.Hour * 1)
	expiry := metav1.Now().Add(30)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: metav1.Now(),
			CertificateExpiryTime:   metav1.Time{expiry},
		}
	}).Once()
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.NotNil(t, err)
}

func TestValidateCertsRotationInterval_NegativeClusterStatus(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.RenewBefore = time.Now()
	expiry := metav1.Now().Add(30)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: metav1.Now(),
			CertificateExpiryTime:   metav1.Time{expiry},
			ClusterGatewayMapping: map[string][]string{
				"cluster-1": {"gateway-1"},
				"cluster-2": {"gateway-2"},
			},
		}
		arg.Status = controllerv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]controllerv1alpha1.StatusOfKeyRotation{

				"gateway-1": controllerv1alpha1.StatusOfKeyRotation{
					Status:               controllerv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Now(),
				},
				"gateway-2": controllerv1alpha1.StatusOfKeyRotation{
					Status:               controllerv1alpha1.InProgress,
					LastUpdatedTimestamp: metav1.Now(),
				},
			},
		}
	}).Once()
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.NotNil(t, err)
	require.Equal(t, err.Type, field.ErrorTypeForbidden)
}

func TestValidateCertsRotationInterval_PositiveClusterStatus(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)

	sliceConfig.Spec.RenewBefore = time.Now()
	expiry := metav1.Now().Add(30)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: metav1.Now(),
			CertificateExpiryTime:   metav1.Time{expiry},
			ClusterGatewayMapping: map[string][]string{
				"cluster-1": {"gateway-1"},
				"cluster-2": {"gateway-2"},
			},
		}
		arg.Status = controllerv1alpha1.VpnKeyRotationStatus{
			CurrentRotationState: map[string]controllerv1alpha1.StatusOfKeyRotation{

				"gateway-1": controllerv1alpha1.StatusOfKeyRotation{
					Status:               controllerv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Now(),
				},
				"gateway-2": controllerv1alpha1.StatusOfKeyRotation{
					Status:               controllerv1alpha1.Complete,
					LastUpdatedTimestamp: metav1.Now(),
				},
			},
		}
	}).Once()
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.Nil(t, err)
}
func setupSliceConfigWebhookValidationTest(name string, namespace string) (*utilMock.Client, *controllerv1alpha1.SliceConfig, context.Context) {
	clientMock := &utilMock.Client{}
	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "SliceConfigWebhookValidationServiceTest", nil)
	return clientMock, sliceConfig, ctx
}
