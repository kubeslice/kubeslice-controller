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
	"fmt"
	"testing"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAccessControlServiceSuite(t *testing.T) {
	for k, v := range ACSTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ACSTestbed = map[string]func(*testing.T){
	"TestAccessControlService_ReconcileWorkerClusterRole_Create":                               AccessControlService_ReconcileWorkerClusterRole_Create,
	"TestAccessControlService_ReconcileWorkerClusterRole_Update":                               AccessControlService_ReconcileWorkerClusterRole_Update,
	"TestAccessControlService_ReconcileReadOnlyRole_Create":                                    AccessControlService_ReconcileReadOnlyRole_Create,
	"TestAccessControlService_ReconcileReadOnlyRole_Update":                                    AccessControlService_ReconcileReadOnlyRole_Update,
	"TestAccessControlService_ReconcileReadWriteRole_Create":                                   AccessControlService_ReconcileReadWriteRole_Create,
	"TestAccessControlServiceReconcileReadWriteRole_Update":                                    AccessControlServiceReconcileReadWriteRole_Update,
	"TestACS_CreateOrUpdateServiceAccountsAndRoleBindings_Create":                              ACS_CreateOrUpdateServiceAccountsAndRoleBindings_Create,
	"TestACS_CreateOrUpdateServiceAccountsAndRoleBindings_SA_exists_RoleBinding_exists_update": ACS_CreateOrUpdateServiceAccountsAndRoleBindings_SA_exists_RoleBinding_exists_update,
	"TestACS_ReconcileWorkerClusterServiceAccountAndRoleBindings":                              ACS_ReconcileWorkerClusterServiceAccountAndRoleBindings,
	"TestACS_ReconcileReadWriteUserServiceAccountAndRoleBindings":                              ACS_ReconcileReadWriteUserServiceAccountAndRoleBindings,
	"TestACS_ReconcileReadOnlyUserServiceAccountAndRoleBindings":                               ACS_ReconcileReadOnlyUserServiceAccountAndRoleBindings,
	"TestACS_RemoveWorkerClusterServiceAccountAndRoleBindings_Happypath":                       ACS_RemoveWorkerClusterServiceAccountAndRoleBindings_Happypath,
	"TestACS_cleanupObsoleteServiceAccountsAndRoleBindings_happypath":                          ACS_cleanupObsoleteServiceAccountsAndRoleBindings_happypath,
	"TestACS_removeServiceAccountsAndRoleBindingsByLabel_happypath":                            ACS_removeServiceAccountsAndRoleBindingsByLabel_happypath,
}

func AccessControlService_ReconcileWorkerClusterRole_Create(t *testing.T) {
	namespace := "kubeslice-controller-cisco"
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleWorkerCluster,
	}
	project := &controllerv1alpha1.Project{}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{verbCreate, verbDelete, verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceServiceExportConfigs},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig + resourceStatusSuffix, resourceWorkerSliceGateways + resourceStatusSuffix, resourceWorkerServiceImport + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch, verbCreate, verbUpdate, verbPatch},
				APIGroups: []string{""},
				Resources: []string{resourceSecrets},
			},
			{
				Verbs:     []string{verbCreate, verbPatch},
				APIGroups: []string{""},
				Resources: []string{resourceEvents},
			},
		},
	}
	actualRole := &rbacv1.Role{}
	clientMock := &utilMock.Client{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	notFoundError := k8sError.NewNotFound(util.Resource("acstest"), "isnotFound")
	clientMock.On("Get", ctx, namespacedName, actualRole).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedRole).Return(nil).Once()
	acsService := AccessControlService{}
	result, err := acsService.ReconcileWorkerClusterRole(ctx, namespace, project, expectedRole.Rules)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func AccessControlService_ReconcileWorkerClusterRole_Update(t *testing.T) {
	namespace := "kubeslice-controller-cisco"
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleWorkerCluster,
	}
	project := &controllerv1alpha1.Project{}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{verbCreate, verbDelete, verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceServiceExportConfigs},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig + resourceStatusSuffix, resourceWorkerSliceGateways + resourceStatusSuffix, resourceWorkerServiceImport + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch, verbCreate, verbUpdate, verbPatch},
				APIGroups: []string{""},
				Resources: []string{resourceSecrets},
			},
			{
				Verbs:     []string{verbCreate, verbPatch},
				APIGroups: []string{""},
				Resources: []string{resourceEvents},
			},
		},
	}
	actualRole := &rbacv1.Role{}
	clientMock := &utilMock.Client{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, namespacedName, actualRole).Return(nil).Once()
	clientMock.On("Update", ctx, expectedRole).Return(nil).Once()
	acsService := AccessControlService{}
	result, err := acsService.ReconcileWorkerClusterRole(ctx, namespace, project, expectedRole.Rules)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func AccessControlService_ReconcileReadOnlyRole_Create(t *testing.T) {
	namespace := "kubeslice-controller-cisco"
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleSharedReadOnly,
	}
	project := &controllerv1alpha1.Project{}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster, resourceSliceConfig, resourceServiceExportConfigs},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
			},
			{
				Verbs:     []string{verbGet},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster + resourceStatusSuffix, resourceSliceConfig + resourceStatusSuffix, resourceServiceExportConfigs + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig + resourceStatusSuffix, resourceWorkerSliceGateways + resourceStatusSuffix, resourceWorkerServiceImport + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{""},
				Resources: []string{resourceSecrets},
			},
		},
	}
	actualRole := &rbacv1.Role{}
	clientMock := &utilMock.Client{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	notFoundError := k8sError.NewNotFound(util.Resource("acstest_readonly_role"), "isnotFound")
	clientMock.On("Get", ctx, namespacedName, actualRole).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedRole).Return(nil).Once()
	acsService := AccessControlService{}
	result, err := acsService.ReconcileReadOnlyRole(ctx, namespace, project, expectedRole.Rules)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func AccessControlService_ReconcileReadOnlyRole_Update(t *testing.T) {
	namespace := "kubeslice-controller-cisco"
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleSharedReadOnly,
	}
	project := &controllerv1alpha1.Project{}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster, resourceSliceConfig, resourceServiceExportConfigs},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
			},
			{
				Verbs:     []string{verbGet},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster + resourceStatusSuffix, resourceSliceConfig + resourceStatusSuffix, resourceServiceExportConfigs + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig + resourceStatusSuffix, resourceWorkerSliceGateways + resourceStatusSuffix, resourceWorkerServiceImport + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{""},
				Resources: []string{resourceSecrets},
			},
		},
	}
	actualRole := &rbacv1.Role{}
	clientMock := &utilMock.Client{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, namespacedName, actualRole).Return(nil).Once()
	clientMock.On("Update", ctx, expectedRole).Return(nil).Once()
	acsService := AccessControlService{}
	result, err := acsService.ReconcileReadOnlyRole(ctx, namespace, project, expectedRole.Rules)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func AccessControlService_ReconcileReadWriteRole_Create(t *testing.T) {
	namespace := "kubeslice-controller-cisco"
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleSharedReadWrite,
	}
	project := &controllerv1alpha1.Project{}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{verbCreate, verbDelete, verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster, resourceSliceConfig, resourceServiceExportConfigs},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster + resourceStatusSuffix, resourceSliceConfig + resourceStatusSuffix, resourceServiceExportConfigs + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig + resourceStatusSuffix, resourceWorkerSliceGateways + resourceStatusSuffix, resourceWorkerServiceImport + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{""},
				Resources: []string{resourceSecrets},
			},
		},
	}
	actualRole := &rbacv1.Role{}
	clientMock := &utilMock.Client{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	notFoundError := k8sError.NewNotFound(util.Resource("acstest_readonly_role"), "isnotFound")
	clientMock.On("Get", ctx, namespacedName, actualRole).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedRole).Return(nil).Once()
	acsService := AccessControlService{}
	result, err := acsService.ReconcileReadWriteRole(ctx, namespace, project, expectedRole.Rules)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func AccessControlServiceReconcileReadWriteRole_Update(t *testing.T) {
	namespace := "kubeslice-controller-cisco"
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleSharedReadWrite,
	}
	project := &controllerv1alpha1.Project{}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{verbCreate, verbDelete, verbUpdate, verbPatch, verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster, resourceSliceConfig, resourceServiceExportConfigs},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig, resourceWorkerSliceGateways, resourceWorkerServiceImport},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceControllers},
				Resources: []string{resourceCluster + resourceStatusSuffix, resourceSliceConfig + resourceStatusSuffix, resourceServiceExportConfigs + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbUpdate, verbPatch, verbGet},
				APIGroups: []string{apiGroupKubeSliceWorker},
				Resources: []string{resourceWorkerSliceConfig + resourceStatusSuffix, resourceWorkerSliceGateways + resourceStatusSuffix, resourceWorkerServiceImport + resourceStatusSuffix},
			},
			{
				Verbs:     []string{verbGet, verbList, verbWatch},
				APIGroups: []string{""},
				Resources: []string{resourceSecrets},
			},
		},
	}
	actualRole := &rbacv1.Role{}
	clientMock := &utilMock.Client{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, namespacedName, actualRole).Return(nil).Once()
	clientMock.On("Update", ctx, expectedRole).Return(nil).Once()
	acsService := AccessControlService{}
	result, err := acsService.ReconcileReadWriteRole(ctx, namespace, project, expectedRole.Rules)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func ACS_CreateOrUpdateServiceAccountsAndRoleBindings_Create(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	readonlynames := []string{"user1"}
	namespace := "cisco"
	project := &controllerv1alpha1.Project{}
	serviceAccountNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(ServiceAccountWorkerCluster, readonlynames[0]),
	}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountNamespacedName.Name,
			Namespace: serviceAccountNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeClusterReadWrite,
			},
		},
	}
	actualServiceAccount := &corev1.ServiceAccount{}
	notFoundError := k8sError.NewNotFound(util.Resource("acstest_readonly_role"), "isnotFound")
	clientMock.On("Get", ctx, serviceAccountNamespacedName, actualServiceAccount).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedServiceAccount).Return(nil)
	roleBindingNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(RoleBindingWorkerCluster, readonlynames[0]),
	}
	expectedRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingNamespacedName.Name,
			Namespace: roleBindingNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeClusterReadWrite,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: roleWorkerCluster,
			Kind: "Role",
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      fmt.Sprintf(ServiceAccountWorkerCluster, readonlynames[0]),
				Namespace: namespace,
				Kind:      "ServiceAccount",
			},
		},
	}
	actualRoleBinding := &rbacv1.RoleBinding{}
	clientMock.On("Get", ctx, roleBindingNamespacedName, actualRoleBinding).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedRoleBinding).Return(nil).Once()
	result, err := acsService.createOrUpdateServiceAccountsAndRoleBindings(ctx, namespace, readonlynames, ServiceAccountWorkerCluster, RoleBindingWorkerCluster, AccessTypeClusterReadWrite, roleWorkerCluster, project)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func ACS_CreateOrUpdateServiceAccountsAndRoleBindings_SA_exists_RoleBinding_exists_update(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	readonlynames := []string{"user1"}
	namespace := "cisco"
	project := &controllerv1alpha1.Project{}
	serviceAccountNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(ServiceAccountWorkerCluster, readonlynames[0]),
	}

	actualServiceAccount := &corev1.ServiceAccount{}

	clientMock.On("Get", ctx, serviceAccountNamespacedName, actualServiceAccount).Return(nil).Once()
	roleBindingNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(RoleBindingWorkerCluster, readonlynames[0]),
	}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingNamespacedName.Name,
			Namespace: roleBindingNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeClusterReadWrite,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: roleWorkerCluster,
			Kind: "Role",
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      fmt.Sprintf(ServiceAccountWorkerCluster, readonlynames[0]),
				Namespace: namespace,
				Kind:      "ServiceAccount",
			},
		},
	}
	actualRoleBinding := &rbacv1.RoleBinding{}
	clientMock.On("Get", ctx, roleBindingNamespacedName, actualRoleBinding).Return(nil).Once()
	clientMock.On("Update", ctx, expectedRoleBinding).Return(nil).Once()
	result, err := acsService.createOrUpdateServiceAccountsAndRoleBindings(ctx, namespace, readonlynames, ServiceAccountWorkerCluster, RoleBindingWorkerCluster, AccessTypeClusterReadWrite, roleWorkerCluster, project)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func ACS_removeServiceAccountsAndRoleBindingsByLabel_happypath(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	clusterName := []string{"cluster-1"}
	namespace := "cisco"
	cluster := &controllerv1alpha1.Cluster{}
	cluster.ObjectMeta.Labels = map[string]string{"ccc": "ggg"}

	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*rbacv1.RoleBindingList)
		arg.Items = []rbacv1.RoleBinding{{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Subjects:   nil,
			RoleRef:    rbacv1.RoleRef{},
		},
			{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Subjects:   nil,
				RoleRef:    rbacv1.RoleRef{},
			},
		}
	}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.ServiceAccountList)
		arg.Items = []corev1.ServiceAccount{{
			TypeMeta:                     metav1.TypeMeta{},
			ObjectMeta:                   metav1.ObjectMeta{},
			Secrets:                      nil,
			ImagePullSecrets:             nil,
			AutomountServiceAccountToken: nil,
		},
			{
				TypeMeta:                     metav1.TypeMeta{},
				ObjectMeta:                   metav1.ObjectMeta{},
				Secrets:                      nil,
				ImagePullSecrets:             nil,
				AutomountServiceAccountToken: nil,
			},
		}
	}).Once()

	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Times(4)
	result, err := acsService.removeServiceAccountsAndRoleBindingsByLabel(ctx, namespace, clusterName, cluster)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func ACS_cleanupObsoleteServiceAccountsAndRoleBindings_happypath(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	readonlynames := []string{"user1"}
	namespace := "cisco"
	project := &controllerv1alpha1.Project{}

	accessType := AccessTypeReadOnly
	annotationKey := fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel)
	annotationValue := accessType
	roleBindingValid_1 := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
	}
	roleBindingInvalid := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: "someothervalue"},
		},
	}
	serviceAccountValid_1 := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	serviceAccountInValid := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: "some invalid value"},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*rbacv1.RoleBindingList)
		arg.Items = []rbacv1.RoleBinding{
			roleBindingValid_1,
			roleBindingInvalid,
		}
	}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.ServiceAccountList)
		arg.Items = []corev1.ServiceAccount{
			serviceAccountValid_1,
			serviceAccountInValid,
		}
	}).Once()

	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Times(2)

	result, err := acsService.cleanupObsoleteServiceAccountsAndRoleBindings(ctx, namespace, readonlynames, ServiceAccountReadOnlyUser, RoleBindingReadOnlyUser, accessType, project)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func ACS_RemoveWorkerClusterServiceAccountAndRoleBindings_Happypath(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	clusterNameString := "cluster-1"
	//clusterName := []string{clusterNameString}

	namespace := "cisco"
	cluster := &controllerv1alpha1.Cluster{}
	cluster.ObjectMeta.Labels = map[string]string{"ccc": "ggg"}

	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*rbacv1.RoleBindingList)
		arg.Items = []rbacv1.RoleBinding{{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Subjects:   nil,
			RoleRef:    rbacv1.RoleRef{},
		},
			{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Subjects:   nil,
				RoleRef:    rbacv1.RoleRef{},
			},
		}
	}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.ServiceAccountList)
		arg.Items = []corev1.ServiceAccount{{
			TypeMeta:                     metav1.TypeMeta{},
			ObjectMeta:                   metav1.ObjectMeta{},
			Secrets:                      nil,
			ImagePullSecrets:             nil,
			AutomountServiceAccountToken: nil,
		},
			{
				TypeMeta:                     metav1.TypeMeta{},
				ObjectMeta:                   metav1.ObjectMeta{},
				Secrets:                      nil,
				ImagePullSecrets:             nil,
				AutomountServiceAccountToken: nil,
			},
		}
	}).Once()

	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Times(4)
	result, err := acsService.RemoveWorkerClusterServiceAccountAndRoleBindings(ctx, clusterNameString, namespace, cluster)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func ACS_ReconcileReadOnlyUserServiceAccountAndRoleBindings(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	readonlynames := []string{"user1"}
	namespace := "cisco"
	project := &controllerv1alpha1.Project{}

	accessType := AccessTypeReadOnly
	annotationKey := fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel)
	annotationValue := accessType
	roleBindingValid_1 := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
	}
	roleBindingInvalid := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: "someothervalue"},
		},
	}
	serviceAccountValid_1 := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	serviceAccountInValid := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: "some invalid value"},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*rbacv1.RoleBindingList)
		arg.Items = []rbacv1.RoleBinding{
			roleBindingValid_1,
			roleBindingInvalid,
		}
	}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.ServiceAccountList)
		arg.Items = []corev1.ServiceAccount{
			serviceAccountValid_1,
			serviceAccountInValid,
		}
	}).Once()

	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Times(2)

	serviceAccountNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(ServiceAccountReadOnlyUser, readonlynames[0]),
	}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountNamespacedName.Name,
			Namespace: serviceAccountNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeReadOnly,
			},
		},
	}
	actualServiceAccount := &corev1.ServiceAccount{}
	notFoundError := k8sError.NewNotFound(util.Resource("acstest_readonly_role"), "isnotFound")
	clientMock.On("Get", ctx, serviceAccountNamespacedName, actualServiceAccount).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedServiceAccount).Return(nil)
	roleBindingNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(RoleBindingReadOnlyUser, readonlynames[0]),
	}
	expectedRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingNamespacedName.Name,
			Namespace: roleBindingNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeReadOnly,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: roleSharedReadOnly,
			Kind: "Role",
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      fmt.Sprintf(ServiceAccountReadOnlyUser, readonlynames[0]),
				Namespace: namespace,
				Kind:      "ServiceAccount",
			},
		},
	}
	actualRoleBinding := &rbacv1.RoleBinding{}
	clientMock.On("Get", ctx, roleBindingNamespacedName, actualRoleBinding).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedRoleBinding).Return(nil).Once()

	result, err := acsService.ReconcileReadOnlyUserServiceAccountAndRoleBindings(ctx, namespace, readonlynames, project)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func ACS_ReconcileReadWriteUserServiceAccountAndRoleBindings(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	readonlynames := []string{"user1"}
	namespace := "cisco"
	project := &controllerv1alpha1.Project{}

	accessType := AccessTypeReadWrite
	annotationKey := fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel)
	annotationValue := accessType
	roleBindingValid_1 := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
	}
	roleBindingInvalid := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: "someothervalue"},
		},
	}
	serviceAccountValid_1 := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	serviceAccountInValid := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: "some invalid value"},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*rbacv1.RoleBindingList)
		arg.Items = []rbacv1.RoleBinding{
			roleBindingValid_1,
			roleBindingInvalid,
		}
	}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.ServiceAccountList)
		arg.Items = []corev1.ServiceAccount{
			serviceAccountValid_1,
			serviceAccountInValid,
		}
	}).Once()

	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Times(2)

	serviceAccountNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(ServiceAccountReadWriteUser, readonlynames[0]),
	}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountNamespacedName.Name,
			Namespace: serviceAccountNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeReadWrite,
			},
		},
	}
	actualServiceAccount := &corev1.ServiceAccount{}
	notFoundError := k8sError.NewNotFound(util.Resource("acstest_readonly_role"), "isnotFound")
	clientMock.On("Get", ctx, serviceAccountNamespacedName, actualServiceAccount).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedServiceAccount).Return(nil)
	roleBindingNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(RoleBindingReadWriteUser, readonlynames[0]),
	}
	expectedRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingNamespacedName.Name,
			Namespace: roleBindingNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeReadWrite,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: roleSharedReadWrite,
			Kind: "Role",
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      fmt.Sprintf(ServiceAccountReadWriteUser, readonlynames[0]),
				Namespace: namespace,
				Kind:      "ServiceAccount",
			},
		},
	}
	actualRoleBinding := &rbacv1.RoleBinding{}
	clientMock.On("Get", ctx, roleBindingNamespacedName, actualRoleBinding).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedRoleBinding).Return(nil).Once()

	result, err := acsService.ReconcileReadWriteUserServiceAccountAndRoleBindings(ctx, namespace, readonlynames, project)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func ACS_ReconcileWorkerClusterServiceAccountAndRoleBindings(t *testing.T) {
	clientMock := &utilMock.Client{}
	acsService := AccessControlService{}
	ctx := prepareACSTestContext(context.Background(), clientMock, nil)
	name := "user1"
	readonlynames := []string{name}
	namespace := "cisco"
	project := &controllerv1alpha1.Project{}
	//clusterName := "cluster-1"

	accessType := AccessTypeClusterReadWrite
	annotationKey := fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel)
	annotationValue := accessType
	roleBindingValid_1 := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
	}
	roleBindingInvalid := rbacv1.RoleBinding{

		ObjectMeta: metav1.ObjectMeta{
			Name:        "valid1",
			Annotations: map[string]string{annotationKey: "someothervalue"},
		},
	}
	serviceAccountValid_1 := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: annotationValue},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	serviceAccountInValid := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "validsa1",
			Annotations: map[string]string{annotationKey: "some invalid value"},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*rbacv1.RoleBindingList)
		arg.Items = []rbacv1.RoleBinding{
			roleBindingValid_1,
			roleBindingInvalid,
		}
	}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*corev1.ServiceAccountList)
		arg.Items = []corev1.ServiceAccount{
			serviceAccountValid_1,
			serviceAccountInValid,
		}
	}).Once()

	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Times(2)

	serviceAccountNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(ServiceAccountWorkerCluster, readonlynames[0]),
	}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountNamespacedName.Name,
			Namespace: serviceAccountNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeClusterReadWrite,
			},
		},
	}
	actualServiceAccount := &corev1.ServiceAccount{}
	notFoundError := k8sError.NewNotFound(util.Resource("acstest_readonly_role"), "isnotFound")
	clientMock.On("Get", ctx, serviceAccountNamespacedName, actualServiceAccount).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedServiceAccount).Return(nil)
	roleBindingNamespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      fmt.Sprintf(RoleBindingWorkerCluster, readonlynames[0]),
	}
	expectedRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingNamespacedName.Name,
			Namespace: roleBindingNamespacedName.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): AccessTypeClusterReadWrite,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: roleWorkerCluster,
			Kind: "Role",
		},
		Subjects: []rbacv1.Subject{
			{
				Name:      fmt.Sprintf(ServiceAccountWorkerCluster, readonlynames[0]),
				Namespace: namespace,
				Kind:      "ServiceAccount",
			},
		},
	}
	actualRoleBinding := &rbacv1.RoleBinding{}
	clientMock.On("Get", ctx, roleBindingNamespacedName, actualRoleBinding).Return(notFoundError).Once()
	clientMock.On("Create", ctx, expectedRoleBinding).Return(nil).Once()

	result, err := acsService.ReconcileWorkerClusterServiceAccountAndRoleBindings(ctx, name, namespace, project)
	expectedResult := ctrl.Result{}
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func prepareACSTestContext(ctx context.Context, client util.Client,
	scheme *runtime.Scheme) context.Context {
	preparedCtx := util.PrepareKubeSliceControllersRequestContext(ctx, client, scheme, "ProjectTestController")
	return preparedCtx
}
