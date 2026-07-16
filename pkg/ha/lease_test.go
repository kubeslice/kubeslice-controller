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

package ha

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// ---- shared test helpers, used across the ha package tests ----

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	return scheme
}

func fakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objs...).Build()
}

// failingWriteClient returns a client whose Create and Update always error,
// simulating an unreachable API server (used to exercise natural fencing).
func failingWriteClient(t *testing.T) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(testScheme(t)).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			return fmt.Errorf("simulated API server down")
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			return fmt.Errorf("simulated API server down")
		},
	}).Build()
}

func testLog() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

func int32Ptr(i int32) *int32 { return &i }

func microTimePtr(tm time.Time) *metav1.MicroTime {
	m := metav1.NewMicroTime(tm)
	return &m
}

func newLease(name, namespace, holder string, renew time.Time) *coordinationv1.Lease {
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &holder,
			LeaseDurationSeconds: int32Ptr(15),
			RenewTime:            microTimePtr(renew),
		},
	}
}

// ---- tests ----

func TestIsLeaseStale(t *testing.T) {
	now := time.Now()
	padding := 5 * time.Second

	fresh := newLease("l", "ns", "hub-a", now.Add(-2*time.Second))
	stale := newLease("l", "ns", "hub-a", now.Add(-60*time.Second))
	noRenew := &coordinationv1.Lease{Spec: coordinationv1.LeaseSpec{LeaseDurationSeconds: int32Ptr(15)}}

	assert.False(t, isLeaseStale(fresh, padding, now), "fresh lease should not be stale")
	assert.True(t, isLeaseStale(stale, padding, now), "old renewTime should be stale")
	assert.True(t, isLeaseStale(noRenew, padding, now), "missing renewTime should be stale")
	assert.True(t, isLeaseStale(nil, padding, now), "nil lease should be stale")
}

func TestAcquireOrRenewLease_CreatesThenRenews(t *testing.T) {
	ctx := context.Background()
	c := fakeClient(t)

	lease, err := acquireOrRenewLease(ctx, c, "l", "ns", "hub-a", 15*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lease.Spec.HolderIdentity)
	assert.Equal(t, "hub-a", *lease.Spec.HolderIdentity)
	require.NotNil(t, lease.Spec.RenewTime)
	require.NotNil(t, lease.Spec.LeaseTransitions)
	assert.Equal(t, int32(0), *lease.Spec.LeaseTransitions)
	firstRenew := lease.Spec.RenewTime.Time

	time.Sleep(2 * time.Millisecond)
	lease2, err := acquireOrRenewLease(ctx, c, "l", "ns", "hub-a", 15*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "hub-a", *lease2.Spec.HolderIdentity)
	assert.False(t, lease2.Spec.RenewTime.Time.Before(firstRenew), "renewTime should not move backwards")
	assert.Equal(t, int32(0), *lease2.Spec.LeaseTransitions, "same holder should not bump transitions")
}

func TestAcquireOrRenewLease_TakeoverBumpsTransitions(t *testing.T) {
	ctx := context.Background()
	existing := newLease("l", "ns", "hub-a", time.Now().Add(-time.Hour))
	existing.Spec.LeaseTransitions = int32Ptr(0)
	c := fakeClient(t, existing)

	lease, err := acquireOrRenewLease(ctx, c, "l", "ns", "hub-b", 15*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "hub-b", *lease.Spec.HolderIdentity)
	require.NotNil(t, lease.Spec.LeaseTransitions)
	assert.Equal(t, int32(1), *lease.Spec.LeaseTransitions, "takeover should increment transitions")
}

func TestAcquireOrRenewLease_SubSecondDurationClampsToOne(t *testing.T) {
	ctx := context.Background()
	c := fakeClient(t)

	lease, err := acquireOrRenewLease(ctx, c, "l", "ns", "hub-a", 500*time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, lease.Spec.LeaseDurationSeconds)
	assert.Equal(t, int32(1), *lease.Spec.LeaseDurationSeconds,
		"a sub-second duration must clamp to 1s, not truncate to 0")
}

func TestGetLease_NotFoundReturnsError(t *testing.T) {
	c := fakeClient(t)
	_, err := getLease(context.Background(), c, "missing", "ns")
	assert.Error(t, err, "a missing lease must surface as an error, not a nil/zero value")
}
