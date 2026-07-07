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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestHealthChecker(r LeaseReader, threshold int) *HealthChecker {
	return NewHealthChecker(HealthCheckerConfig{
		Reader:    r,
		Namespace: "kubeslice-controller",
		LeaseName: "kubeslice-controller-ha",
		Interval:  time.Hour,
		Threshold: threshold,
		Logger:    testLogger(),
	})
}

func freshLease() *LeaseInfo {
	return &LeaseInfo{HolderIdentity: "controller-0", RenewTime: time.Now(), LeaseDurationSeconds: 15}
}

func staleLease() *LeaseInfo {
	return &LeaseInfo{HolderIdentity: "controller-0", RenewTime: time.Now().Add(-2 * time.Minute), LeaseDurationSeconds: 15}
}

func TestHealth_FreshLeaseIsHealthy(t *testing.T) {
	r := &memLeaseReader{}
	r.set(freshLease())
	hc := newTestHealthChecker(r, 3)

	st := hc.check(context.Background())
	assert.True(t, st.IsHealthy)
	assert.Equal(t, 0, st.ConsecutiveFailures)
	assert.Equal(t, "controller-0", st.ObservedHolder)
}

func TestHealth_StaleLeaseTripsAfterThreshold(t *testing.T) {
	r := &memLeaseReader{}
	r.set(staleLease())
	hc := newTestHealthChecker(r, 3)

	hc.check(context.Background())
	assert.True(t, hc.IsHealthy(), "should stay healthy below threshold")
	hc.check(context.Background())
	assert.True(t, hc.IsHealthy(), "should stay healthy below threshold")

	st := hc.check(context.Background())
	assert.False(t, st.IsHealthy, "should be unhealthy at threshold")
	assert.Equal(t, 3, st.ConsecutiveFailures)
}

func TestHealth_RecoversAfterRenewal(t *testing.T) {
	r := &memLeaseReader{}
	r.set(staleLease())
	hc := newTestHealthChecker(r, 2)

	hc.check(context.Background())
	hc.check(context.Background())
	assert.False(t, hc.IsHealthy())

	r.set(freshLease())
	st := hc.check(context.Background())
	assert.True(t, st.IsHealthy)
	assert.Equal(t, 0, st.ConsecutiveFailures)
}

func TestHealth_ReadErrorCountsAsFailure(t *testing.T) {
	r := &memLeaseReader{}
	r.setErr(errors.New("connection refused"))
	hc := newTestHealthChecker(r, 1)

	st := hc.check(context.Background())
	assert.False(t, st.IsHealthy)
	assert.Error(t, st.LastError)
}

func TestHealth_LeaseWithoutHolderIsUnhealthy(t *testing.T) {
	r := &memLeaseReader{}
	r.set(&LeaseInfo{RenewTime: time.Now(), LeaseDurationSeconds: 15})
	hc := newTestHealthChecker(r, 1)

	assert.False(t, hc.check(context.Background()).IsHealthy)
}

func TestHealth_ZeroLeaseDurationDoesNotPanic(t *testing.T) {
	r := &memLeaseReader{}
	// LeaseDurationSeconds intentionally zero — must not panic and must fall
	// back to a derived limit.
	r.set(&LeaseInfo{HolderIdentity: "controller-0", RenewTime: time.Now()})
	hc := newTestHealthChecker(r, 3)

	assert.NotPanics(t, func() { hc.check(context.Background()) })
	assert.True(t, hc.IsHealthy())
}

func TestHealth_RunStopsOnContextCancel(t *testing.T) {
	r := &memLeaseReader{}
	r.set(freshLease())
	hc := NewHealthChecker(HealthCheckerConfig{
		Reader: r, Interval: 10 * time.Millisecond, Threshold: 3, Logger: testLogger(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = hc.Run(ctx)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
