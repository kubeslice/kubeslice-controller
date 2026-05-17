/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 */

package service

import (
	"fmt"
	"sync/atomic"
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestCreateMinimumWorkerSliceGateways_fullMeshPairCount verifies that reconciling
// a slice with n member clusters triggers one cert job per unordered cluster pair
// (C(n,2)), i.e. the current full-mesh behaviour locked before topology modes.
// kubeslice-controller#355
func TestCreateMinimumWorkerSliceGateways_fullMeshPairCount(t *testing.T) {
	t.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", "kubeslice-controller")

	cases := []struct {
		nClusters int
		wantPairs int
	}{
		{nClusters: 3, wantPairs: 3},
		{nClusters: 4, wantPairs: 6},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("n_%d", tc.nClusters), func(t *testing.T) {
			ns := "test-namespace"
			sliceName := "test-slice"

			clusterNames := make([]string, tc.nClusters)
			clusterMap := make(map[string]int, tc.nClusters)
			for i := 0; i < tc.nClusters; i++ {
				name := fmt.Sprintf("cluster-%d", i+1)
				clusterNames[i] = name
				clusterMap[name] = i + 1
			}

			label := map[string]string{
				"original-slice-name": sliceName,
			}

			_, _, jobMock, svc, _, clientMock, _, ctx, mMock := setupWorkerSliceGatewayTest("gw", ns)

			clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				list := args.Get(1).(*workerv1alpha1.WorkerSliceGatewayList)
				list.Items = nil
			}).Once()

			clusterObj := &controllerv1alpha1.Cluster{}
			clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), clusterObj).Return(nil).Run(func(args mock.Arguments) {
				key := args.Get(1).(types.NamespacedName)
				c := args.Get(2).(*controllerv1alpha1.Cluster)
				c.Name = key.Name
				c.Namespace = key.Namespace
			}).Times(tc.nClusters)

			notFound := k8sError.NewNotFound(schema.GroupResource{Group: "", Resource: "WorkerSliceGateway"}, "missing")
			gatewayObj := &workerv1alpha1.WorkerSliceGateway{}
			clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), gatewayObj).
				Return(notFound).Maybe()

			var gatewayCreates int32
			clientMock.On("Create", ctx, mock.MatchedBy(func(obj client.Object) bool {
				_, ok := obj.(*workerv1alpha1.WorkerSliceGateway)
				if ok {
					atomic.AddInt32(&gatewayCreates, 1)
				}
				return ok
			})).Return(nil).Times(2 * tc.wantPairs)

			clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Maybe()
			clientMock.On("Update", ctx, mock.Anything).Return(nil).Maybe()

			sliceConfig := &controllerv1alpha1.SliceConfig{}
			clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), sliceConfig).Return(nil).Run(func(args mock.Arguments) {
				cfg := args.Get(2).(*controllerv1alpha1.SliceConfig)
				cfg.Spec.SliceGatewayProvider = &controllerv1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: controllerv1alpha1.SliceGatewayTypeOpenVPN,
					SliceCaType:      "Local",
				}
			}).Times(tc.wantPairs)

			jobMock.On("CreateJob", ctx, mock.Anything, JobImage, mock.Anything).
				Return(ctrl.Result{}, nil).Times(tc.wantPairs)

			mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Maybe()
			mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Maybe()

			_, err := svc.CreateMinimumWorkerSliceGateways(ctx, sliceName, clusterNames, ns, label, clusterMap,
				"10.10.10.0/16", "/16", nil)
			require.NoError(t, err)
			require.Equal(t, int32(2*tc.wantPairs), gatewayCreates,
				"each mesh edge should create server and client WorkerSliceGateway resources")
			jobMock.AssertNumberOfCalls(t, "CreateJob", tc.wantPairs)
			jobMock.AssertExpectations(t)
		})
	}
}
