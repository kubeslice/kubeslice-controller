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
	"github.com/kubeslice/kubeslice-controller/metrics"
	"reflect"
	"strings"

	"github.com/kubeslice/kubeslice-controller/events"

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
	ruleProvider IAccessControlRuleProvider
	mf           metrics.IMetricRecorder
}

// ReconcileWorkerClusterRole reconciles the worker cluster role
func (a *AccessControlService) ReconcileWorkerClusterRole(ctx context.Context,
	namespace string, owner client.Object) (ctrl.Result, error) {
	namespacedName := client.ObjectKey{
		Namespace: namespace,
		Name:      roleWorkerCluster,
	}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(owner), owner.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: a.ruleProvider.WorkerClusterRoleRules(),
	}
	actualRole := &rbacv1.Role{}
	found, err := util.GetResourceIfExist(ctx, namespacedName, actualRole)
	if err != nil {
		return ctrl.Result{}, err
	}
	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(namespace)

	// Load metrics with project name and namespace
	a.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace)

	if !found {
		err = util.CreateResource(ctx, expectedRole)
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventWorkerClusterRoleCreationFailed)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "creation_failed",
					"event":       string(events.EventWorkerClusterRoleCreationFailed),
					"object_name": expectedRole.Name,
					"object_kind": metricKindRole,
				},
			)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventWorkerClusterRoleCreated)
		a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "created",
				"event":       string(events.EventWorkerClusterRoleCreated),
				"object_name": expectedRole.Name,
				"object_kind": metricKindRole,
			},
		)
	} else {
		err = util.UpdateResource(ctx, expectedRole)
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventWorkerClusterRoleUpdateFailed)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "update_failed",
					"event":       string(events.EventWorkerClusterRoleUpdateFailed),
					"object_name": expectedRole.Name,
					"object_kind": metricKindRole,
				},
			)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventWorkerClusterRoleUpdated)
		a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "updated",
				"event":       string(events.EventWorkerClusterRoleUpdated),
				"object_name": expectedRole.Name,
				"object_kind": metricKindRole,
			},
		)
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
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(owner), owner.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: a.ruleProvider.ReadOnlyRoleRules(),
	}
	actualRole := &rbacv1.Role{}
	found, err := util.GetResourceIfExist(ctx, namespacedName, actualRole)
	if err != nil {
		return ctrl.Result{}, err
	}
	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(namespace)

	// Load metrics with project name and namespace
	a.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace)

	if !found {
		err = util.CreateResource(ctx, expectedRole)
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadOnlyRoleCreationFailed)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "creation_failed",
					"event":       string(events.EventReadOnlyRoleCreationFailed),
					"object_name": expectedRole.Name,
					"object_kind": metricKindRole,
				},
			)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadOnlyRoleCreated)
		a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "created",
				"event":       string(events.EventReadOnlyRoleCreated),
				"object_name": expectedRole.Name,
				"object_kind": metricKindRole,
			},
		)
	} else {
		err = util.UpdateResource(ctx, expectedRole)
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadOnlyRoleUpdateFailed)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "update_failed",
					"event":       string(events.EventReadOnlyRoleUpdateFailed),
					"object_name": expectedRole.Name,
					"object_kind": metricKindRole,
				},
			)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadOnlyRoleUpdated)
		a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "updated",
				"event":       string(events.EventReadOnlyRoleUpdated),
				"object_name": expectedRole.Name,
				"object_kind": metricKindRole,
			},
		)
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
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(owner), owner.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	expectedRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    labels,
		},
		Rules: a.ruleProvider.ReadWriteRoleRules(),
	}
	actualRole := &rbacv1.Role{}
	found, err := util.GetResourceIfExist(ctx, namespacedName, actualRole)
	if err != nil {
		return ctrl.Result{}, err
	}
	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(namespace)

	// Load metrics with project name and namespace
	a.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace)

	if !found {
		err = util.CreateResource(ctx, expectedRole)
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadWriteRoleCreationFailed)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "creation_failed",
					"event":       string(events.EventReadWriteRoleCreationFailed),
					"object_name": expectedRole.Name,
					"object_kind": metricKindRole,
				},
			)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadWriteRoleCreated)
		a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "created",
				"event":       string(events.EventReadWriteRoleCreated),
				"object_name": expectedRole.Name,
				"object_kind": metricKindRole,
			},
		)
	} else {
		err = util.UpdateResource(ctx, expectedRole)
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadWriteRoleUpdateFailed)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "update_failed",
					"event":       string(events.EventReadWriteRoleUpdateFailed),
					"object_name": expectedRole.Name,
					"object_kind": metricKindRole,
				},
			)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, expectedRole, nil, events.EventReadWriteRoleUpdated)
		a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
			map[string]string{
				"action":      "updated",
				"event":       string(events.EventReadWriteRoleUpdated),
				"object_name": expectedRole.Name,
				"object_kind": metricKindRole,
			},
		)
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
	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(namespace)

	// Load metrics with project name and namespace
	a.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace)

	for _, name := range names {
		// Create or update service account
		serviceAccountNamespacedName := client.ObjectKey{
			Namespace: namespace,
			Name:      fmt.Sprintf(svcAccNamePattern, strings.ToLower(name)),
		}
		completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(owner), owner.GetName())
		labels := util.GetOwnerLabel(completeResourceName)
		expectedServiceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountNamespacedName.Name,
				Namespace: serviceAccountNamespacedName.Namespace,
				Labels:    labels,
				Annotations: map[string]string{
					fmt.Sprintf("%s/%s", annotationKubeSliceControllers, AccessTypeAnnotationLabel): accessType,
				},
			},
			Secrets: []corev1.ObjectReference{
				{
					Name: serviceAccountNamespacedName.Name,
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
			// create service account
			err = util.CreateResource(ctx, expectedServiceAccount)
			if err != nil {
				logger.With(zap.Error(err)).Errorf("Couldnt create serviceaccount")
				util.RecordEvent(ctx, eventRecorder, expectedServiceAccount, nil, events.EventServiceAccountCreationFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "creation_failed",
						"event":       string(events.EventServiceAccountCreationFailed),
						"object_name": expectedServiceAccount.Name,
						"object_kind": metricKindServiceAccount,
					},
				)
				return ctrl.Result{}, err
			}
			util.RecordEvent(ctx, eventRecorder, expectedServiceAccount, nil, events.EventServiceAccountCreated)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "created",
					"event":       string(events.EventServiceAccountCreated),
					"object_name": expectedServiceAccount.Name,
					"object_kind": metricKindServiceAccount,
				},
			)
			// create secret for the service account
			secret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        expectedServiceAccount.Name,
					Annotations: map[string]string{"kubernetes.io/service-account.name": expectedServiceAccount.Name},
					Namespace:   namespace,
				},
				Type: "kubernetes.io/service-account-token",
			}
			err = util.CreateResource(ctx, &secret)
			if err != nil {
				logger.With(zap.Error(err)).Errorf("Couldnt create secret")
				util.RecordEvent(ctx, eventRecorder, &secret, nil, events.EventServiceAccountSecretCreationFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "creation_failed",
						"event":       string(events.EventServiceAccountSecretCreationFailed),
						"object_name": secret.Name,
						"object_kind": metricKindSecret,
					},
				)
				return ctrl.Result{}, err
			}
			util.RecordEvent(ctx, eventRecorder, &secret, nil, events.EventServiceAccountSecretCreated)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "created",
					"event":       string(events.EventServiceAccountSecretCreated),
					"object_name": secret.Name,
					"object_kind": metricKindSecret,
				},
			)
		}
	}
	for _, name := range names {
		// Create or update role binding
		roleBindingNamespacedName := client.ObjectKey{
			Namespace: namespace,
			Name:      fmt.Sprintf(roleBindingNamePatterns, strings.ToLower(name)),
		}
		completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(owner), owner.GetName())
		labels := util.GetOwnerLabel(completeResourceName)
		expectedRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingNamespacedName.Name,
				Namespace: roleBindingNamespacedName.Namespace,
				Labels:    labels,
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
				util.RecordEvent(ctx, eventRecorder, expectedRoleBinding, nil, events.EventDefaultRoleBindingCreationFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "creation_failed",
						"event":       string(events.EventDefaultRoleBindingCreationFailed),
						"object_name": expectedRoleBinding.Name,
						"object_kind": metricKindRoleBinding,
					},
				)
				return ctrl.Result{}, err
			}
			util.RecordEvent(ctx, eventRecorder, expectedRoleBinding, nil, events.EventDefaultRoleBindingCreated)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "created",
					"event":       string(events.EventDefaultRoleBindingCreated),
					"object_name": expectedRoleBinding.Name,
					"object_kind": metricKindRoleBinding,
				},
			)
		} else {
			err = util.UpdateResource(ctx, expectedRoleBinding)
			if err != nil {
				util.RecordEvent(ctx, eventRecorder, expectedRoleBinding, nil, events.EventDefaultRoleBindingUpdateFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "update_failed",
						"event":       string(events.EventDefaultRoleBindingUpdateFailed),
						"object_name": expectedRoleBinding.Name,
						"object_kind": metricKindRoleBinding,
					},
				)
				return ctrl.Result{}, err
			}
			if !reflect.DeepEqual(expectedRoleBinding.RoleRef, actualRoleBinding.RoleRef) ||
				!reflect.DeepEqual(expectedRoleBinding.Subjects, actualRoleBinding.Subjects) {
				util.RecordEvent(ctx, eventRecorder, expectedRoleBinding, nil, events.EventDefaultRoleBindingUpdated)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "updated",
						"event":       string(events.EventDefaultRoleBindingUpdated),
						"object_name": expectedRoleBinding.Name,
						"object_kind": metricKindRoleBinding,
					},
				)
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

	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(owner), owner.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	err := util.ListResources(ctx, roleBindings, client.MatchingLabels(labels), client.InNamespace(namespace))
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
	err = util.ListResources(ctx, serviceAccounts, client.MatchingLabels(labels), client.InNamespace(namespace))
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

	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(namespace)

	// Load metrics with project name and namespace
	a.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace)

	// Delete additional role bindings
	for _, activeObj := range activeRoleBindings {
		if !activeObj.active {
			err = util.DeleteResource(ctx, &activeObj.object)
			if err != nil {
				util.RecordEvent(ctx, eventRecorder, &activeObj.object, nil, events.EventInactiveRoleBindingDeletionFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deletion_failed",
						"event":       string(events.EventInactiveRoleBindingDeletionFailed),
						"object_name": activeObj.object.Name,
						"object_kind": metricKindRoleBinding,
					},
				)
				return ctrl.Result{}, err
			}
			util.RecordEvent(ctx, eventRecorder, &activeObj.object, nil, events.EventInactiveRoleBindingDeleted)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deleted",
					"event":       string(events.EventInactiveRoleBindingDeleted),
					"object_name": activeObj.object.Name,
					"object_kind": metricKindRoleBinding,
				},
			)
		}
	}

	// Delete additional service accounts
	for _, activeObj := range activeServiceAccounts {
		if !activeObj.active {
			err = util.DeleteResource(ctx, &activeObj.object)
			if err != nil {
				util.RecordEvent(ctx, eventRecorder, &activeObj.object, nil, events.EventInactiveServiceAccountDeletionFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deletion_failed",
						"event":       string(events.EventInactiveServiceAccountDeletionFailed),
						"object_name": activeObj.object.Name,
						"object_kind": metricKindServiceAccount,
					},
				)
				return ctrl.Result{}, err
			}
			util.RecordEvent(ctx, eventRecorder, &activeObj.object, nil, events.EventInactiveServiceAccountDeleted)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deleted",
					"event":       string(events.EventInactiveServiceAccountDeleted),
					"object_name": activeObj.object.Name,
					"object_kind": metricKindServiceAccount,
				},
			)
		}
	}
	return ctrl.Result{}, err
}

// removeServiceAccountsAndRoleBindingsByLabel removes service accounts and role bindings by label
func (a *AccessControlService) removeServiceAccountsAndRoleBindingsByLabel(ctx context.Context, namespace string,
	names []string, owner client.Object) (ctrl.Result, error) {
	// Fetch existing RoleBindings and assume them for deletion
	roleBindings := &rbacv1.RoleBindingList{}
	completeResourceName := fmt.Sprintf(util.LabelValue, util.GetObjectKind(owner), owner.GetName())
	labels := util.GetOwnerLabel(completeResourceName)
	err := util.ListResources(ctx, roleBindings, client.MatchingLabels(labels), client.InNamespace(namespace))
	if err != nil {
		util.CtxLogger(ctx).With(zap.Error(err)).Errorf("Could not list resources")
		return ctrl.Result{}, err
	}

	// Fetch existing ServiceAccounts and assume them for deletions
	serviceAccounts := &corev1.ServiceAccountList{}
	err = util.ListResources(ctx, serviceAccounts, client.MatchingLabels(labels), client.InNamespace(namespace))
	if err != nil {
		return ctrl.Result{}, err
	}

	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(namespace)

	// Load metrics with project name and namespace
	a.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace)

	// Delete role bindings
	if len(roleBindings.Items) > 0 {
		for _, rb := range roleBindings.Items {
			err = util.DeleteResource(ctx, &rb)
			if err != nil {
				util.RecordEvent(ctx, eventRecorder, &rb, nil, events.EventDefaultRoleBindingDeletionFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deletion_failed",
						"event":       string(events.EventDefaultRoleBindingDeletionFailed),
						"object_name": rb.Name,
						"object_kind": metricKindRoleBinding,
					},
				)
				return ctrl.Result{}, err
			}
			util.RecordEvent(ctx, eventRecorder, &rb, nil, events.EventDefaultRoleBindingDeleted)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deleted",
					"event":       string(events.EventDefaultRoleBindingDeleted),
					"object_name": rb.Name,
					"object_kind": metricKindRoleBinding,
				},
			)
		}
	}

	// Delete service accounts
	if len(serviceAccounts.Items) > 0 {
		for _, sa := range serviceAccounts.Items {
			err = util.DeleteResource(ctx, &sa)
			if err != nil {
				util.RecordEvent(ctx, eventRecorder, &sa, nil, events.EventServiceAccountDeletionFailed)
				a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
					map[string]string{
						"action":      "deletion_failed",
						"event":       string(events.EventServiceAccountDeletionFailed),
						"object_name": sa.Name,
						"object_kind": metricKindServiceAccount,
					},
				)
				return ctrl.Result{}, err
			}
			util.RecordEvent(ctx, eventRecorder, &sa, nil, events.EventServiceAccountDeleted)
			a.mf.RecordCounterMetric(metrics.KubeSliceEventsCounter,
				map[string]string{
					"action":      "deleted",
					"event":       string(events.EventServiceAccountDeleted),
					"object_name": sa.Name,
					"object_kind": metricKindServiceAccount,
				},
			)
		}
	}
	return ctrl.Result{}, err
}
