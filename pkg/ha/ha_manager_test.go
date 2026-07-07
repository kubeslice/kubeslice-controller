/*
 * 	Copyright (c) 2026 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) (*HAManager, *fakeElection, *memCollector) {
	t.Helper()
	election := &fakeElection{}
	collector := &memCollector{}
	collector.set(ref("SliceConfig", "slice-a", "1"))
	lease := &memLeaseReader{}
	lease.set(freshLease())

	hm, err := NewHAManager(HAManagerConfig{
		Identity:            "controller-0",
		Namespace:           "kubeslice-controller",
		LeaseName:           "kubeslice-controller-ha",
		Logger:              testLogger(),
		Election:            election,
		Collector:           collector,
		LeaseReader:         lease,
		SyncInterval:        time.Hour,
		HealthCheckInterval: time.Hour,
	})
	require.NoError(t, err)
	return hm, election, collector
}

func TestNewHAManager_RequiresMandatoryFields(t *testing.T) {
	_, err := NewHAManager(HAManagerConfig{LeaseName: "x", Identity: "y"})
	assert.Error(t, err, "logger required")

	_, err = NewHAManager(HAManagerConfig{Logger: testLogger(), LeaseName: "x"})
	assert.Error(t, err, "identity required")

	_, err = NewHAManager(HAManagerConfig{Logger: testLogger(), Identity: "y"})
	assert.Error(t, err, "lease name required")
}

func TestHAManager_InitialRoleIsStandby(t *testing.T) {
	hm, _, _ := newTestManager(t)
	assert.True(t, hm.IsStandby())
	assert.False(t, hm.IsActive())
	assert.Equal(t, RoleStandby, hm.GetCurrentState().Role)
}

func TestHAManager_PromotionTransitionsToActive(t *testing.T) {
	hm, election, _ := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hm.Start(ctx))
	defer hm.Stop()

	election.promote()

	assert.True(t, hm.IsActive())
	st := hm.GetCurrentState()
	assert.Equal(t, RoleActive, st.Role)
	assert.True(t, st.IsLeader)
	assert.Equal(t, 1, st.Transitions)
}

func TestHAManager_DemotionTransitionsToStandby(t *testing.T) {
	hm, election, _ := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hm.Start(ctx))
	defer hm.Stop()

	election.promote()
	require.True(t, hm.IsActive())
	election.demote()

	assert.True(t, hm.IsStandby())
	st := hm.GetCurrentState()
	assert.Equal(t, RoleStandby, st.Role)
	assert.False(t, st.IsLeader)
	assert.Equal(t, 2, st.Transitions)
}

func TestHAManager_PromotionTriggersImmediateSync(t *testing.T) {
	hm, election, collector := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hm.Start(ctx))
	defer hm.Stop()

	require.NoError(t, hm.sync.WaitForSync(ctx))
	before := hm.sync.SyncCount()

	collector.set(ref("SliceConfig", "slice-a", "1"), ref("Cluster", "worker-1", "1"))
	election.promote()

	// The post-promotion sync must have run and picked up the new resource.
	assert.Greater(t, hm.sync.SyncCount(), before)
	assert.Equal(t, 2, hm.StateSnapshot().Count())
}

func TestHAManager_WaitForActivePromotionUnblocksOnPromotion(t *testing.T) {
	hm, election, _ := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hm.Start(ctx))
	defer hm.Stop()

	result := make(chan error, 1)
	go func() {
		waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer waitCancel()
		result <- hm.WaitForActivePromotion(waitCtx)
	}()

	time.Sleep(20 * time.Millisecond)
	election.promote()

	select {
	case err := <-result:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("WaitForActivePromotion did not unblock")
	}
}

func TestHAManager_WaitForActivePromotionRespectsContext(t *testing.T) {
	hm, _, _ := newTestManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, hm.WaitForActivePromotion(ctx), context.DeadlineExceeded)
}

func TestHAManager_DoubleStartIsRejected(t *testing.T) {
	hm, _, _ := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hm.Start(ctx))
	defer hm.Stop()

	assert.Error(t, hm.Start(ctx))
}

func TestHAManager_StopIsIdempotent(t *testing.T) {
	hm, _, _ := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hm.Start(ctx))

	hm.Stop()
	assert.NotPanics(t, hm.Stop)
}

func TestHAManager_HealthStatusIsExposed(t *testing.T) {
	hm, _, _ := newTestManager(t)
	assert.True(t, hm.HealthStatus().IsHealthy)
}
