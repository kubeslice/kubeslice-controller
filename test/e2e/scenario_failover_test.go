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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// scenarioFailover is issue #299 scenario 2: the Active fails, the Standby
// promotes. Ported from the proven external suite's 40-failover.sh: the
// exact ordered log sequence, the mirror-before-lease-acquire ordering
// check, the timing ceiling, and the Event check (using the real .reason
// field, PromotedToActive — verified against pkg/ha/promotion_event.go,
// not the bash suite's eventTitle-label grep).
func scenarioFailover(t *testing.T, active, standby *hub) {
	ctx := context.Background()

	t0 := time.Now()
	if err := active.clientset.AppsV1().Deployments(controllerNamespace).
		Delete(ctx, managerName, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("killing the active hub's manager: %v", err)
	}

	ceiling := promotionCeiling() + 10*time.Second // matching bash suite's own generous ceiling padding
	waitFor(t, "the standby promotes to active", ceiling, func() (bool, error) {
		return strings.Contains(managerLogs(ctx, t, standby.clientset), "PROMOTED to active"), nil
	})
	elapsed := time.Since(t0)
	assert.LessOrEqual(t, elapsed, ceiling, "promotion took longer than budget %s", ceiling)
	t.Logf("promotion completed in %s (ceiling %s)", elapsed, ceiling)

	logs := managerLogs(ctx, t, standby.clientset)
	assertLogNeverContains(t, logs, "promotion aborted")
	assertLogSequence(t, logs,
		"active hub lease is STALE; evaluating promotion",
		"promotion sequence starting",
		"state mirror stopped and confirmed exited",
		"acquired lease on this hub",
		"published activeController for the new Active",
		"re-enqueued all reconciled types after promotion",
		"PROMOTED to active",
	)

	assert.Equal(t, standby.identity, leaseHolder(ctx, standby.clientset),
		"the standby must now hold the HA lease")

	identity, err := activeControllerIdentity(ctx, standby.client, testProjectNS, testClusterCR)
	if assert.NoError(t, err) {
		assert.Equal(t, standby.identity, identity,
			"the promoted hub must publish itself as the active controller")
	}

	assert.True(t, eventReasonExists(ctx, t, standby.clientset, "PromotedToActive"),
		"a PromotedToActive event must be recorded on the promoted hub")
}
