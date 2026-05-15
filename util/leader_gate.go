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

// Package util — leader_gate.go provides the runtime fence that backs
// kubeslice-controller's Active/Standby HA contract.
//
// Every mutating Kubernetes operation in this controller funnels through
// CreateResource / UpdateResource / UpdateStatus / DeleteResource (and their
// cleanup-binary counterparts). Those helpers consult a LeaderGate before
// issuing the API call so that, when HA mode is enabled, a Standby instance
// (or any code path that escapes controller-runtime's leader-election gate)
// cannot accidentally mutate cluster state.
//
// The default gate is a no-op so existing single-cluster deployments behave
// identically to the pre-HA codebase. main.go installs a manager-backed
// gate at startup when --ha-mode is set to "active" or "standby"; in those
// modes, only the controller-runtime-elected leader is permitted to write.
//
// Design notes (see LFX #305 plan, PoC PR #2):
//   - Default behaviour preserved: zero call-site changes; nil gate or
//     missing context falls back to NoOpLeaderGate.
//   - Defence in depth: this gate is the SECOND fence. The FIRST is
//     controller-runtime's manager-level leader election, which suppresses
//     reconciler dispatch entirely on non-leaders. The gate catches any
//     mutation path that escapes the manager — goroutines started outside
//     mgr.Add, webhook handlers, the standalone cleanup binary, future code
//     that forgets to wire through the manager.
//   - Per-request override: tests and future advanced callers may stash a
//     gate on the per-request context via PrepareKubeSliceControllersRequestContext
//     to inject mocks without touching package state.
package util

import (
	"context"
	"errors"
	"sync/atomic"

	ctrl "sigs.k8s.io/controller-runtime"
)

// ErrNotLeader is returned by LeaderGate.RequireLeader when this instance
// is not the elected leader and is therefore forbidden from mutating
// cluster state. Callers should detect it with errors.Is and treat it as
// a transient condition: requeue rather than escalating to an event/alert.
var ErrNotLeader = errors.New("kubeslice-controller: instance is not the active leader; mutation refused by leader-gate")

// LeaderGate decides whether the calling instance is permitted to perform
// mutating Kubernetes API operations.
//
// Implementations MUST be safe for concurrent use and MUST keep RequireLeader
// cheap (sub-microsecond): the gate is consulted on every API write and on
// every finalizer add/remove.
type LeaderGate interface {
	// RequireLeader returns nil if the caller is permitted to mutate, or a
	// non-nil error wrapping ErrNotLeader otherwise.
	RequireLeader() error
}

// NoOpLeaderGate always permits mutations. It is the default gate for
// non-HA deployments and for code paths whose request context does not
// carry a gate (e.g., the standalone cleanup binary, unit tests that
// don't exercise gating semantics).
type NoOpLeaderGate struct{}

// RequireLeader always returns nil.
func (NoOpLeaderGate) RequireLeader() error { return nil }

// managerElectedGate gates on controller-runtime's leader-election signal.
// mgr.Elected() returns a channel that is closed once this manager has
// won (or will never win, e.g., when leader election is disabled — in
// that case the channel is closed at startup, so the gate permits all
// mutations). See sigs.k8s.io/controller-runtime/pkg/manager.Manager.
type managerElectedGate struct {
	elected <-chan struct{}
}

// NewManagerLeaderGate returns a LeaderGate that permits mutations only
// after the supplied manager has won leader election. It is safe to wire
// unconditionally: a manager with LeaderElection disabled closes Elected()
// at startup, so the gate behaves like NoOpLeaderGate in that mode.
//
// Callers should pass the SAME manager that owns the reconcilers; mixing
// managers (e.g., one for webhooks, one for reconcilers) defeats the gate.
func NewManagerLeaderGate(mgr ctrl.Manager) LeaderGate {
	if mgr == nil {
		// Defensive: a nil manager would panic on Elected(). Treat as
		// permissive so tests / misconfigured setups fail loudly elsewhere
		// rather than at every mutation.
		return NoOpLeaderGate{}
	}
	return &managerElectedGate{elected: mgr.Elected()}
}

// RequireLeader returns nil if this manager has been elected, or
// ErrNotLeader otherwise. The non-blocking select keeps the check at
// nanosecond-cost on the hot path: once Elected() is closed, the receive
// case is always immediately ready.
func (g *managerElectedGate) RequireLeader() error {
	select {
	case <-g.elected:
		return nil
	default:
		return ErrNotLeader
	}
}

// defaultLeaderGate holds the process-wide gate. Stored as an
// atomic.Pointer so SetDefaultLeaderGate and DefaultLeaderGate can be
// called concurrently without locks.
//
// Initialised in init() to a NoOpLeaderGate so any package init or test
// that touches mutation paths before main.go has wired the gate sees
// identical behaviour to the pre-HA codebase.
var defaultLeaderGate atomic.Pointer[LeaderGate]

func init() {
	var initial LeaderGate = NoOpLeaderGate{}
	defaultLeaderGate.Store(&initial)
}

// SetDefaultLeaderGate replaces the process-wide gate. Passing nil resets
// to NoOpLeaderGate. main.go calls this once at startup based on the
// --ha-mode flag; tests may call it to inject mock gates.
//
// The call is safe for concurrent use, but callers should treat the gate
// as effectively immutable after startup: there is no synchronisation
// between an in-flight mutation observing the old gate and a concurrent
// re-set. In practice this is a non-issue because main.go wires the gate
// before any reconciler runs.
func SetDefaultLeaderGate(g LeaderGate) {
	if g == nil {
		g = NoOpLeaderGate{}
	}
	defaultLeaderGate.Store(&g)
}

// DefaultLeaderGate returns the current process-wide gate. Never nil.
func DefaultLeaderGate() LeaderGate {
	return *defaultLeaderGate.Load()
}

// requireLeader is the internal hot-path helper consulted by every
// mutating util function. It prefers a gate embedded in the request
// context (via PrepareKubeSliceControllersRequestContext) over the
// process-wide default, letting tests inject context-scoped gates
// without touching package state.
//
// Resolution order:
//  1. ctx != nil AND ctx carries a kubeslice request context AND that
//     context's leaderGate field is non-nil → use it.
//  2. Otherwise fall back to DefaultLeaderGate(), which is itself
//     NoOpLeaderGate by default.
func requireLeader(ctx context.Context) error {
	if ctx != nil {
		if rc := GetKubeSliceControllerRequestContext(ctx); rc != nil && rc.leaderGate != nil {
			return rc.leaderGate.RequireLeader()
		}
	}
	return DefaultLeaderGate().RequireLeader()
}
