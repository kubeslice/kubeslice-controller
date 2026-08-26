//go:build e2e

/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const standbyReaderBindingName = "kubeslice-ha-standby-reader-binding-e2e"

// scenarioBlip is issue #299 scenario 4: a transient interruption in the
// standby's ability to read the active hub's Lease must not cause a
// promotion. This has zero prior art in the proven external suite (mined
// and confirmed absent) — designed fresh here.
//
// Mechanism: revoke the ClusterRoleBinding behind the standby's
// --ha-active-kubeconfig identity on the Active for a window comfortably
// SHORTER than the detection budget (leaseDuration+padding), then restore
// it. A window this short never lets the standby's cached view of the
// lease actually cross the staleness threshold, so no promotion is even
// considered — the correct, and considerably more deterministic, way to
// prove a sub-budget blip is absorbed than trying to race the guard's
// internal final-dial check, which fires and resolves within a single
// reconcile tick and isn't reliably steerable from outside.
func scenarioBlip(t *testing.T, active, standby *hub) {
	ctx := context.Background()

	blip := detectionBudget() / 2
	revokeStandbyReaderBinding(t, active.kubeconfig)
	t.Logf("blocked the standby's remote-read RBAC for %s (half the %s detection budget)", blip, detectionBudget())
	time.Sleep(blip)
	restoreStandbyReaderBinding(t, active.kubeconfig)

	// Positive control: the standby must have actually been trying and
	// failing during the window, not just sitting idle — an unproven
	// "nothing happened" is vacuous (this project's own established rule).
	assertLogContains(t, managerLogs(ctx, t, standby.clientset),
		"could not read active hub lease; retaining last known view")

	// Give it one more retry period past restoration to resettle, then
	// assert nothing that shouldn't have happened, happened.
	time.Sleep(4 * time.Second)
	logs := managerLogs(ctx, t, standby.clientset)
	assertLogNeverContains(t, logs, "PROMOTED to active")
	assertLogNeverContains(t, logs, "promotion sequence starting")
	assert.Equal(t, active.identity, leaseHolder(ctx, active.clientset),
		"the active hub must still hold the lease after a sub-budget blip")
	assert.Empty(t, leaseHolder(ctx, standby.clientset),
		"the standby must not have promoted itself")
}

func revokeStandbyReaderBinding(t *testing.T, activeKubeconfig string) {
	t.Helper()
	runKind(t, "kubectl", "--kubeconfig", activeKubeconfig, "delete",
		"clusterrolebinding", standbyReaderBindingName)
}

func restoreStandbyReaderBinding(t *testing.T, activeKubeconfig string) {
	t.Helper()
	bindStandbyReaderRole(t, activeKubeconfig, standbyReaderNamespace, standbyReaderSA)
}
