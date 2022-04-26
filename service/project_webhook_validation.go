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
	"os"
	"strings"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var projectCtx context.Context = nil

// r is instance of Project schema
var r *controllerv1alpha1.Project = nil

// ValidateProjectCreate is a function to validate the creation of project
func ValidateProjectCreate(ctx context.Context, project *controllerv1alpha1.Project) error {
	projectCtx = ctx
	r = project
	var allErrs field.ErrorList
	if err := validateAppliedInAveshaNamespace(); err != nil {
		allErrs = append(allErrs, err)
	} else {
		if err = validateProjectName(r.Name); err != nil {
			allErrs = append(allErrs, err)
		}
		projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, r.Name)
		if err := validateProjectNamespaceIfAlreadyExists(projectNamespace); err != nil {
			allErrs = append(allErrs, err)
		}
		errors := validateDNSCompliantSANames()
		allErrs = append(allErrs, errors...)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "Project"}, r.Name, allErrs)
}

// ValidateProjectUpdate is a function to verify the project - service account, role binding, service account names
func ValidateProjectUpdate(ctx context.Context, project *controllerv1alpha1.Project) error {
	projectCtx = ctx
	r = project
	var allErrs field.ErrorList
	projectNamespace := fmt.Sprintf(ProjectNamespacePrefix, r.Name)
	if err := validateServiceAccount(projectNamespace); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateRoleBinding(projectNamespace); err != nil {
		allErrs = append(allErrs, err)
	}
	errors := validateDNSCompliantSANames()
	allErrs = append(allErrs, errors...)
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "Project"}, r.Name, allErrs)
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

// validateAppliedInAveshaNamespace is a function to verify if project is applied in kubeslice
func validateAppliedInAveshaNamespace() *field.Error {
	if r.Namespace != os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE") {
		return field.Invalid(field.NewPath("namespace"), r.Namespace, "project must be applied on kubeslice-manager namespace - "+os.Getenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE"))
	}
	return nil
}

// validateProjectNamespaceIfAlreadyExists is a function validate the whether the project namespace already exists or not
func validateProjectNamespaceIfAlreadyExists(projectNamespace string) *field.Error {
	exist, _ := util.GetResourceIfExist(projectCtx, client.ObjectKey{Name: projectNamespace}, &corev1.Namespace{})
	if exist {
		return field.Invalid(field.NewPath("project namespace"), projectNamespace, "already exists")
	}
	return nil
}

// validateDNSCompliantSANames is a function to validate the service account name whether it is DNS compliant
func validateDNSCompliantSANames() []*field.Error {
	readNames := r.Spec.ServiceAccount.ReadOnly
	var errors []*field.Error
	for _, name := range readNames {
		if !util.IsDNSCompliant(name) {
			invalidError := field.Invalid(field.NewPath("spec").Child("serviceAccount").Child("readOnly"), name, "is not valid.")
			errors = append(errors, invalidError)
		}
	}
	writeNames := r.Spec.ServiceAccount.ReadWrite
	for _, name := range writeNames {
		if !util.IsDNSCompliant(name) {
			invalidError := field.Invalid(field.NewPath("spec").Child("serviceAccount").Child("readWrite"), name, "is not valid.")
			errors = append(errors, invalidError)
		}
	}
	return errors
}

// validateServiceAccount is a function to validate the service account
func validateServiceAccount(projectNamespace string) *field.Error {
	// The field helpers from the kubernetes API machinery help us return nicely
	// structured validation errors.
	err := validateSANamesIfAlreadyExists(r.Spec.ServiceAccount.ReadOnly, projectNamespace, ServiceAccountReadOnlyUser, "readOnly")
	if err != nil {
		return err
	}
	err = validateSANamesIfAlreadyExists(r.Spec.ServiceAccount.ReadWrite, projectNamespace, ServiceAccountReadWriteUser, "readWrite")
	if err != nil {
		return err
	}
	return nil
}

// validateRoleBindingIfExists is a function to verify the role like read only, readwrite
func validateRoleBinding(projectNamespace string) *field.Error {
	// The field helpers from the kubernetes API machinery help us return nicely
	// structured validation errors.
	err := validateRoleBindingIfExists(r.Spec.ServiceAccount.ReadOnly, projectNamespace, RoleBindingReadOnlyUser, "readOnly")
	if err != nil {
		return err
	}
	err = validateRoleBindingIfExists(r.Spec.ServiceAccount.ReadWrite, projectNamespace, RoleBindingReadWriteUser, "readWrite")
	if err != nil {
		return err
	}
	return nil
}

func validateRoleBindingIfExists(names []string, projectNamespace, format, child string) *field.Error {
	labels := util.GetOwnerLabel(r)
	for _, name := range names {
		roleBindingNamespacedName := client.ObjectKey{
			Namespace: projectNamespace,
			Name:      fmt.Sprintf(format, name),
		}
		actualRoleBinding := rbacv1.RoleBinding{}
		exist, _ := util.GetResourceIfExist(projectCtx, roleBindingNamespacedName, &actualRoleBinding)
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
func validateSANamesIfAlreadyExists(names []string, projectNamespace, format, child string) *field.Error {
	labels := util.GetOwnerLabel(r)
	for _, name := range names {
		serviceAccountNamespacedName := client.ObjectKey{
			Namespace: projectNamespace,
			Name:      fmt.Sprintf(format, name),
		}
		sa := corev1.ServiceAccount{}
		exist, _ := util.GetResourceIfExist(projectCtx, serviceAccountNamespacedName, &sa)
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
