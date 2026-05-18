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
	"errors"
	"sync"

	"go.uber.org/zap"
)

func testLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

func ref(kind, name, rv string) ResourceRef {
	return ResourceRef{Kind: kind, Namespace: "kubeslice", Name: name, ResourceVersion: rv, UID: kind + "-" + name}
}

// memCollector is an in-memory ResourceCollector whose contents tests mutate.
type memCollector struct {
	mu   sync.Mutex
	refs []ResourceRef
	err  error
}

func (m *memCollector) set(refs ...ResourceRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refs = refs
}

func (m *memCollector) setErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *memCollector) Collect(_ context.Context) ([]ResourceRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	out := make([]ResourceRef, len(m.refs))
	copy(out, m.refs)
	return out, nil
}

// memLeaseReader is an in-memory LeaseReader.
type memLeaseReader struct {
	mu    sync.Mutex
	lease *LeaseInfo
	err   error
}

func (m *memLeaseReader) set(lease *LeaseInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lease, m.err = lease, nil
}

func (m *memLeaseReader) setErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *memLeaseReader) GetLease(_ context.Context, _, _ string) (*LeaseInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.lease == nil {
		return nil, errors.New("lease not found")
	}
	cp := *m.lease
	return &cp, nil
}

// fakeElection is an ILeaderElection whose leadership transitions tests drive
// explicitly via promote and demote.
type fakeElection struct {
	mu         sync.Mutex
	leader     bool
	onAcquired func(context.Context)
	onLost     func()
}

func (f *fakeElection) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeElection) IsLeader() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.leader
}

func (f *fakeElection) LeaderIdentity() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.leader {
		return "self"
	}
	return ""
}

func (f *fakeElection) SetCallbacks(onAcquired func(context.Context), onLost func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onAcquired, f.onLost = onAcquired, onLost
}

func (f *fakeElection) promote() {
	f.mu.Lock()
	f.leader = true
	cb := f.onAcquired
	f.mu.Unlock()
	if cb != nil {
		cb(context.Background())
	}
}

func (f *fakeElection) demote() {
	f.mu.Lock()
	f.leader = false
	cb := f.onLost
	f.mu.Unlock()
	if cb != nil {
		cb()
	}
}
