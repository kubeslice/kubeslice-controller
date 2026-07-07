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
)

func TestNewLeaderElection_AppliesTimingDefaults(t *testing.T) {
	le := NewLeaderElection(LeaderElectionConfig{
		Identity:  "controller-0",
		LeaseName: "ha-lease",
		Logger:    testLogger(),
	})
	assert.Equal(t, 15*time.Second, le.leaseDuration)
	assert.Equal(t, 10*time.Second, le.renewDeadline)
	assert.Equal(t, 2*time.Second, le.retryPeriod)
}

func TestNewLeaderElection_KeepsExplicitTimings(t *testing.T) {
	le := NewLeaderElection(LeaderElectionConfig{
		Identity:      "controller-0",
		LeaseName:     "ha-lease",
		LeaseDuration: 30 * time.Second,
		RenewDeadline: 20 * time.Second,
		RetryPeriod:   5 * time.Second,
		Logger:        testLogger(),
	})
	assert.Equal(t, 30*time.Second, le.leaseDuration)
	assert.Equal(t, 20*time.Second, le.renewDeadline)
	assert.Equal(t, 5*time.Second, le.retryPeriod)
}

func TestLeaderElection_InitialStateIsNotLeader(t *testing.T) {
	le := NewLeaderElection(LeaderElectionConfig{Identity: "controller-0", LeaseName: "ha-lease", Logger: testLogger()})
	assert.False(t, le.IsLeader())
	assert.Empty(t, le.LeaderIdentity())
}

func TestLeaderElection_RunRequiresClient(t *testing.T) {
	le := NewLeaderElection(LeaderElectionConfig{Identity: "controller-0", LeaseName: "ha-lease", Logger: testLogger()})
	err := le.Run(context.Background())
	assert.Error(t, err)
}

func TestLeaderElection_AcquireCallbackFiresAndSetsLeader(t *testing.T) {
	le := NewLeaderElection(LeaderElectionConfig{Identity: "controller-0", LeaseName: "ha-lease", Logger: testLogger()})

	acquired := false
	le.SetCallbacks(func(context.Context) { acquired = true }, nil)
	le.handleStartedLeading(context.Background())

	assert.True(t, acquired)
	assert.True(t, le.IsLeader())
	assert.Equal(t, "controller-0", le.LeaderIdentity())
}

func TestLeaderElection_LostCallbackFiresAndClearsLeader(t *testing.T) {
	le := NewLeaderElection(LeaderElectionConfig{Identity: "controller-0", LeaseName: "ha-lease", Logger: testLogger()})

	lost := false
	le.SetCallbacks(nil, func() { lost = true })
	le.handleStartedLeading(context.Background())
	le.handleStoppedLeading()

	assert.True(t, lost)
	assert.False(t, le.IsLeader())
}

func TestLeaderElection_NewLeaderUpdatesObservedIdentity(t *testing.T) {
	le := NewLeaderElection(LeaderElectionConfig{Identity: "controller-0", LeaseName: "ha-lease", Logger: testLogger()})
	le.handleNewLeader("controller-1")
	assert.Equal(t, "controller-1", le.LeaderIdentity())
}
