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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
)

// promotionRecorder records the order in which hooks ran and what the write
// fence reported at each point. Ordering is the whole subject of several tests
// below, so it is captured rather than inferred.
type promotionRecorder struct {
	mu           sync.Mutex
	steps        []string
	fenceOpenAt  map[string]bool
	stopMirror   error
	publishErr   error
	kickErr      error
	publishDelay time.Duration
}

func newPromotionRecorder() *promotionRecorder {
	return &promotionRecorder{fenceOpenAt: map[string]bool{}}
}

func (r *promotionRecorder) record(step string, e *ClusterLeaderElector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, step)
	r.fenceOpenAt[step] = e.IsLeader()
}

func (r *promotionRecorder) order() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.steps...)
}

func (r *promotionRecorder) hooks(e *ClusterLeaderElector) PromotionHooks {
	return PromotionHooks{
		StopMirror: func(ctx context.Context) error {
			r.record("stopMirror", e)
			return r.stopMirror
		},
		PublishActiveController: func(ctx context.Context) error {
			r.record("publish", e)
			if r.publishDelay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(r.publishDelay):
				}
			}
			return r.publishErr
		},
		KickReconcilers: func(ctx context.Context) error {
			r.record("kick", e)
			return r.kickErr
		},
		EmitPromotedEvent: func(ctx context.Context, lease *coordinationv1.Lease) error {
			r.record("event", e)
			return nil
		},
	}
}

// standbyReadyToPromote builds a Standby whose Active is unreachable and whose
// cached view has aged out — i.e. one tick away from promoting.
func standbyReadyToPromote(t *testing.T) *ClusterLeaderElector {
	t.Helper()
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode:     ModeStandby,
		Identity: "hub-b",
		Log:      testLog(),
	})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))
	return e
}

func TestPromote_HappyPath(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	require.True(t, promoted)

	assert.True(t, e.IsLeader(), "the write fence must be open once promotion completes")
	assert.Equal(t, ModeActive, e.Mode())

	lease, err := getLease(context.Background(), e.localClient, e.leaseName, e.leaseNS)
	require.NoError(t, err, "promotion must acquire the lease on this hub's own cluster")
	assert.Equal(t, "hub-b", leaseHolder(lease))
}

// TestPromote_StopsMirrorBeforeOpeningTheFence is the ordering bug that both
// issue #297 and an earlier draft of ADR Decision 5 got wrong. In the most
// common trigger — the Active's pod dies while its API server stays healthy —
// the mirror is still live at this moment. Because every mirrored object
// carries the syncer's label and the mirror's conflict guard only skips objects
// WITHOUT it, a fence opened before the mirror stopped means the mirror
// overwrites exactly what the new Active writes, and prune's reverse diff
// resurrects whatever it deletes.
func TestPromote_StopsMirrorBeforeOpeningTheFence(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	require.True(t, promoted)

	order := rec.order()
	require.Contains(t, order, "stopMirror")
	assert.Equal(t, "stopMirror", order[0], "the mirror must be stopped first, before anything else")
	assert.False(t, rec.fenceOpenAt["stopMirror"],
		"the write fence must still be SHUT while the mirror is being stopped — otherwise the "+
			"promoted hub and the still-running mirror write to the same objects")
	assert.False(t, rec.fenceOpenAt["publish"],
		"the fence must still be shut while activeController is published")
}

// TestPromote_FenceOpensOnlyAfterPublish covers the other half: the kick and
// the event must land on a hub whose fence is already open, or the kick's
// re-enqueued requests are dropped without requeue — the exact failure the kick
// exists to fix.
func TestPromote_FenceOpensOnlyAfterPublish(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	_, err := e.promote(context.Background())
	require.NoError(t, err)

	assert.Equal(t, []string{"stopMirror", "publish", "kick", "event"}, rec.order(),
		"promotion order is load-bearing, not incidental")
	assert.True(t, rec.fenceOpenAt["kick"],
		"the kick must run with the fence OPEN, or every re-enqueued request is dropped without requeue")
	assert.True(t, rec.fenceOpenAt["event"])
}

// TestPromote_FenceStaysShutThroughout is the property steps 0 and 8 exist to
// provide, checked from outside the hooks: at no point during the sequence may
// a concurrent Reconcile see itself as leader.
func TestPromote_FenceStaysShutThroughout(t *testing.T) {
	e := standbyReadyToPromote(t)
	// Pretend a previous life left isLeader set. Only the promoting latch should
	// be keeping the fence shut.
	e.isLeader.Store(true)

	observed := make(chan bool, 64)
	rec := newPromotionRecorder()
	hooks := rec.hooks(e)
	inner := hooks.StopMirror
	hooks.StopMirror = func(ctx context.Context) error {
		observed <- e.IsLeader()
		return inner(ctx)
	}
	e.SetPromotionHooks(hooks)

	_, err := e.promote(context.Background())
	require.NoError(t, err)
	close(observed)

	for sawLeader := range observed {
		assert.False(t, sawLeader,
			"IsLeader() must report false for the whole sequence even when isLeader is set, "+
				"because the promoting latch overrides it")
	}
	assert.True(t, e.IsLeader(), "and must report true once the sequence completes")
}

// TestPromote_GuardRefusalIsNotAnError: declining to promote is a correct
// outcome. It must leave the hub a fenced, still-armed Standby.
func TestPromote_GuardRefusalLeavesHubUnchanged(t *testing.T) {
	// Self-health fails: this hub cannot reach its own API server.
	e := NewClusterLeaderElector(failingReadClient(t), failingReadClient(t), Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.NoError(t, err, "a guard refusal is a correct outcome, not an error")
	assert.False(t, promoted)

	assert.False(t, e.IsLeader(), "still fenced")
	assert.Equal(t, ModeStandby, e.Mode(), "still a standby")
	assert.False(t, e.promoting.Load(), "the promotion latch must be released so the next tick can retry")
	assert.NotNil(t, e.lastSeenLease, "still armed")
	assert.Empty(t, rec.order(), "no hook may run once a guard has refused")
}

// TestPromote_MirrorStopFailureAborts: a half-stopped mirror is exactly the
// dual-writer state step 3 exists to prevent, so failing to stop it must abort
// the promotion rather than press on.
func TestPromote_MirrorStopFailureAborts(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	rec.stopMirror = fmt.Errorf("simulated: syncer did not stop")
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.Error(t, err, "the failure must surface, not be swallowed")
	assert.False(t, promoted)

	assert.False(t, e.IsLeader(), "the fence must stay shut")
	assert.Equal(t, ModeStandby, e.Mode())
	assert.False(t, e.promoting.Load(), "the latch must be released so the next tick can retry")
	assert.Equal(t, []string{"stopMirror"}, rec.order(), "nothing after the mirror stop may run")
}

// TestPromote_PublishFailureStillPromotes: publication is a budget, not a
// precondition. A hub that cannot describe itself is still a better Active than
// no Active at all, and the publisher's own loop keeps retrying.
func TestPromote_PublishFailureStillPromotes(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	rec.publishErr = fmt.Errorf("simulated: API server busy")
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	assert.True(t, promoted, "failing to publish must not strand the cluster with no writer")
	assert.True(t, e.IsLeader())
	assert.Contains(t, rec.order(), "kick", "the rest of the sequence must still run")
}

// TestPromote_PublishRespectsGracePeriod: a publication that hangs must not
// hold the write fence shut indefinitely.
func TestPromote_PublishRespectsGracePeriod(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode:                 ModeStandby,
		Identity:             "hub-b",
		PromotionGracePeriod: 50 * time.Millisecond,
		Log:                  testLog(),
	})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))
	rec := newPromotionRecorder()
	rec.publishDelay = 10 * time.Second
	e.SetPromotionHooks(rec.hooks(e))

	done := make(chan struct{})
	go func() {
		defer close(done)
		promoted, err := e.promote(context.Background())
		assert.NoError(t, err)
		assert.True(t, promoted)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("promotion did not bound the publish step by PromotionGracePeriod — a hung " +
			"publication must not hold the write fence shut indefinitely")
	}
	assert.True(t, e.IsLeader())
}

// TestPromote_KickFailureStillPromotes: the kick is important enough to log
// loudly about and not important enough to abandon a promotion for. Without it
// pre-existing mirrored state stays unreconciled, but the hub is still a
// working Active for anything new.
func TestPromote_KickFailureStillPromotes(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	rec.kickErr = fmt.Errorf("simulated: channel full")
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	assert.True(t, promoted)
	assert.Contains(t, rec.order(), "event", "the promotion must still be recorded")
}

func TestPromote_IsOnceOnly(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	require.True(t, promoted)

	// A second attempt on an already-promoted hub must be a no-op. Re-running
	// the sequence would race the renewal loop promotion just started — that
	// goroutine owns lastRenew and is actively writing the Lease — and fight it
	// for the same object. -race found this during development, and this test is
	// what keeps it found.
	before := len(rec.order())
	promoted2, err := e.promote(context.Background())
	require.NoError(t, err)
	assert.True(t, promoted2, "an already-active hub is already in the state promotion produces")
	assert.Equal(t, before, len(rec.order()),
		"no hook may run a second time — the sequence must be genuinely once-only, not merely idempotent")
	assert.True(t, e.IsLeader())
}

func TestPromote_NilHooksAreSkipped(t *testing.T) {
	e := standbyReadyToPromote(t)
	// No SetPromotionHooks call at all.

	promoted, err := e.promote(context.Background())
	require.NoError(t, err, "an elector with no hooks must still be able to take leadership")
	assert.True(t, promoted)
	assert.True(t, e.IsLeader())
	assert.Equal(t, ModeActive, e.Mode())
}

// TestWatchRemoteLease_PromotesAndReturns is the end-to-end path: a Standby
// watching a dead Active must promote and stop watching, because there is no
// longer an Active to watch.
func TestWatchRemoteLease_PromotesAndReturns(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode:        ModeStandby,
		Identity:    "hub-b",
		RetryPeriod: 10 * time.Millisecond,
		Log:         testLog(),
	})
	// Armed against a Lease that has already aged out.
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- e.WatchRemoteLease(ctx) }()

	select {
	case err := <-errCh:
		require.NoError(t, err)
		assert.True(t, e.IsLeader(), "the watch must have promoted before returning")
		assert.Equal(t, ModeActive, e.Mode())
	case <-time.After(3 * time.Second):
		t.Fatal("WatchRemoteLease did not promote and return")
	}
}

// TestWatchRemoteLease_NeverArmedNeverPromotes is the same loop under the
// failure that matters most: a Standby that has never read the Active's Lease
// must sit there warning forever rather than promoting itself.
func TestWatchRemoteLease_NeverArmedNeverPromotes(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), fakeClient(t), Options{
		Mode:        ModeStandby,
		Identity:    "hub-b",
		RetryPeriod: 5 * time.Millisecond,
		Log:         testLog(),
	})
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	require.NoError(t, e.WatchRemoteLease(ctx))

	assert.False(t, e.IsLeader(), "an unarmed standby must never promote, however many ticks pass")
	assert.Equal(t, ModeStandby, e.Mode())
	assert.Empty(t, rec.order(), "the promotion sequence must never have started")
}

// TestPromote_ConcurrentAttemptsRunTheSequenceOnce exercises the latch under
// real concurrency rather than trusting the comment on it. Two goroutines enter
// promote at the same time; exactly one may run the sequence, and neither may
// see a half-promoted hub.
func TestPromote_ConcurrentAttemptsRunTheSequenceOnce(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()

	// Hold the sequence open inside the first hook so the second caller is
	// guaranteed to arrive while the first is still mid-flight.
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	hooks := rec.hooks(e)
	inner := hooks.StopMirror
	hooks.StopMirror = func(ctx context.Context) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return inner(ctx)
	}
	e.SetPromotionHooks(hooks)

	results := make(chan bool, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ok, err := e.promote(context.Background())
		results <- ok
		errs <- err
	}()

	<-entered // the first caller is inside the sequence, holding the latch

	wg.Add(1)
	go func() {
		defer wg.Done()
		ok, err := e.promote(context.Background())
		results <- ok
		errs <- err
	}()
	// Give the second caller time to hit the latch and bail before releasing.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err, "a rejected concurrent attempt is not an error")
	}
	promotedCount := 0
	for ok := range results {
		if ok {
			promotedCount++
		}
	}
	assert.Equal(t, 1, promotedCount,
		"exactly one of two concurrent attempts may report having promoted")
	assert.Equal(t, []string{"stopMirror", "publish", "kick", "event"}, rec.order(),
		"the sequence must have run exactly once, not twice and not partially")
	assert.True(t, e.IsLeader())
}

// TestPromote_LeaseAcquisitionFailureAborts covers the one step whose failure
// means the hub genuinely cannot lead: without the Lease there is nothing
// fencing the old Active, so opening the write fence anyway would be the
// dual-writer state the whole design exists to prevent.
func TestPromote_LeaseAcquisitionFailureAborts(t *testing.T) {
	// Reads succeed (so self-health passes) but writes fail, so the Lease
	// cannot be created.
	e := NewClusterLeaderElector(failingWriteClient(t), failingReadClient(t), Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.Error(t, err, "failing to take the lease must surface")
	assert.False(t, promoted)

	assert.False(t, e.IsLeader(), "the write fence must stay shut without a lease")
	assert.Equal(t, ModeStandby, e.Mode(), "mode must not have flipped")
	assert.False(t, e.promoting.Load(), "the latch must be released so the next tick can retry")
	assert.Equal(t, []string{"stopMirror"}, rec.order(),
		"nothing past the lease acquisition may run")
}

// TestPromote_EventFailureStillPromotes: the Event is a report, not a step. A
// hub that took over but could not say so is still the Active.
func TestPromote_EventFailureStillPromotes(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	hooks := rec.hooks(e)
	hooks.EmitPromotedEvent = func(ctx context.Context, lease *coordinationv1.Lease) error {
		rec.record("event", e)
		return fmt.Errorf("simulated: event recorder unavailable")
	}
	e.SetPromotionHooks(hooks)

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	assert.True(t, promoted, "failing to report a promotion must not undo it")
	assert.True(t, e.IsLeader())
}

// TestPromote_AttachesTheAcquiredLeaseToTheEvent: the Event must describe the
// Lease this hub just took, not some other object — that is what puts it in the
// controller's own namespace.
func TestPromote_AttachesTheAcquiredLeaseToTheEvent(t *testing.T) {
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	hooks := rec.hooks(e)
	var got *coordinationv1.Lease
	hooks.EmitPromotedEvent = func(ctx context.Context, lease *coordinationv1.Lease) error {
		got = lease
		return nil
	}
	e.SetPromotionHooks(hooks)

	_, err := e.promote(context.Background())
	require.NoError(t, err)

	require.NotNil(t, got, "the event hook must receive the acquired lease")
	assert.Equal(t, DefaultLeaseName, got.Name)
	assert.Equal(t, "hub-b", leaseHolder(got), "the lease must already name this hub as holder")
}
