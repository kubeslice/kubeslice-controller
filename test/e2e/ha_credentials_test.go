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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	standbyReaderSA        = "ha-standby-reader"
	standbyReaderNamespace = "default"
	haActiveKubeconfigPath = "/var/run/ha/active.kubeconfig"
	haActiveKubeconfigKey  = "active.kubeconfig"
	haActiveSecretName     = "ha-active-kubeconfig"
)

// buildActiveKubeconfigSecret provisions the Standby's --ha-active-kubeconfig
// credential end to end: a ServiceAccount + token minted on the Active,
// bound to config/ha/active-cluster-clusterrole.yaml's least-privilege
// ClusterRole (applied by hand, exactly as a real operator would per
// config/ha/README.md), packaged as a kubeconfig naming the Active by its
// cross-cluster container DNS name, and stored as a Secret on the Standby
// for deployManager to mount.
//
// This is a one-time credential-provisioning step, deliberately not part of
// this repo's own deploy flow — see config/ha/README.md's "not applied by
// this repo's own deploy flow" section.
func buildActiveKubeconfigSecret(ctx context.Context, t *testing.T, activeKubeconfig, activeName string, standbyClient kubernetes.Interface) {
	t.Helper()

	runKind(t, "kubectl", "--kubeconfig", activeKubeconfig, "create", "serviceaccount",
		standbyReaderSA, "-n", standbyReaderNamespace)
	bindStandbyReaderRole(t, activeKubeconfig, standbyReaderNamespace, standbyReaderSA)

	token := strings.TrimSpace(runKind(t, "kubectl", "--kubeconfig", activeKubeconfig, "create", "token",
		standbyReaderSA, "-n", standbyReaderNamespace, "--duration=6h"))

	caData := strings.TrimSpace(runKind(t, "kubectl", "--kubeconfig", activeKubeconfig, "config", "view",
		"--minify", "--flatten", "-o", "jsonpath={.clusters[0].cluster.certificate-authority-data}"))

	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
  - name: active
    cluster:
      server: %s
      certificate-authority-data: %s
contexts:
  - name: active
    context:
      cluster: active
      user: standby-reader
current-context: active
users:
  - name: standby-reader
    user:
      token: %s
`, controlPlaneAddress(activeName), caData, token)

	_, err := standbyClient.CoreV1().Secrets(controllerNamespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: haActiveSecretName, Namespace: controllerNamespace},
		StringData: map[string]string{haActiveKubeconfigKey: kubeconfig},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating %s secret on the standby: %v", haActiveSecretName, err)
	}
}
