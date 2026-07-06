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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClusterLeaderElector_StandaloneIsAlwaysLeader(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Mode: ModeStandalone, Log: testLog()})
	assert.True(t, e.IsLeader(), "standalone must be leader")
	assert.Equal(t, ModeStandalone, e.Mode())
}

func TestNewClusterLeaderElector_DefaultsToStandalone(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Log: testLog()})
	assert.Equal(t, ModeStandalone, e.Mode(), "empty mode must default to standalone (no regression)")
	assert.True(t, e.IsLeader())
}

func TestActive_BecomesLeaderAfterRenew(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Mode: ModeActive, Log: testLog()})
	assert.False(t, e.IsLeader(), "active is not leader until it renews")
	require.NoError(t, e.renewOnce(context.Background()))
	assert.True(t, e.IsLeader(), "active should hold leadership after a successful renew")
}

func TestActive_LosesLeadershipAfterRenewDeadline(t *testing.T) {
	e := NewClusterLeaderElector(failingWriteClient(t), nil, Options{
		Mode:          ModeActive,
		RenewDeadline: 10 * time.Millisecond,
		Log:           testLog(),
	})
	// Simulate having been the leader, with the last successful renew well past
	// the renew deadline.
	e.isLeader.Store(true)
	e.lastRenew = time.Now().Add(-time.Hour)

	err := e.renewOnce(context.Background())
	require.Error(t, err)
	assert.False(t, e.IsLeader(), "leadership must drop once renewDeadline is exceeded (natural fencing)")
}

func TestStandby_NeverLeaderEvenWhenLeaseStale(t *testing.T) {
	// A fresh lease on the remote: the standby stays a standby.
	freshRemote := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now()))
	e := NewClusterLeaderElector(fakeClient(t), freshRemote, Options{Mode: ModeStandby, Log: testLog()})
	assert.False(t, e.IsLeader())
	stale, err := e.checkRemoteLeaseOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, stale)
	assert.False(t, e.IsLeader())

	// A stale lease on the remote: #294 detects staleness but must NOT promote.
	staleRemote := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour)))
	e2 := NewClusterLeaderElector(fakeClient(t), staleRemote, Options{Mode: ModeStandby, Log: testLog()})
	stale2, err := e2.checkRemoteLeaseOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, stale2, "old renewTime should read as stale")
	assert.False(t, e2.IsLeader(), "standby must not promote in #294 (promotion is #297)")
}
