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

package ha

import "github.com/prometheus/client_golang/prometheus"

// haSyncLagSeconds and haSyncErrorsTotal are RemoteSyncer's mirror-pipeline
// metrics. Registered here via prometheus.MustRegister, the same self-
// registration idiom metrics/prometheus.go already uses for
// KubeSliceEventsCounter, rather than routed through
// metrics.StartMetricsCollector (whose default labels are slice-specific and
// don't apply to a cross-cluster mirror).
var (
	// haSyncLagSeconds observes, per kind and operation, how far behind the
	// mirror is: time.Now() minus the source object's CreationTimestamp for
	// creates, or minus the time the object was first enqueued for
	// update/delete (the more useful number to alert on once a retry has
	// backed off a few times — it reflects total time since the triggering
	// change, not just the last dequeue).
	haSyncLagSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_sync_lag_seconds",
		Help:      "Time between a change on the Active hub and it being reflected on the Standby.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"kind", "operation"})

	// haSyncErrorsTotal counts mirror failures. The syncer keeps running and
	// retries via its workqueue on every increment — this metric never
	// indicates a crash, only a retry in progress.
	haSyncErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_sync_errors_total",
		Help:      "Count of mirror sync failures, by kind and operation.",
	}, []string{"kind", "operation"})

	// haFailoverTotal counts completed promotions. A Standby that takes over
	// increments this exactly once, after the write fence has opened.
	haFailoverTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_failover_total",
		Help:      "Count of completed promotions from Standby to Active.",
	})

	// haPromotionsAbortedTotal counts the times a Standby decided the Active
	// looked gone and then refused to promote anyway. Without it every guard is
	// invisible in production: a hub that correctly declines to take over looks
	// identical to one that never noticed anything. These are the branches worth
	// demonstrating, because they are what stops a configuration mistake or a
	// local network failure from becoming a split brain.
	haPromotionsAbortedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_promotions_aborted_total",
		Help:      "Count of promotions considered and then refused, by reason.",
	}, []string{"reason"})
)

// Reasons recorded on haPromotionsAbortedTotal.
const (
	// abortSelfUnhealthy: this hub could not reach its own API server, so the
	// evidence for the Active being gone is equally consistent with this hub
	// being the broken one.
	abortSelfUnhealthy = "self_unhealthy"
	// abortLeaseLive: the final read found a live Lease — the Active renewed
	// between polls, so the staleness verdict was a polling race.
	abortLeaseLive = "lease_live"
	// abortAlreadyPromoting: a concurrent tick is already running the sequence.
	abortAlreadyPromoting = "already_promoting"
)

func init() {
	prometheus.MustRegister(haSyncLagSeconds, haSyncErrorsTotal, haFailoverTotal, haPromotionsAbortedTotal)
}
