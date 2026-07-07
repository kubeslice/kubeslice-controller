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
	"sync"
	"time"

	"go.uber.org/zap"
)

// StateSynchronizer keeps a Standby controller warm by periodically collecting
// the set of KubeSlice resources, building a revisioned snapshot, and
// computing the drift since the previous snapshot. On promotion the manager
// triggers an immediate sync so the new Active starts from a current view.
type StateSynchronizer struct {
	collector ResourceCollector
	interval  time.Duration
	logger    *zap.SugaredLogger

	mu        sync.RWMutex
	snapshot  Snapshot
	synced    bool
	syncCount uint64
	lastError error

	syncedOnce chan struct{}
	closeOnce  sync.Once
	onDiff     func(SnapshotDiff)
}

// StateSynchronizerConfig configures a StateSynchronizer.
type StateSynchronizerConfig struct {
	Collector ResourceCollector
	Interval  time.Duration
	Logger    *zap.SugaredLogger
	// OnDiff, if set, is invoked with every non-empty diff. Used for metrics
	// and event recording.
	OnDiff func(SnapshotDiff)
}

// NewStateSynchronizer builds a StateSynchronizer.
func NewStateSynchronizer(cfg StateSynchronizerConfig) *StateSynchronizer {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	return &StateSynchronizer{
		collector:  cfg.Collector,
		interval:   cfg.Interval,
		logger:     cfg.Logger,
		onDiff:     cfg.OnDiff,
		snapshot:   Snapshot{Resources: map[string]ResourceRef{}},
		syncedOnce: make(chan struct{}),
	}
}

// Run performs an immediate sync, then resyncs on the configured interval
// until ctx is cancelled.
func (ss *StateSynchronizer) Run(ctx context.Context) error {
	if _, err := ss.Sync(ctx); err != nil {
		ss.logger.Warnw("initial state sync failed", "error", err)
	}

	ticker := time.NewTicker(ss.interval)
	defer ticker.Stop()
	ss.logger.Infow("state synchronizer started", "interval", ss.interval)

	for {
		select {
		case <-ctx.Done():
			ss.logger.Info("state synchronizer stopped")
			return ctx.Err()
		case <-ticker.C:
			if _, err := ss.Sync(ctx); err != nil {
				ss.logger.Warnw("state sync failed", "error", err)
			}
		}
	}
}

// Sync collects current state, swaps in a new snapshot, and returns the diff
// against the previous snapshot.
func (ss *StateSynchronizer) Sync(ctx context.Context) (SnapshotDiff, error) {
	refs, err := ss.collector.Collect(ctx)
	if err != nil {
		ss.mu.Lock()
		ss.lastError = err
		ss.mu.Unlock()
		return SnapshotDiff{}, err
	}

	next := Snapshot{
		Timestamp: time.Now(),
		Resources: make(map[string]ResourceRef, len(refs)),
	}
	for _, r := range refs {
		next.Resources[r.key()] = r
	}

	ss.mu.Lock()
	prev := ss.snapshot
	next.Revision = prev.Revision + 1
	diff := diffSnapshots(prev, next)
	ss.snapshot = next
	ss.syncCount++
	ss.lastError = nil
	firstSync := !ss.synced
	ss.synced = true
	ss.mu.Unlock()

	if firstSync {
		ss.closeOnce.Do(func() { close(ss.syncedOnce) })
	}
	if !diff.Empty() {
		ss.logger.Infow("state snapshot updated",
			"revision", next.Revision, "resources", next.Count(),
			"added", len(diff.Added), "removed", len(diff.Removed), "updated", len(diff.Updated))
		if ss.onDiff != nil {
			ss.onDiff(diff)
		}
	}
	return diff, nil
}

// Snapshot returns the most recent state snapshot.
func (ss *StateSynchronizer) Snapshot() Snapshot {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.snapshot
}

// IsSynced reports whether at least one successful sync has completed.
func (ss *StateSynchronizer) IsSynced() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.synced
}

// SyncCount returns the number of successful syncs performed.
func (ss *StateSynchronizer) SyncCount() uint64 {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.syncCount
}

// WaitForSync blocks until the first successful sync completes or ctx is done.
func (ss *StateSynchronizer) WaitForSync(ctx context.Context) error {
	select {
	case <-ss.syncedOnce:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// diffSnapshots computes additions, removals, and resourceVersion updates
// between two snapshots.
func diffSnapshots(prev, next Snapshot) SnapshotDiff {
	var diff SnapshotDiff
	for k, n := range next.Resources {
		p, ok := prev.Resources[k]
		if !ok {
			diff.Added = append(diff.Added, n)
			continue
		}
		if p.ResourceVersion != n.ResourceVersion || p.UID != n.UID {
			diff.Updated = append(diff.Updated, n)
		}
	}
	for k, p := range prev.Resources {
		if _, ok := next.Resources[k]; !ok {
			diff.Removed = append(diff.Removed, p)
		}
	}
	return diff
}
