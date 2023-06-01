// Code generated by mockery v2.28.1. DO NOT EDIT.

package mocks

import (
	context "context"

	client "sigs.k8s.io/controller-runtime/pkg/client"

	mock "github.com/stretchr/testify/mock"

	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// IAccessControlService is an autogenerated mock type for the IAccessControlService type
type IAccessControlService struct {
	mock.Mock
}

// ReconcileReadOnlyRole provides a mock function with given fields: ctx, namespace, owner
func (_m *IAccessControlService) ReconcileReadOnlyRole(ctx context.Context, namespace string, owner client.Object) (reconcile.Result, error) {
	ret := _m.Called(ctx, namespace, owner)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, client.Object) (reconcile.Result, error)); ok {
		return rf(ctx, namespace, owner)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, client.Object) reconcile.Result); ok {
		r0 = rf(ctx, namespace, owner)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, client.Object) error); ok {
		r1 = rf(ctx, namespace, owner)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileReadOnlyUserServiceAccountAndRoleBindings provides a mock function with given fields: ctx, namespace, names, owner
func (_m *IAccessControlService) ReconcileReadOnlyUserServiceAccountAndRoleBindings(ctx context.Context, namespace string, names []string, owner client.Object) (reconcile.Result, error) {
	ret := _m.Called(ctx, namespace, names, owner)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, client.Object) (reconcile.Result, error)); ok {
		return rf(ctx, namespace, names, owner)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, client.Object) reconcile.Result); ok {
		r0 = rf(ctx, namespace, names, owner)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, []string, client.Object) error); ok {
		r1 = rf(ctx, namespace, names, owner)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileReadWriteRole provides a mock function with given fields: ctx, namespace, owner
func (_m *IAccessControlService) ReconcileReadWriteRole(ctx context.Context, namespace string, owner client.Object) (reconcile.Result, error) {
	ret := _m.Called(ctx, namespace, owner)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, client.Object) (reconcile.Result, error)); ok {
		return rf(ctx, namespace, owner)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, client.Object) reconcile.Result); ok {
		r0 = rf(ctx, namespace, owner)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, client.Object) error); ok {
		r1 = rf(ctx, namespace, owner)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileReadWriteUserServiceAccountAndRoleBindings provides a mock function with given fields: ctx, namespace, names, owner
func (_m *IAccessControlService) ReconcileReadWriteUserServiceAccountAndRoleBindings(ctx context.Context, namespace string, names []string, owner client.Object) (reconcile.Result, error) {
	ret := _m.Called(ctx, namespace, names, owner)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, client.Object) (reconcile.Result, error)); ok {
		return rf(ctx, namespace, names, owner)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, client.Object) reconcile.Result); ok {
		r0 = rf(ctx, namespace, names, owner)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, []string, client.Object) error); ok {
		r1 = rf(ctx, namespace, names, owner)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileWorkerClusterRole provides a mock function with given fields: ctx, namespace, owner
func (_m *IAccessControlService) ReconcileWorkerClusterRole(ctx context.Context, namespace string, owner client.Object) (reconcile.Result, error) {
	ret := _m.Called(ctx, namespace, owner)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, client.Object) (reconcile.Result, error)); ok {
		return rf(ctx, namespace, owner)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, client.Object) reconcile.Result); ok {
		r0 = rf(ctx, namespace, owner)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, client.Object) error); ok {
		r1 = rf(ctx, namespace, owner)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileWorkerClusterServiceAccountAndRoleBindings provides a mock function with given fields: ctx, clusterName, namespace, owner
func (_m *IAccessControlService) ReconcileWorkerClusterServiceAccountAndRoleBindings(ctx context.Context, clusterName string, namespace string, owner client.Object) (reconcile.Result, error) {
	ret := _m.Called(ctx, clusterName, namespace, owner)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, client.Object) (reconcile.Result, error)); ok {
		return rf(ctx, clusterName, namespace, owner)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, client.Object) reconcile.Result); ok {
		r0 = rf(ctx, clusterName, namespace, owner)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, client.Object) error); ok {
		r1 = rf(ctx, clusterName, namespace, owner)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveWorkerClusterServiceAccountAndRoleBindings provides a mock function with given fields: ctx, clusterName, namespace, owner
func (_m *IAccessControlService) RemoveWorkerClusterServiceAccountAndRoleBindings(ctx context.Context, clusterName string, namespace string, owner client.Object) (reconcile.Result, error) {
	ret := _m.Called(ctx, clusterName, namespace, owner)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, client.Object) (reconcile.Result, error)); ok {
		return rf(ctx, clusterName, namespace, owner)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, client.Object) reconcile.Result); ok {
		r0 = rf(ctx, clusterName, namespace, owner)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, client.Object) error); ok {
		r1 = rf(ctx, clusterName, namespace, owner)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewIAccessControlService interface {
	mock.TestingT
	Cleanup(func())
}

// NewIAccessControlService creates a new instance of IAccessControlService. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewIAccessControlService(t mockConstructorTestingTNewIAccessControlService) *IAccessControlService {
	mock := &IAccessControlService{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
