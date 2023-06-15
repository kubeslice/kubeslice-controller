// Code generated by mockery v2.22.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// IVpnKeyRotationService is an autogenerated mock type for the IVpnKeyRotationService type
type IVpnKeyRotationService struct {
	mock.Mock
}

// CreateMinimalVpnKeyRotationConfig provides a mock function with given fields: ctx, sliceName, namespace, r
func (_m *IVpnKeyRotationService) CreateMinimalVpnKeyRotationConfig(ctx context.Context, sliceName string, namespace string, r int) error {
	ret := _m.Called(ctx, sliceName, namespace, r)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, int) error); ok {
		r0 = rf(ctx, sliceName, namespace, r)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ReconcileClusters provides a mock function with given fields: ctx, sliceName, namespace, clusters
func (_m *IVpnKeyRotationService) ReconcileClusters(ctx context.Context, sliceName string, namespace string, clusters []string) (*v1alpha1.VpnKeyRotation, error) {
	ret := _m.Called(ctx, sliceName, namespace, clusters)

	var r0 *v1alpha1.VpnKeyRotation
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []string) (*v1alpha1.VpnKeyRotation, error)); ok {
		return rf(ctx, sliceName, namespace, clusters)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []string) *v1alpha1.VpnKeyRotation); ok {
		r0 = rf(ctx, sliceName, namespace, clusters)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.VpnKeyRotation)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, []string) error); ok {
		r1 = rf(ctx, sliceName, namespace, clusters)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileVpnKeyRotation provides a mock function with given fields: ctx, req
func (_m *IVpnKeyRotationService) ReconcileVpnKeyRotation(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ret := _m.Called(ctx, req)

	var r0 reconcile.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, reconcile.Request) (reconcile.Result, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, reconcile.Request) reconcile.Result); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	if rf, ok := ret.Get(1).(func(context.Context, reconcile.Request) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewIVpnKeyRotationService interface {
	mock.TestingT
	Cleanup(func())
}

// NewIVpnKeyRotationService creates a new instance of IVpnKeyRotationService. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewIVpnKeyRotationService(t mockConstructorTestingTNewIVpnKeyRotationService) *IVpnKeyRotationService {
	mock := &IVpnKeyRotationService{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
