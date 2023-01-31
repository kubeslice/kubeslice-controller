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
	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	utilmock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestClusterWebhookSuite(t *testing.T) {
	for k, v := range ClusterWebHookValidationTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ClusterWebHookValidationTestbed = map[string]func(*testing.T){
	"TestValidateClusterCreateFail":                         testValidateClusterCreateOtherThanProjectNamespace,
	"TestValidateClusterUpdateFailNetworkInterfaceNotEmpty": testValidateClusterUpdateFailNetworkInterfaceNotEmpty,
	"TestValidateClusterDeleteFail":                         testValidateClusterDeleteFail,
	"TestValidateClusterGeolocationFailOnCreate":            testValidateClusterGeolocationFailOnCreate,
	"TestValidateClusterGeolocationFailOnUpdate":            testValidateClusterGeolocationFailOnUpdate,
	"TestValidateClusterGeolocationPassOnCreate":            testValidateClusterGeolocationPassOnCreate,
	"TestValidateClusterGeolocationPassOnUpdate":            testValidateClusterGeolocationPassOnUpdate,
}

func testValidateClusterCreateOtherThanProjectNamespace(t *testing.T) {
	cluster := &controllerv1alpha1.Cluster{}
	actualNamespace := corev1.Namespace{}
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Namespace: cluster.Name,
		Name:      cluster.Namespace,
	}, &actualNamespace).Return(nil).Once()
	ans := ValidateClusterCreate(ctx, cluster)
	require.NotNil(t, ans)
	require.Contains(t, ans.Error(), "cluster must be applied on project namespace")
	clientMock.AssertExpectations(t)
}

func testValidateClusterUpdateFailNetworkInterfaceNotEmpty(t *testing.T) {
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	cluster1 := &controllerv1alpha1.Cluster{
		Spec: controllerv1alpha1.ClusterSpec{
			NetworkInterface: "random1",
		},
	}
	cluster2 := &controllerv1alpha1.Cluster{
		Spec: controllerv1alpha1.ClusterSpec{
			NetworkInterface: "random2",
		},
	}
	err := ValidateClusterUpdate(ctx, cluster1, runtime.Object(cluster2))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "network interface can't be changed")
	clientMock.AssertExpectations(t)
}

func testValidateClusterDeleteFail(t *testing.T) {
	cluster := &controllerv1alpha1.Cluster{}
	clientMock := &utilmock.Client{}
	workerSlice := &workerv1alpha1.WorkerSliceConfigList{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	label := map[string]string{"worker-cluster": cluster.Name}
	clientMock.On("List", ctx, workerSlice, client.MatchingLabels(label), client.InNamespace(cluster.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceConfigList)
		if arg.Items == nil {
			arg.Items = make([]workerv1alpha1.WorkerSliceConfig, 1)
		}
		arg.Items[0].ClusterName = "cisco"
	}).Once()
	err := ValidateClusterDelete(ctx, cluster)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "The cluster cannot be deleted which is participating in slice config")
	clientMock.AssertExpectations(t)
}

func testValidateClusterGeolocationFailOnCreate(t *testing.T) {
	cluster := &controllerv1alpha1.Cluster{
		Spec: controllerv1alpha1.ClusterSpec{ClusterProperty: controllerv1alpha1.ClusterProperty{GeoLocation: controllerv1alpha1.GeoLocation{CloudProvider: "", CloudRegion: "", Latitude: "1213", Longitude: "4567"}}},
	}
	actualNamespace := corev1.Namespace{}
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Namespace: cluster.Name,
		Name:      cluster.Namespace,
	}, &actualNamespace).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		arg.Labels = map[string]string{util.LabelName: fmt.Sprintf(util.LabelValue, "Project", cluster.Namespace)}
	}).Once()
	err := ValidateClusterCreate(ctx, cluster)
	require.Contains(t, err.Error(), "Latitude and longitude are not valid")
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func testValidateClusterGeolocationFailOnUpdate(t *testing.T) {
	cluster := &controllerv1alpha1.Cluster{
		Spec: controllerv1alpha1.ClusterSpec{ClusterProperty: controllerv1alpha1.ClusterProperty{GeoLocation: controllerv1alpha1.GeoLocation{CloudProvider: "", CloudRegion: "", Latitude: "1213", Longitude: "4567"}}},
	}
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	err := ValidateClusterUpdate(ctx, cluster, runtime.Object(cluster))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Latitude and longitude are not valid")
	clientMock.AssertExpectations(t)
}

func testValidateClusterGeolocationPassOnCreate(t *testing.T) {
	cluster := &controllerv1alpha1.Cluster{
		Spec: controllerv1alpha1.ClusterSpec{ClusterProperty: controllerv1alpha1.ClusterProperty{GeoLocation: controllerv1alpha1.GeoLocation{CloudProvider: "", CloudRegion: "", Latitude: "23.43345", Longitude: "83.43345"}}},
	}
	actualNamespace := corev1.Namespace{}
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: cluster.Namespace,
	}, &actualNamespace).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		arg.Labels = map[string]string{util.LabelName: fmt.Sprintf(util.LabelValue, "Project", cluster.Namespace)}
	}).Once()
	err := ValidateClusterCreate(ctx, cluster)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testValidateClusterGeolocationPassOnUpdate(t *testing.T) {
	cluster := &controllerv1alpha1.Cluster{
		Spec: controllerv1alpha1.ClusterSpec{ClusterProperty: controllerv1alpha1.ClusterProperty{GeoLocation: controllerv1alpha1.GeoLocation{CloudProvider: "", CloudRegion: "", Latitude: "23.43345", Longitude: "83.43345"}}},
	}
	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	err := ValidateClusterUpdate(ctx, cluster, runtime.Object(cluster))
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
