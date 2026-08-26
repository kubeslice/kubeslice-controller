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

// Package e2e drives the Hub-and-Spoke (partial mesh) control-plane against a
// real, disposable Kind cluster running this branch's controller image.
//
// Scope is deliberately controller-focused: it runs the real controller
// (reconcilers + admission webhooks + CRDs) but stands in fake, pre-registered
// worker Cluster CRs instead of installing real worker-operators, NSM, or
// gateway pods. That makes the suite fast and deterministic and covers exactly
// where this feature's code lives — topology resolution, gateway edge creation,
// the RouteEntireSliceSubnet flag, and topology webhook validation. The
// dataplane (real tunnels, spoke-to-spoke traffic) is exercised by the manual
// runbook in docs/hub-and-spoke-testing.md, not here.
//
// The cluster name is prefixed e2e-hns- and created/destroyed within a single
// run, so it never touches a developer's own Kind clusters. Everything shells
// out to the kind/docker/kubectl/make CLIs.
package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	kindClusterName = "e2e-hns"
	kubeContext     = "kind-" + kindClusterName
	controllerNS    = "kubeslice-controller"
	projectNS       = "kubeslice-avesha"
	sliceName       = "e2e-hns-slice"
)

// run executes a command, failing the test with combined output on error so a
// failure always shows what actually happened, not just "exit status 1".
func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out.String())
	}
	return out.String()
}

// tryRun is like run but returns the combined output and success flag instead
// of failing the test — used for admission-webhook rejection checks, where a
// non-zero exit is the expected outcome.
func tryRun(name string, stdin string, args ...string) (string, bool) {
	cmd := exec.Command(name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err == nil
}

// kubectl runs kubectl against the e2e cluster.
func kubectl(t *testing.T, args ...string) string {
	t.Helper()
	return run(t, "", "kubectl", append([]string{"--context", kubeContext}, args...)...)
}

// applyYAML pipes a manifest to `kubectl apply -f -`, failing on error.
func applyYAML(t *testing.T, yaml string) {
	t.Helper()
	cmd := exec.Command("kubectl", "--context", kubeContext, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("apply failed: %v\n%s\n---manifest---\n%s", err, out.String(), yaml)
	}
}

// tryApplyYAML pipes a manifest to apply and reports (output, accepted). Used
// for the invalid-topology webhook cases where rejection is expected.
func tryApplyYAML(yaml string) (string, bool) {
	return tryRun("kubectl", yaml, "--context", kubeContext, "apply", "-f", "-")
}

// waitFor polls until cond returns true or the timeout elapses.
func waitFor(t *testing.T, desc string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(3 * time.Second)
	}
	t.Fatalf("timed out after %s waiting for: %s", timeout, desc)
}

// repoRoot returns the controller repo root (two levels up from test/e2e).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// buildControllerImage builds this branch's controller image and returns its tag.
func buildControllerImage(t *testing.T) string {
	t.Helper()
	tag := fmt.Sprintf("kubeslice-controller-e2e:%d", time.Now().Unix())
	run(t, repoRoot(t), "docker", "build", "-t", tag, ".")
	return tag
}

// createCluster creates the disposable Kind cluster and registers teardown so
// every exit path (including t.Fatal in a later step) deletes it.
func createCluster(t *testing.T) {
	t.Helper()
	// clean any leftover from a previously killed run, then create fresh
	_ = exec.Command("kind", "delete", "cluster", "--name", kindClusterName).Run()
	run(t, "", "kind", "create", "cluster", "--name", kindClusterName, "--image", "kindest/node:v1.29.2")
	t.Cleanup(func() {
		_ = exec.Command("kind", "delete", "cluster", "--name", kindClusterName).Run()
	})
}

// deployController installs cert-manager, loads the image, deploys the
// controller via `make deploy`, applies the two local-dev patches, and waits
// for the manager to be ready.
func deployController(t *testing.T, image string) {
	t.Helper()
	root := repoRoot(t)

	run(t, "", "kubectl", "--context", kubeContext, "apply", "-f",
		"https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml")
	run(t, "", "kubectl", "--context", kubeContext, "wait", "--for=condition=Available",
		"deployment", "--all", "-n", "cert-manager", "--timeout=180s")

	run(t, "", "kind", "load", "docker-image", image, "--name", kindClusterName)

	// make deploy has no --context flag; it targets the current context.
	run(t, "", "kubectl", "config", "use-context", kubeContext)
	run(t, root, "make", "deploy", "IMG="+image)

	// local-dev patches (see docs/kubeslice-local-dev-setup): a reachable
	// kube-rbac-proxy image and configmaps RBAC the reconciler needs.
	_ = exec.Command("kubectl", "--context", kubeContext, "set", "image",
		"deployment/kubeslice-controller-manager",
		"kube-rbac-proxy=quay.io/brancz/kube-rbac-proxy:v0.8.0", "-n", controllerNS).Run()
	_ = exec.Command("kubectl", "--context", kubeContext, "patch", "clusterrole",
		"kubeslice-controller-controller-role", "--type=json",
		"-p", `[{"op":"add","path":"/rules/-","value":{"apiGroups":[""],"resources":["configmaps"],"verbs":["get","list","watch"]}}]`).Run()

	run(t, "", "kubectl", "--context", kubeContext, "rollout", "status",
		"deployment/kubeslice-controller-manager", "-n", controllerNS, "--timeout=180s")
}

// setupProjectAndWorkers creates the project and three fake, pre-registered
// worker Cluster CRs (worker-1/2/3), standing in for a real registration flow.
func setupProjectAndWorkers(t *testing.T) {
	t.Helper()
	applyYAML(t, `
apiVersion: controller.kubeslice.io/v1alpha1
kind: Project
metadata: {name: avesha, namespace: `+controllerNS+`}
spec: {serviceAccount: {readWrite: [admin]}}`)

	waitFor(t, "project namespace "+projectNS+" exists", 60*time.Second, func() bool {
		_, ok := tryRun("kubectl", "", "--context", kubeContext, "get", "ns", projectNS)
		return ok
	})

	for _, n := range []string{"1", "2", "3"} {
		applyYAML(t, `
apiVersion: controller.kubeslice.io/v1alpha1
kind: Cluster
metadata: {name: worker-`+n+`, namespace: `+projectNS+`}
spec: {networkInterface: eth0}`)
	}
	// patch status → Registered with a cniSubnet/nodeIP so the SliceConfig
	// reconciler treats them as usable members.
	time.Sleep(3 * time.Second)
	for _, n := range []string{"1", "2", "3"} {
		run(t, "", "kubectl", "--context", kubeContext, "patch", "cluster", "worker-"+n,
			"-n", projectNS, "--subresource=status", "--type=merge",
			"-p", `{"status":{"registrationStatus":"Registered","clusterHealth":{"clusterHealthStatus":"Normal"},"nodeIPs":["172.18.0.1`+n+`"],"networkPresent":true,"cniSubnet":["10.244.0.0/16"]}}`)
	}
}

// gatewayColumns returns "name host route" lines for the slice's gateways.
func gatewayColumns(t *testing.T) string {
	t.Helper()
	return kubectl(t, "get", "workerslicegateways", "-n", projectNS,
		"-o", "custom-columns=N:.metadata.name,HOST:.spec.gatewayHostType,ROUTE:.spec.routeEntireSliceSubnet",
		"--no-headers")
}

// gatewayCount returns how many slice gateways currently exist.
func gatewayCount(t *testing.T) int {
	t.Helper()
	lines := strings.TrimSpace(gatewayColumns(t))
	if lines == "" {
		return 0
	}
	return len(strings.Split(lines, "\n"))
}
