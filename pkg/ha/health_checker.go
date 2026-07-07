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
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HealthChecker monitors the Active controller's lease and reports whether the
// Active instance appears alive. A Standby uses this signal to anticipate a
// failover before it formally wins the election.
type HealthChecker struct {
	reader    LeaseReader
	namespace string
	leaseName string
	interval  time.Duration
	threshold int
	logger    *zap.SugaredLogger

	mu     sync.RWMutex
	status HealthStatus
}

// HealthCheckerConfig configures a HealthChecker.
type HealthCheckerConfig struct {
	Reader    LeaseReader
	Namespace string
	LeaseName string
	Interval  time.Duration
	// Threshold is the number of consecutive failed checks before the Active
	// controller is reported unhealthy.
	Threshold int
	Logger    *zap.SugaredLogger
}

// NewHealthChecker builds a HealthChecker with defaults applied.
func NewHealthChecker(cfg HealthCheckerConfig) *HealthChecker {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 3
	}
	return &HealthChecker{
		reader:    cfg.Reader,
		namespace: cfg.Namespace,
		leaseName: cfg.LeaseName,
		interval:  cfg.Interval,
		threshold: cfg.Threshold,
		logger:    cfg.Logger,
		status: HealthStatus{
			IsHealthy: true,
			LastCheck: time.Now(),
		},
	}
}

// Run performs an immediate health check, then rechecks on the configured
// interval until ctx is cancelled.
func (hc *HealthChecker) Run(ctx context.Context) error {
	hc.check(ctx)

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	hc.logger.Infow("health checker started", "interval", hc.interval, "threshold", hc.threshold)

	for {
		select {
		case <-ctx.Done():
			hc.logger.Info("health checker stopped")
			return ctx.Err()
		case <-ticker.C:
			hc.check(ctx)
		}
	}
}

// check performs a single evaluation of the lease and updates health state.
func (hc *HealthChecker) check(ctx context.Context) HealthStatus {
	now := time.Now()

	lease, err := hc.reader.GetLease(ctx, hc.namespace, hc.leaseName)
	if err != nil {
		return hc.record(false, fmt.Errorf("reading lease: %w", err), "", 0, now)
	}
	if lease.HolderIdentity == "" || lease.RenewTime.IsZero() {
		return hc.record(false, errors.New("lease has no holder or renew time"), lease.HolderIdentity, 0, now)
	}

	leaseDuration := time.Duration(lease.LeaseDurationSeconds) * time.Second
	if leaseDuration <= 0 {
		leaseDuration = hc.interval * time.Duration(hc.threshold)
	}
	age := now.Sub(lease.RenewTime)
	if age > leaseDuration {
		err := fmt.Errorf("lease stale: not renewed for %s (limit %s)",
			age.Round(time.Second), leaseDuration)
		return hc.record(false, err, lease.HolderIdentity, age, now)
	}
	return hc.record(true, nil, lease.HolderIdentity, age, now)
}

// record folds a single check result into the running health status. A single
// failed check does not flip health; the threshold of consecutive failures
// must be met, which debounces transient API hiccups.
func (hc *HealthChecker) record(ok bool, checkErr error, holder string, age time.Duration, now time.Time) HealthStatus {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.status.LastCheck = now
	hc.status.ObservedHolder = holder
	hc.status.LeaseAge = age
	hc.status.LastError = checkErr

	if ok {
		hc.status.ConsecutiveFailures = 0
		if !hc.status.IsHealthy {
			hc.status.IsHealthy = true
			hc.status.LastTransition = now
			hc.logger.Infow("active controller recovered", "holder", holder)
		}
		return hc.status
	}

	hc.status.ConsecutiveFailures++
	hc.logger.Warnw("active controller health check failed",
		"failures", hc.status.ConsecutiveFailures, "error", checkErr)
	if hc.status.IsHealthy && hc.status.ConsecutiveFailures >= hc.threshold {
		hc.status.IsHealthy = false
		hc.status.LastTransition = now
		hc.logger.Errorw("active controller marked unhealthy",
			"failures", hc.status.ConsecutiveFailures)
	}
	return hc.status
}

// IsHealthy reports whether the Active controller is currently healthy.
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.status.IsHealthy
}

// Status returns the detailed health status.
func (hc *HealthChecker) Status() HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.status
}

// crLeaseReader is the production LeaseReader backed by a controller-runtime
// client. It safely handles the optional fields of a coordination/v1 Lease.
type crLeaseReader struct {
	client client.Client
}

// NewLeaseReader returns a LeaseReader that reads a coordination/v1 Lease via
// the supplied controller-runtime client.
func NewLeaseReader(c client.Client) LeaseReader {
	return &crLeaseReader{client: c}
}

func (r *crLeaseReader) GetLease(ctx context.Context, namespace, name string) (*LeaseInfo, error) {
	lease := &coordinationv1.Lease{}
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, lease); err != nil {
		return nil, err
	}
	info := &LeaseInfo{}
	if lease.Spec.HolderIdentity != nil {
		info.HolderIdentity = *lease.Spec.HolderIdentity
	}
	if lease.Spec.RenewTime != nil {
		info.RenewTime = lease.Spec.RenewTime.Time
	}
	if lease.Spec.LeaseDurationSeconds != nil {
		info.LeaseDurationSeconds = *lease.Spec.LeaseDurationSeconds
	}
	return info, nil
}
