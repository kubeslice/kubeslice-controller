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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"strings"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateProjectCreate is a function to validate the creation of project
func ValidateProjectCreate(ctx context.Context, project *controllerv1alpha1.Project) error {
	if err := validateAppliedInControllerNamespace(ctx, project); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	if err := validateProjectName(project.Name); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	if err := validateProjectNamespaceIfAlreadyExists(ctx, project.Name); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	if err := validateDNSCompliantSANames(ctx, project); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	return nil
}

// ValidateProjectUpdate is a function to verify the project - service account, role binding, service account names
func ValidateProjectUpdate(ctx context.Context, project *controllerv1alpha1.Project) error {
	if err := validateServiceAccount(ctx, project); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	if err := validateRoleBinding(ctx, project); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	if err := validateDNSCompliantSANames(ctx, project); err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	return nil
}

func ValidateProjectDelete(ctx context.Context, project *controllerv1alpha1.Project) error {
	if exists := validateIfSliceConfigExists(ctx, project); exists {
		err := field.Forbidden(field.NewPath("Project"), "The Project can be delete only after deleting the slice config")
		return apierrors.NewInvalid(schema.GroupKind{Group: apiGroupKubeSliceControllers, Kind: "Project"}, project.Name, field.ErrorList{err})
	}
	return nil
}

func validateIfSliceConfigExists(ctx context.Context, project *controllerv1alpha1.Project) bool {
	sliceConfig := &controllerv1alpha1.SliceConfigList{}
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.GetName())
	err := util.ListResources(ctx, sliceConfig, client.InNamespace(projectNamespace))
	if err == nil && len(sliceConfig.Items) > 0 {
		return true
	}
	return false
}

func validateProjectName(name string) *field.Error {
	if strings.Contains(name, ".") {
		return field.Invalid(field.NewPath("name"), name, "cannot contain dot(.)")
	}
	if len(name) > 30 {
		return field.Invalid(field.NewPath("name"), name, "cannot contain more than 30 characters")
	}
	return nil
}

// validateAppliedInControllerNamespace is a function to verify if project is applied in kubeslice
func validateAppliedInControllerNamespace(ctx context.Context, project *controllerv1alpha1.Project) *field.Error {
	if project.Namespace != os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE") {
		return field.Invalid(field.NewPath("namespace"), project.Namespace, "project must be applied on kubeslice-manager namespace - "+os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE"))
	}
	return nil
}

// validateProjectNamespaceIfAlreadyExists is a function validate the whether the project namespace already exists or not
func validateProjectNamespaceIfAlreadyExists(ctx context.Context, projectName string) *field.Error {
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, projectName)
	if exist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: projectNamespace}, &corev1.Namespace{}); exist {
		return field.Invalid(field.NewPath("project namespace"), projectNamespace, "already exists")
	}
	return nil
}

// validateDNSCompliantSANames is a function to validate the service account name whether it is DNS compliant
func validateDNSCompliantSANames(ctx context.Context, project *controllerv1alpha1.Project) *field.Error {
	readNames := project.Spec.ServiceAccount.ReadOnly
	for _, name := range readNames {
		if !util.IsDNSCompliant(name) {
			return field.Invalid(field.NewPath("spec").Child("serviceAccount").Child("readOnly"), name, "is not valid.")
		}
	}
	writeNames := project.Spec.ServiceAccount.ReadWrite
	for _, name := range writeNames {
		if !util.IsDNSCompliant(name) {
			return field.Invalid(field.NewPath("spec").Child("serviceAccount").Child("readWrite"), name, "is not valid.")
		}
	}
	return nil
}

// validateServiceAccount is a function to validate the service account
func validateServiceAccount(ctx context.Context, project *controllerv1alpha1.Project) *field.Error {
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.Name)
	// structured validation errors.
	err := validateSANamesIfAlreadyExists(ctx, project, project.Spec.ServiceAccount.ReadOnly, projectNamespace, ServiceAccountReadOnlyUser, "readOnly")
	if err != nil {
		return err
	}
	err = validateSANamesIfAlreadyExists(ctx, project, project.Spec.ServiceAccount.ReadWrite, projectNamespace, ServiceAccountReadWriteUser, "readWrite")
	if err != nil {
		return err
	}
	return nil
}

// validateRoleBindingIfExists is a function to verify the role like read only, readwrite
func validateRoleBinding(ctx context.Context, project *controllerv1alpha1.Project) *field.Error {
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, project.Name)
	// structured validation errors.
	err := validateRoleBindingIfExists(ctx, project, project.Spec.ServiceAccount.ReadOnly, projectNamespace, RoleBindingReadOnlyUser, "readOnly")
	if err != nil {
		return err
	}
	err = validateRoleBindingIfExists(ctx, project, project.Spec.ServiceAccount.ReadWrite, projectNamespace, RoleBindingReadWriteUser, "readWrite")
	if err != nil {
		return err
	}
	return nil
}

func validateRoleBindingIfExists(ctx context.Context, project *controllerv1alpha1.Project, names []string, projectNamespace, format, child string) *field.Error {
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	for _, name := range names {
		roleBindingNamespacedName := client.ObjectKey{
			Namespace: projectNamespace,
			Name:      fmt.Sprintf(format, name),
		}
		actualRoleBinding := rbacv1.RoleBinding{}
		exist, _ := util.GetResourceIfExist(ctx, roleBindingNamespacedName, &actualRoleBinding)
		if exist {
			for key, value := range labels {
				if actualRoleBinding.Labels[key] != value {
					return field.Invalid(field.NewPath("spec").Child("roleBinding").Child(child), name, "already exists")
				}
			}
		}
	}
	return nil
}

// validateSANamesIfAlreadyExists is a function to validate the service account name if already exists
func validateSANamesIfAlreadyExists(ctx context.Context, project *controllerv1alpha1.Project, names []string, projectNamespace, format, child string) *field.Error {
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(project), project.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	for _, name := range names {
		serviceAccountNamespacedName := client.ObjectKey{
			Namespace: projectNamespace,
			Name:      fmt.Sprintf(format, name),
		}
		sa := corev1.ServiceAccount{}
		exist, _ := util.GetResourceIfExist(ctx, serviceAccountNamespacedName, &sa)
		if exist {
			for key, value := range labels {
				if sa.Labels[key] != value {
					return field.Invalid(field.NewPath("spec").Child("serviceAccount").Child(child), name, "already exists")
				}
			}
		}
	}
	return nil
}
