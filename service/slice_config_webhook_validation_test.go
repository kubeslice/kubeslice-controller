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
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithParticipatingClusterRegistrationInProgress":                     CreateValidateSliceConfigWithParticipatingClusterRegistrationInProgress,
	"SliceConfigWebhookValidation_CreateValidateSliceConfigWithParticipatingClusterHealthNotNormal":                            CreateValidateSliceConfigWithParticipatingClusterHealthNotNormal,
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
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNewClustersDoesNotExist":                                        UpdateValidateSliceConfigWithNewClustersDoesNotExist,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNewClusterUnhealthy":                                            UpdateValidateSliceConfigWithNewClusterUnhealthy,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigWithNewClusterRegistrationPending":                                  UpdateValidateSliceConfigWithNewClusterRegistrationPending,
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
	"SliceConfigWebhookValidation_UpdateValidateSliceGatewayServiceType":                                                       UpdateValidateSliceConfig_PreventUpdate_SliceGatewayServiceType,
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
	"TestValidateCertsRotationInterval_Positive":                                                                               TestValidateCertsRotationInterval_Positive,
	"TestValidateCertsRotationInterval_Negative":                                                                               TestValidateCertsRotationInterval_Negative,
	"TestValidateCertsRotationInterval_inProgressClusterStatus":                                                                TestValidateCertsRotationInterval_NegativeClusterStatus,
	"TestValidateCertsRotationInterval_PositiveClusterStatus":                                                                  TestValidateCertsRotationInterval_PositiveClusterStatus,
	"TestValidateRotationInterval_Change_Decreased":                                                                            TestValidateRotationInterval_Change_Decreased,
	"TestValidateRotationInterval_Change_Increased":                                                                            TestValidateRotationInterval_Change_Increased,
	"TestValidateRotationInterval_NoChange":                                                                                    TestValidateRotationInterval_NoChange,
	"SliceConfigWebhookValidation_UpdateValidateSliceConfigUpdatingVPNCipher":                                                  UpdateValidateSliceConfigUpdatingVPNCipher,
	"Test_validateSlicegatewayServiceType":                                                                                     test_validateSlicegatewayServiceType,
}

func test_validateSlicegatewayServiceType(t *testing.T) {
	name := "test-slice"
	namespace := "test-ns"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	// if defined, cluster name can't be empty
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{
		SliceGatewayType: "some",
		SliceCaType:      "value",
	}
	// slicegatewayServiceType definition is optional
	err := validateSlicegatewayServiceType(ctx, sliceConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)

	sliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Type:     "Loadbalancer",
			Protocol: "UDP",
		},
	}
	err = validateSlicegatewayServiceType(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayServiceType: Invalid value:")
	require.Contains(t, err.Error(), "Cluster name can't be empty")
	clientMock.AssertExpectations(t)

	// if defined, cluster name should be part of slice
	sliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster:  "demo-cluster",
			Type:     "Loadbalancer",
			Protocol: "UDP",
		},
	}
	err = validateSlicegatewayServiceType(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayServiceType: Invalid value:")
	require.Contains(t, err.Error(), "Cluster is not participating in slice config")
	clientMock.AssertExpectations(t)

	// shouldn't define service config for same cluster
	sliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster:  "demo-cluster",
			Type:     "Loadbalancer",
			Protocol: "UDP",
		},
		{
			Cluster:  "demo-cluster",
			Type:     "Nodeport",
			Protocol: "TCP",
		},
	}
	sliceConfig.Spec.Clusters = []string{"demo-cluster"}
	err = validateSlicegatewayServiceType(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayServiceType: Invalid value:")
	require.Contains(t, err.Error(), "Duplicate entries for same cluster are not allowed")
	clientMock.AssertExpectations(t)

	// happy scenario: all check passes
	sliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster:  "demo-cluster",
			Type:     "Loadbalancer",
			Protocol: "UDP",
		},
	}
	sliceConfig.Spec.Clusters = []string{"demo-cluster"}
	err = validateSlicegatewayServiceType(ctx, sliceConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)

	// happy scenario with wild card: all check passes
	sliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster:  "*",
			Type:     "Loadbalancer",
			Protocol: "UDP",
		},
	}
	sliceConfig.Spec.Clusters = []string{"demo-cluster", "c2", "cx"}
	err = validateSlicegatewayServiceType(ctx, sliceConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfig_PreventUpdate_SliceGatewayServiceType(t *testing.T) {
	name := "test-slice"
	namespace := "test-ns"
	oldSliceConfig := controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: controllerv1alpha1.SliceConfigSpec{
			SliceGatewayProvider: &controllerv1alpha1.WorkerSliceGatewayProvider{
				SliceGatewayServiceType: []controllerv1alpha1.SliceGatewayServiceType{
					{
						Cluster: "c1",
						Type:    "LoadBalancer",
					},
				},
			},
			Clusters: []string{"c1"},
		},
	}
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	newSliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster: "c1",
			Type:    "NodePort",
		},
	}
	newSliceConfig.Spec.Clusters = []string{"c1"}
	// loadbalancer to nodeport not allowed
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayServiceType: Forbidden:")
	require.Contains(t, err.Error(), "updating gateway service type is not allowed")
	clientMock.AssertExpectations(t)

	// tcp to udp & vice-versa not allowed
	oldSliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster:  "c1",
			Protocol: "TCP",
		},
	}
	newSliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster:  "c1",
			Protocol: "UDP",
		},
	}
	require.NotNil(t, err)
	_, err = ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayServiceType: Forbidden:")
	require.Contains(t, err.Error(), "updating gateway protocol is not allowed")
	clientMock.AssertExpectations(t)

	// if no protocol is defined default value is assumed to be UDP, thus protocol can't be set to TCP afterwards
	oldSliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{}
	newSliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType = []controllerv1alpha1.SliceGatewayServiceType{
		{
			Cluster:  "c1",
			Protocol: "TCP",
		},
	}
	require.NotNil(t, err)
	_, err = ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayServiceType: Forbidden:")
	require.Contains(t, err.Error(), "updating gateway protocol is not allowed")
	clientMock.AssertExpectations(t)
}

func CreateValidateProjectNamespaceDoesNotExist(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: namespace,
	}, &corev1.Namespace{}).Return(notFoundError).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	sliceConfig.Spec.MaxClusters = 16
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	clusterCniSubnet := "48.2.0.0/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.MaxClusters = 16
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.MaxClusters = 16
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters[0]: Invalid value:")
	require.Contains(t, err.Error(), sliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithParticipatingClusterRegistrationInProgress(t *testing.T) {
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusInProgress
	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters[0]: Invalid value: \"cluster-1\": cluster registration is not completed.")
	clientMock.AssertExpectations(t)
}

func CreateValidateSliceConfigWithParticipatingClusterHealthNotNormal(t *testing.T) {
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusWarning,
		}
	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters[0]: Invalid value: \"cluster-1\": cluster health is not normal")
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}

	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Status.NodeIPs: Not found:")
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.CniSubnet = []string{}
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.MaxClusters = 16
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType:               "SomeType",
		BandwidthGuaranteedKbps: 5120,
		BandwidthCeilingKbps:    4096,
	}
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.MaxClusters = 16
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	sliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*", "cluster-1"},
		},
	}
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.MaxClusters = 16
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.MaxClusters = 16
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
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
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.MaxClusters = 16
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	sliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
	sliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
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
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceSubnet(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.SliceSubnet = "192.168.1.0/16"
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceSubnet: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingVPNCipher(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.SliceSubnet = "192.168.1.0/16"
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-128-CBC",
	}
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceSubnet = "192.168.1.0/16"
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.VPNConfig.Cipher: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	oldSliceConfig.Spec.SliceType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceType = "TYPE_2"
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceType: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceGatewayType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	oldSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	oldSliceConfig.Spec.SliceGatewayProvider.SliceGatewayType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	newSliceConfig.Spec.SliceGatewayProvider.SliceGatewayType = "TYPE_2"
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceGatewayType: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceCaType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	oldSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	oldSliceConfig.Spec.SliceGatewayProvider.SliceCaType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	newSliceConfig.Spec.SliceGatewayProvider.SliceCaType = "TYPE_2"
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.SliceGatewayProvider.SliceCaType: Invalid value:")
	require.Contains(t, err.Error(), "cannot be updated")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigUpdatingSliceIpamType(t *testing.T) {
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	oldSliceConfig.Spec.SliceIpamType = "TYPE_1"
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceIpamType = "TYPE_2"
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(&oldSliceConfig))
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
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	t.Log(err.Error())
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters[0]: Invalid value:")
	require.Contains(t, err.Error(), "cluster is not registered")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[0])
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithNewClustersDoesNotExist(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	oldSliceConfig := newSliceConfig.DeepCopy()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2", "cluster-3"}
	notFoundError := k8sError.NewNotFound(util.Resource("SliceConfigWebhookValidationTest"), "isNotFound")
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[2],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(notFoundError).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(oldSliceConfig))
	t.Log(err.Error())
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), "cluster is not registered")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[2])
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithNewClusterRegistrationPending(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clusterCniSubnet := "10.10.1.1/16"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	oldSliceConfig := newSliceConfig.DeepCopy()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2", "cluster-3"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[2],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusInProgress
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(oldSliceConfig))
	t.Log(err.Error())
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), "cluster registration is not completed")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[2])
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithNewClusterUnhealthy(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clusterCniSubnet := "10.10.1.1/16"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	oldSliceConfig := newSliceConfig.DeepCopy()
	newSliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2", "cluster-3"}
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[2],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusWarning,
		}
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(oldSliceConfig))
	t.Log(err.Error())
	require.Contains(t, err.Error(), "Spec.Clusters: Invalid value:")
	require.Contains(t, err.Error(), "cluster health is not normal")
	require.Contains(t, err.Error(), newSliceConfig.Spec.Clusters[2])
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Status.NodeIPs: Not found")
	clientMock.AssertExpectations(t)
}
func UpdateValidateSliceConfigWithNodeIPsInSpecIsSuccess(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	clusterCniSubnet := "10.10.1.1/16"
	newSliceConfig.Spec.Clusters = []string{"cluster-1"}
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
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
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func UpdateValidateSliceConfigWithNodeIPsInStatusIsSuccess(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	clusterCniSubnet := "10.10.1.1/16"
	newSliceConfig.Spec.Clusters = []string{"cluster-1"}
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
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
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.QosProfileDetails.BandwidthGuaranteedKbps: Invalid value:")
	require.Contains(t, err.Error(), "BandwidthGuaranteedKbps cannot be greater than BandwidthCeilingKbps")
	clientMock.AssertExpectations(t)
}

func UpdateValidateSliceConfigWithExternalGatewayConfigClusterHasAsteriskAndOtherClusterTogether(t *testing.T) {
	name := "slice_config"
	namespace := "namespace"
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{
		SliceGatewayType: "SomeType",
		SliceCaType:      "SomeType",
	}
	newSliceConfig.Spec.ExternalGatewayConfig = []controllerv1alpha1.ExternalGatewayConfig{
		{
			Clusters: []string{"*", "cluster-1"},
		},
	}
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	newSliceConfig.Spec.MaxClusters = 2
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	newSliceConfig.Spec.QosProfileDetails = &controllerv1alpha1.QOSProfile{
		QueueType: "SomeType",
	}
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.MaxClusters = 2
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.MaxClusters = 2
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
	clusterCniSubnet := "10.10.1.1/16"
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.NetworkPresent = true
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
	newSliceConfig.Spec.SliceSubnet = "192.168.0.0/16"
	newSliceConfig.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{}
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
		arg.Status.NetworkPresent = true
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.NetworkPresent = true
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
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
	_, err := ValidateSliceConfigDelete(ctx, newSliceConfig)
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
	_, err := ValidateSliceConfigDelete(ctx, newSliceConfig)
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
	_, err := ValidateSliceConfigDelete(ctx, newSliceConfig)
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
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
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.CniSubnet = []string{clusterCniSubnet}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = true
	}).Once()
	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(newSliceConfig))
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
func TestValidateCertsRotationInterval_Positive(t *testing.T) {
	now := metav1.Now()
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.RenewBefore = &now
	expiry := metav1.Now().Add(30)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: &now,
			CertificateExpiryTime:   &metav1.Time{Time: expiry},
		}
	}).Once()

	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.Nil(t, err)
}

func TestValidateCertsRotationInterval_Negative(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	// RenewBefore is 1 hour after, decline
	renewBefore := metav1.Time{Time: metav1.Now().Add(time.Hour * 1)}
	sliceConfig.Spec.RenewBefore = &renewBefore
	expiry := metav1.Now().Add(30)
	now := metav1.Now()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: &now,
			CertificateExpiryTime:   &metav1.Time{Time: expiry},
		}
	}).Once()
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.NotNil(t, err)
}

func TestValidateCertsRotationInterval_NegativeClusterStatus(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	now := metav1.Now()
	sliceConfig.Spec.RenewBefore = &now
	expiry := metav1.Now().Add(30)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: &now,
			CertificateExpiryTime:   &metav1.Time{Time: expiry},
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
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.NotNil(t, err)
	require.Equal(t, err.Type, field.ErrorTypeForbidden)
}

func TestValidateCertsRotationInterval_PositiveClusterStatus(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	now := metav1.Now()
	sliceConfig.Spec.RenewBefore = &now
	expiry := metav1.Now().Add(30)

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			CertificateCreationTime: &now,
			CertificateExpiryTime:   &metav1.Time{expiry},
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
	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	oldSliceConfig := controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	err := validateRenewNowInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.Nil(t, err)
}

// rotationInterval updates TC
func TestValidateRotationInterval_NoChange(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.RotationInterval = 30

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:        name,
			RotationInterval: 30,
		}
	}).Once()
	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	oldSliceConfig := controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			RotationInterval: 30,
		},
	}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	_, err := validateRotationIntervalInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.Nil(t, err)
}
func TestValidateRotationInterval_Change_Increased(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	sliceConfig.Spec.RotationInterval = 45
	now := metav1.Now()
	expiry := metav1.Now().Add(30)

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			RotationInterval:        30,
			CertificateCreationTime: &now,
			CertificateExpiryTime:   &metav1.Time{expiry},
		}
	}).Once()
	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	oldSliceConfig := controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			RotationInterval: 30,
		},
	}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	expectedResp := metav1.NewTime(now.AddDate(0, 0, 45).Add(-1 * time.Hour))
	gotResp, err := validateRotationIntervalInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.Nil(t, err)
	require.Equal(t, &expectedResp, gotResp.Spec.CertificateExpiryTime)
}
func TestValidateRotationInterval_Change_Decreased(t *testing.T) {
	name := "slice_config"
	namespace := "randomNamespace"
	clientMock, sliceConfig, ctx := setupSliceConfigWebhookValidationTest(name, namespace)
	// new interval
	sliceConfig.Spec.RotationInterval = 30
	now := metav1.Now()
	expiry := metav1.Now().Add(45)

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.ObjectMeta = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		}
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               name,
			RotationInterval:        45,
			CertificateCreationTime: &now,
			CertificateExpiryTime:   &metav1.Time{expiry},
		}
	}).Once()
	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	oldSliceConfig := controllerv1alpha1.SliceConfig{
		Spec: controllerv1alpha1.SliceConfigSpec{
			RotationInterval: 45,
		},
	}
	oldSliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}
	expectedResp := metav1.NewTime(now.AddDate(0, 0, 30).Add(-1 * time.Hour))
	gotResp, err := validateRotationIntervalInSliceConfig(ctx, sliceConfig, &oldSliceConfig)
	require.Nil(t, err)
	require.Equal(t, &expectedResp, gotResp.Spec.CertificateExpiryTime)
}

func TestValidateSliceConfigCreate_NoNetHappyCase(t *testing.T) {
	name := "slice_config"
	namespace := "random"
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
	sliceConfig.Spec.OverlayNetworkDeploymentMode = controllerv1alpha1.NONET
	sliceConfig.Spec.Clusters = []string{"cluster-1", "cluster-2"}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	sliceConfig.Spec.MaxClusters = 2
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = false
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[1],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = false
	}).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = false
	}).Once()

	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func TestValidateSliceConfigCreate_SingleNetWithNoNetCLustersError(t *testing.T) {
	name := "slice_config"
	namespace := "random"
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
	sliceConfig.Spec.OverlayNetworkDeploymentMode = controllerv1alpha1.SINGLENET
	sliceConfig.Spec.Clusters = []string{"cluster-1"}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces = make([]controllerv1alpha1.SliceNamespaceSelection, 1)
	}
	if sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters == nil {
		sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = make([]string, 1)
	}
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Namespace = "randomNamespace"
	sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces[0].Clusters = []string{"cluster-1"}
	sliceConfig.Spec.MaxClusters = 2
	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      sliceConfig.Spec.Clusters[0],
		Namespace: namespace,
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = []string{"10.10.1.1"}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
		arg.Status.NetworkPresent = false
	}).Once()

	_, err := ValidateSliceConfigCreate(ctx, sliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "cluster network is not present")
	clientMock.AssertExpectations(t)
}
func TestValidateSliceConfigUpdate_SingleToNoNetError(t *testing.T) {
	oldSliceConfig := &controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.OverlayNetworkDeploymentMode = controllerv1alpha1.SINGLENET
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest("slice_config", "random")
	newSliceConfig.Spec.OverlayNetworkDeploymentMode = controllerv1alpha1.NONET

	_, err := ValidateSliceConfigUpdate(ctx, newSliceConfig, runtime.Object(oldSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Forbidden: Slice cannot be transitioned to no-network mode from single-network mode")
	clientMock.AssertExpectations(t)

}

func TestValidateClustersOnUpdate_NetworkSliceOnNoNetClustersError(t *testing.T) {
	oldSliceConfig := &controllerv1alpha1.SliceConfig{}
	oldSliceConfig.Spec.OverlayNetworkDeploymentMode = controllerv1alpha1.SINGLENET
	clientMock, newSliceConfig, ctx := setupSliceConfigWebhookValidationTest("slice_config", "random")
	newSliceConfig.Spec.OverlayNetworkDeploymentMode = controllerv1alpha1.SINGLENET
	newSliceConfig.Spec.Clusters = []string{"cluster-1"}

	clientMock.On("Get", ctx, client.ObjectKey{
		Name:      newSliceConfig.Spec.Clusters[0],
		Namespace: "random",
	}, &controllerv1alpha1.Cluster{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Status.NodeIPs = []string{"1.1.1.1"}
		arg.Status.NetworkPresent = false
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusRegistered
		arg.Status.ClusterHealth = &controllerv1alpha1.ClusterHealth{
			ClusterHealthStatus: controllerv1alpha1.ClusterHealthStatusNormal,
		}
	}).Twice()

	err := validateClustersOnUpdate(ctx, newSliceConfig, oldSliceConfig)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "cluster network is not present")
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
	sliceConfig.Spec.VPNConfig = &controllerv1alpha1.VPNConfiguration{
		Cipher: "AES-256-CBC",
	}

	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "SliceConfigWebhookValidationServiceTest", nil)
	return clientMock, sliceConfig, ctx
}

func TestValidateTopologyConfig_FullMesh(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyFullMesh,
	}
	clusters := []string{"c1", "c2", "c3"}

	err := validateTopologyConfig(topology, clusters)
	require.Nil(t, err)
}

func TestValidateTopologyConfig_CustomMatrix(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyCustom,
		ConnectivityMatrix: []controllerv1alpha1.ConnectivityEntry{
			{SourceCluster: "c1", TargetClusters: []string{"c2", "c3"}},
		},
	}
	clusters := []string{"c1", "c2", "c3"}

	err := validateTopologyConfig(topology, clusters)
	require.Nil(t, err)
}

func TestValidateTopologyConfig_CustomEmptyMatrix(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType:       controllerv1alpha1.TopologyCustom,
		ConnectivityMatrix: []controllerv1alpha1.ConnectivityEntry{},
	}
	clusters := []string{"c1", "c2"}

	err := validateTopologyConfig(topology, clusters)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "required for custom topology")
}

func TestValidateTopologyConfig_InvalidClusterInMatrix(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyCustom,
		ConnectivityMatrix: []controllerv1alpha1.ConnectivityEntry{
			{SourceCluster: "invalid", TargetClusters: []string{"c2"}},
		},
	}
	clusters := []string{"c1", "c2"}

	err := validateTopologyConfig(topology, clusters)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
}

func TestValidateTopologyConfig_InvalidForbiddenEdge(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyRestricted,
		ForbiddenEdges: []controllerv1alpha1.ForbiddenEdge{
			{SourceCluster: "invalid", TargetClusters: []string{"c1"}},
		},
	}
	clusters := []string{"c1", "c2"}

	err := validateTopologyConfig(topology, clusters)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
}

func TestValidateTopologyConfig_InvalidType(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: "invalid-type",
	}
	clusters := []string{"c1", "c2"}

	err := validateTopologyConfig(topology, clusters)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "must be one of")
}

func TestValidateTopologyConfig_RestrictedIsolatedClusters(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyRestricted,
		ForbiddenEdges: []controllerv1alpha1.ForbiddenEdge{
			{SourceCluster: "c1", TargetClusters: []string{"c2", "c3"}},
			{SourceCluster: "c2", TargetClusters: []string{"c1", "c3"}},
			{SourceCluster: "c3", TargetClusters: []string{"c1", "c2"}},
		},
	}
	clusters := []string{"c1", "c2", "c3"}

	err := validateTopologyConfig(topology, clusters)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "isolated clusters")
}

func TestValidateTopologyConfig_RestrictedPartiallyConnected(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyRestricted,
		ForbiddenEdges: []controllerv1alpha1.ForbiddenEdge{
			{SourceCluster: "c1", TargetClusters: []string{"c3"}},
		},
	}
	clusters := []string{"c1", "c2", "c3"}

	err := validateTopologyConfig(topology, clusters)
	require.Nil(t, err)
}

func TestValidateTopologyConfig_NilTopology(t *testing.T) {
	err := validateTopologyConfig(nil, []string{"c1", "c2"})
	require.Nil(t, err)
}

func TestValidateTopologyConfig_EmptyTopologyType(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: "",
	}
	err := validateTopologyConfig(topology, []string{"c1", "c2"})
	require.Nil(t, err)
}

func TestValidateCustomTopology_InvalidTargetCluster(t *testing.T) {
	matrix := []controllerv1alpha1.ConnectivityEntry{
		{SourceCluster: "c1", TargetClusters: []string{"invalid"}},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateCustomTopology(matrix, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
}

func TestValidateCustomTopology_InvalidSourceCluster(t *testing.T) {
	matrix := []controllerv1alpha1.ConnectivityEntry{
		{SourceCluster: "invalid", TargetClusters: []string{"c2"}},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateCustomTopology(matrix, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
}

func TestValidateCustomTopology_MultipleInvalidTargets(t *testing.T) {
	matrix := []controllerv1alpha1.ConnectivityEntry{
		{SourceCluster: "c1", TargetClusters: []string{"c2", "invalid1", "invalid2"}},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateCustomTopology(matrix, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
}

func TestValidateRestrictedTopology_EmptyForbiddenEdges(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType:   controllerv1alpha1.TopologyRestricted,
		ForbiddenEdges: []controllerv1alpha1.ForbiddenEdge{},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateRestrictedTopology(topology, clusterSet, basePath)
	require.Nil(t, err)
}

func TestValidateRestrictedTopology_SingleCluster(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType:   controllerv1alpha1.TopologyRestricted,
		ForbiddenEdges: []controllerv1alpha1.ForbiddenEdge{},
	}
	clusterSet := map[string]struct{}{"c1": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateRestrictedTopology(topology, clusterSet, basePath)
	require.Nil(t, err)
}

func TestValidateRestrictedTopology_FullyDisconnected(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyRestricted,
		ForbiddenEdges: []controllerv1alpha1.ForbiddenEdge{
			{SourceCluster: "c1", TargetClusters: []string{"c2"}},
			{SourceCluster: "c2", TargetClusters: []string{"c1"}},
		},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateRestrictedTopology(topology, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "isolated clusters")
}

func TestValidateRestrictedTopology_FourClustersOneIsolated(t *testing.T) {
	topology := &controllerv1alpha1.TopologyConfig{
		TopologyType: controllerv1alpha1.TopologyRestricted,
		ForbiddenEdges: []controllerv1alpha1.ForbiddenEdge{
			{SourceCluster: "c1", TargetClusters: []string{"c4"}},
			{SourceCluster: "c2", TargetClusters: []string{"c4"}},
			{SourceCluster: "c3", TargetClusters: []string{"c4"}},
			{SourceCluster: "c4", TargetClusters: []string{"c1", "c2", "c3"}},
		},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}, "c3": {}, "c4": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateRestrictedTopology(topology, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "isolated clusters")
}

func TestValidateForbiddenEdges_EmptyEdges(t *testing.T) {
	edges := []controllerv1alpha1.ForbiddenEdge{}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateForbiddenEdges(edges, clusterSet, basePath)
	require.Nil(t, err)
}

func TestValidateForbiddenEdges_MultipleTargets(t *testing.T) {
	edges := []controllerv1alpha1.ForbiddenEdge{
		{SourceCluster: "c1", TargetClusters: []string{"c2", "c3"}},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}, "c3": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateForbiddenEdges(edges, clusterSet, basePath)
	require.Nil(t, err)
}

func TestValidateForbiddenEdges_InvalidSourceCluster(t *testing.T) {
	edges := []controllerv1alpha1.ForbiddenEdge{
		{SourceCluster: "invalid", TargetClusters: []string{"c2"}},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateForbiddenEdges(edges, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
}

func TestValidateForbiddenEdges_InvalidTargetCluster(t *testing.T) {
	edges := []controllerv1alpha1.ForbiddenEdge{
		{SourceCluster: "c1", TargetClusters: []string{"invalid"}},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateForbiddenEdges(edges, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
}

func TestValidateForbiddenEdges_InvalidTargetInList(t *testing.T) {
	edges := []controllerv1alpha1.ForbiddenEdge{
		{SourceCluster: "c1", TargetClusters: []string{"c2", "invalid", "c3"}},
	}
	clusterSet := map[string]struct{}{"c1": {}, "c2": {}, "c3": {}}
	basePath := field.NewPath("spec", "topologyConfig")

	err := validateForbiddenEdges(edges, clusterSet, basePath)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "not in spec.clusters")
	require.Contains(t, err.Error(), "targetClusters[1]")
}

func TestBuildForbiddenSetStatic_MultipleEdgesMultipleTargets(t *testing.T) {
	edges := []controllerv1alpha1.ForbiddenEdge{
		{SourceCluster: "c1", TargetClusters: []string{"c2", "c3"}},
		{SourceCluster: "c2", TargetClusters: []string{"c4"}},
	}

	forbidden := buildForbiddenSetStatic(edges)
	require.Len(t, forbidden, 3)
	require.True(t, forbidden["c1-c2"])
	require.True(t, forbidden["c1-c3"])
	require.True(t, forbidden["c2-c4"])
}

func TestBuildForbiddenSetStatic_Empty(t *testing.T) {
	forbidden := buildForbiddenSetStatic([]controllerv1alpha1.ForbiddenEdge{})
	require.Empty(t, forbidden)
}

