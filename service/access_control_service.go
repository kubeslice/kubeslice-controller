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
	"strings"

	"github.com/kubeslice/kubeslice-controller/util"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IAccessControlService interface {
	ReconcileWorkerClusterRole(ctx context.Context, namespace string, owner client.Object) (ctrl.Result, error)
	ReconcileReadOnlyRole(ctx context.Context, namespace string, owner client.Object) (ctrl.Result, error)
	ReconcileReadWriteRole(ctx context.Context, namespace string, owner client.Object) (ctrl.Result, error)
	ReconcileReadOnlyUserServiceAccountAndRoleBindings(ctx context.Context, namespace string,
		names []string, owner client.Object) (ctrl.Result, error)
	ReconcileReadWriteUserServiceAccountAndRoleBindings(ctx context.Context, namespace string,
		names []string, owner client.Object) (ctrl.Result, error)
	ReconcileWorkerClusterServiceAccountAndRoleBindings(ctx context.Context, clusterName,
		namespace string, owner client.Object) (ctrl.Result, error)
	RemoveWorkerClusterServiceAccountAndRoleBindings(ctx context.Context, clusterName,
		namespace string, owner client.Object) (ctrl.Result, error)
}

// activeRoleBinding gives the active status of rolebinding
type activeRoleBinding struct {
	object rbacv1.RoleBinding
	active bool
}

// activeServiceAccount gives the active status of Service account
type activeServiceAccount struct {
	object corev1.ServiceAccount
	active bool
}

type AccessControlService struct {
}

// ReconcileWorkerClusterRole reconciles the worker cluster role
func (a *AccessControlService) ReconcileWorkerClusterRole(ctx context.Context,
	namespace string, owner client.Object) (ctrl.Result, error) {
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleWorkerCluster,
	}
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    util.GetOwnerLabel(owner),
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
				Verbs:     []string{verbGet, verbList, verbWatch},
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
	found, err := util.GetResourceIfExist(ctx, namespacedName, actualRole)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		err = util.CreateResource(ctx, expectedRole)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		err = util.UpdateResource(ctx, expectedRole)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// ReconcileReadOnlyRole reconciles the read only role for the project users
func (a *AccessControlService) ReconcileReadOnlyRole(ctx context.Context, namespace string, owner client.Object) (ctrl.Result,
	error) {
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleSharedReadOnly,
	}
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    util.GetOwnerLabel(owner),
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
	found, err := util.GetResourceIfExist(ctx, namespacedName, actualRole)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		err = util.CreateResource(ctx, expectedRole)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		err = util.UpdateResource(ctx, expectedRole)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// ReconcileReadWriteRole reconciles the read write role binding for project users
func (a *AccessControlService) ReconcileReadWriteRole(ctx context.Context,
	namespace string, owner client.Object) (ctrl.Result, error) {
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleSharedReadWrite,
	}
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    util.GetOwnerLabel(owner),
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
	found, err := util.GetResourceIfExist(ctx, namespacedName, actualRole)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		err = util.CreateResource(ctx, expectedRole)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		err = util.UpdateResource(ctx, expectedRole)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// ReconcileReadOnlyUserServiceAccountAndRoleBindings reconciles the service account and role bindings for read only users
func (a *AccessControlService) ReconcileReadOnlyUserServiceAccountAndRoleBindings(ctx context.Context, namespace string,
	names []string, owner client.Object) (ctrl.Result, error) {
	// Cleanup obsolete service accounts and role binding
	if shouldReturn, reconResult, reconErr := util.IsReconciled(a.cleanupObsoleteServiceAccountsAndRoleBindings(ctx,
		namespace, names, ServiceAccountReadOnlyUser, RoleBindingReadOnlyUser, AccessTypeReadOnly, owner)); shouldReturn {
		return reconResult, reconErr
	}
	// Create or update required service accounts and role bindings
	if shouldReturn, reconResult, reconErr := util.IsReconciled(a.createOrUpdateServiceAccountsAndRoleBindings(ctx, namespace,
		names, ServiceAccountReadOnlyUser, RoleBindingReadOnlyUser, AccessTypeReadOnly, roleSharedReadOnly, owner)); shouldReturn {
		return reconResult, reconErr
	}
	return ctrl.Result{}, nil
}

// ReconcileReadWriteUserServiceAccountAndRoleBindings reconciles the service account and role bindings for read write users
func (a *AccessControlService) ReconcileReadWriteUserServiceAccountAndRoleBindings(ctx context.Context,
	namespace string, names []string, owner client.Object) (ctrl.Result, error) {
	// Cleanup obsolete service accounts and role binding
	if shouldReturn, reconResult, reconErr := util.IsReconciled(a.cleanupObsoleteServiceAccountsAndRoleBindings(ctx, namespace, names,
		ServiceAccountReadWriteUser, RoleBindingReadWriteUser, AccessTypeReadWrite, owner)); shouldReturn {
		return reconResult, reconErr
	}

	// Create or update required service accounts and role bindings
	if shouldReturn, reconResult, reconErr := util.IsReconciled(a.createOrUpdateServiceAccountsAndRoleBindings(ctx, namespace, names,
		ServiceAccountReadWriteUser, RoleBindingReadWriteUser, AccessTypeReadWrite, roleSharedReadWrite, owner)); shouldReturn {
		return reconResult, reconErr
	}
	return ctrl.Result{}, nil
}

// ReconcileWorkerClusterServiceAccountAndRoleBindings reconciles the service account and role bindings for worker cluster
func (a *AccessControlService) ReconcileWorkerClusterServiceAccountAndRoleBindings(ctx context.Context, clusterName,
	namespace string, owner client.Object) (ctrl.Result, error) {
	names := []string{clusterName}
	if shouldReturn, reconResult, reconErr := util.IsReconciled(a.cleanupObsoleteServiceAccountsAndRoleBindings(ctx,
		namespace, names, ServiceAccountWorkerCluster, RoleBindingWorkerCluster, AccessTypeClusterReadWrite, owner)); shouldReturn {
		return reconResult, reconErr
	}
	// Create or update required service accounts and role bindings
	if shouldReturn, reconResult, reconErr := util.IsReconciled(a.createOrUpdateServiceAccountsAndRoleBindings(ctx,
		namespace, names, ServiceAccountWorkerCluster, RoleBindingWorkerCluster, AccessTypeClusterReadWrite, roleWorkerCluster, owner)); shouldReturn {
		return reconResult, reconErr
	}
	return ctrl.Result{}, nil
}

// RemoveWorkerClusterServiceAccountAndRoleBindings remove the service account and role bindings for worker cluster
func (a *AccessControlService) RemoveWorkerClusterServiceAccountAndRoleBindings(ctx context.Context, clusterName,
	namespace string, owner client.Object) (ctrl.Result, error) {
	names := []string{clusterName}
	if shouldReturn, reconResult, reconErr := util.IsReconciled(a.removeServiceAccountsAndRoleBindingsByLabel(ctx,
		namespace, names, owner)); shouldReturn {
		return reconResult, reconErr
	}
	return ctrl.Result{}, nil
}

// createOrUpdateServiceAccountsAndRoleBindings creates or updates service accounts and role bindings for the given project names
func (a *AccessControlService) createOrUpdateServiceAccountsAndRoleBindings(ctx context.Context, namespace string,
	names []string, svcAccNamePattern string, roleBindingNamePatterns string, accessType string, roleName string, owner client.Object) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	for _, name := range names {
		// Create or update service account
		serviceAccountNamespacedName := client.ObjectKey{
			Namespace: namespace,
			Name:      fmt.Sprintf(svcAccNamePattern, strings.ToLower(name)),
		}
		expectedServiceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountNamespacedName.Name,
				Namespace: serviceAccountNamespacedName.Namespace,
				Labels:    util.GetOwnerLabel(owner),
				Annotations: map[string]string{
					fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): accessType,
				},
			},
		}
		actualServiceAccount := &corev1.ServiceAccount{}
		foundSa, err := util.GetResourceIfExist(ctx, serviceAccountNamespacedName, actualServiceAccount)
		if err != nil {
			logger.With(zap.Error(err)).Errorf("Couldnt fetch serviceaccoubt")
			return ctrl.Result{}, err
		}
		if !foundSa {
			err = util.CreateResource(ctx, expectedServiceAccount)
			if err != nil {
				logger.With(zap.Error(err)).Errorf("Couldnt create serviceaccoubt")
				return ctrl.Result{}, err
			}
		}
	}
	for _, name := range names {
		// Create or update role binding
		roleBindingNamespacedName := client.ObjectKey{
			Namespace: namespace,
			Name:      fmt.Sprintf(roleBindingNamePatterns, strings.ToLower(name)),
		}
		expectedRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingNamespacedName.Name,
				Namespace: roleBindingNamespacedName.Namespace,
				Labels:    util.GetOwnerLabel(owner),
				Annotations: map[string]string{
					fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): accessType,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Name: roleName,
				Kind: "Role",
			},
			Subjects: []rbacv1.Subject{
				{
					Name:      fmt.Sprintf(svcAccNamePattern, strings.ToLower(name)),
					Namespace: namespace,
					Kind:      "ServiceAccount",
				},
			},
		}
		actualRoleBinding := &rbacv1.RoleBinding{}
		foundRb, err := util.GetResourceIfExist(ctx, roleBindingNamespacedName, actualRoleBinding)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !foundRb {
			err = util.CreateResource(ctx, expectedRoleBinding)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			err = util.UpdateResource(ctx, expectedRoleBinding)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// cleanupObsoleteServiceAccountsAndRoleBindings deletes service accounts and role bindings for the given project names
func (a *AccessControlService) cleanupObsoleteServiceAccountsAndRoleBindings(ctx context.Context, namespace string,
	names []string, svcAccNamePattern string, roleBindingNamePatterns string, accessType string, owner client.Object) (ctrl.Result, error) {
	// Fetch existing RoleBindings and assume them for deletion
	activeRoleBindings := map[string]activeRoleBinding{}
	roleBindings := &rbacv1.RoleBindingList{}
	err := util.ListResources(ctx, roleBindings, client.MatchingLabels(util.GetOwnerLabel(owner)), client.InNamespace(namespace))
	if err != nil {
		util.CtxLogger(ctx).With(zap.Error(err)).Errorf("Could not list resources")
		return ctrl.Result{}, err
	}
	if roleBindings.Items != nil && len(roleBindings.Items) > 0 {
		for _, role := range roleBindings.Items {
			if role.Annotations[fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel)] == accessType {
				activeRoleBindings[role.Name] = activeRoleBinding{active: false, object: role}
			}
		}
	}

	// Fetch existing ServiceAccounts and assume them for deletions
	activeServiceAccounts := map[string]activeServiceAccount{}
	serviceAccounts := &corev1.ServiceAccountList{}
	err = util.ListResources(ctx, serviceAccounts, client.MatchingLabels(util.GetOwnerLabel(owner)), client.InNamespace(namespace))
	if err != nil {
		return ctrl.Result{}, err
	}
	if serviceAccounts != nil && len(serviceAccounts.Items) > 0 {
		for _, sa := range serviceAccounts.Items {
			if sa.Annotations[fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel)] == accessType {
				activeServiceAccounts[sa.Name] = activeServiceAccount{active: false, object: sa}
			}
		}
	}

	// Mark current names as active
	for _, name := range names {
		activeServiceAccounts[fmt.Sprintf(svcAccNamePattern, strings.ToLower(name))] = activeServiceAccount{active: true}
		activeRoleBindings[fmt.Sprintf(roleBindingNamePatterns, strings.ToLower(name))] = activeRoleBinding{active: true}
	}

	// Delete additional role bindings
	for _, activeObj := range activeRoleBindings {
		if !activeObj.active {
			err = util.DeleteResource(ctx, &activeObj.object)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Delete additional service accounts
	for _, activeObj := range activeServiceAccounts {
		if !activeObj.active {
			err = util.DeleteResource(ctx, &activeObj.object)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, err
}

// removeServiceAccountsAndRoleBindingsByLabel removes service accounts and role bindings by label
func (a *AccessControlService) removeServiceAccountsAndRoleBindingsByLabel(ctx context.Context, namespace string,
	names []string, owner client.Object) (ctrl.Result, error) {
	// Fetch existing RoleBindings and assume them for deletion
	roleBindings := &rbacv1.RoleBindingList{}
	err := util.ListResources(ctx, roleBindings, client.MatchingLabels(util.GetOwnerLabel(owner)), client.InNamespace(namespace))
	if err != nil {
		util.CtxLogger(ctx).With(zap.Error(err)).Errorf("Could not list resources")
		return ctrl.Result{}, err
	}

	// Fetch existing ServiceAccounts and assume them for deletions
	serviceAccounts := &corev1.ServiceAccountList{}
	err = util.ListResources(ctx, serviceAccounts, client.MatchingLabels(util.GetOwnerLabel(owner)), client.InNamespace(namespace))
	if err != nil {
		return ctrl.Result{}, err
	}

	// Delete role bindings
	if len(roleBindings.Items) > 0 {
		for _, rb := range roleBindings.Items {
			err = util.DeleteResource(ctx, &rb)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Delete service accounts
	if len(serviceAccounts.Items) > 0 {
		for _, sa := range serviceAccounts.Items {
			err = util.DeleteResource(ctx, &sa)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, err
}
