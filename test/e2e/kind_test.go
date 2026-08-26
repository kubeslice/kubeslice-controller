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

// Package e2e drives issue #299's Active/Standby HA scenarios against real,
// disposable Kind clusters. It never touches a developer's own kind
// clusters: every cluster name is prefixed e2e-ha- and created/destroyed
// within a single test run.
//
// Everything here shells out to the kind/docker/kubectl CLIs, matching the
// proven external suite at ~/Projects/lfx/e2e rather than adding a new
// vendored dependency for cluster orchestration.
package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const kindClusterPrefix = "e2e-ha-"

// runKind runs a kind/docker/kubectl command, failing the test with combined
// output on error. Every caller in this file goes through this so a failure
// always shows the operator what actually happened, not just "exit status 1".
func runKind(t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out.String())
	}
	return out.String()
}

// kindCreateCluster creates a disposable Kind cluster and registers its
// teardown, so every exit path — including t.Fatal from a later step in the
// same test — still deletes it. It's on the shared "kind" Docker network by
// default, so other kind clusters can reach it by
// "<name>-control-plane:6443", matching the proven suite's cross-cluster DNS
// approach.
func kindCreateCluster(t *testing.T, name string) {
	t.Helper()
	full := kindClusterPrefix + name
	t.Cleanup(func() {
		cmd := exec.Command("kind", "delete", "cluster", "--name", full)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		_ = cmd.Run()
	})
	runKind(t, "kind", "create", "cluster", "--name", full, "--wait", "120s")
}

// kindKubeconfigPath writes the cluster's kubeconfig to a temp file and
// returns its path — some callers (building a cross-cluster
// --ha-active-kubeconfig Secret) need the file itself, not just a
// *rest.Config.
func kindKubeconfigPath(t *testing.T, name string) string {
	t.Helper()
	full := kindClusterPrefix + name
	dir := t.TempDir()
	path := filepath.Join(dir, full+".kubeconfig")
	kubeconfig := runKind(t, "kind", "get", "kubeconfig", "--name", full)
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatalf("writing kubeconfig for %s: %v", full, err)
	}
	return path
}

// kindRESTConfig builds a *rest.Config from an already-fetched kubeconfig
// path, for use with client-go/controller-runtime clients directly.
func kindRESTConfig(t *testing.T, kubeconfigPath string) *rest.Config {
	t.Helper()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Fatalf("building rest.Config from %s: %v", kubeconfigPath, err)
	}
	return cfg
}

// controlPlaneAddress is the address other kind clusters on the shared
// "kind" Docker network reach this one at — a container DNS name, not an IP:
// Kind node IPs permute across container restarts, exactly the trap the
// real multicloud demo documented (see memory: "Kind docker IP
// reassignment").
func controlPlaneAddress(name string) string {
	return fmt.Sprintf("https://%s%s-control-plane:6443", kindClusterPrefix, name)
}

// kindLoadImage pushes a locally-built image into the cluster's node(s),
// working around this machine's broken `kind load docker-image` (Docker
// 29's containerd image store exports multi-platform indexes with missing
// blobs, so `docker save` + `kind load image-archive` is used instead —
// documented in memory as the established fix).
func kindLoadImage(t *testing.T, name, image string) {
	t.Helper()
	full := kindClusterPrefix + name
	archive := filepath.Join(t.TempDir(), "image.tar")
	runKind(t, "docker", "save", "--platform", "linux/amd64", image, "-o", archive)
	runKind(t, "kind", "load", "image-archive", archive, "--name", full)
}

// waitFor polls cond until it returns true or the timeout elapses, matching
// the proven suite's own wait_for helper (lib.sh).
func waitFor(t *testing.T, desc string, timeout time.Duration, cond func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ok, err := cond()
		if ok {
			return
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	if lastErr != nil {
		t.Fatalf("timed out after %s waiting for %s: %v", timeout, desc, lastErr)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, desc)
}
