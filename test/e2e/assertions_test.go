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
	"io"
	"strings"
	"testing"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

const haLeaseName = "kubeslice-controller-ha"

// managerLogs returns the manager container's full logs — current
// container only; a restarted pod's previous-container logs are not
// stitched in, since none of these scenarios restart the manager pod
// itself (unlike the worker-operator's restart-to-reconnect flow).
func managerLogs(ctx context.Context, t *testing.T, client kubernetes.Interface) string {
	t.Helper()
	pods, err := client.CoreV1().Pods(controllerNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "control-plane=controller-manager",
	})
	if err != nil || len(pods.Items) == 0 {
		t.Fatalf("listing manager pods: %v (found %d)", err, len(pods.Items))
	}
	req := client.CoreV1().Pods(controllerNamespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{Container: "manager"})
	stream, err := req.Stream(ctx)
	if err != nil {
		t.Fatalf("streaming manager logs: %v", err)
	}
	defer stream.Close()
	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("reading manager logs: %v", err)
	}
	return string(body)
}

// getHALease reads the cross-cluster HA Lease this hub renews or watches
// locally (see docs/ha-runbook.md's "two Leases" vocabulary — this is
// kubeslice-controller-ha, not controller-runtime's own --leader-elect
// Lease).
func getHALease(ctx context.Context, client kubernetes.Interface) (*coordinationv1.Lease, error) {
	return client.CoordinationV1().Leases(controllerNamespace).Get(ctx, haLeaseName, metav1.GetOptions{})
}

func leaseHolder(ctx context.Context, client kubernetes.Interface) string {
	l, err := getHALease(ctx, client)
	if err != nil || l.Spec.HolderIdentity == nil {
		return ""
	}
	return *l.Spec.HolderIdentity
}

// activeControllerIdentity reads status.activeController.activeIdentity off
// the given Cluster CR — this repo's own CRD, confirmed to already carry
// the field (unlike a chart-installed copy).
func activeControllerIdentity(ctx context.Context, c ctrlclient.Client, ns, name string) (string, error) {
	cr := &controllerv1alpha1.Cluster{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, cr); err != nil {
		return "", err
	}
	if cr.Status.ActiveController == nil {
		return "", nil
	}
	return cr.Status.ActiveController.ActiveIdentity, nil
}

// assertLogSequence checks that every needle in order appears in logs, each
// at or after the previous one's position — the same ordering check the
// proven external suite runs for the promotion sequence, and fails loudly
// naming the first needle that's missing or out of order.
func assertLogSequence(t *testing.T, logs string, needles ...string) {
	t.Helper()
	lastIdx := -1
	for _, needle := range needles {
		idx := strings.Index(logs, needle)
		if idx == -1 {
			t.Fatalf("log sequence broken: %q never appeared", needle)
		}
		if idx < lastIdx {
			t.Fatalf("log sequence broken: %q appeared before the previous step", needle)
		}
		lastIdx = idx
	}
}

func assertLogNeverContains(t *testing.T, logs, needle string) {
	t.Helper()
	if strings.Contains(logs, needle) {
		t.Fatalf("logs unexpectedly contain %q", needle)
	}
}

func assertLogContains(t *testing.T, logs, needle string) {
	t.Helper()
	if !strings.Contains(logs, needle) {
		t.Fatalf("logs never contain %q", needle)
	}
}

func eventReasonExists(ctx context.Context, t *testing.T, client kubernetes.Interface, reason string) bool {
	t.Helper()
	events, err := client.CoreV1().Events(controllerNamespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("reason=%s", reason),
	})
	if err != nil {
		t.Fatalf("listing events for reason=%s: %v", reason, err)
	}
	return len(events.Items) > 0
}
