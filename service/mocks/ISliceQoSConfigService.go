// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ISliceQoSConfigService is an autogenerated mock type for the ISliceQoSConfigService type
type ISliceQoSConfigService struct {
	mock.Mock
}

// ReconcileSliceQoSConfig provides a mock function with given fields: ctx, req
func (_m *ISliceQoSConfigService) ReconcileSliceQoSConfig(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
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