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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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

func TestNewClusterLeaderElector_LeaseNamespacePrefersDownwardAPIEnvVar(t *testing.T) {
	t.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", "kubeslice-avesha")
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Log: testLog()})
	assert.Equal(t, "kubeslice-avesha", e.leaseNS,
		"an empty LeaseNamespace must prefer the controller's own runtime namespace over the hard-coded default")
}

func TestNewClusterLeaderElector_LeaseNamespaceFallsBackWhenEnvVarUnset(t *testing.T) {
	t.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", "")
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Log: testLog()})
	assert.Equal(t, DefaultLeaseNamespace, e.leaseNS,
		"with no env var and no explicit Options.LeaseNamespace, must fall back to DefaultLeaseNamespace")
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

func TestCheckRemoteLeaseOnce_PropagatesGetError(t *testing.T) {
	remote := fakeClient(t) // the Active's lease is not present on the remote
	e := NewClusterLeaderElector(fakeClient(t), remote, Options{Mode: ModeStandby, Log: testLog()})

	stale, err := e.checkRemoteLeaseOnce(context.Background())
	require.Error(t, err, "a missing remote lease must surface as an error, not silently report fresh")
	assert.False(t, stale)
}

func TestRenewOnce_KeepsLeadershipWithinRenewDeadline(t *testing.T) {
	e := NewClusterLeaderElector(failingWriteClient(t), nil, Options{
		Mode:          ModeActive,
		RenewDeadline: time.Hour,
		Log:           testLog(),
	})
	e.isLeader.Store(true)
	e.lastRenew = time.Now() // just renewed, well within the deadline

	err := e.renewOnce(context.Background())
	require.Error(t, err, "a failed renew attempt must still surface an error to the caller")
	assert.True(t, e.IsLeader(), "leadership must be kept while still within renewDeadline (transient failure)")
}

func TestSetLeader_LogsOnlyOnTransition(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core).Sugar()

	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Mode: ModeActive, Identity: "hub-a", Log: log})

	e.setLeader(true)
	e.setLeader(true) // no transition; must not log again
	e.setLeader(false)
	e.setLeader(false) // no transition; must not log again

	assert.Equal(t, 1, logs.FilterMessage("LeadershipAcquired").Len(),
		"LeadershipAcquired must be logged exactly once per actual transition")
	assert.Equal(t, 1, logs.FilterMessage("LeadershipLost").Len(),
		"LeadershipLost must be logged exactly once per actual transition")
}

func TestStartLeaseRenewal_NoopWhenNotActive(t *testing.T) {
	for _, mode := range []HAMode{ModeStandby, ModeStandalone} {
		e := NewClusterLeaderElector(fakeClient(t), fakeClient(t), Options{Mode: mode, Log: testLog()})
		err := e.StartLeaseRenewal(context.Background())
		assert.NoError(t, err, "StartLeaseRenewal must be a no-op outside active mode")
	}
}

func TestStartLeaseRenewal_ReturnsNilOnContextCancellation(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{
		Mode:        ModeActive,
		RetryPeriod: 20 * time.Millisecond,
		Log:         testLog(),
	})
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- e.StartLeaseRenewal(ctx) }()

	require.Eventually(t, e.IsLeader, time.Second, 5*time.Millisecond,
		"elector should acquire leadership before shutdown")

	cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err, "a graceful shutdown (context cancellation) must not be reported as an error")
	case <-time.After(time.Second):
		t.Fatal("StartLeaseRenewal did not return after context cancellation")
	}
	assert.False(t, e.IsLeader(), "leadership must be released on shutdown")
}

func TestWatchRemoteLease_NoopWhenNotStandby(t *testing.T) {
	for _, mode := range []HAMode{ModeActive, ModeStandalone} {
		e := NewClusterLeaderElector(fakeClient(t), nil, Options{Mode: mode, Log: testLog()})
		err := e.WatchRemoteLease(context.Background())
		assert.NoError(t, err, "WatchRemoteLease must be a no-op outside standby mode")
	}
}

func TestWatchRemoteLease_RequiresRemoteClientInStandbyMode(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Mode: ModeStandby, Log: testLog()})
	err := e.WatchRemoteLease(context.Background())
	assert.Error(t, err, "standby mode without a remote client must fail fast instead of watching nothing")
}

func TestWatchRemoteLease_ReturnsNilOnContextCancellation(t *testing.T) {
	remote := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now()))
	e := NewClusterLeaderElector(fakeClient(t), remote, Options{
		Mode:        ModeStandby,
		RetryPeriod: 20 * time.Millisecond,
		Log:         testLog(),
	})
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- e.WatchRemoteLease(ctx) }()

	time.Sleep(50 * time.Millisecond) // let at least one watch tick fire
	cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err, "a graceful shutdown (context cancellation) must not be reported as an error")
	case <-time.After(time.Second):
		t.Fatal("WatchRemoteLease did not return after context cancellation")
	}
}

// --- issue #297: detection ---------------------------------------------------

// TestNeverArmed_NeverBecomesCandidate is the regression test for the single
// most dangerous mistake available in this file. isLeaseStale(nil, ...) returns
// TRUE, so folding the lastSeenLease nil check into that call would make a
// Standby that has never once read the Active's Lease promote itself on its
// first tick — turning a broken kubeconfig or a missing RBAC grant into a
// guaranteed split brain. If someone "simplifies" the two conditions into one,
// this test fails.
func TestNeverArmed_NeverBecomesCandidate(t *testing.T) {
	// A remote client with no Lease at all: every read fails, so the elector
	// never arms.
	e := NewClusterLeaderElector(fakeClient(t), fakeClient(t), Options{Mode: ModeStandby, Log: testLog()})

	for i := 0; i < 5; i++ {
		candidate, err := e.checkRemoteLeaseOnce(context.Background())
		require.Error(t, err, "the read must genuinely be failing for this test to mean anything")
		assert.False(t, candidate,
			"an elector that has never read the Active's lease must never become a promotion candidate, "+
				"however long it waits — that is a configuration failure, not a dead Active")
	}
	assert.Nil(t, e.lastSeenLease, "a failed read must not populate the cached lease")
}

// TestUnreadableLease_RetainsCacheAndGoesStale is the whole point of #297's
// detection change: an Active whose API server dies (reads fail) must be
// detected exactly like an Active whose pod dies (reads succeed, renewTime
// frozen). Before this, a read failure reported "not stale" forever and the
// loss of an entire hub was undetectable.
func TestUnreadableLease_RetainsCacheAndGoesStale(t *testing.T) {
	// Arm against a fresh lease.
	fresh := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now())
	e := NewClusterLeaderElector(fakeClient(t), fakeClient(t, fresh), Options{Mode: ModeStandby, Log: testLog()})

	candidate, err := e.checkRemoteLeaseOnce(context.Background())
	require.NoError(t, err)
	require.False(t, candidate, "a fresh lease is not a candidate")
	require.NotNil(t, e.lastSeenLease, "a successful read must arm the elector")

	// Now the Active's API server dies: every subsequent read fails.
	e.remoteClient = fakeClient(t)

	candidate, err = e.checkRemoteLeaseOnce(context.Background())
	require.Error(t, err)
	assert.False(t, candidate, "the retained lease is still fresh; a failed read alone proves nothing")
	assert.NotNil(t, e.lastSeenLease, "a failed read must retain the last good view, not clear it")

	// Age the retained view past leaseDuration + padding. This is what a real
	// clock does on its own while reads keep failing.
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))

	candidate, err = e.checkRemoteLeaseOnce(context.Background())
	require.Error(t, err, "reads are still failing")
	assert.True(t, candidate,
		"an unreachable Active whose newest known renewTime has aged out must become a candidate — "+
			"this is the case that was previously undetectable")
}

// TestReadFailure_DoesNotRefreshLastGoodRead guards the other half of the
// retention rule: a failed read must not advance any freshness marker.
func TestReadFailure_DoesNotRefreshLastGoodRead(t *testing.T) {
	fresh := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now())
	e := NewClusterLeaderElector(fakeClient(t), fakeClient(t, fresh), Options{Mode: ModeStandby, Log: testLog()})

	_, err := e.checkRemoteLeaseOnce(context.Background())
	require.NoError(t, err)
	armedAt := e.lastGoodRead
	require.False(t, armedAt.IsZero())

	e.remoteClient = fakeClient(t)
	_, err = e.checkRemoteLeaseOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, armedAt, e.lastGoodRead, "a failed read must not count as a good read")
}

// TestSuccessfulRead_ReplacesCachedLease covers the recovery direction: an
// Active that comes back must clear the candidacy, not leave a stale verdict
// latched.
func TestSuccessfulRead_ReplacesCachedLease(t *testing.T) {
	stale := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))
	e := NewClusterLeaderElector(fakeClient(t), fakeClient(t, stale), Options{Mode: ModeStandby, Log: testLog()})

	candidate, err := e.checkRemoteLeaseOnce(context.Background())
	require.NoError(t, err)
	require.True(t, candidate, "an old renewTime reads as stale")

	// The Active recovers and renews.
	e.remoteClient = fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now()))

	candidate, err = e.checkRemoteLeaseOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, candidate, "a recovered Active must clear candidacy on the next successful read")
}
