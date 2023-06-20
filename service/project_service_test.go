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

	"github.com/kubeslice/kubeslice-controller/metrics"
	metricMock "github.com/kubeslice/kubeslice-controller/metrics/mocks"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sapimachinery "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestProjectSuite(t *testing.T) {
	for k, v := range ProjectTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ProjectTestbed = map[string]func(*testing.T){
	"TestReconcileProject_CreatesResourcesAndReturnsReconciliationComplete_Happypath":                                  TestReconcileProject_AddsREconciler_CreatesResourcesAndReturnsReconciliationComplete_Happypath,
	"TestReconcileProject_ReturnsReconciliationCompleteAndErrorWhenGetProjectNamespaceFailsWithErrorOtherThanNotFound": TestReconcileProject_ReturnsReconciliationCompleteAndErrorWhenGetProjectNamespaceFailsWithErrorOtherThanNotFound,
	"TestReconcileProject_ReturnsReconciliationCompleteAndNilErrorWhenGetProjectNamespaceIsNotFound":                   TestReconcileProject_ReturnsReconciliationCompleteAndNilErrorWhenGetProjectNamespaceIsNotFound,
	"TestReconcileProject_Delete_Happypath":                                                                            TestReconcileProject_Delete_Happypath,
	"TestReconcileProject_DeletionFailed":                                                                              TestReconcileProject_DeletionFailed,
	"TestReconcileProject_DoNotCallFinalizerIfItExists":                                                                TestReconcileProject_DoNotCallFinalizerIfItExists,
}

func TestReconcileProject_Delete_Happypath(t *testing.T) {
	projectName := "cisco"
	namespace := "controller-manager"
	time := k8sapimachinery.Now()
	nsServiceMock, _, projectService, requestObj, clientMock, project, ctx, clusterServiceMock, sliceConfigServiceMock, serviceExportConfigServiceMock, sliceQoSConfigServiceMock, mMock := setupProjectTest(projectName, namespace)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, project).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Project)
		arg.ObjectMeta.DeletionTimestamp = &time

	})
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.GetName())
	clusterServiceMock.On("DeleteClusters", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	serviceExportConfigServiceMock.On("DeleteServiceExportConfigs", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	nsServiceMock.On("DeleteNamespace", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	sliceConfigServiceMock.On("DeleteSliceConfigs", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	sliceQoSConfigServiceMock.On("DeleteSliceQoSConfig", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	//delete finalizer
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()

	result, err := projectService.ReconcileProject(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	nsServiceMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func TestReconcileProject_DeletionFailed(t *testing.T) {
	projectName := "cisco"
	namespace := "controller-manager"
	time := k8sapimachinery.Now()
	nsServiceMock, _, projectService, requestObj, clientMock, project, ctx, clusterServiceMock, sliceConfigServiceMock, serviceExportConfigServiceMock, sliceQoSConfigServiceMock, mMock := setupProjectTest(projectName, namespace)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, project).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Project)
		arg.ObjectMeta.DeletionTimestamp = &time

	})
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.GetName())
	clusterServiceMock.On("DeleteClusters", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	serviceExportConfigServiceMock.On("DeleteServiceExportConfigs", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	nsServiceMock.On("DeleteNamespace", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	sliceConfigServiceMock.On("DeleteSliceConfigs", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	sliceQoSConfigServiceMock.On("DeleteSliceQoSConfig", ctx, projectNamespace).Return(ctrl.Result{}, nil).Once()
	//delete finalizer
	updateError := errors.New("update error")
	clientMock.On("Update", ctx, mock.Anything).Return(updateError).Once()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()

	result, err := projectService.ReconcileProject(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), updateError.Error())
	clientMock.AssertExpectations(t)
	nsServiceMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func TestReconcileProject_AddsREconciler_CreatesResourcesAndReturnsReconciliationComplete_Happypath(t *testing.T) {
	projectName := "cisco"
	namespace := "controller-manager"
	//time := k8sapimachinery.Now()
	readOnlyServiceAccounts := []string{"sany-ro"}
	readWriteServiceAccounts := []string{"imran-rw"}
	nsServiceMock, acsServicemOCK, projectService, requestObj, clientMock, project, ctx, _, _, _, _, mMock := setupProjectTest(projectName, namespace)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, project).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Project)
		arg.Spec.ServiceAccount.ReadOnly = readOnlyServiceAccounts
		arg.Spec.ServiceAccount.ReadWrite = readWriteServiceAccounts
	}).Once()
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.GetName())

	//add finalizer
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()

	nsServiceMock.On("ReconcileProjectNamespace", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileWorkerClusterRole", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadOnlyRole", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadWriteRole", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadOnlyUserServiceAccountAndRoleBindings", ctx, projectNamespace, readOnlyServiceAccounts, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadWriteUserServiceAccountAndRoleBindings", ctx, projectNamespace, readWriteServiceAccounts, mock.Anything).Return(ctrl.Result{}, nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()

	// create default sliceQoSConfig
	sliceQoSConfigNamespacedName := types.NamespacedName{Name: util.DefaultSliceQOSConfigName, Namespace: projectNamespace}
	errorNotFound := k8sError.NewNotFound(schema.ParseGroupResource("sliceQoSConfig"), sliceQoSConfigNamespacedName.Name)
	clientMock.On("Get", ctx, sliceQoSConfigNamespacedName, mock.Anything).Return(errorNotFound).Once()
	clientMock.On("Create", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()

	result, err := projectService.ReconcileProject(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	nsServiceMock.AssertExpectations(t)
	acsServicemOCK.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func TestReconcileProject_DoNotCallFinalizerIfItExists(t *testing.T) {
	projectName := "cisco"
	namespace := "controller-manager"
	//time := k8sapimachinery.Now()
	readOnlyServiceAccounts := []string{"sany-ro"}
	readWriteServiceAccounts := []string{"imran-rw"}
	nsServiceMock, acsServicemOCK, projectService, requestObj, clientMock, project, ctx, _, _, _, _, mMock := setupProjectTest(projectName, namespace)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, project).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Project)
		controllerutil.AddFinalizer(arg, ProjectFinalizer)
		//arg.ObjectMeta.Finalizers = []string{ProjectFinalizer}
		arg.Spec.ServiceAccount.ReadOnly = readOnlyServiceAccounts
		arg.Spec.ServiceAccount.ReadWrite = readWriteServiceAccounts
	}).Once()
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.GetName())

	nsServiceMock.On("ReconcileProjectNamespace", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileWorkerClusterRole", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadOnlyRole", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadWriteRole", ctx, projectNamespace, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadOnlyUserServiceAccountAndRoleBindings", ctx, projectNamespace, readOnlyServiceAccounts, mock.Anything).Return(ctrl.Result{}, nil).Once()
	acsServicemOCK.On("ReconcileReadWriteUserServiceAccountAndRoleBindings", ctx, projectNamespace, readWriteServiceAccounts, mock.Anything).Return(ctrl.Result{}, nil).Once()
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	sliceQoSConfigNamespacedName := types.NamespacedName{Name: util.DefaultSliceQOSConfigName, Namespace: projectNamespace}

	clientMock.On("Get", ctx, sliceQoSConfigNamespacedName, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceQoSConfig)
		arg.Name = util.DefaultSliceQOSConfigName
		arg.Namespace = projectNamespace
	}).Once()
	result, err := projectService.ReconcileProject(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	nsServiceMock.AssertExpectations(t)
	acsServicemOCK.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func TestReconcileProject_ReturnsReconciliationCompleteAndErrorWhenGetProjectNamespaceFailsWithErrorOtherThanNotFound(t *testing.T) {
	projectName := "do-not-exist"
	namespace := "controller-manager"
	_, _, projectService, requestObj, clientMock, project, ctx, _, _, _, _, _ := setupProjectTest(projectName, namespace)
	err1 := errors.New("testinternalerror")
	clientMock.On("Get", ctx, requestObj.NamespacedName, project).Return(err1)
	result, err := projectService.ReconcileProject(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.Error(t, err)
	require.Equal(t, result, expectedResult)
	require.Equal(t, err, err1)
	clientMock.AssertExpectations(t)
}

func TestReconcileProject_ReturnsReconciliationCompleteAndNilErrorWhenGetProjectNamespaceIsNotFound(t *testing.T) {
	projectName := "do-not-exist"
	namespace := "controller-manager"
	_, _, projectService, requestObj, clientMock, project, ctx, _, _, _, _, _ := setupProjectTest(projectName, namespace)
	notFoundError := k8sError.NewNotFound(util.Resource("projecttest"), "isnotFound")
	clientMock.On("Get", ctx, requestObj.NamespacedName, project).Return(notFoundError).Once()
	result, err := projectService.ReconcileProject(ctx, requestObj)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func setupProjectTest(name string, namespace string) (*mocks.INamespaceService, *mocks.IAccessControlService, ProjectService, ctrl.Request, *utilMock.Client, *controllerv1alpha1.Project, context.Context, *mocks.IClusterService, *mocks.ISliceConfigService, *mocks.IServiceExportConfigService, *mocks.ISliceQoSConfigService, *metricMock.IMetricRecorder) {
	nsServiceMock := &mocks.INamespaceService{}
	acsServicemOCK := &mocks.IAccessControlService{}
	clusterServiceMock := &mocks.IClusterService{}
	sliceConfigServiceMock := &mocks.ISliceConfigService{}
	sliceQoSConfigServiceMock := &mocks.ISliceQoSConfigService{}
	serviceExportConfigServiceMock := &mocks.IServiceExportConfigService{}
	mMock := &metricMock.IMetricRecorder{}
	projectService := ProjectService{
		ns:  nsServiceMock,
		acs: acsServicemOCK,
		c:   clusterServiceMock,
		sc:  sliceConfigServiceMock,
		se:  serviceExportConfigServiceMock,
		q:   sliceQoSConfigServiceMock,
		mf:  mMock,
	}

	projectName := types.NamespacedName{
		Namespace: name,
		Name:      namespace,
	}
	requestObj := ctrl.Request{
		projectName,
	}
	clientMock := &utilMock.Client{}
	project := &controllerv1alpha1.Project{}

	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareProjectTestContext(context.Background(), clientMock, scheme)
	return nsServiceMock, acsServicemOCK, projectService, requestObj, clientMock, project, ctx, clusterServiceMock, sliceConfigServiceMock, serviceExportConfigServiceMock, sliceQoSConfigServiceMock, mMock
}

func prepareProjectTestContext(ctx context.Context, client util.Client,
	scheme *runtime.Scheme) context.Context {
	eventRecorder := events.NewEventRecorder(client, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	preparedCtx := util.PrepareKubeSliceControllersRequestContext(ctx, client, scheme, "ProjectTestController", &eventRecorder)
	return preparedCtx
}
