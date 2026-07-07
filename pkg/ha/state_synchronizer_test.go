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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSynchronizer(c ResourceCollector) *StateSynchronizer {
	return NewStateSynchronizer(StateSynchronizerConfig{
		Collector: c,
		Interval:  time.Hour,
		Logger:    testLogger(),
	})
}

func TestSync_FirstSyncReportsAllResourcesAdded(t *testing.T) {
	col := &memCollector{}
	col.set(ref("SliceConfig", "slice-a", "1"), ref("Cluster", "worker-1", "1"))
	ss := newTestSynchronizer(col)

	diff, err := ss.Sync(context.Background())
	require.NoError(t, err)
	assert.Len(t, diff.Added, 2)
	assert.Empty(t, diff.Removed)
	assert.Empty(t, diff.Updated)
	assert.True(t, ss.IsSynced())
	assert.Equal(t, uint64(1), ss.Snapshot().Revision)
	assert.Equal(t, 2, ss.Snapshot().Count())
}

func TestSync_DetectsAddRemoveAndUpdate(t *testing.T) {
	col := &memCollector{}
	col.set(ref("SliceConfig", "slice-a", "1"), ref("Cluster", "worker-1", "1"))
	ss := newTestSynchronizer(col)

	_, err := ss.Sync(context.Background())
	require.NoError(t, err)

	// slice-a updated (new resourceVersion), worker-1 removed, slice-b added.
	col.set(ref("SliceConfig", "slice-a", "2"), ref("SliceConfig", "slice-b", "1"))
	diff, err := ss.Sync(context.Background())
	require.NoError(t, err)

	require.Len(t, diff.Added, 1)
	assert.Equal(t, "slice-b", diff.Added[0].Name)
	require.Len(t, diff.Removed, 1)
	assert.Equal(t, "worker-1", diff.Removed[0].Name)
	require.Len(t, diff.Updated, 1)
	assert.Equal(t, "slice-a", diff.Updated[0].Name)
	assert.Equal(t, uint64(2), ss.Snapshot().Revision)
}

func TestSync_NoChangeProducesEmptyDiff(t *testing.T) {
	col := &memCollector{}
	col.set(ref("SliceConfig", "slice-a", "7"))
	ss := newTestSynchronizer(col)

	_, err := ss.Sync(context.Background())
	require.NoError(t, err)
	diff, err := ss.Sync(context.Background())
	require.NoError(t, err)
	assert.True(t, diff.Empty())
	assert.Equal(t, uint64(2), ss.SyncCount())
}

func TestSync_CollectorErrorPreservesLastSnapshot(t *testing.T) {
	col := &memCollector{}
	col.set(ref("SliceConfig", "slice-a", "1"))
	ss := newTestSynchronizer(col)
	_, err := ss.Sync(context.Background())
	require.NoError(t, err)

	col.setErr(errors.New("api server unreachable"))
	_, err = ss.Sync(context.Background())
	require.Error(t, err)
	// The last good snapshot must survive a failed collection.
	assert.Equal(t, 1, ss.Snapshot().Count())
	assert.Equal(t, uint64(1), ss.Snapshot().Revision)
}

func TestWaitForSync_UnblocksAfterFirstSync(t *testing.T) {
	col := &memCollector{}
	col.set(ref("Cluster", "worker-1", "1"))
	ss := newTestSynchronizer(col)

	go func() {
		_, _ = ss.Sync(context.Background())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.NoError(t, ss.WaitForSync(ctx))
}

func TestWaitForSync_RespectsContextCancellation(t *testing.T) {
	ss := newTestSynchronizer(&memCollector{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, ss.WaitForSync(ctx), context.Canceled)
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	col := &memCollector{}
	col.set(ref("Cluster", "worker-1", "1"))
	ss := newTestSynchronizer(col)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = ss.Run(ctx)
		close(done)
	}()

	require.NoError(t, ss.WaitForSync(context.Background()))
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
