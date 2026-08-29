//go:build e2e

/*
 *  Copyright (c) 2026 Avesha, Inc. All rights reserved.
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
	"testing"
	"time"
)

// TestHubAndSpokeE2E stands up one disposable Kind cluster with this branch's
// controller, three fake registered workers, and runs the Hub-and-Spoke
// control-plane scenarios against it in order. The heavy setup (image build +
// cluster + controller deploy) happens once; each scenario is a subtest.
func TestHubAndSpokeE2E(t *testing.T) {
	image := buildControllerImage(t)
	createCluster(t)
	deployController(t, image)
	setupProjectAndWorkers(t)

	// Baseline must run first: it applies the slice the later scenarios mutate.
	t.Run("PartialMesh_HubAndSpokeSkipsSpokeToSpoke", scenarioPartialMesh)
	t.Run("FullMesh_Unaffected", scenarioFullMeshUnaffected)
	t.Run("TopologyChange_ReconcilesFlag", scenarioTopologyChangeReconcilesFlag)
	t.Run("HubChange_NoStaleServerFlag", scenarioHubChangeNoStaleFlag)
	t.Run("Webhook_RejectsInvalidTopologies", scenarioWebhookRejectsInvalid)
	t.Run("StatusFields_Persist", scenarioStatusFieldsPersist)
}

// sliceManifest builds a SliceConfig manifest with the given topology block.
// topology is the YAML for the spec.topology field (or "" to omit it).
func sliceManifest(topology string) string {
	m := `
apiVersion: controller.kubeslice.io/v1alpha1
kind: SliceConfig
metadata: {name: ` + sliceName + `, namespace: ` + projectNS + `}
spec:
  sliceType: Application
  sliceSubnet: 10.11.0.0/16
  sliceGatewayProvider: {sliceGatewayType: OpenVPN, sliceCaType: Local}
  sliceIpamType: Local
  clusters: [worker-1, worker-2, worker-3]
`
	if topology != "" {
		m += "  " + topology + "\n"
	}
	m += `  qosProfileDetails: {queueType: HTB, priority: 1, tcType: BANDWIDTH_CONTROL, bandwidthCeilingKbps: 5120, bandwidthGuaranteedKbps: 2560, dscpClass: AF11}
  namespaceIsolationProfile: {isolationEnabled: false}`
	return m
}

// waitForGatewayCount blocks until exactly n slice gateways exist.
func waitForGatewayCount(t *testing.T, n int) {
	t.Helper()
	waitFor(t, "slice has exactly gateways", 60*time.Second, func() bool {
		return gatewayCount(t) == n
	})
}
