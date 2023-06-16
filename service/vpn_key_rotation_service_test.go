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
	"time"

	"bou.ke/monkey"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type createMinimalVpnKeyRotationConfigTestCase struct {
	name                      string
	sliceName                 string
	namespace                 string
	expectedErr               error
	getArg1, getArg2, getArg3 interface{}
	getRet1                   interface{}
	createArg1, createArg2    interface{}
	createRet1                interface{}
}

func setupTestCase() (context.Context, *utilMock.Client, VpnKeyRotationService) {
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	return util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, scheme, "ClusterTestController", nil), clientMock, VpnKeyRotationService{}
}

func Test_CreateMinimalVpnKeyRotationConfig(t *testing.T) {
	testCases := []createMinimalVpnKeyRotationConfigTestCase{
		{
			name:        "should create vpnkeyrotation config successfully",
			sliceName:   "demo-slice",
			namespace:   "demo-namespace",
			expectedErr: nil,
			getArg1:     mock.Anything,
			getArg2:     mock.Anything,
			getArg3:     mock.Anything,
			getRet1:     kubeerrors.NewNotFound(util.Resource("VpnKeyRotationConfigTest"), "VpnKeyRotationConfig not found"),
			createArg1:  mock.Anything,
			createArg2:  mock.Anything,
			createRet1:  nil,
		},
		{
			name:        "should return error if creating vpnkeyrotation config fails",
			sliceName:   "demo-slice",
			namespace:   "demo-namespace",
			expectedErr: errors.New("Failed to create vpnkeyrotation"),
			getArg1:     mock.Anything,
			getArg2:     mock.Anything,
			getArg3:     mock.Anything,
			getRet1:     kubeerrors.NewNotFound(util.Resource("VpnKeyRotationConfigTest"), "VpnKeyRotationConfig not found"),
			createArg1:  mock.Anything,
			createArg2:  mock.Anything,
			createRet1:  errors.New("Failed to create vpnkeyrotation"),
		},
	}

	for _, tc := range testCases {
		runCreateMinimalVpnKeyRotationConfigTestCase(t, tc)
	}
}

func runCreateMinimalVpnKeyRotationConfigTestCase(t *testing.T, tc createMinimalVpnKeyRotationConfigTestCase) {
	ctx, clientMock, vpn := setupTestCase()
	clientMock.
		On("Get", tc.getArg1, tc.getArg2, tc.getArg3).
		Return(tc.getRet1).Once()

	clientMock.
		On("Create", tc.createArg1, tc.createArg2).
		Return(tc.createRet1).Once()

	gotErr := vpn.CreateMinimalVpnKeyRotationConfig(ctx, tc.sliceName, tc.namespace, 90)
	require.Equal(t, gotErr, tc.expectedErr)
	clientMock.AssertExpectations(t)
}

type reconcileClustersTestCase struct {
	name                      string
	sliceName                 string
	namespace                 string
	expectedErr               error
	expectedResp              *controllerv1alpha1.VpnKeyRotation
	existingClusters          []string
	addclusters               []string
	getArg1, getArg2, getArg3 interface{}
	getRet1                   interface{}
	updateArg1, updateArg2    interface{}
	updateRet1                interface{}
}

func Test_ReconcileClusters(t *testing.T) {
	testCases := []reconcileClustersTestCase{
		{
			name:        "should update vpnkeyrotation CR with cluster names sucessfully",
			sliceName:   "demo-slice",
			namespace:   "demo-ns",
			expectedErr: nil,
			expectedResp: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-slice",
					Namespace: "demo-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					Clusters:  []string{"worker-1", "worker-2"},
					SliceName: "demo-slice",
				},
			},
			existingClusters: []string{},
			addclusters:      []string{"worker-1", "worker-2"},
			getArg1:          mock.Anything,
			getArg2:          mock.Anything,
			getArg3:          mock.Anything,
			getRet1:          nil,
			updateArg1:       mock.Anything,
			updateArg2:       mock.Anything,
			updateRet1:       nil,
		},
		{
			name:        "should update cluster list in vpnkeyrotation CR when a cluster is added",
			sliceName:   "demo-slice",
			namespace:   "demo-ns",
			expectedErr: nil,
			expectedResp: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-slice",
					Namespace: "demo-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					Clusters:  []string{"worker-1", "worker-2", "worker-3"},
					SliceName: "demo-slice",
				},
			},
			existingClusters: []string{"worker-1", "worker-2"},
			addclusters:      []string{"worker-1", "worker-2", "worker-3"},
			getArg1:          mock.Anything,
			getArg2:          mock.Anything,
			getArg3:          mock.Anything,
			getRet1:          nil,
			updateArg1:       mock.Anything,
			updateArg2:       mock.Anything,
			updateRet1:       nil,
		},
	}

	for _, tc := range testCases {
		runReconcileClustersTestCase(t, tc)
	}
}

func runReconcileClustersTestCase(t *testing.T, tc reconcileClustersTestCase) {
	ctx, clientMock, vpn := setupTestCase()

	clientMock.
		On("Get", tc.getArg1, tc.getArg2, tc.getArg3).
		Return(tc.getRet1).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		arg.Name = tc.sliceName
		arg.Namespace = tc.namespace
		arg.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			SliceName: tc.sliceName,
			Clusters:  tc.existingClusters,
		}
	}).Once()

	clientMock.
		On("Update", tc.updateArg1, tc.updateArg2).Return(tc.updateRet1).Once()

	gotResp, gotErr := vpn.ReconcileClusters(ctx, tc.sliceName, tc.namespace, tc.addclusters)
	require.Equal(t, gotErr, tc.expectedErr)

	require.Equal(t, gotResp, tc.expectedResp)
	clientMock.AssertExpectations(t)
}

type constructClusterGatewayMappingTestCase struct {
	name                         string
	sliceConfig                  *controllerv1alpha1.SliceConfig
	expectedResp                 map[string][]string
	expectedErr                  error
	listArg1, listArg2, listArg3 interface{}
	listRet1                     interface{}
}

func Test_ConstructClusterGatewayMapping(t *testing.T) {
	testCases := []constructClusterGatewayMappingTestCase{
		{
			name: "should return error in case list fails",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-namespace",
				},
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"worker-1", "worker-2"},
				},
			},
			expectedResp: nil,
			expectedErr:  errors.New("workerslicegateway not found"),
			listArg1:     mock.Anything,
			listArg2:     mock.Anything,
			listArg3:     mock.Anything,
			listRet1:     errors.New("workerslicegateway not found"),
		},
		{
			name: "should return a valid constructed map",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-namespace",
				},
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"worker-1", "worker-2"},
				},
			},
			expectedResp: map[string][]string{
				"worker-1": {"test-slice-worker-1-worker-2"},
				"worker-2": {"test-slice-worker-2-worker-1"},
			},
			expectedErr: nil,
			listArg1:    mock.Anything,
			listArg2:    mock.Anything,
			listArg3:    mock.Anything,
			listRet1:    nil,
		},
	}
	for _, tc := range testCases {
		runClusterGatewayMappingTestCase(t, tc)
	}
}

func runClusterGatewayMappingTestCase(t *testing.T, tc constructClusterGatewayMappingTestCase) {
	ctx, clientMock, vpn := setupTestCase()

	if tc.expectedErr != nil {
		clientMock.
			On("List", tc.listArg1, tc.listArg2, tc.listArg3).
			Return(tc.listRet1).Once()
	} else {
		clientMock.
			On("List", tc.listArg1, tc.listArg2, tc.listArg3).
			Return(tc.listRet1).Run(func(args mock.Arguments) {
			w := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
			w.Items = append(w.Items, workerv1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice-worker-1-worker-2",
					Labels: map[string]string{
						"worker-cluster":      "worker-1",
						"original-slice-name": tc.sliceConfig.Name,
					},
				},
			})
		}).Once()

		clientMock.
			On("List", tc.listArg1, tc.listArg2, tc.listArg3).
			Return(tc.listRet1).Run(func(args mock.Arguments) {
			w := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
			w.Items = append(w.Items, workerv1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice-worker-2-worker-1",
					Labels: map[string]string{
						"worker-cluster":      "worker-2",
						"original-slice-name": tc.sliceConfig.Name,
					},
				},
			})
		}).Once()
	}

	gotResp, gotErr := vpn.constructClusterGatewayMapping(ctx, tc.sliceConfig)
	require.Equal(t, gotErr, tc.expectedErr)

	require.Equal(t, gotResp, tc.expectedResp)
	clientMock.AssertExpectations(t)
}

type getSliceConfigTestCase struct {
	name                      string
	sliceName                 string
	namespace                 string
	expectedResp              *controllerv1alpha1.SliceConfig
	expectedErr               error
	getArg1, getArg2, getArg3 interface{}
	getRet1                   interface{}
}

func Test_getSliceConfig(t *testing.T) {
	testCases := []getSliceConfigTestCase{
		{
			name:         "it should return error in sliceconfig not found",
			sliceName:    "test-slice",
			namespace:    "test-ns",
			expectedResp: nil,
			expectedErr:  errors.New("sliceconfig not found"),
			getArg1:      mock.Anything,
			getArg2:      mock.Anything,
			getArg3:      mock.Anything,
			getRet1:      errors.New("sliceconfig not found"),
		},
		{
			name:      "it should return sliceconfig successfully",
			sliceName: "test-slice",
			namespace: "test-ns",
			expectedResp: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
			},
			expectedErr: nil,
			getArg1:     mock.Anything,
			getArg2:     mock.Anything,
			getArg3:     mock.Anything,
			getRet1:     nil,
		},
	}
	for _, tc := range testCases {
		runGetSliceConfigTestCase(t, tc)
	}
}

func runGetSliceConfigTestCase(t *testing.T, tc getSliceConfigTestCase) {
	ctx, clientMock, vpn := setupTestCase()

	clientMock.
		On("Get", tc.getArg1, tc.getArg2, tc.getArg3).
		Return(tc.getRet1).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		arg.Name = tc.sliceName
		arg.Namespace = tc.namespace
	}).Once()

	gotResp, gotErr := vpn.getSliceConfig(ctx, tc.sliceName, tc.namespace)
	require.Equal(t, gotErr, tc.expectedErr)

	require.Equal(t, gotResp, tc.expectedResp)
	clientMock.AssertExpectations(t)
}

type reconcileVpnKeyRotationConfigTestCase struct {
	name                               string
	arg1                               *controllerv1alpha1.VpnKeyRotation
	arg2                               *controllerv1alpha1.SliceConfig
	expectedErr                        error
	expectedResp                       *controllerv1alpha1.VpnKeyRotation
	updateArg1, updateArg2, updateArg3 interface{}
	updateRet1                         interface{}
}

func Test_reconcileVpnKeyRotationConfig(t *testing.T) {
	testCases := []reconcileVpnKeyRotationConfigTestCase{
		{
			name: "should update CertCreation TS and Expiry TS",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName:        "test-slice",
					Clusters:         []string{"worker-1", "worker-2"},
					RotationInterval: 30,
				},
			},
			arg2: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
			},
			expectedErr: nil,
			expectedResp: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName:               "test-slice",
					Clusters:                []string{"worker-1", "worker-2"},
					RotationInterval:        30,
					CertificateCreationTime: metav1.NewTime(time.Date(2021, 06, 16, 20, 34, 58, 651387237, time.UTC)),
				},
			},
			updateArg1: mock.Anything,
			updateArg2: mock.Anything,
			updateArg3: mock.Anything,
			updateRet1: nil,
		},
	}
	for _, tc := range testCases {
		runReconcileVpnKeyRotationConfig(t, &tc)
	}
}

func runReconcileVpnKeyRotationConfig(t *testing.T, tc *reconcileVpnKeyRotationConfigTestCase) {
	ctx, clientMock, vpn := setupTestCase()

	clientMock.
		On("Update", tc.updateArg1, tc.updateArg2).Return(tc.updateRet1).Once()

	// Mocking metav1.Now() with a fixed time value
	patch := monkey.Patch(metav1.Now, func() metav1.Time {
		return metav1.NewTime(time.Date(2021, 06, 16, 20, 34, 58, 651387237, time.UTC))
	})
	defer patch.Unpatch()

	gotResp, gotErr := vpn.reconcileVpnKeyRotationConfig(ctx, tc.arg1, tc.arg2)
	require.Equal(t, gotErr, tc.expectedErr)

	require.False(t, gotResp.Spec.CertificateCreationTime.IsZero())
	require.False(t, gotResp.Spec.CertificateExpiryTime.IsZero())
	clientMock.AssertExpectations(t)
}
