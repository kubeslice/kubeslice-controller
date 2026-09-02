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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResumeAsActive answers a question --ha-mode alone cannot: is this hub,
// configured as a Standby, already the Active?
//
// Promotion is a runtime transition. When a Standby promotes it takes the HA
// Lease in its own cluster and opens its write fence, but nothing rewrites its
// Deployment — the flag still reads standby, and main.go reads that flag only at
// start-up. So restarting a promoted hub (a node reboot, an eviction, an image
// bump) brings it back as a Standby. If the hub it was promoted away from is
// also configured as a Standby — which is exactly what the runbook's restore
// procedure leaves behind — the pair then has *no* Active: each mirrors from the
// other, overwriting as it goes, until one's padding expires and it promotes.
// The state does converge, but through a window of mutual overwriting and with
// an arbitrary winner.
//
// No new state is needed to fix that, because the Lease this hub already holds
// is itself the durable record of the promotion. Two conditions must both hold:
//
//   - The Lease in this cluster names this identity. Only this hub ever writes
//     that Lease, so finding its own name there means this hub promoted and was
//     not deliberately demoted — an intentional demotion deletes the Lease.
//   - The Lease is not stale, by the same isLeaseStale test a Standby applies to
//     the Active's Lease. A Lease this hub stopped renewing for long enough that
//     the other hub could have taken over reads as stale here too, and this hub
//     then defers rather than risk becoming a second Active.
//
// Both true means the restart was short enough that no takeover can have
// happened, so resuming as Active is both correct and safe. Anything else
// returns false and the configured mode stands. That asymmetry is deliberate:
// split brain is an explicit non-goal of ADR #293 Decision 8, so this resolves
// only the unambiguous case and never guesses.
//
// The returned string is a human-readable reason, meant for the start-up log in
// both the true and false cases — an operator reading "started as standby"
// should be able to see why, without reconstructing the Lease state by hand.
func ResumeAsActive(ctx context.Context, local client.Client, opts Options) (bool, string, error) {
	opts = applyDefaults(opts)

	if opts.Mode != ModeStandby {
		return false, fmt.Sprintf("configured mode is %s, not standby", opts.Mode), nil
	}

	lease, err := getLease(ctx, local, opts.LeaseName, opts.LeaseNamespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, "this cluster holds no HA lease, so this hub has never promoted", nil
		}
		// Deliberately an error rather than a false: an unreadable Lease is not
		// evidence of anything, and the caller logs it and continues in the
		// configured mode. Treating a read failure as "not the Active" would
		// silently demote a promoted hub on a transient API blip.
		return false, "", fmt.Errorf("reading local HA lease %s/%s: %w", opts.LeaseNamespace, opts.LeaseName, err)
	}

	holder := ""
	if lease.Spec.HolderIdentity != nil {
		holder = *lease.Spec.HolderIdentity
	}
	if holder != opts.Identity {
		return false, fmt.Sprintf("the HA lease is held by %q, not by this hub (%q)", holder, opts.Identity), nil
	}

	if isLeaseStale(lease, opts.PaddingSeconds, time.Now()) {
		return false, "this hub's own HA lease is stale, so the other hub may already have taken over", nil
	}

	return true, fmt.Sprintf("this hub already holds a live HA lease as %q", opts.Identity), nil
}
