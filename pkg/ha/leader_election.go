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
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// LeaderElection drives a Kubernetes Lease-based leader election. It is the
// production implementation of ILeaderElection.
type LeaderElection struct {
	client        kubernetes.Interface
	identity      string
	namespace     string
	leaseName     string
	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
	logger        *zap.SugaredLogger

	mu             sync.RWMutex
	isLeader       bool
	leaderIdentity string
	onAcquired     func(ctx context.Context)
	onLost         func()
}

// LeaderElectionConfig configures a LeaderElection. Zero-valued durations are
// replaced with safe defaults.
type LeaderElectionConfig struct {
	Client        kubernetes.Interface
	Identity      string
	Namespace     string
	LeaseName     string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
	Logger        *zap.SugaredLogger
}

// NewLeaderElection builds a LeaderElection, applying defaults for any unset
// timing parameters.
func NewLeaderElection(cfg LeaderElectionConfig) *LeaderElection {
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = 15 * time.Second
	}
	if cfg.RenewDeadline <= 0 {
		cfg.RenewDeadline = 10 * time.Second
	}
	if cfg.RetryPeriod <= 0 {
		cfg.RetryPeriod = 2 * time.Second
	}
	return &LeaderElection{
		client:        cfg.Client,
		identity:      cfg.Identity,
		namespace:     cfg.Namespace,
		leaseName:     cfg.LeaseName,
		leaseDuration: cfg.LeaseDuration,
		renewDeadline: cfg.RenewDeadline,
		retryPeriod:   cfg.RetryPeriod,
		logger:        cfg.Logger,
	}
}

// SetCallbacks registers the transition hooks invoked when leadership is
// acquired or lost. It must be called before Run.
func (le *LeaderElection) SetCallbacks(onAcquired func(ctx context.Context), onLost func()) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.onAcquired = onAcquired
	le.onLost = onLost
}

// Run blocks, contending for leadership until ctx is cancelled. On context
// cancellation the lease is released so a Standby can take over promptly.
func (le *LeaderElection) Run(ctx context.Context) error {
	if le.client == nil {
		return fmt.Errorf("leader election: kubernetes client is required")
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{Name: le.leaseName, Namespace: le.namespace},
		Client:    le.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: le.identity,
		},
	}

	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   le.leaseDuration,
		RenewDeadline:   le.renewDeadline,
		RetryPeriod:     le.retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: le.handleStartedLeading,
			OnStoppedLeading: le.handleStoppedLeading,
			OnNewLeader:      le.handleNewLeader,
		},
	})
	if err != nil {
		return fmt.Errorf("creating leader elector: %w", err)
	}

	le.logger.Infow("starting leader election", "lease", le.namespace+"/"+le.leaseName, "identity", le.identity)
	elector.Run(ctx)
	return ctx.Err()
}

func (le *LeaderElection) handleStartedLeading(ctx context.Context) {
	le.mu.Lock()
	le.isLeader = true
	le.leaderIdentity = le.identity
	onAcquired := le.onAcquired
	le.mu.Unlock()

	le.logger.Infow("acquired leadership", "identity", le.identity)
	if onAcquired != nil {
		onAcquired(ctx)
	}
}

func (le *LeaderElection) handleStoppedLeading() {
	le.mu.Lock()
	le.isLeader = false
	onLost := le.onLost
	le.mu.Unlock()

	le.logger.Infow("lost leadership", "identity", le.identity)
	if onLost != nil {
		onLost()
	}
}

func (le *LeaderElection) handleNewLeader(identity string) {
	le.mu.Lock()
	le.leaderIdentity = identity
	le.mu.Unlock()
	if identity != le.identity {
		le.logger.Infow("observed new leader", "leader", identity)
	}
}

// IsLeader reports whether this instance currently holds the lease.
func (le *LeaderElection) IsLeader() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.isLeader
}

// LeaderIdentity returns the identity of the last observed leader.
func (le *LeaderElection) LeaderIdentity() string {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.leaderIdentity
}
