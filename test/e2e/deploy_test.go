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
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	// service.ControllerNamespace ("kubeslice-controller") is hardcoded in
	// several places in service/*.go beyond just the (here disabled)
	// admission webhook — project_service.go and namespace_service.go use
	// it directly when managing a Project's namespace and events. Deploying
	// anywhere else would silently break Project reconciliation, so this
	// fixture matches every real deployment's namespace rather than
	// config/manager/manager.yaml's kustomize-only "system" placeholder.
	controllerNamespace = "kubeslice-controller"
	managerName         = "manager"
)

// repoRoot locates the repo root from this test package's own directory,
// so applyCRDs/applyRBAC work regardless of the working directory `go test`
// is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	// test/e2e -> repo root is two directories up.
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolving working directory: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

// applyCRDs applies this repo's own CRD source (config/crd/bases) — already
// confirmed to include status.activeController, unlike the chart-installed
// copies that hit the field-pruning bug on real clusters.
func applyCRDs(t *testing.T, kubeconfig string) {
	t.Helper()
	kubectlApply(t, kubeconfig, filepath.Join(repoRoot(t), "config", "crd", "bases"))
}

// applyRBAC applies the minimal set of RBAC objects the manager needs to
// run: its own ClusterRole/binding plus the leader-election Role/binding for
// controller-runtime's own (unrelated) --leader-elect Lease. Deliberately
// skips the auth-proxy and ovpn RBAC in config/rbac/kustomization.yaml's
// full list — this fixture runs with ENABLE_WEBHOOKS=false and no metrics
// proxy sidecar, so neither is needed.
//
// These files hard-code "namespace: system" (kubebuilder's kustomize-only
// placeholder, normally rewritten by config/default/kustomization.yaml's
// namespace transform) — applied directly, without kustomize, that string
// is substituted here for controllerNamespace instead.
func applyRBAC(t *testing.T, kubeconfig string) {
	t.Helper()
	rbacDir := filepath.Join(repoRoot(t), "config", "rbac")
	for _, f := range []string{
		"service_account.yaml",
		"role.yaml",
		"role_binding.yaml",
		"leader_election_role.yaml",
		"leader_election_role_binding.yaml",
	} {
		kubectlApplyWithNamespaceSubstitution(t, kubeconfig, filepath.Join(rbacDir, f))
	}
	grantClusterWideConfigMapRead(t, kubeconfig)
}

// grantClusterWideConfigMapRead closes a real gap found by running this
// fixture live: at least one reconciler's informer cache watches ConfigMap
// cluster-wide, but config/rbac/role.yaml — this repo's own kubebuilder
// scaffold — never grants ConfigMap access at all, only
// leader_election_role.yaml's namespaced grant (which can't satisfy a
// cluster-wide watch). Real deployments must get this from elsewhere (the
// Helm chart's own broader RBAC, not anything in this repo's config/), so
// this fixture grants it directly rather than trying to reproduce whatever
// the chart does.
func grantClusterWideConfigMapRead(t *testing.T, kubeconfig string) {
	t.Helper()
	runKind(t, "kubectl", "--kubeconfig", kubeconfig, "create", "clusterrole",
		"e2e-configmap-reader", "--verb=get,list,watch", "--resource=configmaps")
	runKind(t, "kubectl", "--kubeconfig", kubeconfig, "create", "clusterrolebinding",
		"e2e-configmap-reader-binding", "--clusterrole=e2e-configmap-reader",
		"--serviceaccount="+controllerNamespace+":controller-manager")
}

func kubectlApply(t *testing.T, kubeconfig, path string) {
	t.Helper()
	runKind(t, "kubectl", "--kubeconfig", kubeconfig, "apply", "-f", path)
}

// kubectlApplyWithNamespaceSubstitution applies a kubebuilder-scaffolded
// RBAC file that assumes kustomize's namespace transform. That transform
// does two things a literal string substitution alone can't: it rewrites
// any EXISTING "namespace: system" (including inside a subjects[] entry,
// which this function's sed-equivalent handles) AND it INJECTS
// metadata.namespace on objects that omit it entirely, relying on kustomize
// to supply one (leader_election_role.yaml and
// leader_election_role_binding.yaml both do this) — a plain string
// replacement can't add a field that isn't there, so `kubectl apply -n` is
// used as well, which supplies the same default kustomize would.
func kubectlApplyWithNamespaceSubstitution(t *testing.T, kubeconfig, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	rewritten := strings.ReplaceAll(string(raw), "namespace: system", "namespace: "+controllerNamespace)

	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "-n", controllerNamespace, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(rewritten)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("kubectl apply -f - (%s): %v\n%s", path, err, out.String())
	}
}

// hubConfig is everything that differs between the Active and the Standby
// when deploying the manager.
type hubConfig struct {
	Image string
	Args  []string
	// ActiveKubeconfigSecret, when set, is mounted at
	// /var/run/ha/active.kubeconfig and referenced by
	// --ha-active-kubeconfig — the Standby-only remote credential.
	ActiveKubeconfigSecret string
}

// createControllerNamespace must run before applyRBAC (which creates
// objects inside it) and before deployManager.
func createControllerNamespace(ctx context.Context, t *testing.T, client kubernetes.Interface) {
	t.Helper()
	_, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: controllerNamespace},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating namespace %s: %v", controllerNamespace, err)
	}
}

// deployManager creates the manager Deployment with the given per-hub
// configuration, then waits for it to become ready. createControllerNamespace
// must already have been called for this cluster.
func deployManager(ctx context.Context, t *testing.T, client kubernetes.Interface, cfg hubConfig) {
	t.Helper()

	env := []corev1.EnvVar{
		{Name: "ENABLE_WEBHOOKS", Value: "false"},
		// main.go's --ha-lease-namespace flag defaults from this env var
		// (via a downward-API fieldRef, same as config/manager/manager.yaml)
		// to the controller's own namespace.
		{Name: "KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
		}},
	}

	volumes := []corev1.Volume{}
	mounts := []corev1.VolumeMount{}
	if cfg.ActiveKubeconfigSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "ha-active-kubeconfig",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: cfg.ActiveKubeconfigSecret},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "ha-active-kubeconfig",
			MountPath: "/var/run/ha",
			ReadOnly:  true,
		})
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerName,
			Namespace: controllerNamespace,
			Labels:    map[string]string{"control-plane": "controller-manager"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"control-plane": "controller-manager"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"control-plane": "controller-manager"}},
				Spec: corev1.PodSpec{
					ServiceAccountName: "controller-manager",
					Volumes:            volumes,
					Containers: []corev1.Container{
						{
							Name:         "manager",
							Image:        cfg.Image,
							Command:      []string{"/manager"},
							Args:         cfg.Args,
							Env:          env,
							VolumeMounts: mounts,
							LivenessProbe: &corev1.Probe{
								ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8081)}},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromInt32(8081)}},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
	if _, err := client.AppsV1().Deployments(controllerNamespace).Create(ctx, deploy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("creating manager deployment: %v", err)
	}

	waitFor(t, "manager deployment ready", 180*time.Second, func() (bool, error) {
		d, err := client.AppsV1().Deployments(controllerNamespace).Get(ctx, managerName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return d.Status.ReadyReplicas == 1, nil
	})
}

// bindStandbyReaderRole applies the least-privilege ClusterRole from
// config/ha/active-cluster-clusterrole.yaml on the Active, bound to the
// given ServiceAccount identity — the credential behind the Standby's
// --ha-active-kubeconfig. Not part of this repo's own deploy flow by
// design (config/ha/README.md), so the fixture applies it by hand, exactly
// as a real operator would.
func bindStandbyReaderRole(t *testing.T, activeKubeconfig, saNamespace, saName string) {
	t.Helper()
	crFile := filepath.Join(repoRoot(t), "config", "ha", "active-cluster-clusterrole.yaml")
	kubectlApply(t, activeKubeconfig, crFile)
	runKind(t, "kubectl", "--kubeconfig", activeKubeconfig, "create", "clusterrolebinding",
		"kubeslice-ha-standby-reader-binding-e2e",
		"--clusterrole=kubeslice-ha-standby-reader",
		"--serviceaccount="+saNamespace+":"+saName)
}

func int32Ptr(v int32) *int32 { return &v }
