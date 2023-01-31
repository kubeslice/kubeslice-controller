// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import context "context"
import mock "github.com/stretchr/testify/mock"
import reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

// IClusterService is an autogenerated mock type for the IClusterService type
type IClusterService struct {
	mock.Mock
}

// DeleteClusters provides a mock function with given fields: ctx, namespace
func (_m *IClusterService) DeleteClusters(ctx context.Context, namespace string) (reconcile.Result, error) {
	ret := _m.Called(ctx, namespace)

	var r0 reconcile.Result
	if rf, ok := ret.Get(0).(func(context.Context, string) reconcile.Result); ok {
		r0 = rf(ctx, namespace)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, namespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileCluster provides a mock function with given fields: ctx, req
func (_m *IClusterService) ReconcileCluster(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ret := _m.Called(ctx, req)

	var r0 reconcile.Result
	if rf, ok := ret.Get(0).(func(context.Context, reconcile.Request) reconcile.Result); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Get(0).(reconcile.Result)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, reconcile.Request) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
