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

// Package ha provides Active/Standby high-availability primitives for the
// KubeSlice controller: lease-based leader election, a versioned state
// synchronizer that keeps a Standby instance warm, lease health monitoring,
// and an HA manager that orchestrates failover. The ActiveGuard reconcile
// wrapper ensures only the Active instance mutates cluster state.
package ha

import (
	"context"
	"time"
)

// ControllerRole is the HA role currently held by this controller instance.
type ControllerRole string

const (
	RoleActive  ControllerRole = "Active"
	RoleStandby ControllerRole = "Standby"
)

// ControllerState is a point-in-time snapshot of this instance's HA state.
type ControllerState struct {
	Role           ControllerRole
	Identity       string
	IsLeader       bool
	LeaderIdentity string
	Transitions    int
	TransitionTime time.Time
}

// HealthStatus describes the observed health of the Active controller's lease.
type HealthStatus struct {
	IsHealthy           bool
	LastCheck           time.Time
	LastTransition      time.Time
	ConsecutiveFailures int
	ObservedHolder      string
	LeaseAge            time.Duration
	LastError           error
}

// ResourceRef is a lightweight identity of a KubeSlice resource. The Standby
// instance tracks refs (not full bodies) so it can detect drift and warm its
// cache cheaply, then reconcile authoritatively once promoted.
type ResourceRef struct {
	Kind            string
	Namespace       string
	Name            string
	ResourceVersion string
	UID             string
}

func (r ResourceRef) key() string {
	return r.Kind + "/" + r.Namespace + "/" + r.Name
}

// ResourceCollector enumerates the resources that constitute the controller's
// authoritative state. The production implementation lists KubeSlice CRDs;
// tests supply an in-memory implementation.
type ResourceCollector interface {
	Collect(ctx context.Context) ([]ResourceRef, error)
}

// Snapshot is an immutable, revisioned view of collected state.
type Snapshot struct {
	Revision  uint64
	Timestamp time.Time
	Resources map[string]ResourceRef
}

// Count returns the number of resources in the snapshot.
func (s Snapshot) Count() int { return len(s.Resources) }

// SnapshotDiff is the delta between two consecutive snapshots.
type SnapshotDiff struct {
	Added   []ResourceRef
	Removed []ResourceRef
	Updated []ResourceRef
}

// Empty reports whether the diff carries no changes.
func (d SnapshotDiff) Empty() bool {
	return len(d.Added)+len(d.Removed)+len(d.Updated) == 0
}

// LeaseInfo is the subset of a coordination/v1 Lease the HA package needs.
type LeaseInfo struct {
	HolderIdentity       string
	RenewTime            time.Time
	LeaseDurationSeconds int32
}

// LeaseReader reads the leader-election Lease. Abstracted so the health
// checker can be exercised without a live API server.
type LeaseReader interface {
	GetLease(ctx context.Context, namespace, name string) (*LeaseInfo, error)
}

// ILeaderElection is the leader-election contract consumed by the HA manager.
type ILeaderElection interface {
	Run(ctx context.Context) error
	IsLeader() bool
	LeaderIdentity() string
	SetCallbacks(onAcquired func(ctx context.Context), onLost func())
}
