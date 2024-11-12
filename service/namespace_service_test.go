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

	"github.com/kubeslice/kubeslice-controller/metrics"
	metricMock "github.com/kubeslice/kubeslice-controller/metrics/mocks"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNamespaceSuite(t *testing.T) {
	for k, v := range NamespaceTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var NamespaceTestbed = map[string]func(*testing.T){
	"TestReconcileProjectNamespace_NamespaceGetsCreatedWithOwnerLabelAndReturnsReconciliationComplete_Happypath": TestReconcileProjectNamespace_NamespaceGetsCreatedWithOwnerLabelAndReturnsReconciliationComplete_Happypath,
	// "TestReconcileProjectNamespace_DoesNothingIfNamespaceExistAlready":                                           TestReconcileProjectNamespace_DoesNothingIfNamespaceExistAlready,
	"TestDeleteNamespace_DeletesObjectWithReconciliationComplete": TestDeleteNamespace_DeletesObjectWithReconciliationComplete,
	"TestDeleteNamespace_DoesNothingIfNamespaceDoNotExist":        TestDeleteNamespace_DoesNothingIfNamespaceDoNotExist,
}

func TestReconcileProjectNamespace_NamespaceGetsCreatedWithOwnerLabelAndReturnsReconciliationComplete_Happypath(t *testing.T) {
	namespaceName := "cisco"
	mMock := &metricMock.IMetricRecorder{}
	namespaceService := NamespaceService{
		mf: mMock,
	}
	namespace := &corev1.Namespace{}
	namespaceObject := client.ObjectKey{
		Name: namespaceName,
	}
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	notFoundError := k8sError.NewNotFound(util.Resource("namespacetest"), "isnotFound")
	ctx := prepareNamespaceTestContext(context.Background(), clientMock, scheme)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, namespaceObject, namespace).Return(notFoundError)

	project := &controllerv1alpha1.Project{}

	labels := namespaceService.getResourceLabel(namespaceName, project)

	projectToCreateWithLabel := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespaceName,
			Labels: labels,
		},
	}
	clientMock.On("Create", ctx, projectToCreateWithLabel).Return(nil)
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := namespaceService.ReconcileProjectNamespace(ctx, namespaceName, project)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

// func TestReconcileProjectNamespace_DoesNothingIfNamespaceExistAlready(t *testing.T) {
// 	namespaceName := "cisco"
// 	mMock := &metricMock.IMetricRecorder{}
// 	namespaceService := NamespaceService{
// 		mf: mMock,
// 	}

// 	namespace := &corev1.Namespace{}
// 	namespaceObject := client.ObjectKey{
// 		Name: namespaceName,
// 	}
// 	clientMock := &utilMock.Client{}
// 	scheme := runtime.NewScheme()
// 	controllerv1alpha1.AddToScheme(scheme)
// 	ctx := prepareNamespaceTestContext(context.Background(), clientMock, scheme)
// 	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
// 	clientMock.On("Get", ctx, namespaceObject, namespace).Return(nil)
// 	project := &controllerv1alpha1.Project{}
// 	project.ObjectMeta.Labels = map[string]string{"testLabel": "testValue"}
// 	result, err := namespaceService.ReconcileProjectNamespace(ctx, namespaceName, project)
// 	expectedResult := ctrl.Result{}
// 	require.Equal(t, result, expectedResult)
// 	require.Nil(t, err)
// 	clientMock.AssertExpectations(t)
// 	mMock.AssertExpectations(t)
// }

func TestDeleteNamespace_DeletesObjectWithReconciliationComplete(t *testing.T) {
	namespaceName := "cisco"
	mMock := &metricMock.IMetricRecorder{}
	namespaceService := NamespaceService{
		mf: mMock,
	}

	namespace := &corev1.Namespace{}
	namespaceObject := client.ObjectKey{
		Name: namespaceName,
	}
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareNamespaceTestContext(context.Background(), clientMock, scheme)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, namespaceObject, namespace).Return(nil)
	clientMock.On("Delete", ctx, mock.Anything).Return(nil)
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := namespaceService.DeleteNamespace(ctx, namespaceName)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func TestDeleteNamespace_DoesNothingIfNamespaceDoNotExist(t *testing.T) {
	namespaceName := "cisco"
	mMock := &metricMock.IMetricRecorder{}
	namespaceService := NamespaceService{
		mf: mMock,
	}

	namespace := &corev1.Namespace{}
	namespaceObject := client.ObjectKey{
		Name: namespaceName,
	}
	clientMock := &utilMock.Client{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	notFoundError := k8sError.NewNotFound(util.Resource("namespacetest"), "isnotFound")
	ctx := prepareNamespaceTestContext(context.Background(), clientMock, scheme)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, namespaceObject, namespace).Return(notFoundError)
	result, err := namespaceService.DeleteNamespace(ctx, namespaceName)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func prepareNamespaceTestContext(ctx context.Context, client util.Client, scheme *runtime.Scheme) context.Context {
	eventRecorder := events.NewEventRecorder(client, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	preparedCtx := util.PrepareKubeSliceControllersRequestContext(ctx, client, scheme, "NamespaceTestController", &eventRecorder)
	return preparedCtx
}
