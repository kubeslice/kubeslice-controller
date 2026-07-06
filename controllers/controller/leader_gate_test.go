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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kubeslice/kubeslice-controller/pkg/ha"
)

// TestReconcile_StandbySkipsAndLogs verifies the HA write fence: a Standby
// controller returns immediately from Reconcile without touching its service
// (left nil here — a leaking gate would panic), and logs the skip message on
// every call, proving IsLeader() is evaluated per invocation.
func TestReconcile_StandbySkipsAndLogs(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core).Sugar()

	standby := ha.NewClusterLeaderElector(nil, nil, ha.Options{
		Mode: ha.ModeStandby,
		Log:  zap.NewNop().Sugar(),
	})
	require.False(t, standby.IsLeader(), "standby must not be leader")

	r := &SliceConfigReconciler{
		Log:           logger,
		LeaderElector: standby,
		// SliceConfigService is intentionally nil: the gate must return before it.
	}

	const calls = 3
	for i := 0; i < calls; i++ {
		res, err := r.Reconcile(context.Background(), ctrl.Request{})
		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, res)
	}

	assert.Equal(t, calls, logs.FilterMessage("standby mode, skipping reconcile").Len(),
		"expected one skip log per Reconcile call")
}
