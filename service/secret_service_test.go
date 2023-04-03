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

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"

	"testing"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dailymotion/allure-go"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSecretSuite(t *testing.T) {
	for k, v := range SecretTestBed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var SecretTestBed = map[string]func(*testing.T){
	"Secret_DeleteHappyCase":           SecretDeleteHappyCase,
	"Secret_GetErrorOtherThanNotFound": SecretGetErrorOtherThanNotFound,
	"Secret_GetErrorNotFound":          SecretGetErrorNotFound,
	"Secret_FoundButErrorOnDelete":     SecretFoundButErrorOnDelete,
}

func SecretDeleteHappyCase(t *testing.T) {
	secretService, clientMock, secret, ctx := setupSecretTest("secret", "namespace")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), &corev1.Secret{}).Return(nil).Once()
	clientMock.On("Delete", ctx, secret).Return(nil).Once()
	result, err := secretService.DeleteSecret(ctx, secret.Namespace, secret.Name)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func SecretGetErrorOtherThanNotFound(t *testing.T) {
	secretService, clientMock, secret, ctx := setupSecretTest("secret", "namespace")
	err1 := errors.New("internal_error")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), &corev1.Secret{}).Return(err1).Once()
	result, err2 := secretService.DeleteSecret(ctx, secret.Namespace, secret.Name)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	clientMock.AssertExpectations(t)
}

func SecretGetErrorNotFound(t *testing.T) {
	secretService, clientMock, secret, ctx := setupSecretTest("secret", "namespace")
	notFoundError := k8sError.NewNotFound(util.Resource("SecretTest"), "isNotFound")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), &corev1.Secret{}).Return(notFoundError).Once()
	result, err := secretService.DeleteSecret(ctx, secret.Namespace, secret.Name)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, expectedResult, result)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func SecretFoundButErrorOnDelete(t *testing.T) {
	secretService, clientMock, secret, ctx := setupSecretTest("secret", "namespace")
	err1 := errors.New("internal_error")
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), &corev1.Secret{}).Return(nil).Once()
	clientMock.On("Delete", ctx, secret).Return(err1).Once()
	result, err2 := secretService.DeleteSecret(ctx, secret.Namespace, secret.Name)
	expectedResult := ctrl.Result{}
	require.Error(t, err2)
	require.Equal(t, expectedResult, result)
	require.Equal(t, err1, err2)
	clientMock.AssertExpectations(t)
}

func setupSecretTest(name string, namespace string) (SecretService, *utilMock.Client, *corev1.Secret, context.Context) {
	secretService := SecretService{}
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	eventRecorder := events.NewEventRecorder(clientMock, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, scheme, "SecretServiceTest", &eventRecorder)
	return secretService, clientMock, secret, ctx
}
