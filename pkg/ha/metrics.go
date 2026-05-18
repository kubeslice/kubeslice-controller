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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	metricsOnce sync.Once

	metricActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kubeslice_ha_active",
		Help: "1 if this controller instance is the Active leader, else 0.",
	})
	metricTransitions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubeslice_ha_leadership_transitions_total",
		Help: "Total number of Active/Standby leadership transitions.",
	})
	metricSyncedResources = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kubeslice_ha_synced_resources",
		Help: "Number of KubeSlice resources in the latest state snapshot.",
	})
	metricLastSync = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kubeslice_ha_last_sync_timestamp_seconds",
		Help: "Unix timestamp of the last successful state synchronization.",
	})
	metricActiveHealthy = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kubeslice_ha_active_healthy",
		Help: "1 if the Active controller lease is observed healthy, else 0.",
	})
	metricReconcileSkipped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubeslice_ha_reconcile_skipped_total",
		Help: "Total reconcile requests skipped because this instance is Standby.",
	})
)

// registerMetrics registers the HA metrics with the controller-runtime
// registry exactly once, so repeated HA manager construction is safe.
func registerMetrics() {
	metricsOnce.Do(func() {
		metrics.Registry.MustRegister(
			metricActive,
			metricTransitions,
			metricSyncedResources,
			metricLastSync,
			metricActiveHealthy,
			metricReconcileSkipped,
		)
	})
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
