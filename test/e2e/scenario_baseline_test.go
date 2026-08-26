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
	"k8s.io/apimachinery/pkg/types"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// scenarioBaseline is issue #299 scenario 1: normal operation, CRD sync
// Active -> Standby. Ported from the proven external suite's
// 20-baseline.sh assertions.
func scenarioBaseline(t *testing.T, active, standby *hub) {
	ctx := context.Background()

	assert.Equal(t, active.identity, leaseHolder(ctx, active.clientset),
		"the active hub must hold its own HA lease")
	assert.Empty(t, leaseHolder(ctx, standby.clientset),
		"the standby must hold no live lease of its own")

	// The active-publisher writes on its own periodic interval (30s
	// default), not synchronously when the Cluster CR is created, so this
	// has to be a wait, not a one-shot check.
	waitFor(t, "the active hub publishes itself as the active controller", 45*time.Second, func() (bool, error) {
		identity, err := activeControllerIdentity(ctx, active.client, testProjectNS, testClusterCR)
		return identity == active.identity, err
	})

	waitFor(t, "the mirror copies the Cluster CR to the standby", 90*time.Second, func() (bool, error) {
		mirrored := &controllerv1alpha1.Cluster{}
		if err := standby.client.Get(ctx, types.NamespacedName{Namespace: testProjectNS, Name: testClusterCR}, mirrored); err != nil {
			return false, err
		}
		return mirrored.Labels["ha.kubeslice.io/synced-from"] == "active", nil
	})

	waitFor(t, "the standby's mirrored Cluster CR names the Active", 45*time.Second, func() (bool, error) {
		identity, err := activeControllerIdentity(ctx, standby.client, testProjectNS, testClusterCR)
		return identity == active.identity, err
	})

	// Positive control (this project's own established rule: an unproven
	// convergence is vacuous) — touch the object on the Active and require
	// the change to arrive on the Standby, proving the mirror is live, not
	// just correct from a one-time initial copy.
	probeValue := time.Now().Format(time.RFC3339Nano)
	waitFor(t, "the mirror is live (positive control)", 90*time.Second, func() (bool, error) {
		live := &controllerv1alpha1.Cluster{}
		if err := active.client.Get(ctx, types.NamespacedName{Namespace: testProjectNS, Name: testClusterCR}, live); err != nil {
			return false, err
		}
		if live.Annotations == nil {
			live.Annotations = map[string]string{}
		}
		live.Annotations["e2e-probe"] = probeValue
		if err := active.client.Update(ctx, live); err != nil {
			return false, err
		}
		mirrored := &controllerv1alpha1.Cluster{}
		if err := standby.client.Get(ctx, types.NamespacedName{Namespace: testProjectNS, Name: testClusterCR}, mirrored); err != nil {
			return false, err
		}
		return mirrored.Annotations["e2e-probe"] == probeValue, nil
	})

	assertLogContains(t, managerLogs(ctx, t, standby.clientset), "watching active hub lease")
}
