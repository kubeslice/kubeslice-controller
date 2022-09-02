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
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	"github.com/dailymotion/allure-go"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkerSliceConfigWebhookValidationSuite(t *testing.T) {
	for k, v := range WorkerSliceConfigWebhookValidationTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var WorkerSliceConfigWebhookValidationTestBed = map[string]func(*testing.T){
	"WorkerSliceConfigWebhookValidation_UpdateValidateWorkerSliceConfigUpdatingOctet": UpdateValidateWorkerSliceConfigUpdatingOctet,
	"WorkerSliceConfigWebhookValidation_UpdateValidateWorkerSliceConfigWithoutErrors": UpdateValidateWorkerSliceConfigWithoutErrors,
}

func UpdateValidateWorkerSliceConfigUpdatingOctet(t *testing.T) {
	name := "worker_slice_config"
	namespace := "namespace"
	clientMock, newWorkerSliceConfig, ctx := setupWorkerSliceConfigWebhookValidationTest(name, namespace)
	existingWorkerSliceConfig := workerv1alpha1.WorkerSliceConfig{}
	a1 := 1
	existingWorkerSliceConfig.Spec.Octet = &a1
	a2 := 2
	newWorkerSliceConfig.Spec.Octet = &a2
	err := ValidateWorkerSliceConfigUpdate(ctx, &existingWorkerSliceConfig, runtime.Object(newWorkerSliceConfig))
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Spec.Octet: Invalid value:")
	clientMock.AssertExpectations(t)
}

func UpdateValidateWorkerSliceConfigWithoutErrors(t *testing.T) {
	name := "worker_slice_config"
	namespace := "namespace"
	clientMock, newWorkerSliceConfig, ctx := setupWorkerSliceConfigWebhookValidationTest(name, namespace)
	a1 := 1
	newWorkerSliceConfig.Spec.Octet = &a1
	err := ValidateWorkerSliceConfigUpdate(ctx, newWorkerSliceConfig, runtime.Object(newWorkerSliceConfig))
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func setupWorkerSliceConfigWebhookValidationTest(name string, namespace string) (*utilMock.Client, *workerv1alpha1.WorkerSliceConfig, context.Context) {
	clientMock := &utilMock.Client{}
	workerSliceConfig := &workerv1alpha1.WorkerSliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "WorkerSliceConfigWebhookValidationServiceTest")
	return clientMock, workerSliceConfig, ctx
}
