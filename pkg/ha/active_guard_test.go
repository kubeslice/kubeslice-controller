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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type staticRole struct{ active bool }

func (s staticRole) IsActive() bool { return s.active }

type countingReconciler struct{ calls int }

func (c *countingReconciler) Reconcile(context.Context, reconcile.Request) (reconcile.Result, error) {
	c.calls++
	return reconcile.Result{}, nil
}

func sampleRequest() reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "kubeslice", Name: "slice-a"}}
}

func TestActiveGuard_DelegatesWhenActive(t *testing.T) {
	delegate := &countingReconciler{}
	guard := NewActiveGuard(staticRole{active: true}, delegate, testLogger())

	_, err := guard.Reconcile(context.Background(), sampleRequest())
	assert.NoError(t, err)
	assert.Equal(t, 1, delegate.calls)
}

func TestActiveGuard_SkipsWhenStandby(t *testing.T) {
	delegate := &countingReconciler{}
	guard := NewActiveGuard(staticRole{active: false}, delegate, testLogger())

	res, err := guard.Reconcile(context.Background(), sampleRequest())
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, res)
	assert.Equal(t, 0, delegate.calls, "Standby must not mutate cluster state")
}

func TestActiveGuard_FollowsRoleChange(t *testing.T) {
	delegate := &countingReconciler{}
	role := &mutableRole{}
	guard := NewActiveGuard(role, delegate, testLogger())

	_, _ = guard.Reconcile(context.Background(), sampleRequest())
	assert.Equal(t, 0, delegate.calls)

	role.active = true
	_, _ = guard.Reconcile(context.Background(), sampleRequest())
	assert.Equal(t, 1, delegate.calls)
}

type mutableRole struct{ active bool }

func (m *mutableRole) IsActive() bool { return m.active }
