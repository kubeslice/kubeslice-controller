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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

const (
	testProjectName = "e2e"
	testProjectNS   = "kubeslice-controller-project-" + testProjectName // service.ProjectNamespacePrefix
)

// createTestClusterCR creates a minimal Project (which the ProjectReconciler
// turns into a namespace) and a Cluster CR inside it — standing in for what
// a real worker's registration flow would create, since this suite is
// controller-focused and never runs a real worker-operator.
func createTestClusterCR(ctx context.Context, t *testing.T, c ctrlclient.Client) {
	t.Helper()

	project := &controllerv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: testProjectName, Namespace: controllerNamespace},
	}
	if err := c.Create(ctx, project); err != nil {
		t.Fatalf("creating Project %s: %v", testProjectName, err)
	}

	waitFor(t, "the project namespace exists", 60*time.Second, func() (bool, error) {
		err := c.Get(ctx, types.NamespacedName{Name: testProjectNS}, &corev1.Namespace{})
		return err == nil, nil
	})

	cluster := &controllerv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: testClusterCR, Namespace: testProjectNS},
	}
	if err := c.Create(ctx, cluster); err != nil {
		t.Fatalf("creating Cluster %s: %v", testClusterCR, err)
	}
}
