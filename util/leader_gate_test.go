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

package util

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// newGateForTest constructs a managerElectedGate from a channel we control.
// The package-private constructor lets tests exercise the same code path
// the production NewManagerLeaderGate uses, without spinning up a real
// controller-runtime manager (which would drag in API server scaffolding).
func newGateForTest(elected <-chan struct{}) LeaderGate {
	return &managerElectedGate{elected: elected}
}

func TestNoOpLeaderGate_AlwaysPermits(t *testing.T) {
	gate := NoOpLeaderGate{}
	require.NoError(t, gate.RequireLeader())
	// Idempotency: many calls, same result.
	for i := 0; i < 1000; i++ {
		require.NoError(t, gate.RequireLeader())
	}
}

func TestManagerElectedGate_BlocksUntilElected(t *testing.T) {
	elected := make(chan struct{})
	gate := newGateForTest(elected)

	err := gate.RequireLeader()
	require.Error(t, err, "should block before election")
	require.ErrorIs(t, err, ErrNotLeader, "must wrap ErrNotLeader so callers can detect it with errors.Is")

	close(elected)
	require.NoError(t, gate.RequireLeader(), "must permit after election")

	// Once permitted, stays permitted (the channel cannot re-open).
	for i := 0; i < 100; i++ {
		require.NoError(t, gate.RequireLeader())
	}
}

func TestManagerElectedGate_AlreadyClosed(t *testing.T) {
	// Real controller-runtime behaviour: when LeaderElection is disabled,
	// mgr.Elected() returns a channel that is already closed. The gate
	// must permit immediately in that case so it's safe to wire
	// unconditionally.
	elected := make(chan struct{})
	close(elected)
	gate := newGateForTest(elected)
	require.NoError(t, gate.RequireLeader())
}

func TestNewManagerLeaderGate_NilManagerIsDefensive(t *testing.T) {
	// A nil manager would panic on Elected(); the constructor should
	// fall back to NoOpLeaderGate so a misconfigured wiring fails
	// loudly elsewhere instead of at every mutation.
	gate := NewManagerLeaderGate(nil)
	require.IsType(t, NoOpLeaderGate{}, gate)
	require.NoError(t, gate.RequireLeader())
}

func TestSetDefaultLeaderGate_RoundTrip(t *testing.T) {
	// Preserve original default so this test doesn't leak state into
	// other tests that read DefaultLeaderGate().
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	elected := make(chan struct{})
	gate := newGateForTest(elected)
	SetDefaultLeaderGate(gate)
	require.Same(t, gate, DefaultLeaderGate())

	require.ErrorIs(t, DefaultLeaderGate().RequireLeader(), ErrNotLeader)
	close(elected)
	require.NoError(t, DefaultLeaderGate().RequireLeader())
}

func TestSetDefaultLeaderGate_NilResetsToNoOp(t *testing.T) {
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	SetDefaultLeaderGate(nil)
	require.IsType(t, NoOpLeaderGate{}, DefaultLeaderGate(),
		"passing nil must reset the package default to NoOpLeaderGate, not store a typed nil")
	require.NoError(t, DefaultLeaderGate().RequireLeader())
}

func TestPackageDefault_StartsAsNoOp(t *testing.T) {
	// Note: ordering with other tests matters. We do not assume nobody
	// has called SetDefaultLeaderGate before us; we assert only that the
	// default — whatever it currently is — permits mutations by default,
	// which is the actual contract callers depend on.
	require.NoError(t, DefaultLeaderGate().RequireLeader(),
		"the package default must permit mutations so pre-HA call sites behave identically")
}

func TestSetDefaultLeaderGate_ConcurrentSafe(t *testing.T) {
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	// Race: writers set the gate, readers consult it. With the
	// atomic.Pointer backing, neither side blocks the other.
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					SetDefaultLeaderGate(NoOpLeaderGate{})
				}
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10000; j++ {
				_ = DefaultLeaderGate().RequireLeader()
			}
		}()
	}
	close(stop)
	wg.Wait()
}

func TestRequireLeader_PrefersContextOverDefault(t *testing.T) {
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	// Default permits.
	SetDefaultLeaderGate(NoOpLeaderGate{})

	// Per-request context carries a gate that does NOT permit.
	rc := &kubeSliceControllerRequestContext{
		leaderGate: newGateForTest(make(chan struct{})), // open channel = not leader
	}
	ctx := context.WithValue(context.Background(), kubeSliceControllerContext, rc)

	err := requireLeader(ctx)
	require.Error(t, err, "context gate must override permissive default")
	require.ErrorIs(t, err, ErrNotLeader)
}

func TestRequireLeader_FallsBackToDefaultWhenContextHasNoGate(t *testing.T) {
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	// Default refuses.
	SetDefaultLeaderGate(newGateForTest(make(chan struct{})))

	// Context has request-context but no gate (mimics legacy callers
	// who construct the struct directly without calling Prepare-).
	rc := &kubeSliceControllerRequestContext{leaderGate: nil}
	ctx := context.WithValue(context.Background(), kubeSliceControllerContext, rc)

	err := requireLeader(ctx)
	require.Error(t, err, "nil context gate must fall through to package default")
	require.ErrorIs(t, err, ErrNotLeader)
}

func TestRequireLeader_NilContext(t *testing.T) {
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	// Default permits.
	SetDefaultLeaderGate(NoOpLeaderGate{})
	//nolint:staticcheck // intentionally passing nil to assert defensive behaviour
	require.NoError(t, requireLeader(nil))

	// Default refuses.
	SetDefaultLeaderGate(newGateForTest(make(chan struct{})))
	//nolint:staticcheck // intentionally passing nil to assert defensive behaviour
	err := requireLeader(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotLeader)
}

func TestRequireLeader_NoRequestContext(t *testing.T) {
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	// A bare context.Background() has no request context at all; must
	// not panic and must consult the package default.
	SetDefaultLeaderGate(NoOpLeaderGate{})
	require.NoError(t, requireLeader(context.Background()))

	SetDefaultLeaderGate(newGateForTest(make(chan struct{})))
	require.ErrorIs(t, requireLeader(context.Background()), ErrNotLeader)
}

func TestErrNotLeader_IsSentinel(t *testing.T) {
	// Callers (reconcilers) detect this via errors.Is to decide whether
	// to requeue vs alert. Verify the sentinel survives the typical
	// fmt.Errorf wrap chain used by the mutation helpers.
	wrapped := errors.Join(ErrNotLeader, errors.New("higher-level context"))
	require.ErrorIs(t, wrapped, ErrNotLeader)
}

func TestCtxLeaderGate_ReadsContextThenFallsBack(t *testing.T) {
	original := DefaultLeaderGate()
	t.Cleanup(func() { SetDefaultLeaderGate(original) })

	// Empty context → package default.
	SetDefaultLeaderGate(NoOpLeaderGate{})
	gate := CtxLeaderGate(context.Background())
	require.IsType(t, NoOpLeaderGate{}, gate)

	// Context with explicit gate wins.
	customGate := NoOpLeaderGate{}
	rc := &kubeSliceControllerRequestContext{leaderGate: customGate}
	ctx := context.WithValue(context.Background(), kubeSliceControllerContext, rc)
	require.Equal(t, customGate, CtxLeaderGate(ctx))

	// Nil context defended.
	//nolint:staticcheck // intentionally passing nil to assert defensive behaviour
	require.IsType(t, NoOpLeaderGate{}, CtxLeaderGate(nil))
}
