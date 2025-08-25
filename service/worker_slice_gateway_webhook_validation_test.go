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

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dailymotion/allure-go"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkerSliceGatewayWebhookValidationSuite(t *testing.T) {
	for k, v := range WorkerSliceGatewayWebhookValidationTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var WorkerSliceGatewayWebhookValidationTestBed = map[string]func(*testing.T){
	"WorkerSliceGatewayWebhookValidation_UpdateValidateWorkerSliceGatewayUpdatingGatewayNumber": UpdateValidateWorkerSliceGatewayUpdatingGatewayNumber,
	"WorkerSliceGatewayWebhookValidation_UpdateValidateWorkerSliceGatewayWithoutErrors":         UpdateValidateWorkerSliceGatewayWithoutErrors,
}

func UpdateValidateWorkerSliceGatewayUpdatingGatewayNumber(t *testing.T) {
	name := "worker_slice_Gateway"
	namespace := "namespace"
	clientMock, newWorkerSliceGateway, ctx := setupWorkerSliceGatewayWebhookValidationTest(name, namespace)
	existingWorkerSliceGateway := workerv1alpha1.WorkerSliceGateway{}
	existingWorkerSliceGateway.Spec.GatewayNumber = 1
	newWorkerSliceGateway.Spec.GatewayNumber = 2
	_, err := ValidateWorkerSliceGatewayUpdate(ctx, &existingWorkerSliceGateway, runtime.Object(newWorkerSliceGateway))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.GatewayNumber: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateWorkerSliceGatewayWithoutErrors(t *testing.T) {
	name := "worker_slice_Gateway"
	namespace := "namespace"
	clientMock, newWorkerSliceGateway, ctx := setupWorkerSliceGatewayWebhookValidationTest(name, namespace)
	_, err := ValidateWorkerSliceGatewayUpdate(ctx, newWorkerSliceGateway, runtime.Object(newWorkerSliceGateway))
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func setupWorkerSliceGatewayWebhookValidationTest(name string, namespace string) (*utilMock.Client, *workerv1alpha1.WorkerSliceGateway, context.Context) {
	clientMock := &utilMock.Client{}
	workerSliceGateway := &workerv1alpha1.WorkerSliceGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "WorkerSliceGatewayWebhookValidationServiceTest", nil)
	return clientMock, workerSliceGateway, ctx
}
