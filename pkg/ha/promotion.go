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
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
)

// PromotionHooks are the effects promotion has outside the elector itself.
// They are injected rather than imported so the elector stays independent of
// the mirror, the publisher and the controller-runtime manager — and so the
// whole sequence is testable without any of them.
//
// Every hook is optional; a nil hook is skipped. That is what lets a Standby
// run the sequence in a unit test, and it is why the elector needs no knowledge
// of what a RemoteSyncer or a reconciler even is.
type PromotionHooks struct {
	// StopMirror cancels the RemoteSyncer and must not return until it has
	// fully stopped writing, or until ctx expires — whichever comes first.
	// Returning early, before the mirror has confirmed it stopped, is what step
	// 3 of promote exists to prevent; returning an error on expiry is correct
	// and aborts the promotion. Bounded by promotionGracePeriod.
	StopMirror func(ctx context.Context) error

	// PublishActiveController writes status.activeController on this hub's own
	// Cluster CRs, so workers can discover the new Active. Bounded by
	// promotionGracePeriod; on expiry promotion proceeds anyway.
	PublishActiveController func(ctx context.Context) error

	// KickReconcilers re-enqueues every object of every reconciled type. The
	// write fence drops rather than requeues while a hub is Standby, so without
	// this a promoted hub reconciles nothing that already existed until the
	// informer resync period (10h by default).
	KickReconcilers func(ctx context.Context) error

	// EmitPromotedEvent records the promotion as a Kubernetes Event against the
	// newly-acquired Lease.
	EmitPromotedEvent func(ctx context.Context, lease *coordinationv1.Lease) error
}

// promote runs the full Standby -> Active sequence. It reports whether
// leadership was actually taken; a guard refusal returns (false, nil), because
// declining to promote is a correct outcome and not an error.
//
// The order below is deliberate and two steps of it are load-bearing enough to
// be worth stating up front, because both issue #297 and an earlier draft of
// ADR Decision 5 got them wrong:
//
//  0. take the promotion latch — the write fence is held SHUT from here
//  1. (caller) staleness verdict
//  2. guards: self-health, then the final bounded dial
//  3. STOP the mirror, and WAIT for it to confirm it stopped
//  4. (caller) stop watching the Active's Lease
//  5. acquire the Lease on THIS hub's own cluster
//  6. mode = Active
//  7. publish status.activeController        [budget: promotionGracePeriod]
//  8. release the latch and take leadership — THE WRITE FENCE OPENS HERE
//  9. start renewing our own Lease
//  10. re-enqueue every object of every reconciled type
//  11. emit PromotedToActive and count the failover
func (e *ClusterLeaderElector) promote(ctx context.Context) (bool, error) {
	// Step 0. One-way and once-only, in two parts.
	//
	// The latch stops two attempts interleaving. But it is released on success,
	// so it cannot by itself stop a second attempt on an already-promoted hub —
	// and that is not merely redundant, it is unsafe: promotion has by then
	// started a renewal loop that owns lastRenew and is actively writing the
	// Lease, so a re-run both races that goroutine and fights it for the same
	// object. (-race caught exactly this.) A hub that is already Active is
	// already in the state promotion produces, so report success and do nothing.
	if e.Mode() == ModeActive {
		e.log.Debugw("already active; nothing to promote")
		return true, nil
	}
	if !e.promoting.CompareAndSwap(false, true) {
		haPromotionsAbortedTotal.WithLabelValues(abortAlreadyPromoting).Inc()
		e.log.Warnw("promotion already in progress; ignoring concurrent attempt")
		return false, nil
	}
	// Re-check under the latch: two callers could both have passed the mode
	// check above before either took it.
	if e.Mode() == ModeActive {
		e.promoting.Store(false)
		return true, nil
	}
	// Until step 8 succeeds, any exit path must put the hub back exactly as it
	// was: still a fenced Standby, still armed, free to try again next tick.
	promoted := false
	defer func() {
		if !promoted {
			e.promoting.Store(false)
		}
	}()

	// A precondition rather than a guard, stated because it is otherwise
	// implicit: without a client to the Active there is no way to have observed
	// it alive, so there is nothing that could justify concluding it is gone.
	// WatchRemoteLease refuses to start without one and the arming rule cannot
	// be satisfied without one either, so this is unreachable from the loop —
	// but promote is a method, and a future caller (a forced-promotion path, for
	// instance) would otherwise crash inside the final dial rather than be told
	// no.
	if e.remoteClient == nil {
		return false, fmt.Errorf("cannot promote without a client to the active hub")
	}

	e.log.Infow("promotion sequence starting", "identity", e.identity, "lease", e.leaseName)

	// Step 2.
	if !e.guardsAllowPromotion(ctx) {
		return false, nil
	}

	// Step 3. This must complete before the write fence opens, and it is a real
	// dual-writer bug rather than a stylistic preference. The most common
	// trigger for a promotion is the Active's controller pod dying while its API
	// server stays perfectly healthy — in which case the mirror's informers are
	// still live and still mirroring at this exact moment. Every mirrored object
	// carries the syncer's own label, and the mirror's conflict guard only skips
	// objects WITHOUT it, so the mirror is entitled to overwrite precisely the
	// objects a promoted hub's reconcilers are about to write. Worse, prune's
	// reverse diff re-enqueues Active-side objects missing locally, so anything
	// the new Active legitimately deletes gets resurrected within one sync
	// interval. Opening the fence first means promoting into a hub that is
	// fighting itself.
	//
	// The wait is bounded, and that bound matters more than it looks. Waiting
	// indefinitely is the tempting choice, because proceeding without a stopped
	// mirror is exactly the dual-writer state above. But the watch loop calls
	// promote synchronously, so an unbounded wait on a mirror that never exits
	// blocks the loop: no further polls, no further staleness evaluation, no
	// failover ever, and nothing logged after the sequence started. Choosing
	// "never promote into a dual writer" that way silently buys "never promote
	// at all", which is strictly worse. Bounded, expiry aborts the attempt
	// loudly and the next tick retries — so a mirror that is merely slow costs
	// one tick, and a mirror that is genuinely stuck is visible instead of mute.
	if e.hooks.StopMirror != nil {
		stopCtx, cancel := context.WithTimeout(ctx, e.promotionGracePeriod)
		err := e.hooks.StopMirror(stopCtx)
		cancel()
		if err != nil {
			// Abort rather than continue. A mirror that has not confirmed it
			// stopped is the dual-writer state this step exists to prevent, so
			// the safe move is to stay a fenced Standby and retry next tick.
			return false, fmt.Errorf("stopping the state mirror before promotion: %w", err)
		}
		e.log.Infow("state mirror stopped and confirmed exited")
	}

	// Step 5. Note where the freshness judgement is NOT: acquireOrRenewLease
	// takes the Lease unconditionally, which stays correct here because this
	// Lease lives on the promoting hub's own cluster, where last-writer-wins is
	// the right rule. Judging whether the *other* hub is still alive is the
	// remote read's job, and it already happened in step 2. (#294 follow-up F3.)
	lease, err := acquireOrRenewLease(ctx, e.localClient, e.leaseName, e.leaseNS, e.identity, e.leaseDuration)
	if err != nil {
		return false, fmt.Errorf("acquiring the lease on this hub: %w", err)
	}
	e.lastRenew = time.Now()
	e.log.Infow("acquired lease on this hub", "lease", e.leaseName, "namespace", e.leaseNS)

	// Step 6.
	e.setMode(ModeActive)

	// Step 7. A budget, not a precondition: a hub that cannot publish is still a
	// better Active than no Active at all, and the publisher's own loop keeps
	// retrying afterwards. Failing here would strand the cluster with no writer.
	if e.hooks.PublishActiveController != nil {
		pubCtx, cancel := context.WithTimeout(ctx, e.promotionGracePeriod)
		err := e.hooks.PublishActiveController(pubCtx)
		cancel()
		if err != nil {
			e.log.Errorw("could not publish activeController within the promotion grace period; "+
				"continuing anyway, the publisher loop will retry",
				"error", err, "gracePeriod", e.promotionGracePeriod)
		} else {
			e.log.Infow("published activeController for the new Active")
		}
	}

	// Step 8. Steps 0 and 8 bracket everything above, so the fence stayed shut
	// for the entire sequence — which is what gives "step 7 completes before the
	// reconcilers are live" real teeth without inventing any external status
	// surface.
	e.promoting.Store(false)
	e.setLeader(true)
	promoted = true

	// Step 9. StartLeaseRenewal blocks until ctx is done, so it owns a goroutine.
	// It is safe to start only now: it checks the mode, which step 6 has set.
	go func() {
		if err := e.StartLeaseRenewal(ctx); err != nil {
			e.log.Errorw("lease renewal loop exited after promotion", "error", err)
		}
	}()

	// Step 10. Not optional. The fence drops reconcile requests rather than
	// requeuing them, so flipping it causes no reconcile at all: everything the
	// Standby ignored is gone, not parked, and nothing fires again until an
	// object changes or the informer resyncs. Mirrored objects carry no
	// finalizers until a reconciler re-adds them, so until this runs, deleting a
	// SliceConfig on the promoted hub skips deboarding entirely.
	//
	// Ordered after step 8 on purpose: a kick delivered while the fence was
	// still shut would be dropped without requeue, which is the exact failure it
	// exists to fix.
	//
	// Bounded like the steps before it, for a reason specific to how the kick
	// will be implemented: it pushes one event per object into a channel per
	// reconciled type, and those channels are only drained once the manager is
	// running. main.go starts this watch loop before mgr.Start, so a kick that
	// arrives in that window has nothing reading the other end. Unbounded, it
	// would block here forever on an already-promoted hub — leadership taken,
	// fence open, and the promotion never finishing or reporting itself.
	if e.hooks.KickReconcilers != nil {
		kickCtx, cancel := context.WithTimeout(ctx, e.promotionGracePeriod)
		err := e.hooks.KickReconcilers(kickCtx)
		cancel()
		if err != nil {
			e.log.Errorw("could not re-enqueue objects after promotion; pre-existing state may "+
				"stay unreconciled until the informer resync period", "error", err)
		} else {
			e.log.Infow("re-enqueued all reconciled types after promotion")
		}
	}

	// Step 11.
	haFailoverTotal.Inc()
	if e.hooks.EmitPromotedEvent != nil {
		eventCtx, cancel := context.WithTimeout(ctx, e.promotionGracePeriod)
		err := e.hooks.EmitPromotedEvent(eventCtx, lease)
		cancel()
		if err != nil {
			e.log.Errorw("could not emit PromotedToActive event", "error", err)
		}
	}
	e.log.Infow("PROMOTED to active", "identity", e.identity, "lease", e.leaseName)
	return true, nil
}
