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
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
)

// newScheme registers exactly what these tests read/write: core types plus
// this repo's own controller/worker CRD groups.
func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("adding core scheme: %v", err)
	}
	if err := controllerv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding controller scheme: %v", err)
	}
	if err := workerv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding worker scheme: %v", err)
	}
	return scheme
}

// newControllerRuntimeClient builds a client.Client for the CRD reads/writes
// these tests do (the Cluster CR) — a typed clientset (kubernetes.Interface)
// handles everything else (Leases, Events, Pods, Deployments, Secrets).
func newControllerRuntimeClient(t *testing.T, cfg *rest.Config) ctrlclient.Client {
	t.Helper()
	c, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: newScheme(t)})
	if err != nil {
		t.Fatalf("building controller-runtime client: %v", err)
	}
	return c
}
