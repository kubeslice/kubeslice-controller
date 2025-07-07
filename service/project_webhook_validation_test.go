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
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestProjectWebhookSuite(t *testing.T) {
	for k, v := range ProjectWebhookTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ProjectWebhookTestbed = map[string]func(*testing.T){
	"TestValidateProjectCreate_Applied_Namespace_Error": TestValidateProjectCreate_Applied_Namespace_Error,
	// "TestValidateProjectCreate_FailsIfNamespaceAlreadyExists":                            TestValidateProjectCreate_FailsIfNamespaceAlreadyExists,
	"TestValidateProjectCreate_FailsIf_Sa_Name_Not_DNS_Compliant":                        TestValidateProjectCreate_FailsIf_Sa_Name_Not_DNS_Compliant,
	"TestValidateProjectCreate_FailsIfNameContainsDot":                                   TestValidateProjectCreate_FailsIfNameContainsDot,
	"TestValidateProjectCreate_FailsIfNameContainsGreaterThan30Characters":               TestValidateProjectCreate_FailsIfNameContainsGreaterThan30Characters,
	"TestValidateProjectCreate_HappyPath":                                                TestValidateProjectCreate_HappyPath,
	"Test_ValidateProjectUpdate_ThrowsErrorIf_SA_Readonly_do_not_exist":                  Test_ValidateProjectUpdate_ThrowsErrorIf_SA_Readonly_already_exist,
	"Test_ValidateProjectUpdate_ThrowsErrorIf_SA_ReadWrite_do_not_exist":                 Test_ValidateProjectUpdate_ThrowsErrorIf_SA_ReadWrite_already_exist,
	"Test_ValidateProjectUpdate_ThrowsErrorIf_SA_DNS_Invalid_throws_error":               Test_ValidateProjectUpdate_ThrowsErrorIf_SA_DNS_Invalid_throws_error,
	"Test_ValidateProjectUpdate_ThrowsErrorIf_RoleBinding_Readonly_exists_throws_error":  Test_ValidateProjectUpdate_ThrowsErrorIf_RoleBinding_Readonly_exists_throws_error,
	"Test_ValidateProjectUpdate_ThrowsErrorIf_RoleBinding_ReadWrite_exists_throws_error": Test_ValidateProjectUpdate_ThrowsErrorIf_RoleBinding_ReadWrite_exists_throws_error,
	"Test_ValidateProjectUpdate_Happy":                                                   Test_ValidateProjectUpdate_Happy,
	"Test_ValidateProjectDelete_FailsIfSliceConfigExists":                                Test_ValidateProjectDelete_FailsIfSliceConfigExists,
}

func TestValidateProjectCreate_Applied_Namespace_Error(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	project.ObjectMeta.Name = "sdf"
	worngNamespace := "wrongNamespace"
	project.ObjectMeta.Namespace = worngNamespace
	namespace := "avesha-controller"
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	_, err := ValidateProjectCreate(ctx, project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid", os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE"), worngNamespace)
	clientMock.AssertExpectations(t)
}

func TestValidateProjectCreate_FailsIfNameContainsDot(t *testing.T) { //todo
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "a.1"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	_, err := ValidateProjectCreate(ctx, project)
	require.Contains(t, err.Error(), "cannot contain dot")
	clientMock.AssertExpectations(t)
}

func TestValidateProjectCreate_FailsIfNameContainsGreaterThan30Characters(t *testing.T) { //todo
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "this-name-contains-31characters"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	_, err := ValidateProjectCreate(ctx, project)
	require.Contains(t, err.Error(), "cannot contain more than")
	clientMock.AssertExpectations(t)
}

// func TestValidateProjectCreate_FailsIfNamespaceAlreadyExists(t *testing.T) { //todo
// 	project := &controllerv1alpha1.Project{}
// 	clientMock := &utilMock.Client{}
// 	name := "testProject"
// 	project.ObjectMeta.Name = name
// 	namespace := "avesha-controller"
// 	project.ObjectMeta.Namespace = namespace
// 	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
// 	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
// 	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
// 	err := ValidateProjectCreate(ctx, project)
// 	require.Error(t, err)
// 	require.Contains(t, err.Error(), "already exists", namespace)
// 	clientMock.AssertExpectations(t)
// }

func TestValidateProjectCreate_FailsIf_Sa_Name_Not_DNS_Compliant(t *testing.T) { //todo
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	invalidName1 := "$$232***"
	invalidName2 := "$$2*****#32***"
	invalidNameRw1 := "$$2***%%**#32***"
	invalidNameRw2 := "$$2**%***#32***"
	project.Spec.ServiceAccount.ReadOnly = []string{invalidName1, invalidName2}
	project.Spec.ServiceAccount.ReadWrite = []string{invalidNameRw1, invalidNameRw2}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	// notFoundError := k8sError.NewNotFound(util.Resource("projecttest"), "isnotFound")
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	// clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Once()
	_, err := ValidateProjectCreate(ctx, project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid value", invalidName1, invalidName2)
	require.Contains(t, err.Error(), "Invalid value", invalidNameRw1, invalidNameRw2)
	clientMock.AssertExpectations(t)
}

func TestValidateProjectCreate_HappyPath(t *testing.T) { //todo
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	validName1 := "arun"
	validName2 := "john"
	validNameRw1 := "kumar"
	validNameRw2 := "mike"
	project.Spec.ServiceAccount.ReadOnly = []string{validName1, validName2}
	project.Spec.ServiceAccount.ReadWrite = []string{validNameRw1, validNameRw2}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	// notFoundError := k8sError.NewNotFound(util.Resource("projecttest"), "isnotFound")
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	// clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Once()
	_, err := ValidateProjectCreate(ctx, project)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func Test_ValidateProjectUpdate_ThrowsErrorIf_SA_Readonly_already_exist(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	alreadyExistName1 := "arun"
	alreadyExistName2 := "john"
	alreadyExistNameRw1 := "kumar"
	alreadyExistNameRw2 := "mike"
	project.Spec.ServiceAccount.ReadOnly = []string{alreadyExistName1, alreadyExistName2}
	project.Spec.ServiceAccount.ReadWrite = []string{alreadyExistNameRw1, alreadyExistNameRw2}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	_, err := ValidateProjectUpdate(ctx, project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists", alreadyExistName1)
	clientMock.AssertExpectations(t)
}

func Test_ValidateProjectUpdate_ThrowsErrorIf_SA_ReadWrite_already_exist(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	//alreadyExistName1 := "arun"
	alreadyExistNameRw1 := "kumar"
	project.Spec.ServiceAccount.ReadOnly = []string{}
	project.Spec.ServiceAccount.ReadWrite = []string{alreadyExistNameRw1}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Times(1)
	_, err := ValidateProjectUpdate(ctx, project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists", alreadyExistNameRw1)
	clientMock.AssertExpectations(t)
}

func Test_ValidateProjectUpdate_ThrowsErrorIf_SA_DNS_Invalid_throws_error(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	invalidName1 := "$$232***"
	invalidName2 := "$$2*****#32***"
	invalidNameRw1 := "$$2***%%**#32***"
	invalidNameRw2 := "$$2**%***#32***"
	project.Spec.ServiceAccount.ReadOnly = []string{invalidName1, invalidName2}
	project.Spec.ServiceAccount.ReadWrite = []string{invalidNameRw1, invalidNameRw2}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	// notFoundError := k8sError.NewNotFound(util.Resource("projecttest"), "isnotFound")
	// clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Times(4)
	//calls rolebinding next
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Times(1)
	_, err := ValidateProjectUpdate(ctx, project)
	require.Contains(t, err.Error(), "Invalid value", invalidName1, invalidName2)
	require.Contains(t, err.Error(), "Invalid value", invalidNameRw1, invalidNameRw2)
	clientMock.AssertExpectations(t)
}

func Test_ValidateProjectUpdate_ThrowsErrorIf_RoleBinding_Readonly_exists_throws_error(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	existsReadonlyName := "arun"
	validName2 := "icecream"
	validNameRw1 := "kwalitywalls"
	validNameRw2 := "omnommnomm"
	project.Spec.ServiceAccount.ReadOnly = []string{existsReadonlyName, validName2}
	project.Spec.ServiceAccount.ReadWrite = []string{validNameRw1, validNameRw2}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	notFoundError := k8sError.NewNotFound(util.Resource("projecttest"), "isnotFound")
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Times(4)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Times(1)
	_, err := ValidateProjectUpdate(ctx, project)
	require.Contains(t, err.Error(), "spec.roleBinding.readOnly", existsReadonlyName)
	clientMock.AssertExpectations(t)
}

func Test_ValidateProjectUpdate_ThrowsErrorIf_RoleBinding_ReadWrite_exists_throws_error(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	validName1 := "arun"
	validName2 := "icecream"
	existsReadWriteName := "omnommnomm"
	validNameRw2 := "kwalitywalls"
	project.Spec.ServiceAccount.ReadOnly = []string{validName1, validName2}
	project.Spec.ServiceAccount.ReadWrite = []string{existsReadWriteName, validNameRw2}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	notFoundError := k8sError.NewNotFound(util.Resource("projecttest"), "isnotFound")
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Times(4)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Times(2)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Times(1)
	_, err := ValidateProjectUpdate(ctx, project)
	require.Contains(t, err.Error(), "spec.roleBinding.readWrite", existsReadWriteName)
	clientMock.AssertExpectations(t)
}

func Test_ValidateProjectUpdate_Happy(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	clientMock := &utilMock.Client{}
	name := "testProject"
	project.ObjectMeta.Name = name
	namespace := "avesha-controller"
	project.ObjectMeta.Namespace = namespace
	validName1 := "arun"
	validName2 := "icecream"
	validNameRw1 := "kwalitywalls"
	validNameRw2 := "omnommnomm"
	project.Spec.ServiceAccount.ReadOnly = []string{validName1, validName2}
	project.Spec.ServiceAccount.ReadWrite = []string{validNameRw1, validNameRw2}
	os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", namespace)
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	notFoundError := k8sError.NewNotFound(util.Resource("projecttest"), "isnotFound")
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Times(4)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(notFoundError).Times(4)
	_, err := ValidateProjectUpdate(ctx, project)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func Test_ValidateProjectDelete_FailsIfSliceConfigExists(t *testing.T) {
	project := &controllerv1alpha1.Project{}
	sliceConfig := &controllerv1alpha1.SliceConfigList{}
	clientMock := &utilMock.Client{}
	ctx := prepareProjectWebhookTestContext(context.Background(), clientMock, nil)
	clientMock.On("List", ctx, sliceConfig, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.SliceConfigList)
		if arg.Items == nil {
			arg.Items = make([]controllerv1alpha1.SliceConfig, 1)
			arg.Items[0].Name = "sliceConfig1"
		}
	}).Once()
	_, err := ValidateProjectDelete(ctx, project)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func prepareProjectWebhookTestContext(ctx context.Context, client client.Client,
	scheme *runtime.Scheme) context.Context {
	preparedCtx := util.PrepareKubeSliceControllersRequestContext(ctx, client, scheme, "ProjectWebhookTestController", nil)
	return preparedCtx
}
