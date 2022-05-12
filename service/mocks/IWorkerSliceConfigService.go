// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import context "context"
import mock "github.com/stretchr/testify/mock"
import reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

import v1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"

// IWorkerSliceConfigService is an autogenerated mock type for the IWorkerSliceConfigService type
type IWorkerSliceConfigService struct {
	mock.Mock
}

// ComputeClusterMap provides a mock function with given fields: clusterNames, workerSliceConfigs
func (_m *IWorkerSliceConfigService) ComputeClusterMap(clusterNames []string, workerSliceConfigs []v1alpha1.WorkerSliceConfig) map[string]int {
	ret := _m.Called(clusterNames, workerSliceConfigs)

	var r0 map[string]int
	if rf, ok := ret.Get(0).(func([]string, []v1alpha1.WorkerSliceConfig) map[string]int); ok {
		r0 = rf(clusterNames, workerSliceConfigs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]int)
		}
	}

	return r0
}

// CreateMinimalWorkerSliceConfig provides a mock function with given fields: ctx, clusters, namespace, label, name, sliceSubnet
func (_m *IWorkerSliceConfigService) CreateMinimalWorkerSliceConfig(ctx context.Context, clusters []string, namespace string, label map[string]string, name string, sliceSubnet string) (map[string]int, error) {
	ret := _m.Called(ctx, clusters, namespace, label, name, sliceSubnet)

	var r0 map[string]int
	if rf, ok := ret.Get(0).(func(context.Context, []string, string, map[string]string, string, string) map[string]int); ok {
		r0 = rf(ctx, clusters, namespace, label, name, sliceSubnet)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]int)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string, string, map[string]string, string, string) error); ok {
		r1 = rf(ctx, clusters, namespace, label, name, sliceSubnet)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteWorkerSliceConfigByLabel provides a mock function with given fields: ctx, label, namespace
func (_m *IWorkerSliceConfigService) DeleteWorkerSliceConfigByLabel(ctx context.Context, label map[string]string, namespace string) error {
	ret := _m.Called(ctx, label, namespace)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, map[string]string, string) error); ok {
		r0 = rf(ctx, label, namespace)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ListWorkerSliceConfigs provides a mock function with given fields: ctx, ownerLabel, namespace
func (_m *IWorkerSliceConfigService) ListWorkerSliceConfigs(ctx context.Context, ownerLabel map[string]string, namespace string) ([]v1alpha1.WorkerSliceConfig, error) {
	ret := _m.Called(ctx, ownerLabel, namespace)

	var r0 []v1alpha1.WorkerSliceConfig
	if rf, ok := ret.Get(0).(func(context.Context, map[string]string, string) []v1alpha1.WorkerSliceConfig); ok {
		r0 = rf(ctx, ownerLabel, namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]v1alpha1.WorkerSliceConfig)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, map[string]string, string) error); ok {
		r1 = rf(ctx, ownerLabel, namespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReconcileWorkerSliceConfig provides a mock function with given fields: ctx, req
func (_m *IWorkerSliceConfigService) ReconcileWorkerSliceConfig(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
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
