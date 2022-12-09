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
	"github.com/dailymotion/allure-go"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8sapimachinery "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestWorkerSliceGatewayRecyclerSuite(t *testing.T) {
	for k, v := range WorkerSliceGatewayRecyclerTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var WorkerSliceGatewayRecyclerTestBed = map[string]func(*testing.T){
	"WorkerSliceGatewaysRecycler_DeleteWorkerSliceGatewaysRecyclerByLabelHappyCase":     DeleteWorkerSliceGatewaysRecyclerByLabelHappyCase,
	"WorkerSliceGatewaysRecycler_DeleteWorkerSliceGatewaysRecyclerByLabelErrorOnDelete": DeleteWorkerSliceGatewaysRecyclerByLabelErrorOnDelete,
	"WorkerSliceGatewaysRecycler_DeleteWorkerSliceGatewaysRecyclerByLabelErrorOnList":   DeleteWorkerSliceGatewaysRecyclerByLabelErrorOnList,
}

func DeleteWorkerSliceGatewaysRecyclerByLabelHappyCase(t *testing.T) {
	workerSliceGatewayRecyclerService, requestObj, clientMock, _, ctx := setupWorkerSliceGatewayRecyclerTest("red", "namespace")
	label1 := map[string]string{
		"slice_name": requestObj.Name,
	}
	workerSliceGatewayRecyclers := &workerv1alpha1.WorkerSliceGwRecyclerList{}
	clientMock.On("List", ctx, workerSliceGatewayRecyclers, client.MatchingLabels(label1), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGwRecyclerList)
		arg.Items = []workerv1alpha1.WorkerSliceGwRecycler{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
					SliceName: requestObj.Name,
				},
				Status: workerv1alpha1.WorkerSliceGwRecyclerStatus{},
			},
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
					SliceName: requestObj.Name,
				},
				Status: workerv1alpha1.WorkerSliceGwRecyclerStatus{},
			},
		}
	}).Once()
	wsgwr := &workerv1alpha1.WorkerSliceGwRecycler{
		Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
			SliceName: requestObj.Name,
		},
	}
	clientMock.On("Delete", ctx, wsgwr).Return(nil).Twice()
	err1 := workerSliceGatewayRecyclerService.DeleteWorkerSliceGatewayRecyclersByLabel(ctx, label1, requestObj.Namespace)
	require.NoError(t, nil)
	require.Nil(t, err1)
	clientMock.AssertExpectations(t)
}

func DeleteWorkerSliceGatewaysRecyclerByLabelErrorOnDelete(t *testing.T) {
	workerSliceGatewayRecyclerService, requestObj, clientMock, _, ctx := setupWorkerSliceGatewayRecyclerTest("red", "namespace")
	label1 := map[string]string{
		"slice_name": requestObj.Name,
	}
	workerSliceGatewayRecyclers := &workerv1alpha1.WorkerSliceGwRecyclerList{}
	clientMock.On("List", ctx, workerSliceGatewayRecyclers, client.MatchingLabels(label1), client.InNamespace(requestObj.Namespace)).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGwRecyclerList)
		arg.Items = []workerv1alpha1.WorkerSliceGwRecycler{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
					SliceName: requestObj.Name,
				},
				Status: workerv1alpha1.WorkerSliceGwRecyclerStatus{},
			},
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
					SliceName: requestObj.Name,
				},
				Status: workerv1alpha1.WorkerSliceGwRecyclerStatus{},
			},
		}
	}).Once()
	wsgwr := &workerv1alpha1.WorkerSliceGwRecycler{
		Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
			SliceName: requestObj.Name,
		},
	}
	clientMock.On("Delete", ctx, wsgwr).Return(nil).Once()
	err1 := errors.New("internal_error")
	clientMock.On("Delete", ctx, wsgwr).Return(err1).Once()
	err := workerSliceGatewayRecyclerService.DeleteWorkerSliceGatewayRecyclersByLabel(ctx, label1, requestObj.Namespace)
	require.Error(t, err)
	require.Equal(t, err, err1)
	clientMock.AssertExpectations(t)
	clientMock.AssertExpectations(t)
}

func DeleteWorkerSliceGatewaysRecyclerByLabelErrorOnList(t *testing.T) {
	workerSliceGatewayRecyclerService, requestObj, clientMock, _, ctx := setupWorkerSliceGatewayRecyclerTest("red", "namespace")
	label1 := map[string]string{
		"slice_name": requestObj.Name,
	}
	workerSliceGatewayRecyclers := &workerv1alpha1.WorkerSliceGwRecyclerList{}
	err1 := errors.New("internal_error")
	clientMock.On("List", ctx, workerSliceGatewayRecyclers, client.MatchingLabels(label1), client.InNamespace(requestObj.Namespace)).Return(err1).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*workerv1alpha1.WorkerSliceGwRecyclerList)
		arg.Items = []workerv1alpha1.WorkerSliceGwRecycler{
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
					SliceName: requestObj.Name,
				},
				Status: workerv1alpha1.WorkerSliceGwRecyclerStatus{},
			},
			{
				TypeMeta:   k8sapimachinery.TypeMeta{},
				ObjectMeta: k8sapimachinery.ObjectMeta{},
				Spec: workerv1alpha1.WorkerSliceGwRecyclerSpec{
					SliceName: requestObj.Name,
				},
				Status: workerv1alpha1.WorkerSliceGwRecyclerStatus{},
			},
		}
	}).Once()
	err := workerSliceGatewayRecyclerService.DeleteWorkerSliceGatewayRecyclersByLabel(ctx, label1, requestObj.Namespace)
	require.Error(t, err)
	require.Equal(t, err, err1)
	clientMock.AssertExpectations(t)
	clientMock.AssertExpectations(t)
}

func setupWorkerSliceGatewayRecyclerTest(name string, namespace string) (WorkerSliceGatewayRecyclerService, ctrl.Request, *utilMock.Client, *workerv1alpha1.WorkerSliceGwRecycler, context.Context) {
	workerSliceGatewayRecyclerMock := WorkerSliceGatewayRecyclerService{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	requestObj := ctrl.Request{
		NamespacedName: namespacedName,
	}
	clientMock := &utilMock.Client{}
	workerSliceGatewayRecycler := &workerv1alpha1.WorkerSliceGwRecycler{}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "WorkerSliceGatewayRecyclerTest")
	return workerSliceGatewayRecyclerMock, requestObj, clientMock, workerSliceGatewayRecycler, ctx
}
