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
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HAManager orchestrates leader election, state synchronization, and health
// checking to provide Active/Standby high availability for the controller.
type HAManager struct {
	identity string
	logger   *zap.SugaredLogger

	election ILeaderElection
	sync     *StateSynchronizer
	health   *HealthChecker

	mu    sync.RWMutex
	state ControllerState

	promotions chan struct{}
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	started    bool
}

// HAManagerConfig configures an HAManager. Collector, Election, and
// LeaseReader may be supplied to override the production implementations;
// this is primarily used by tests.
type HAManagerConfig struct {
	Identity  string
	Namespace string
	LeaseName string
	Logger    *zap.SugaredLogger

	// KubeClient drives the default lease-based leader election.
	KubeClient kubernetes.Interface
	// Client drives the default state collector and lease reader.
	Client client.Client

	LeaseDuration       time.Duration
	RenewDeadline       time.Duration
	RetryPeriod         time.Duration
	SyncInterval        time.Duration
	HealthCheckInterval time.Duration
	FailureThreshold    int

	// Optional overrides.
	Collector   ResourceCollector
	Election    ILeaderElection
	LeaseReader LeaseReader
}

// NewHAManager constructs an HAManager, wiring production components for any
// dependency not explicitly overridden.
func NewHAManager(cfg HAManagerConfig) (*HAManager, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("ha manager: logger is required")
	}
	if cfg.Identity == "" {
		return nil, fmt.Errorf("ha manager: identity is required")
	}
	if cfg.LeaseName == "" {
		return nil, fmt.Errorf("ha manager: lease name is required")
	}
	registerMetrics()

	election := cfg.Election
	if election == nil {
		if cfg.KubeClient == nil {
			return nil, fmt.Errorf("ha manager: KubeClient or Election is required")
		}
		election = NewLeaderElection(LeaderElectionConfig{
			Client:        cfg.KubeClient,
			Identity:      cfg.Identity,
			Namespace:     cfg.Namespace,
			LeaseName:     cfg.LeaseName,
			LeaseDuration: cfg.LeaseDuration,
			RenewDeadline: cfg.RenewDeadline,
			RetryPeriod:   cfg.RetryPeriod,
			Logger:        cfg.Logger,
		})
	}

	collector := cfg.Collector
	if collector == nil {
		if cfg.Client == nil {
			return nil, fmt.Errorf("ha manager: Client or Collector is required")
		}
		collector = NewKubeSliceCollector(cfg.Client)
	}

	leaseReader := cfg.LeaseReader
	if leaseReader == nil {
		if cfg.Client == nil {
			return nil, fmt.Errorf("ha manager: Client or LeaseReader is required")
		}
		leaseReader = NewLeaseReader(cfg.Client)
	}

	synchronizer := NewStateSynchronizer(StateSynchronizerConfig{
		Collector: collector,
		Interval:  cfg.SyncInterval,
		Logger:    cfg.Logger,
		OnDiff: func(d SnapshotDiff) {
			cfg.Logger.Debugw("standby state drift detected",
				"added", len(d.Added), "removed", len(d.Removed), "updated", len(d.Updated))
		},
	})

	healthChecker := NewHealthChecker(HealthCheckerConfig{
		Reader:    leaseReader,
		Namespace: cfg.Namespace,
		LeaseName: cfg.LeaseName,
		Interval:  cfg.HealthCheckInterval,
		Threshold: cfg.FailureThreshold,
		Logger:    cfg.Logger,
	})

	hm := &HAManager{
		identity:   cfg.Identity,
		logger:     cfg.Logger,
		election:   election,
		sync:       synchronizer,
		health:     healthChecker,
		promotions: make(chan struct{}, 1),
		state: ControllerState{
			Role:           RoleStandby,
			Identity:       cfg.Identity,
			TransitionTime: time.Now(),
		},
	}
	metricActive.Set(0)
	return hm, nil
}

// Start launches leader election, state synchronization, and health checking.
// It returns immediately; the components run until Stop is called or the
// supplied context is cancelled.
func (hm *HAManager) Start(ctx context.Context) error {
	hm.mu.Lock()
	if hm.started {
		hm.mu.Unlock()
		return fmt.Errorf("ha manager: already started")
	}
	hm.started = true
	runCtx, cancel := context.WithCancel(ctx)
	hm.cancel = cancel
	hm.mu.Unlock()

	hm.election.SetCallbacks(hm.handleLeadershipAcquired, hm.handleLeadershipLost)

	hm.logger.Infow("starting HA manager", "identity", hm.identity)
	hm.run(runCtx, hm.sync.Run)
	hm.run(runCtx, hm.health.Run)
	hm.run(runCtx, hm.election.Run)
	hm.run(runCtx, hm.publishMetrics)
	return nil
}

// run starts fn in a tracked goroutine.
func (hm *HAManager) run(ctx context.Context, fn func(context.Context) error) {
	hm.wg.Add(1)
	go func() {
		defer hm.wg.Done()
		if err := fn(ctx); err != nil && err != context.Canceled {
			hm.logger.Debugw("HA component exited", "error", err)
		}
	}()
}

// Stop signals all components to terminate and waits for them to finish.
func (hm *HAManager) Stop() {
	hm.mu.Lock()
	cancel := hm.cancel
	started := hm.started
	hm.mu.Unlock()

	if !started || cancel == nil {
		return
	}
	hm.logger.Info("stopping HA manager")
	cancel()
	hm.wg.Wait()
	hm.logger.Info("HA manager stopped")
}

func (hm *HAManager) handleLeadershipAcquired(ctx context.Context) {
	hm.mu.Lock()
	hm.state.Role = RoleActive
	hm.state.IsLeader = true
	hm.state.LeaderIdentity = hm.identity
	hm.state.Transitions++
	hm.state.TransitionTime = time.Now()
	hm.mu.Unlock()

	metricActive.Set(1)
	metricTransitions.Inc()
	hm.logger.Infow("promoted to Active", "identity", hm.identity)

	// Refresh state immediately so the new Active reconciles from a current
	// view rather than the last periodic snapshot.
	if _, err := hm.sync.Sync(ctx); err != nil {
		hm.logger.Warnw("post-promotion sync failed", "error", err)
	}

	select {
	case hm.promotions <- struct{}{}:
	default:
	}
}

func (hm *HAManager) handleLeadershipLost() {
	hm.mu.Lock()
	hm.state.Role = RoleStandby
	hm.state.IsLeader = false
	hm.state.Transitions++
	hm.state.TransitionTime = time.Now()
	hm.mu.Unlock()

	metricActive.Set(0)
	metricTransitions.Inc()
	hm.logger.Infow("demoted to Standby", "identity", hm.identity)
}

// publishMetrics periodically exports synchronizer and health gauges.
func (hm *HAManager) publishMetrics(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			snap := hm.sync.Snapshot()
			metricSyncedResources.Set(float64(snap.Count()))
			if !snap.Timestamp.IsZero() {
				metricLastSync.Set(float64(snap.Timestamp.Unix()))
			}
			metricActiveHealthy.Set(boolToFloat(hm.health.IsHealthy()))
		}
	}
}

// GetCurrentState returns a snapshot of this instance's HA state.
func (hm *HAManager) GetCurrentState() ControllerState {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.state
}

// IsActive reports whether this instance is currently the Active controller.
func (hm *HAManager) IsActive() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.state.Role == RoleActive
}

// IsStandby reports whether this instance is currently a Standby controller.
func (hm *HAManager) IsStandby() bool {
	return !hm.IsActive()
}

// HealthStatus returns the observed health of the Active controller's lease.
func (hm *HAManager) HealthStatus() HealthStatus {
	return hm.health.Status()
}

// StateSnapshot returns the most recent synchronized state snapshot.
func (hm *HAManager) StateSnapshot() Snapshot {
	return hm.sync.Snapshot()
}

// WaitForActivePromotion blocks until this instance is promoted to Active or
// ctx is cancelled.
func (hm *HAManager) WaitForActivePromotion(ctx context.Context) error {
	if hm.IsActive() {
		return nil
	}
	select {
	case <-hm.promotions:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
