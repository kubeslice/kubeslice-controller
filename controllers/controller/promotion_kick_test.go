/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * 	Unless required by applicable law or agreed to in writing, software
 * 	distributed under the License is distributed on an "AS IS" BASIS,
 * 	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * 	See the License for the specific language governing permissions and
 * 	limitations under the License.
 */

package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
)

func kickTestManager(t *testing.T) manager.Manager {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, controllerv1alpha1.AddToScheme(s))

	// The address is never dialled: SetupWithManager only registers watches, it
	// does not start the manager.
	mgr, err := manager.New(&rest.Config{Host: "127.0.0.1:1"}, manager.Options{
		Scheme:  s,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	require.NoError(t, err)
	return mgr
}

// TestSetupWithManager_NilPromotionKick guards a footgun introduced by wiring
// the HA promotion kick into every reconciler. source.Channel rejects a nil
// channel — "must specify Channel.Source" — when the manager starts the source,
// so registering the watch unconditionally breaks every caller that does not
// wire a kick. The envtest suite in this package is one such caller, and any
// out-of-tree consumer constructing these reconcilers is another.
//
// Nil must mean "no extra watch", which is what makes the field genuinely
// optional and keeps a non-HA deployment identical to before.
func TestSetupWithManager_NilPromotionKick(t *testing.T) {
	mgr := kickTestManager(t)
	r := &ClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Log:    util.NewLogger().With("name", "test"),
	}
	require.NoError(t, r.SetupWithManager(mgr),
		"a reconciler with no promotion kick must set up cleanly")
}

func TestSetupWithManager_WithPromotionKick(t *testing.T) {
	mgr := kickTestManager(t)
	// A different type from the nil case: controller-runtime enforces globally
	// unique controller names, so reusing ClusterReconciler here would collide
	// with the test above rather than test anything.
	r := &ProjectReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Log:           util.NewLogger().With("name", "test"),
		PromotionKick: make(chan event.GenericEvent, 1),
	}
	require.NoError(t, r.SetupWithManager(mgr),
		"and one with a kick must register the extra watch without error")
}
