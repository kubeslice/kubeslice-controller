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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// The guards answer a different question from the staleness verdict. The
// verdict says "the Active's newest proof of life has aged out". The guards ask
// "is that actually evidence the Active is gone, or evidence of something else?"
//
// They exist because the costs are not symmetric. A missed promotion is
// downtime: visible, and an operator can force a takeover. A false promotion is
// two hubs writing to their own copies of the world at once, silently, with the
// mirror still running in one direction — objects overwriting each other,
// prune's reverse diff resurrecting deletes, workers receiving contradictory
// instructions, and recovery by hand. Two bounded reads to avoid the second is
// a good trade inside a budget that already tolerates leaseDuration + padding.

// selfHealthy reports whether this hub can reach its own API server.
//
// This is the only cheap way to tell "the Active is gone" apart from "my own
// networking is broken". A dead Active API server, a partition between the
// hubs, and this hub losing its own network all produce byte-identical
// observations from here: reads of the remote Lease simply stop succeeding.
// Asking whether the local API server still answers is what separates the last
// case from the first two — and it is the most likely cause of a false
// promotion after outright misconfiguration.
//
// It reads this hub's own Lease through the existing local client, so it needs
// no new client and no new RBAC: config/rbac/leader_election_role.yaml already
// grants leases in the controller's own namespace.
//
// NotFound counts as HEALTHY, and getting this backwards would block every
// real first failover. On a first-ever promotion no Lease exists on this hub
// yet, and a NotFound response means the API server answered — which is
// precisely the thing being tested. Only transport errors, timeouts and server
// errors indicate an unhealthy self.
func (e *ClusterLeaderElector) selfHealthy(ctx context.Context) bool {
	selfCtx, cancel := context.WithTimeout(ctx, e.promotionDialTimeout)
	defer cancel()

	_, err := getLease(selfCtx, e.localClient, e.leaseName, e.leaseNS)
	if err == nil || apierrors.IsNotFound(err) {
		return true
	}
	e.log.Errorw("refusing to promote: this hub cannot reach its own API server",
		"error", err, "timeout", e.promotionDialTimeout)
	return false
}

// activeStillAlive performs the final dial: one fresh, timeout-bounded read of
// the Active's Lease at decision time. It reports true only if that read
// succeeds AND returns a Lease that is not stale — i.e. the Active is
// demonstrably still holding leadership and promotion must be abandoned.
//
// What this buys, honestly, differs per path:
//
//   - Active reachable but its renewTime frozen: real value. Polling happens
//     every retryPeriod, so the Active could have renewed moments after the
//     last poll. One fresh read at decision time closes that race.
//   - Active unreachable: almost nothing. It is the next failed read after a
//     sustained run of failed reads, and it only catches an outage that ended
//     within the last tick.
//
// It is NOT a split-brain guard, and nothing in this codebase should imply
// otherwise. In a genuine sustained partition this read travels the same broken
// path as every other read, fails in the same way, and the Standby promotes
// regardless. What actually provides safety on the unreachable path is
// duration — a sustained failure across the whole leaseDuration + padding
// budget rather than one bad read — and the arming rule. Split-brain remains
// the explicit non-goal of ADR #293 Decision 8.
//
// The bound is not optional. main.go builds the remote client with a plain
// uncached client.New and no timeout, so a dial to a black-holed API server
// (packets dropped, no RST) blocks until the OS TCP timeout — minutes, far
// outside the failover budget. Every call here is wrapped.
func (e *ClusterLeaderElector) activeStillAlive(ctx context.Context) bool {
	dialCtx, cancel := context.WithTimeout(ctx, e.promotionDialTimeout)
	defer cancel()

	lease, err := getLease(dialCtx, e.remoteClient, e.leaseName, e.leaseNS)
	if err != nil {
		e.log.Infow("final dial to the active hub failed; treating it as gone",
			"error", err, "timeout", e.promotionDialTimeout)
		return false
	}
	if isLeaseStale(lease, e.padding, time.Now()) {
		e.log.Infow("final dial reached the active hub but its lease is still stale; proceeding",
			"holder", leaseHolder(lease), "renewTime", leaseRenewStr(lease))
		return false
	}

	// The Active renewed between our last poll and now. Refresh the cached view
	// so the next tick evaluates against this newer evidence rather than
	// re-deriving the same stale verdict.
	e.lastSeenLease = lease
	e.lastGoodRead = time.Now()
	e.log.Infow("refusing to promote: the active hub renewed its lease between polls",
		"holder", leaseHolder(lease), "renewTime", leaseRenewStr(lease))
	return true
}

// guardsAllowPromotion runs both guards in order and records why it refused.
// Aborting deliberately changes nothing else: the cached Lease is not cleared
// and the elector is not disarmed, so the next tick re-evaluates from scratch
// and a genuinely recovered Active heals the state naturally on its next
// successful read.
func (e *ClusterLeaderElector) guardsAllowPromotion(ctx context.Context) bool {
	if !e.selfHealthy(ctx) {
		haPromotionsAbortedTotal.WithLabelValues(abortSelfUnhealthy).Inc()
		return false
	}
	if e.activeStillAlive(ctx) {
		haPromotionsAbortedTotal.WithLabelValues(abortLeaseLive).Inc()
		return false
	}
	return true
}
