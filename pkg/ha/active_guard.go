/*
 * 	Copyright (c) 2026 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
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

package ha

import (
	"context"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RoleProvider exposes the current HA role. *HAManager satisfies it.
type RoleProvider interface {
	IsActive() bool
}

// ActiveGuard wraps a reconcile.Reconciler so that reconciliation runs only
// while this instance is Active. A Standby instance drops requests without
// requeue, leaving cluster state untouched until it is promoted — at which
// point controller-runtime's own resync re-enqueues the work.
type ActiveGuard struct {
	role     RoleProvider
	delegate reconcile.Reconciler
	logger   *zap.SugaredLogger
}

// NewActiveGuard wraps delegate with an Active-only gate.
func NewActiveGuard(role RoleProvider, delegate reconcile.Reconciler, logger *zap.SugaredLogger) *ActiveGuard {
	return &ActiveGuard{role: role, delegate: delegate, logger: logger}
}

// Reconcile delegates to the wrapped reconciler when Active; otherwise it
// records a skip and returns an empty result.
func (g *ActiveGuard) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if !g.role.IsActive() {
		metricReconcileSkipped.Inc()
		if g.logger != nil {
			g.logger.Debugw("skipping reconcile: instance is Standby", "request", req.String())
		}
		return reconcile.Result{}, nil
	}
	return g.delegate.Reconcile(ctx, req)
}
