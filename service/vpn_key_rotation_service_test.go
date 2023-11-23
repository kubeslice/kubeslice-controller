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
	"testing"
	"time"

	"bou.ke/monkey"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
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

func setupTestCase() (context.Context, *utilMock.Client, VpnKeyRotationService, *mocks.IWorkerSliceGatewayService, *mocks.IWorkerSliceConfigService) {
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	wg := &mocks.IWorkerSliceGatewayService{}
	ws := &mocks.IWorkerSliceConfigService{}
	eventRecorder := events.NewEventRecorder(clientMock, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	return util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, scheme, "ClusterTestController", &eventRecorder), clientMock, VpnKeyRotationService{
		wsgs: wg,
		wscs: ws,
	}, wg, ws
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
	ctx, clientMock, vpn, _, _ := setupTestCase()
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
	ctx, clientMock, vpn, _, _ := setupTestCase()

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
	ctx, clientMock, vpn, _, _ := setupTestCase()

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
	ctx, clientMock, vpn, _, _ := setupTestCase()

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
	now                                metav1.Time
	reconcileResult                    ctrl.Result
}

func Test_reconcileVpnKeyRotationConfig(t *testing.T) {
	ts := metav1.NewTime(time.Date(2021, 06, 16, 20, 34, 58, 651387237, time.UTC))
	expiryTs := metav1.NewTime(ts.AddDate(0, 0, 30).Add(-1 * time.Hour))
	newTs := metav1.NewTime(time.Date(2021, 07, 16, 20, 34, 58, 651387237, time.UTC))
	testCases := []reconcileVpnKeyRotationConfigTestCase{
		{
			name: "should update CertCreation TS and Expiry TS when it is nil",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName:        "test-slice",
					Clusters:         []string{"worker-1", "worker-2"},
					RotationInterval: 30,
					RotationCount:    1,
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
					CertificateCreationTime: &ts,
					CertificateExpiryTime:   &expiryTs,
					RotationCount:           1,
				},
			},
			updateArg1:      mock.Anything,
			updateArg2:      mock.Anything,
			updateArg3:      mock.Anything,
			updateRet1:      nil,
			now:             ts,
			reconcileResult: ctrl.Result{},
		},
		{
			name: "should requeue after firing new jobs",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName:               "test-slice",
					Clusters:                []string{"worker-1", "worker-2"},
					RotationInterval:        30,
					CertificateCreationTime: &ts,
					CertificateExpiryTime:   &expiryTs,
					RotationCount:           1,
				},
			},
			arg2: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
			},
			expectedErr:     nil,
			expectedResp:    nil,
			updateArg1:      mock.Anything,
			updateArg2:      mock.Anything,
			updateArg3:      mock.Anything,
			updateRet1:      nil,
			now:             newTs,
			reconcileResult: ctrl.Result{RequeueAfter: 30 * time.Second},
		},
	}
	for _, tc := range testCases {
		runReconcileVpnKeyRotationConfig(t, &tc)
	}
}
func runReconcileVpnKeyRotationConfig(t *testing.T, tc *reconcileVpnKeyRotationConfigTestCase) {
	ctx, clientMock, vpn, wg, ws := setupTestCase()

	// Mocking metav1.Now() with a fixed time value
	patch := monkey.Patch(metav1.Now, func() metav1.Time {
		return tc.now
	})
	defer patch.Unpatch()
	// NOTE: Monkey pathcing sometimes requires the inlining to be disabled
	// use go test -gcflags=-l
	// setup Expectations
	gwList := &workerv1alpha1.WorkerSliceGatewayList{}
	clientMock.
		On("List", mock.Anything, gwList, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		w := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		w.Items = append(w.Items,
			workerv1alpha1.WorkerSliceGateway{
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					GatewayHostType: "Server",
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						GatewayName: "test-slice-worker-2-worker-1",
					},
				},
			},
			workerv1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice-worker-2-worker-1",
				},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					GatewayHostType: "Client",
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						GatewayName: "test-slice-worker-1-worker-2",
					},
				},
			})
	}).Times(1)

	jobList := &batchv1.JobList{}

	clientMock.
		On("List", mock.Anything, jobList, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		w := args.Get(1).(*batchv1.JobList)
		w.Items = append(w.Items,
			batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-1",
					Labels: map[string]string{
						"SLICE_NAME": "test-slice",
					},
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-2",
					Labels: map[string]string{
						"SLICE_NAME": "test-slice",
					},
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			})
	}).Times(2)

	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()

	workerSliceConfigs := workerv1alpha1.WorkerSliceConfigList{
		Items: []workerv1alpha1.WorkerSliceConfig{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice-config-1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice-config-2",
				},
			},
		},
	}

	ws.On("ListWorkerSliceConfigs", mock.Anything, mock.Anything, mock.Anything).Return(workerSliceConfigs.Items, nil).Once()

	clusterMap := map[string]int{
		"cluster-1": 1,
		"cluster-2": 2,
	}

	ws.On("ComputeClusterMap", mock.Anything, mock.Anything).Return(clusterMap).Once()

	gwAddress := util.WorkerSliceGatewayNetworkAddresses{}

	wg.On("BuildNetworkAddresses", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(gwAddress).Once()

	wg.On("GenerateCerts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	clientMock.
		On("Update", tc.updateArg1, tc.updateArg2).Return(tc.updateRet1).Once()

	reconcileResult, gotResp, gotErr := vpn.reconcileVpnKeyRotationConfig(ctx, tc.arg1, tc.arg2)
	require.Equal(t, gotErr, tc.expectedErr)

	require.Equal(t, tc.expectedResp, gotResp)
	require.Equal(t, tc.reconcileResult, reconcileResult)
}

type listClientPairGatewayTesCase struct {
	name         string
	arg1         *workerv1alpha1.WorkerSliceGatewayList
	arg2         string
	expectedResp *workerv1alpha1.WorkerSliceGateway
	expectedErr  error
}

func Test_listClientPairGateway(t *testing.T) {
	testCases := []listClientPairGatewayTesCase{
		{
			name: "it should return correct workerslicegw",
			arg1: &workerv1alpha1.WorkerSliceGatewayList{
				Items: []workerv1alpha1.WorkerSliceGateway{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-slice-worker-1-worker-2",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-slice-worker-2-worker-1",
						},
					},
				},
			},
			arg2: "test-slice-worker-1-worker-2",
			expectedResp: &workerv1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice-worker-1-worker-2",
				},
			},
			expectedErr: nil,
		},
		{
			name: "it should return error",
			arg1: &workerv1alpha1.WorkerSliceGatewayList{
				Items: []workerv1alpha1.WorkerSliceGateway{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-slice-worker-1-worker-2",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-slice-worker-2-worker-1",
						},
					},
				},
			},
			arg2:         "test-slice-worker-1-worker-3",
			expectedResp: nil,
			expectedErr:  fmt.Errorf("cannot find gateway %s", "test-slice-worker-1-worker-3"),
		},
	}
	for _, tc := range testCases {
		runlistClientPairGatewayTestCase(t, tc)
	}
}

func runlistClientPairGatewayTestCase(t *testing.T, tc listClientPairGatewayTesCase) {
	_, _, vpn, _, _ := setupTestCase()

	gotResp, gotErr := vpn.listClientPairGateway(tc.arg1, tc.arg2)
	require.Equal(t, gotErr, tc.expectedErr)
	require.Equal(t, gotResp, tc.expectedResp)
}

type verifyAllJobsAreCompletedTestCase struct {
	name                         string
	arg1                         string
	expectedResp                 JobStatus
	completionType               corev1.ConditionStatus
	failedStatus                 corev1.ConditionStatus
	listArg1, listArg2, listArg3 interface{}
	listRet1                     interface{}
}

func Test_verifyAllJobsAreCompleted(t *testing.T) {
	testCases := []verifyAllJobsAreCompletedTestCase{
		{
			name:           "should return JobStatusComplete if all jobs are in completed",
			arg1:           "test-slice",
			expectedResp:   JobStatusComplete,
			listArg1:       mock.Anything,
			listArg2:       mock.Anything,
			listArg3:       mock.Anything,
			listRet1:       nil,
			completionType: corev1.ConditionTrue,
			failedStatus:   corev1.ConditionFalse,
		},
		{
			name:           "should return JobStatusRunning if all jobs are not in complete state",
			arg1:           "test-slice",
			expectedResp:   JobStatusRunning,
			listArg1:       mock.Anything,
			listArg2:       mock.Anything,
			listArg3:       mock.Anything,
			listRet1:       nil,
			completionType: corev1.ConditionFalse,
			failedStatus:   corev1.ConditionFalse,
		},
		{
			name:         "should return JobStatusListError if listing job fails",
			arg1:         "test-slice",
			expectedResp: JobStatusListError,
			listArg1:     mock.Anything,
			listArg2:     mock.Anything,
			listArg3:     mock.Anything,
			listRet1:     fmt.Errorf("cannot list jobs"),
		},
		{
			name:         "should return JobStatusError if jobs are in failed state",
			arg1:         "test-slice",
			expectedResp: JobStatusError,
			listArg1:     mock.Anything,
			listArg2:     mock.Anything,
			listArg3:     mock.Anything,
			listRet1:     nil,
			failedStatus: corev1.ConditionTrue,
		},
	}
	for _, tc := range testCases {
		runVerifyAllJobsAreCompleted(t, tc)
	}
}

func runVerifyAllJobsAreCompleted(t *testing.T, tc verifyAllJobsAreCompletedTestCase) {
	ctx, clientMock, vpn, _, _ := setupTestCase()

	if tc.failedStatus == corev1.ConditionTrue {
		clientMock.
			On("List", tc.listArg1, tc.listArg2, tc.listArg3).
			Return(tc.listRet1).Run(func(args mock.Arguments) {
			w := args.Get(1).(*batchv1.JobList)
			w.Items = append(w.Items,
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-1",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobComplete,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-2",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobFailed,
								Status: tc.failedStatus,
							},
						},
					},
				})
		}).Once()
	}

	if tc.completionType == corev1.ConditionTrue {
		clientMock.
			On("List", tc.listArg1, tc.listArg2, tc.listArg3).
			Return(tc.listRet1).Run(func(args mock.Arguments) {
			w := args.Get(1).(*batchv1.JobList)
			w.Items = append(w.Items,
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-1",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobComplete,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-2",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobComplete,
								Status: tc.completionType,
							},
						},
					},
				})
		}).Once()
	} else {
		clientMock.
			On("List", tc.listArg1, tc.listArg2, tc.listArg3).
			Return(tc.listRet1).Run(func(args mock.Arguments) {
			w := args.Get(1).(*batchv1.JobList)
			w.Items = append(w.Items,
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-1",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				},
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-2",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				})
		}).Once()
	}
	gotResp, _ := vpn.verifyAllJobsAreCompleted(ctx, tc.arg1)

	require.Equal(t, gotResp, tc.expectedResp)
}

type triggerJobsForCertCreationTestCase struct {
	name                                                                               string
	arg1                                                                               *controllerv1alpha1.VpnKeyRotation
	arg2                                                                               *controllerv1alpha1.SliceConfig
	expectedResp                                                                       error
	listArg1, listArg2, listArg3                                                       interface{}
	listRet1                                                                           interface{}
	listWorkerSliceConfigsArg1, listWorkerSliceConfigsArg2, listWorkerSliceConfigsArg3 interface{}
	listWorkerSliceConfigsRet1                                                         interface{}
	workerSliceGateways                                                                []workerv1alpha1.WorkerSliceGateway
	generateCertsRet1                                                                  interface{}
}

func Test_triggerJobsForCertCreation(t *testing.T) {
	testCases := []triggerJobsForCertCreationTestCase{
		{
			name: "should return error in case listing workerslicegateways failed",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice",
				},
			},
			arg2:         &controllerv1alpha1.SliceConfig{},
			expectedResp: fmt.Errorf("Error listing workerslicegateways"),
			listArg1:     mock.Anything,
			listArg2:     mock.Anything,
			listArg3:     mock.Anything,
			listRet1:     fmt.Errorf("Error listing workerslicegateways"),
		},
		{
			name: "should return error in case client workerslicegateway is not present",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice",
				},
			},
			arg2:         &controllerv1alpha1.SliceConfig{},
			expectedResp: fmt.Errorf("cannot find gateway %s", "test-slice-worker2-worker-1"),
			listArg1:     mock.Anything,
			listArg2:     mock.Anything,
			listArg3:     mock.Anything,
			listRet1:     nil,
			workerSliceGateways: []workerv1alpha1.WorkerSliceGateway{
				{
					Spec: workerv1alpha1.WorkerSliceGatewaySpec{
						GatewayHostType: "Server",
						RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
							GatewayName: "test-slice-worker2-worker-1",
						},
					},
				},
			},
		},
		{
			name: "should return error in case listing workersliceconfigs failed",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice",
				},
			},
			arg2: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice",
				},
				Spec: controllerv1alpha1.SliceConfigSpec{
					MaxClusters: 3,
				},
			},
			expectedResp:               fmt.Errorf("error listing workersliceconfigs"),
			listArg1:                   mock.Anything,
			listArg2:                   mock.Anything,
			listArg3:                   mock.Anything,
			listRet1:                   nil,
			listWorkerSliceConfigsArg1: mock.Anything,
			listWorkerSliceConfigsArg2: mock.Anything,
			listWorkerSliceConfigsArg3: mock.Anything,
			listWorkerSliceConfigsRet1: fmt.Errorf("error listing workersliceconfigs"),
			workerSliceGateways: []workerv1alpha1.WorkerSliceGateway{
				{
					Spec: workerv1alpha1.WorkerSliceGatewaySpec{
						GatewayHostType: "Server",
						RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
							GatewayName: "test-slice-worker-2-worker-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-slice-worker-2-worker-1",
					},
					Spec: workerv1alpha1.WorkerSliceGatewaySpec{
						GatewayHostType: "Client",
						RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
							GatewayName: "test-slice-worker-1-worker-2",
						},
					},
				},
			},
		},
		{
			name: "should return error in case generating certs failed",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice",
				},
			},
			arg2: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice",
				},
				Spec: controllerv1alpha1.SliceConfigSpec{
					MaxClusters: 3,
				},
			},
			expectedResp:               fmt.Errorf("error generating Certs"),
			listArg1:                   mock.Anything,
			listArg2:                   mock.Anything,
			listArg3:                   mock.Anything,
			listRet1:                   nil,
			listWorkerSliceConfigsArg1: mock.Anything,
			listWorkerSliceConfigsArg2: mock.Anything,
			listWorkerSliceConfigsArg3: mock.Anything,
			listWorkerSliceConfigsRet1: nil,
			workerSliceGateways: []workerv1alpha1.WorkerSliceGateway{
				{
					Spec: workerv1alpha1.WorkerSliceGatewaySpec{
						GatewayHostType: "Server",
						RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
							GatewayName: "test-slice-worker-2-worker-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-slice-worker-2-worker-1",
					},
					Spec: workerv1alpha1.WorkerSliceGatewaySpec{
						GatewayHostType: "Client",
						RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
							GatewayName: "test-slice-worker-1-worker-2",
						},
					},
				},
			},
			generateCertsRet1: fmt.Errorf("error generating Certs"),
		},
		{
			name: "should return error in case generating certs failed",
			arg1: &controllerv1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "test-ns",
				},
				Spec: controllerv1alpha1.VpnKeyRotationSpec{
					SliceName: "test-slice",
				},
			},
			arg2: &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-slice",
				},
				Spec: controllerv1alpha1.SliceConfigSpec{
					MaxClusters: 3,
				},
			},
			expectedResp:               nil,
			listArg1:                   mock.Anything,
			listArg2:                   mock.Anything,
			listArg3:                   mock.Anything,
			listRet1:                   nil,
			listWorkerSliceConfigsArg1: mock.Anything,
			listWorkerSliceConfigsArg2: mock.Anything,
			listWorkerSliceConfigsArg3: mock.Anything,
			listWorkerSliceConfigsRet1: nil,
			workerSliceGateways: []workerv1alpha1.WorkerSliceGateway{
				{
					Spec: workerv1alpha1.WorkerSliceGatewaySpec{
						GatewayHostType: "Server",
						RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
							GatewayName: "test-slice-worker-2-worker-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-slice-worker-2-worker-1",
					},
					Spec: workerv1alpha1.WorkerSliceGatewaySpec{
						GatewayHostType: "Client",
						RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
							GatewayName: "test-slice-worker-1-worker-2",
						},
					},
				},
			},
			generateCertsRet1: nil,
		},
	}
	for _, tc := range testCases {
		runTriggerJobsForCertCreation(t, tc)
	}
}

func runTriggerJobsForCertCreation(t *testing.T, tc triggerJobsForCertCreationTestCase) {
	ctx, clientMock, vpn, wg, ws := setupTestCase()

	clientMock.
		On("List", tc.listArg1, tc.listArg2, tc.listArg3).
		Return(tc.listRet1).Run(func(args mock.Arguments) {
		w := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		w.Items = tc.workerSliceGateways
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

	ws.On("ListWorkerSliceConfigs", tc.listWorkerSliceConfigsArg1, tc.listWorkerSliceConfigsArg2, tc.listWorkerSliceConfigsArg3).Return(workerSliceConfigs.Items, tc.listWorkerSliceConfigsRet1).Once()

	clusterMap := map[string]int{
		"cluster-1": 1,
		"cluster-2": 2,
	}

	ws.On("ComputeClusterMap", tc.listWorkerSliceConfigsArg1, tc.listWorkerSliceConfigsArg2).Return(clusterMap).Once()

	gwAddress := util.WorkerSliceGatewayNetworkAddresses{}

	wg.On("BuildNetworkAddresses", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(gwAddress).Once()

	wg.On("GenerateCerts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.generateCertsRet1).Once()

	gotResp := vpn.triggerJobsForCertCreation(ctx, tc.arg1, tc.arg2)
	require.Equal(t, gotResp, tc.expectedResp)

	clientMock.AssertExpectations(t)
}

type reconcileVpnKeyRotationTestCase struct {
	name                      string
	request                   ctrl.Request
	expectedResponse          ctrl.Result
	expectedError             error
	getArg1, getArg2, getArg3 interface{}
	getRet1                   interface{}
	getRet2                   interface{}
	completionType            corev1.ConditionStatus
	clusterGatewayMapping     map[string][]string
}

func Test_reconcileVpnKeyRotation(t *testing.T) {
	testCases := []reconcileVpnKeyRotationTestCase{
		{
			name: "should return error in case vpnkeyrotation config not found",
			request: ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: "test-ns",
				Name:      "test-slice",
			}},
			expectedResponse: ctrl.Result{},
			expectedError:    fmt.Errorf("vpnkeyrotation config not found"),
			getArg1:          mock.Anything,
			getArg2:          mock.Anything,
			getArg3:          mock.Anything,
			getRet1:          fmt.Errorf("vpnkeyrotation config not found"),
		},
		{
			name: "should return error in case slice config not found",
			request: ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: "test-ns",
				Name:      "test-slice",
			}},
			expectedResponse: ctrl.Result{},
			expectedError:    fmt.Errorf("slice config not found"),
			getArg1:          mock.Anything,
			getArg2:          mock.Anything,
			getArg3:          mock.Anything,
			getRet1:          nil,
			getRet2:          fmt.Errorf("slice config not found"),
		},
		{
			name: "should successfully requeue after building clusterGatewayMapping",
			request: ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: "test-ns",
				Name:      "test-slice",
			}},
			expectedResponse: ctrl.Result{Requeue: true},
			expectedError:    nil,
			getArg1:          mock.Anything,
			getArg2:          mock.Anything,
			getArg3:          mock.Anything,
			getRet1:          nil,
			getRet2:          nil,
		},
		{
			name: "should wait and requeue till all the jobs are in comlpletion state",
			request: ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: "test-ns",
				Name:      "test-slice",
			}},
			expectedResponse: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectedError:    nil,
			getArg1:          mock.Anything,
			getArg2:          mock.Anything,
			getArg3:          mock.Anything,
			getRet1:          nil,
			getRet2:          nil,
			completionType:   corev1.ConditionFalse,
			clusterGatewayMapping: map[string][]string{
				"worker-1": {"test-slice-worker-1-worker-2"},
				"worker-2": {"test-slice-worker-2-worker-1"},
			},
		},
		{
			name: "should successfully requeue before 1 hour of expiry",
			request: ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: "test-ns",
				Name:      "test-slice",
			}},
			expectedResponse: ctrl.Result{RequeueAfter: metav1.NewTime(time.Date(2021, 06, 16, 20, 34, 58, 651387237, time.UTC)).AddDate(0, 0, 30).Add(-1 * time.Hour).Sub(time.Date(2021, 06, 16, 20, 34, 58, 651387237, time.UTC))},
			expectedError:    nil,
			getArg1:          mock.Anything,
			getArg2:          mock.Anything,
			getArg3:          mock.Anything,
			getRet1:          nil,
			getRet2:          nil,
			completionType:   corev1.ConditionTrue,
			clusterGatewayMapping: map[string][]string{
				"worker-1": {"test-slice-worker-1-worker-2"},
				"worker-2": {"test-slice-worker-2-worker-1"},
			},
		},
	}
	for _, tc := range testCases {
		runReconcileVpnKeyRotation(t, tc)
	}
}

func runReconcileVpnKeyRotation(t *testing.T, tc reconcileVpnKeyRotationTestCase) {
	ctx, clientMock, vpn, _, _ := setupTestCase()

	clientMock.
		On("Get", tc.getArg1, tc.getArg2, tc.getArg3).
		Return(tc.getRet1).Run(func(args mock.Arguments) {
		v := args.Get(2).(*controllerv1alpha1.VpnKeyRotation)
		v.ObjectMeta = metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-ns",
		}
		v.Spec = controllerv1alpha1.VpnKeyRotationSpec{
			Clusters:              []string{"worker-1", "worker-2"},
			RotationInterval:      30,
			SliceName:             "test-slice",
			ClusterGatewayMapping: tc.clusterGatewayMapping,
		}
	}).Once()

	clientMock.
		On("Get", tc.getArg1, tc.getArg2, tc.getArg3).
		Return(tc.getRet2).Run(func(args mock.Arguments) {
		s := args.Get(2).(*controllerv1alpha1.SliceConfig)
		s.ObjectMeta = metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-ns",
		}
		s.Spec = controllerv1alpha1.SliceConfigSpec{
			Clusters:         []string{"worker-1", "worker-2"},
			RotationInterval: 30,
		}
	}).Once()

	clientMock.
		On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		w := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		w.Items = append(w.Items, workerv1alpha1.WorkerSliceGateway{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-slice-worker-1-worker-2",
				Labels: map[string]string{
					"worker-cluster":      "worker-1",
					"original-slice-name": "test-slice",
				},
			},
		})
	}).Once()

	clientMock.
		On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		w := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
		w.Items = append(w.Items, workerv1alpha1.WorkerSliceGateway{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-slice-worker-2-worker-1",
				Labels: map[string]string{
					"worker-cluster":      "worker-2",
					"original-slice-name": "test-slice",
				},
			},
		})
	}).Once()

	clientMock.
		On("Update", mock.Anything, mock.Anything).Return(nil).Once()

	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()

	if tc.completionType == corev1.ConditionTrue {
		clientMock.
			On("List", mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Run(func(args mock.Arguments) {
			w := args.Get(1).(*batchv1.JobList)
			w.Items = append(w.Items,
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-slice-worker-1-worker-2",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobComplete,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-slice-worker-2-worker-1",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobComplete,
								Status: tc.completionType,
							},
						},
					},
				})
		}).Once()
	} else if tc.completionType == corev1.ConditionFalse {
		clientMock.
			On("List", mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Run(func(args mock.Arguments) {
			w := args.Get(1).(*batchv1.JobList)
			w.Items = append(w.Items,
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-slice-worker-1-worker-2",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				},
				batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-slice-worker-2-worker-1",
						Labels: map[string]string{
							"SLICE_NAME": "test-slice",
						},
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				})
		}).Once()
	}

	clientMock.
		On("Update", mock.Anything, mock.Anything).Return(nil).Once()

	// Mocking metav1.Now() with a fixed time value
	patch := monkey.Patch(metav1.Now, func() metav1.Time {
		return metav1.NewTime(time.Date(2021, 06, 16, 20, 34, 58, 651387237, time.UTC))
	})
	defer patch.Unpatch()

	gotResp, gotErr := vpn.ReconcileVpnKeyRotation(ctx, tc.request)

	require.Equal(t, tc.expectedError, gotErr)
	require.Equal(t, tc.expectedResponse, gotResp)

}
