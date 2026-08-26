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
	"fmt"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubeslice/kubeslice-controller/pkg/ha"
)

// detectionBudget and promotionCeiling mirror the formula mined from the
// proven external e2e suite (~/Projects/lfx/e2e/40-failover.sh):
// BUDGET = leaseDuration + padding; CEILING = BUDGET + promotionGrace. None
// of these flags are overridden per-hub, so the package defaults apply.
func detectionBudget() time.Duration {
	return ha.DefaultLeaseDuration + ha.DefaultPaddingSeconds
}

func promotionCeiling() time.Duration {
	return detectionBudget() + ha.DefaultPromotionGracePeriod
}

// hub is one simulated hub cluster: a real, disposable Kind cluster running
// this branch's controller-manager image.
type hub struct {
	name       string // without the kindClusterPrefix
	identity   string // --ha-identity
	kubeconfig string
	restConfig *rest.Config
	clientset  kubernetes.Interface
	client     ctrlclient.Client
}

// The Cluster CR every scenario mirrors/promotes around. Kept minimal —
// this suite is controller-focused (issue #299's own title), so it never
// installs a real worker-operator, NSM, or cert-manager; ENABLE_WEBHOOKS is
// off and no worker actually registers itself.
const testClusterCR = "e2e-worker-1"

// newHubFixture creates the Active/Standby pair once for the whole test,
// wires the cross-cluster HA credential, and registers guaranteed teardown.
// Ordering mirrors main.go's own startup expectations: the Active must
// exist and be holding its lease before the Standby's remote client can
// read anything meaningful from it.
func newHubFixture(t *testing.T) (active, standby *hub) {
	t.Helper()
	ctx := context.Background()
	image := buildControllerImage(t)

	active = newHubCluster(t, "hub-active", "active-hub-1")
	kindLoadImage(t, active.name, image)
	deployManager(ctx, t, active.clientset, hubConfig{
		Image: image,
		Args: []string{
			"--leader-elect=false", // this repo's OWN --leader-elect Lease is unrelated to HA; one replica needs none of it
			"--ha-mode=active",
			"--ha-identity=" + active.identity,
			// Must be set to anything other than the shipped placeholder
			// (service.ControllerEndpoint) or the active-publisher refuses
			// to write status.activeController at all.
			"--controller-end-point=" + controlPlaneAddress(active.name),
		},
	})
	waitFor(t, "active hub holds its HA lease", 60*time.Second, func() (bool, error) {
		return leaseHolder(ctx, active.clientset) == active.identity, nil
	})

	standby = newHubCluster(t, "hub-standby", "standby-hub-1")
	kindLoadImage(t, standby.name, image)
	buildActiveKubeconfigSecret(ctx, t, active.kubeconfig, active.name, standby.clientset)
	deployManager(ctx, t, standby.clientset, hubConfig{
		Image: image,
		Args: []string{
			"--leader-elect=false",
			"--ha-mode=standby",
			"--ha-identity=" + standby.identity,
			"--ha-active-kubeconfig=" + haActiveKubeconfigPath,
			"--controller-end-point=" + controlPlaneAddress(standby.name),
		},
		ActiveKubeconfigSecret: haActiveSecretName,
	})
	waitFor(t, "standby is armed and watching the active hub lease", 60*time.Second, func() (bool, error) {
		return strings.Contains(managerLogs(ctx, t, standby.clientset), "watching active hub lease"), nil
	})

	// A minimal Cluster CR for the mirror/promotion scenarios to act on —
	// standing in for what a real worker's registration flow would create.
	createTestClusterCR(ctx, t, active.client)

	return active, standby
}

func newHubCluster(t *testing.T, name, identity string) *hub {
	t.Helper()
	kindCreateCluster(t, name)
	kubeconfig := kindKubeconfigPath(t, name)
	cfg := kindRESTConfig(t, kubeconfig)
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("building clientset for %s: %v", name, err)
	}
	applyCRDs(t, kubeconfig)
	createControllerNamespace(context.Background(), t, clientset)
	applyRBAC(t, kubeconfig) // creates objects in controllerNamespace; must run after it exists
	return &hub{
		name:       name,
		identity:   identity,
		kubeconfig: kubeconfig,
		restConfig: cfg,
		clientset:  clientset,
		client:     newControllerRuntimeClient(t, cfg),
	}
}

func TestActiveStandbyHA(t *testing.T) {
	active, standby := newHubFixture(t)

	t.Run("BaselineSync", func(t *testing.T) {
		scenarioBaseline(t, active, standby)
	})
	t.Run("TransientBlipDoesNotPromote", func(t *testing.T) {
		scenarioBlip(t, active, standby)
	})
	t.Run("FailoverPromotion", func(t *testing.T) {
		scenarioFailover(t, active, standby)
	})
	t.Run("ReconciliationResumesOnThePromotedHub", func(t *testing.T) {
		// standby is now the Active after the previous subtest.
		scenarioReconnect(t, standby)
	})
}

func buildControllerImage(t *testing.T) string {
	t.Helper()
	tag := fmt.Sprintf("kubeslice-controller-e2e:%d", time.Now().Unix())
	runKind(t, "docker", "build", "--platform", "linux/amd64", "-t", tag, repoRoot(t))
	return tag
}
