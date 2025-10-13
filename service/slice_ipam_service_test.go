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
	"testing"
	"time"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/metrics"
	metricMock "github.com/kubeslice/kubeslice-controller/metrics/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilmock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"

	"github.com/dailymotion/allure-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSliceIpamSuite(t *testing.T) {
	for k, v := range SliceIpamTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var SliceIpamTestbed = map[string]func(*testing.T){
	"TestReconcileSliceIpamNotFound":               testReconcileSliceIpamNotFound,
	"TestReconcileSliceIpamSuccessful":             testReconcileSliceIpamSuccessful,
	"TestReconcileSliceIpamDeletion":               testReconcileSliceIpamDeletion,
	"TestAllocateSubnetForClusterSuccess":          testAllocateSubnetForClusterSuccess,
	"TestAllocateSubnetForClusterNotFound":         testAllocateSubnetForClusterNotFound,
	"TestAllocateSubnetForClusterAlreadyAllocated": testAllocateSubnetForClusterAlreadyAllocated,
	"TestReleaseSubnetForClusterSuccess":           testReleaseSubnetForClusterSuccess,
	"TestReleaseSubnetForClusterNotFound":          testReleaseSubnetForClusterNotFound,
	"TestGetClusterSubnetSuccess":                  testGetClusterSubnetSuccess,
	"TestGetClusterSubnetNotFound":                 testGetClusterSubnetNotFound,
	"TestCreateSliceIpamSuccess":                   testCreateSliceIpamSuccess,
	"TestDeleteSliceIpamSuccess":                   testDeleteSliceIpamSuccess,
	"TestHandleSliceIpamDeletionWithActiveAllocs":  testHandleSliceIpamDeletionWithActiveAllocs,
	"TestHandleSliceIpamDeletionNoActiveAllocs":    testHandleSliceIpamDeletionNoActiveAllocs,
	"TestReconcileSliceIpamStateNewResource":       testReconcileSliceIpamStateNewResource,
	"TestReconcileSliceIpamStateExistingResource":  testReconcileSliceIpamStateExistingResource,
	"TestCleanupExpiredAllocations":                testCleanupExpiredAllocations,
	"TestCleanupExpiredAllocationsNoExpired":       testCleanupExpiredAllocationsNoExpired,
}

func testReconcileSliceIpamNotFound(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	sliceIpamName := types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "test-slice-ipam",
	}
	requestObj := ctrl.Request{
		NamespacedName: sliceIpamName,
	}

	clientMock := &utilmock.Client{}
	sliceIpam := &v1alpha1.SliceIpam{}
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, nil)

	clientMock.On("Get", ctx, requestObj.NamespacedName, sliceIpam).Return(kubeerrors.NewNotFound(util.Resource("SliceIpamTest"), "slice ipam not found"))

	result, err := sliceIpamService.ReconcileSliceIpam(ctx, requestObj)

	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testReconcileSliceIpamSuccessful(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	sliceIpamName := types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "test-slice-ipam",
	}
	requestObj := ctrl.Request{
		NamespacedName: sliceIpamName,
	}

	clientMock := &utilmock.Client{}
	scheme := runtime.NewScheme()
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, scheme)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-slice-ipam",
			Namespace:  "test-namespace",
			Finalizers: []string{SliceIpamFinalizer},
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.0.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			TotalSubnets: 0, // Will trigger calculation
		},
	}

	// Metrics chaining: WithProject returns a MetricRecorder, not the mock
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})
	// syncWithSliceConfig will attempt to Get the corresponding SliceConfig; return NotFound to skip sync
	clientMock.On("Get", ctx, types.NamespacedName{Name: sliceIpam.Name, Namespace: sliceIpam.Namespace}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(kubeerrors.NewNotFound(util.Resource("SliceConfig"), "sliceconfig not found"))
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil)

	result, err := sliceIpamService.ReconcileSliceIpam(ctx, requestObj)

	require.True(t, result.RequeueAfter > 0) // Should requeue after some time
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileSliceIpamDeletion(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	sliceIpamName := types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "test-slice-ipam",
	}
	requestObj := ctrl.Request{
		NamespacedName: sliceIpamName,
	}

	clientMock := &utilmock.Client{}
	scheme := runtime.NewScheme()
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, scheme)

	deletionTime := metav1.Now()
	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-slice-ipam",
			Namespace:         "test-namespace",
			DeletionTimestamp: &deletionTime,
			Finalizers:        []string{SliceIpamFinalizer},
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.0.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{}, // No active allocations
		},
	}

	// Metrics chaining: WithProject returns a MetricRecorder, not the mock
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})
	// syncWithSliceConfig is not called when DeletionTimestamp is set, so no SliceConfig Get needed
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil)

	result, err := sliceIpamService.ReconcileSliceIpam(ctx, requestObj)

	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testAllocateSubnetForClusterSuccess(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	scheme := runtime.NewScheme()
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, scheme)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.0.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{},
			AvailableSubnets: 256,
			TotalSubnets:     256,
		},
	}

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil)

	subnet, err := sliceIpamService.AllocateSubnetForCluster(ctx, "test-slice", "test-cluster", "test-namespace")

	require.NoError(t, err)
	require.NotEmpty(t, subnet)
	require.Contains(t, subnet, "10.0.") // Should be from the slice subnet
	clientMock.AssertExpectations(t)
}

func testAllocateSubnetForClusterNotFound(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, nil)

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(kubeerrors.NewNotFound(util.Resource("SliceIpam"), "slice ipam not found"))

	subnet, err := sliceIpamService.AllocateSubnetForCluster(ctx, "test-slice", "test-cluster", "test-namespace")

	require.Error(t, err)
	require.Empty(t, subnet)
	require.Contains(t, err.Error(), "not found")
	clientMock.AssertExpectations(t)
}

func testAllocateSubnetForClusterAlreadyAllocated(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	scheme := runtime.NewScheme()
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, scheme)

	existingSubnet := "10.0.1.0/24"
	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.0.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "test-cluster",
					Subnet:      existingSubnet,
					AllocatedAt: metav1.Now(),
					Status:      "Allocated",
				},
			},
			AvailableSubnets: 255,
			TotalSubnets:     256,
		},
	}

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})

	subnet, err := sliceIpamService.AllocateSubnetForCluster(ctx, "test-slice", "test-cluster", "test-namespace")

	require.NoError(t, err)
	require.Equal(t, existingSubnet, subnet)
	clientMock.AssertExpectations(t)
}

func testReleaseSubnetForClusterSuccess(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	scheme := runtime.NewScheme()
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, scheme)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.0.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "test-cluster",
					Subnet:      "10.0.1.0/24",
					AllocatedAt: metav1.Now(),
					Status:      "Allocated",
				},
			},
			AvailableSubnets: 255,
			TotalSubnets:     256,
		},
	}

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})
	clientMock.On("Update", ctx, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil)

	err := sliceIpamService.ReleaseSubnetForCluster(ctx, "test-slice", "test-cluster", "test-namespace")

	require.NoError(t, err)
	clientMock.AssertExpectations(t)
}

func testReleaseSubnetForClusterNotFound(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	scheme := runtime.NewScheme()
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, scheme)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.0.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{}, // No allocations
			AvailableSubnets: 256,
			TotalSubnets:     256,
		},
	}

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})

	err := sliceIpamService.ReleaseSubnetForCluster(ctx, "test-slice", "test-cluster", "test-namespace")

	require.Error(t, err)
	require.Contains(t, err.Error(), "no active subnet allocation found")
	clientMock.AssertExpectations(t)
}

func testGetClusterSubnetSuccess(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, nil)

	expectedSubnet := "10.0.1.0/24"
	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "test-cluster",
					Subnet:      expectedSubnet,
					AllocatedAt: metav1.Now(),
					Status:      "Allocated",
				},
			},
		},
	}

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})

	subnet, err := sliceIpamService.GetClusterSubnet(ctx, "test-slice", "test-cluster", "test-namespace")

	require.NoError(t, err)
	require.Equal(t, expectedSubnet, subnet)
	clientMock.AssertExpectations(t)
}

func testGetClusterSubnetNotFound(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, nil)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{}, // No allocations
		},
	}

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})

	subnet, err := sliceIpamService.GetClusterSubnet(ctx, "test-slice", "test-cluster", "test-namespace")

	require.Error(t, err)
	require.Empty(t, subnet)
	require.Contains(t, err.Error(), "no active subnet allocation found")
	clientMock.AssertExpectations(t)
}

func testCreateSliceIpamSuccess(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	scheme := runtime.NewScheme()
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, scheme)

	sliceConfig := &v1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.SliceConfigSpec{
			SliceSubnet: "10.0.0.0/16",
		},
	}
	// CreateSliceIpam first checks if SliceIpam exists; return not found
	clientMock.On("Get", ctx, types.NamespacedName{Name: sliceConfig.Name, Namespace: sliceConfig.Namespace}, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(kubeerrors.NewNotFound(util.Resource("SliceIpam"), "not found"))
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil)

	err := sliceIpamService.CreateSliceIpam(ctx, sliceConfig)

	require.NoError(t, err)
	clientMock.AssertExpectations(t)
}

func testDeleteSliceIpamSuccess(t *testing.T) {
	mMock := &metricMock.IMetricRecorder{}
	sliceIpamService := NewSliceIpamService(mMock)

	clientMock := &utilmock.Client{}
	ctx := prepareSliceIpamTestContext(context.Background(), clientMock, nil)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
	}

	key := types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}
	clientMock.On("Get", ctx, key, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*v1alpha1.SliceIpam)
		*arg = *sliceIpam
	})
	clientMock.On("Delete", ctx, mock.AnythingOfType("*v1alpha1.SliceIpam")).Return(nil)

	err := sliceIpamService.DeleteSliceIpam(ctx, "test-slice", "test-namespace")

	require.NoError(t, err)
	clientMock.AssertExpectations(t)
}

func prepareSliceIpamTestContext(ctx context.Context, client client.Client, scheme *runtime.Scheme) context.Context {
	if scheme == nil {
		scheme = runtime.NewScheme()
	}
	v1alpha1.AddToScheme(scheme)
	workerv1alpha1.AddToScheme(scheme)
	eventRecorder := events.NewEventRecorder(client, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	return util.PrepareKubeSliceControllersRequestContext(ctx, client, scheme, "SliceIpamTestController", &eventRecorder)
}

// Test for handleSliceIpamDeletion with active allocations
func testHandleSliceIpamDeletionWithActiveAllocs(t *testing.T) {
	ctx := context.Background()
	clientMock := &utilmock.Client{}
	mMock := &metricMock.IMetricRecorder{}
	scheme := runtime.NewScheme()

	sliceIpamService := NewSliceIpamService(mMock)
	ctx = prepareSliceIpamTestContext(ctx, clientMock, scheme)

	// Create SliceIpam with active allocations
	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-slice",
			Namespace:  "test-namespace",
			Finalizers: []string{SliceIpamFinalizer},
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceSubnet: "192.168.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "192.168.1.0/24",
					Status:      "Allocated",
				},
				{
					ClusterName: "cluster2",
					Subnet:      "192.168.2.0/24",
					Status:      "InUse",
				},
			},
		},
	}

	result, err := sliceIpamService.handleSliceIpamDeletion(ctx, sliceIpam)

	require.NoError(t, err)
	require.Equal(t, time.Minute*5, result.RequeueAfter)
	require.True(t, result.Requeue || result.RequeueAfter > 0)
}

// Test for handleSliceIpamDeletion with no active allocations
func testHandleSliceIpamDeletionNoActiveAllocs(t *testing.T) {
	ctx := context.Background()
	clientMock := &utilmock.Client{}
	mMock := &metricMock.IMetricRecorder{}
	scheme := runtime.NewScheme()

	sliceIpamService := NewSliceIpamService(mMock)
	ctx = prepareSliceIpamTestContext(ctx, clientMock, scheme)

	// Create SliceIpam with only released allocations
	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-slice",
			Namespace:  "test-namespace",
			Finalizers: []string{SliceIpamFinalizer},
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceSubnet: "192.168.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "192.168.1.0/24",
					Status:      "Released",
				},
			},
		},
	}

	clientMock.On("Update", ctx, sliceIpam, mock.Anything).Return(nil).Once()

	result, err := sliceIpamService.handleSliceIpamDeletion(ctx, sliceIpam)

	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)
	require.NotContains(t, sliceIpam.Finalizers, SliceIpamFinalizer)
	clientMock.AssertExpectations(t)
}

// Test for reconcileSliceIpamState with new resource (TotalSubnets = 0)
func testReconcileSliceIpamStateNewResource(t *testing.T) {
	ctx := context.Background()
	clientMock := &utilmock.Client{}
	mMock := &metricMock.IMetricRecorder{}
	scheme := runtime.NewScheme()

	sliceIpamService := NewSliceIpamService(mMock)
	ctx = prepareSliceIpamTestContext(ctx, clientMock, scheme)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceSubnet: "192.168.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			TotalSubnets: 0, // New resource
		},
	}

	// syncWithSliceConfig will attempt to Get the corresponding SliceConfig; return NotFound to skip sync
	clientMock.On("Get", ctx, types.NamespacedName{Name: sliceIpam.Name, Namespace: sliceIpam.Namespace}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(kubeerrors.NewNotFound(util.Resource("SliceConfig"), "sliceconfig not found"))
	clientMock.On("Update", ctx, sliceIpam, mock.Anything).Return(nil).Once()

	result, err := sliceIpamService.reconcileSliceIpamState(ctx, sliceIpam)

	require.NoError(t, err)
	require.Equal(t, time.Hour, result.RequeueAfter)
	require.Greater(t, sliceIpam.Status.TotalSubnets, 0)
	require.Equal(t, sliceIpam.Status.TotalSubnets, sliceIpam.Status.AvailableSubnets)
	clientMock.AssertExpectations(t)
}

// Test for reconcileSliceIpamState with existing resource (TotalSubnets > 0)
func testReconcileSliceIpamStateExistingResource(t *testing.T) {
	ctx := context.Background()
	clientMock := &utilmock.Client{}
	mMock := &metricMock.IMetricRecorder{}
	scheme := runtime.NewScheme()

	sliceIpamService := NewSliceIpamService(mMock)
	ctx = prepareSliceIpamTestContext(ctx, clientMock, scheme)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceSubnet: "192.168.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			TotalSubnets: 256, // Existing resource
		},
	}

	// syncWithSliceConfig will attempt to Get the corresponding SliceConfig; return NotFound to skip sync
	clientMock.On("Get", ctx, types.NamespacedName{Name: sliceIpam.Name, Namespace: sliceIpam.Namespace}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(kubeerrors.NewNotFound(util.Resource("SliceConfig"), "sliceconfig not found"))

	result, err := sliceIpamService.reconcileSliceIpamState(ctx, sliceIpam)

	require.NoError(t, err)
	require.Equal(t, time.Hour, result.RequeueAfter)
	require.Equal(t, 256, sliceIpam.Status.TotalSubnets) // Should remain unchanged
}

// Test for cleanupExpiredAllocations with expired allocations
func testCleanupExpiredAllocations(t *testing.T) {
	ctx := context.Background()
	clientMock := &utilmock.Client{}
	mMock := &metricMock.IMetricRecorder{}
	scheme := runtime.NewScheme()

	sliceIpamService := NewSliceIpamService(mMock)
	ctx = prepareSliceIpamTestContext(ctx, clientMock, scheme)

	// Create time 25 hours ago (expired)
	expiredTime := metav1.NewTime(time.Now().Add(-25 * time.Hour))
	recentTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "expired-cluster",
					Subnet:      "192.168.1.0/24",
					Status:      "Released",
					ReleasedAt:  &expiredTime, // Expired
				},
				{
					ClusterName: "recent-cluster",
					Subnet:      "192.168.2.0/24",
					Status:      "Released",
					ReleasedAt:  &recentTime, // Not expired
				},
				{
					ClusterName: "active-cluster",
					Subnet:      "192.168.3.0/24",
					Status:      "Allocated", // Active
				},
			},
		},
	}

	// cleanupExpiredAllocations is called from reconcile but here we call it directly; no SliceConfig Get needed
	clientMock.On("Update", ctx, sliceIpam, mock.Anything).Return(nil).Once()

	sliceIpamService.cleanupExpiredAllocations(ctx, sliceIpam)

	require.Len(t, sliceIpam.Status.AllocatedSubnets, 2)
	require.Equal(t, "recent-cluster", sliceIpam.Status.AllocatedSubnets[0].ClusterName)
	require.Equal(t, "active-cluster", sliceIpam.Status.AllocatedSubnets[1].ClusterName)
	clientMock.AssertExpectations(t)
}

// Test for cleanupExpiredAllocations with no expired allocations
func testCleanupExpiredAllocationsNoExpired(t *testing.T) {
	ctx := context.Background()
	clientMock := &utilmock.Client{}
	mMock := &metricMock.IMetricRecorder{}
	scheme := runtime.NewScheme()

	sliceIpamService := NewSliceIpamService(mMock)
	ctx = prepareSliceIpamTestContext(ctx, clientMock, scheme)

	recentTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "192.168.1.0/24",
					Status:      "Released",
					ReleasedAt:  &recentTime, // Recent
				},
				{
					ClusterName: "cluster2",
					Subnet:      "192.168.2.0/24",
					Status:      "Allocated", // Active
				},
			},
		},
	}

	originalLen := len(sliceIpam.Status.AllocatedSubnets)

	sliceIpamService.cleanupExpiredAllocations(ctx, sliceIpam)

	// No cleanup should occur - no expired allocations
	require.Len(t, sliceIpam.Status.AllocatedSubnets, originalLen)
	require.Equal(t, "cluster1", sliceIpam.Status.AllocatedSubnets[0].ClusterName)
	require.Equal(t, "cluster2", sliceIpam.Status.AllocatedSubnets[1].ClusterName)

	// clientMock should not be called for Update since no cleanup happened
}
