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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// scenarioReconnect is issue #299 scenario 3 (worker reconnection),
// scoped to this repo's own half of it, per the issue's own "(controller
// focus)" title: the actual worker-side resolve/reconnect logic lives in
// worker-operator (#467/#468/#469, already done there). What this
// controller must prove after a promotion is that it is genuinely
// reconciling *new* objects, not just holding the lease — a fresh write is
// exactly what a reconnecting worker would produce.
func scenarioReconnect(t *testing.T, promoted *hub) {
	ctx := context.Background()
	const freshCluster = "e2e-worker-2"

	cluster := &controllerv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: freshCluster, Namespace: testProjectNS},
	}
	if err := promoted.client.Create(ctx, cluster); err != nil {
		t.Fatalf("creating a fresh Cluster CR on the promoted hub: %v", err)
	}

	// SecretName is what ClusterService.ReconcileCluster actually sets once
	// it has created (or found) this cluster's ServiceAccount and minted its
	// token Secret — the controller's own, always-set signal that it
	// genuinely reconciled a new object, unlike RegistrationStatus, which is
	// only ever written by a real worker's own registration flow (not
	// present in this controller-focused fixture).
	waitFor(t, "the promoted hub reconciles the fresh Cluster CR", 60*time.Second, func() (bool, error) {
		got := &controllerv1alpha1.Cluster{}
		if err := promoted.client.Get(ctx, types.NamespacedName{Namespace: testProjectNS, Name: freshCluster}, got); err != nil {
			return false, err
		}
		return got.Status.SecretName != "", nil
	})

	identity, err := activeControllerIdentity(ctx, promoted.client, testProjectNS, testClusterCR)
	if assert.NoError(t, err) {
		assert.Equal(t, promoted.identity, identity,
			"the promoted hub must still name itself as the active controller after handling new work")
	}
}
