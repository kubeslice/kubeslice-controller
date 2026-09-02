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
	"strings"
	"testing"
	"time"
)

// gw returns the fully-qualified gateway object name for a source→dest pair.
func gw(source, dest string) string { return sliceName + "-worker-" + source + "-worker-" + dest }

// gatewayRow returns the (host, route) columns for a gateway, or ("","") if absent.
func gatewayRow(t *testing.T, name string) (host, route string) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(gatewayColumns(t)), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[0] == name {
			return f[1], f[2]
		}
	}
	return "", ""
}

func assertGateway(t *testing.T, name, wantHost, wantRoute string) {
	t.Helper()
	host, route := gatewayRow(t, name)
	if host == "" {
		t.Fatalf("gateway %s does not exist (expected host=%s route=%s)", name, wantHost, wantRoute)
	}
	if host != wantHost || route != wantRoute {
		t.Fatalf("gateway %s: got host=%s route=%s, want host=%s route=%s", name, host, route, wantHost, wantRoute)
	}
}

func assertNoGateway(t *testing.T, name string) {
	t.Helper()
	if host, _ := gatewayRow(t, name); host != "" {
		t.Fatalf("gateway %s exists but should not", name)
	}
}

// serversWithRoute counts gateways that are Server-side AND carry route=true —
// which must always be zero (a hub/server never routes the whole slice).
func serversWithRoute(t *testing.T) int {
	t.Helper()
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(gatewayColumns(t)), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[1] == "Server" && f[2] == "true" {
			n++
		}
	}
	return n
}

// scenarioPartialMesh applies a HubAndSpoke slice (hub=worker-1) and asserts the
// controller builds only the two hub↔spoke pairs, with the spoke side flagged
// and no spoke↔spoke gateway.
func scenarioPartialMesh(t *testing.T) {
	applyYAML(t, sliceManifest("topology: {mode: HubAndSpoke, hubs: [worker-1]}"))
	waitForGatewayCount(t, 4)
	assertGateway(t, gw("1", "2"), "Server", "<none>")
	assertGateway(t, gw("1", "3"), "Server", "<none>")
	assertGateway(t, gw("2", "1"), "Client", "true")
	assertGateway(t, gw("3", "1"), "Client", "true")
	assertNoGateway(t, gw("2", "3"))
	assertNoGateway(t, gw("3", "2"))
}

// scenarioFullMeshUnaffected switches the slice to full mesh and asserts every
// pair is built with the flag off — the backward-compatibility guarantee.
func scenarioFullMeshUnaffected(t *testing.T) {
	run(t, "", "kubectl", "--context", kubeContext, "patch", "sliceconfig", sliceName,
		"-n", projectNS, "--type=json", "-p", `[{"op":"remove","path":"/spec/topology"}]`)
	waitForGatewayCount(t, 6)
	if n := serversWithRoute(t); n != 0 {
		t.Fatalf("full mesh: %d server gateways carry route=true, want 0", n)
	}
	// no gateway of any role should carry the flag in full mesh
	for _, line := range strings.Split(strings.TrimSpace(gatewayColumns(t)), "\n") {
		if f := strings.Fields(line); len(f) == 3 && f[2] == "true" {
			t.Fatalf("full mesh: gateway %s unexpectedly carries route=true", f[0])
		}
	}
}

// scenarioTopologyChangeReconcilesFlag switches back to HubAndSpoke and asserts
// the surviving spoke gateways get RouteEntireSliceSubnet reconciled to true
// (a FullMesh→HubAndSpoke switch must not leave the flag stale-false).
func scenarioTopologyChangeReconcilesFlag(t *testing.T) {
	run(t, "", "kubectl", "--context", kubeContext, "patch", "sliceconfig", sliceName,
		"-n", projectNS, "--type=merge",
		"-p", `{"spec":{"topology":{"mode":"HubAndSpoke","hubs":["worker-1"]}}}`)
	waitForGatewayCount(t, 4)
	assertGateway(t, gw("2", "1"), "Client", "true")
	assertGateway(t, gw("3", "1"), "Client", "true")
	assertNoGateway(t, gw("2", "3"))
}

// scenarioHubChangeNoStaleFlag does a hub-change round-trip (worker-1 → worker-2
// → worker-1) and asserts no server gateway is left carrying route=true — the
// regression fixed by reconciling the flag on both sides of an existing pair.
func scenarioHubChangeNoStaleFlag(t *testing.T) {
	run(t, "", "kubectl", "--context", kubeContext, "patch", "sliceconfig", sliceName,
		"-n", projectNS, "--type=merge",
		"-p", `{"spec":{"topology":{"mode":"HubAndSpoke","hubs":["worker-2"]}}}`)
	// Wait for the hub=worker-2 topology to settle before switching back, rather
	// than sleeping a fixed interval: worker-1 is now a spoke, so its client side
	// must carry the flag and worker-2's side must be a plain Server.
	waitFor(t, "topology settles to hub=worker-2 before switching back", 60*time.Second, func() bool {
		if gatewayCount(t) != 4 {
			return false
		}
		host12, route12 := gatewayRow(t, gw("1", "2"))
		host21, route21 := gatewayRow(t, gw("2", "1"))
		return host12 == "Client" && route12 == "true" &&
			host21 == "Server" && route21 == "<none>"
	})
	run(t, "", "kubectl", "--context", kubeContext, "patch", "sliceconfig", sliceName,
		"-n", projectNS, "--type=merge",
		"-p", `{"spec":{"topology":{"mode":"HubAndSpoke","hubs":["worker-1"]}}}`)
	// give the reconciler time to settle back to hub=worker-1
	waitFor(t, "hub-change round-trip leaves no server with route=true", 60*time.Second, func() bool {
		return gatewayCount(t) == 4 && serversWithRoute(t) == 0
	})
	assertGateway(t, gw("1", "2"), "Server", "<none>")
	assertGateway(t, gw("2", "1"), "Client", "true")
}

// scenarioWebhookRejectsInvalid asserts the admission webhook (and CRD schema)
// reject every malformed topology.
func scenarioWebhookRejectsInvalid(t *testing.T) {
	cases := []struct {
		name     string
		topology string
	}{
		{"two hubs", "topology: {mode: HubAndSpoke, hubs: [worker-1, worker-2]}"},
		{"hub not a member", "topology: {mode: HubAndSpoke, hubs: [worker-9]}"},
		{"no hubs", "topology: {mode: HubAndSpoke, hubs: []}"},
		{"hubs without mode", "topology: {hubs: [worker-1]}"},
		{"FullMesh with hubs", "topology: {mode: FullMesh, hubs: [worker-1]}"},
		{"unknown mode", "topology: {mode: Banana, hubs: [worker-1]}"},
		{"duplicate hubs", "topology: {mode: HubAndSpoke, hubs: [worker-1, worker-1]}"},
	}
	for _, c := range cases {
		// use a distinct name per case so a stray accepted object is obvious
		manifest := strings.Replace(sliceManifest(c.topology), sliceName, sliceName+"-bad", 1)
		out, accepted := tryApplyYAML(manifest)
		if accepted {
			// clean up the wrongly-accepted object before failing
			_, _ = tryRun("kubectl", "", "--context", kubeContext, "delete", "sliceconfig", sliceName+"-bad", "-n", projectNS)
			t.Fatalf("invalid topology %q was accepted, expected rejection", c.name)
		}
		lowerOut := strings.ToLower(out)
		if !strings.Contains(lowerOut, "invalid") && !strings.Contains(lowerOut, "unsupported") && !strings.Contains(lowerOut, "too many") {
			t.Fatalf("invalid topology %q rejected without a clear error:\n%s", c.name, out)
		}
	}
}

// scenarioStatusFieldsPersist patches the #471 connection-status fields on a
// gateway and asserts they survive (before #471 the CRD lacked them and the API
// server pruned them).
func scenarioStatusFieldsPersist(t *testing.T) {
	name := gw("2", "1")
	run(t, "", "kubectl", "--context", kubeContext, "patch", "workerslicegateway", name,
		"-n", projectNS, "--subresource=status", "--type=merge",
		"-p", `{"status":{"connectionState":"Connected","reason":"TunnelEstablished","message":"up","lastTransitionTime":"2026-01-01T00:00:00Z"}}`)
	got := strings.TrimSpace(kubectl(t, "get", "workerslicegateway", name, "-n", projectNS,
		"-o", `jsonpath={.status.connectionState}|{.status.reason}|{.status.message}`))
	want := "Connected|TunnelEstablished|up"
	if got != want {
		t.Fatalf("status fields did not persist: got %q, want %q", got, want)
	}
}
