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

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// HA's metrics, registered on controller-runtime's registry (issue #298).
//
// The registry choice is the one decision here that changes whether any of this
// is observable at all, so it is stated rather than left to be discovered.
// controller-runtime's metrics server serves ctrlmetrics.Registry and nothing
// else (pkg/metrics/server/server.go builds its handler with
// promhttp.HandlerFor(metrics.Registry, ...)), which is the endpoint
// --metrics-bind-address exposes and the one kube-rbac-proxy fronts in
// config/default/manager_auth_proxy_patch.yaml. These metrics were previously
// registered with prometheus.MustRegister, i.e. onto the client library's
// DEFAULT registry — which this repo does serve, but only from
// metrics.StartMetricsCollector's own ListenAndServe on service.MetricPort
// (18080), a port that appears in no manifest in config/: no containerPort, no
// Service, and nothing for the kube-rbac-proxy to authenticate. So every HA
// metric was being collected and then published where nothing could scrape it.
//
// metrics/prometheus.go looks like a precedent for the default registry but is
// not one: KubeSliceEventsCounter is created through a factory built on
// ctrlmetrics.Registry (prometheus.go:38) and only additionally registered on
// the default one, so it reaches the standard endpoint by the first path. The
// monitoring framework's own default labels are slice-specific and do not apply
// to a cross-cluster mirror, which is why these stay hand-rolled rather than
// going through mfm.NewMetricsFactory.
//
// **Why the role-scoped gauges carry a `mode` label.** Several of these describe
// a role rather than the process: a Standby has no meaningful
// ha_lease_last_renew_time_seconds any more than an Active has a meaningful
// ha_remote_lease_age_seconds, and nothing has a meaningful
// ha_last_promotion_timestamp_seconds until it has actually promoted. Those must
// be ABSENT where they do not apply, not zero — a zeroed timestamp gauge reads
// as 1970, so `time() - metric` returns decades and any alert built on it fires
// permanently.
//
// Simply not calling Set() does NOT achieve that, which is the trap this layout
// exists to avoid. A plain registered Gauge always collects, reporting 0 until
// something sets it; only a *Vec with no children collects nothing at all. So
// every role-scoped gauge here is a GaugeVec labelled `mode`, and its child is
// created only by the role it belongs to. Verified on a live pair: an Active
// publishes no ha_armed and no ha_remote_lease_age_seconds, a Standby publishes
// no ha_lease_last_renew_time_seconds, and a hub that has never promoted
// publishes no ha_last_promotion_timestamp_seconds.
//
// ha_leader_status is deliberately NOT scoped this way. It is a plain Gauge that
// both roles publish, because 0 is its alertable value and
// `sum(ha_leader_status) != 1` — no Active, or two — has to be expressible across
// the pair. ha_sync_queue_depth stays plain for the mundane reason that 0 is
// truthful on an Active: there is no backlog because there is no queue.
var (
	// haLeaderStatus is the write fence, exported. 1 means this instance holds
	// leadership and its reconcilers are writing; 0 means they are not.
	//
	// It tracks the durable isLeader flag, so it reads 0 for an Active that has
	// lost its Lease but is still running — which is the point, that being the
	// state worth paging on. It is deliberately not IsLeader(), which also
	// reports false for the duration of a promotion; that window is what
	// ha_promotion_duration_seconds measures.
	haLeaderStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_leader_status",
		Help:      "1 if this controller instance currently holds HA leadership (Active), 0 if it does not (Standby).",
	})

	// haLeaseLastRenewTime is the Unix timestamp of the last successful renewal
	// of this hub's own Lease. Active only. Alert on age, not on the value:
	// `time() - kubeslice_controller_ha_lease_last_renew_time_seconds` crossing
	// renewDeadline means leadership is about to be released.
	haLeaseLastRenewTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_lease_last_renew_time_seconds",
		Help:      "Unix timestamp of the last successful renewal of this hub's own HA Lease (Active only).",
	}, []string{"mode"})

	// haLeaseRenewErrorsTotal counts failed renewal attempts. These are the
	// near-misses that precede a self-demotion: renewOnce keeps leadership while
	// it is still inside renewDeadline, so a hub can be failing every renewal
	// for seconds before LeadershipLost fires and this is the only signal in
	// that window.
	haLeaseRenewErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_lease_renew_errors_total",
		Help:      "Count of failed attempts to renew this hub's own HA Lease.",
	})

	// haSyncLagSeconds observes, per kind and operation, how far behind the
	// mirror is: time.Now() minus the source object's CreationTimestamp for
	// creates, or minus the time the object was first enqueued for
	// update/delete (the more useful number to alert on once a retry has
	// backed off a few times — it reflects total time since the triggering
	// change, not just the last dequeue).
	//
	// Buckets are the ones issue #298 specifies. They are wider and coarser than
	// prometheus.DefBuckets, which was the previous value and the wrong shape
	// here: DefBuckets spends five of its eleven buckets below 100ms, a range a
	// cross-cluster mirror never operates in, and stops at 10s, well short of
	// the multi-second-to-minutes lag that actually matters.
	haSyncLagSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_sync_lag_seconds",
		Help:      "Time between a change on the Active hub and it being reflected on the Standby.",
		Buckets:   syncLagBuckets,
	}, []string{"kind", "operation"})

	// haSyncErrorsTotal counts mirror failures. The syncer keeps running and
	// retries via its workqueue on every increment — this metric never
	// indicates a crash, only a retry in progress.
	haSyncErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_sync_errors_total",
		Help:      "Count of mirror sync failures, by kind and operation.",
	}, []string{"kind", "operation"})

	// haSyncQueueDepth is the mirror workqueue's length.
	//
	// It answers a question ha_sync_lag_seconds structurally cannot: lag is only
	// observed for items that finished, so a syncer wedged behind a growing
	// backlog reports healthy lag from the few items still completing while
	// falling further behind. Depth is what distinguishes "keeping up" from
	// "keeping up with a fraction of the work".
	haSyncQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_sync_queue_depth",
		Help:      "Number of keys currently waiting in the mirror workqueue.",
	})

	// haFailoverTotal counts completed promotions. A Standby that takes over
	// increments this exactly once, after the write fence has opened.
	haFailoverTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_failover_total",
		Help:      "Count of completed promotions from Standby to Active.",
	})

	// haLastPromotionTimestamp is the Unix timestamp of the last completed
	// promotion, for `time() - metric` = "how long have we been running on the
	// promoted hub".
	//
	// Process-local, like every counter here: a restart clears it, and an
	// unset gauge means "this process has not promoted", NOT "this hub never
	// did". The durable record of a past failover is the PromotedToActive Event
	// and the Lease's own holderIdentity/renewTime.
	haLastPromotionTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_last_promotion_timestamp_seconds",
		Help:      "Unix timestamp of the last promotion this process completed.",
	}, []string{"mode"})

	// haPromotionDurationSeconds is the wall time of the whole promote()
	// sequence, labelled by outcome.
	//
	// The label is load-bearing rather than decorative. An aborted attempt's
	// duration is genuinely worth having — a StopMirror that times out spends
	// the entire promotionGracePeriod before giving up — but averaged in with
	// real promotions it would corrupt the only number anyone actually asks for,
	// which is how long a successful failover takes.
	haPromotionDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_promotion_duration_seconds",
		Help:      "Wall-clock duration of the promotion sequence, by outcome.",
		Buckets:   promotionBuckets,
	}, []string{"outcome"})

	// haPromotionStepDurationSeconds breaks that total down by step.
	//
	// The total says a promotion was slow; only the breakdown says which of the
	// bounded steps spent the budget, and they fail for unrelated reasons — a
	// mirror that will not stop, a local API server that will not take the
	// Lease, a kick with nothing draining its channels yet. Every step measured
	// here already brackets itself in a context.WithTimeout, so these are the
	// same boundaries promotion already treats as its budget units.
	haPromotionStepDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_promotion_step_duration_seconds",
		Help:      "Wall-clock duration of each individual step of the promotion sequence.",
		Buckets:   promotionBuckets,
	}, []string{"step"})

	// haFailoverDetectionSeconds is how long it took to notice: the Active's
	// last observed renewTime to the moment this hub committed to promoting.
	//
	// This is the number that validates the failover budget empirically. Added
	// to ha_promotion_duration_seconds it is the total window in which the
	// cluster had no writer, which is the only figure an operator with an SLO
	// cares about. By construction it lands near leaseDuration + padding; the
	// spread above that is the cost of polling every retryPeriod.
	//
	// Recorded once per *successful* promotion rather than on every stale tick.
	// Sampling it per tick would mean a hub whose guards keep refusing emits an
	// ever-growing detection time forever, which describes the refusal rather
	// than any detection.
	haFailoverDetectionSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_failover_detection_seconds",
		Help:      "Time from the Active hub's last observed Lease renewal to this hub committing to promotion.",
		Buckets:   detectionBuckets,
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

	// haRemoteLeaseAgeSeconds is the age of the newest Lease this Standby has
	// read from the Active: now minus that Lease's renewTime. Standby only.
	//
	// The leading indicator, and the one gauge to graph. Everything else in this
	// file reports a failover that already happened; this one climbs beforehand,
	// so an alert at a fraction of leaseDuration + padding fires while there is
	// still time to look. It keeps climbing when reads fail, by design — a
	// retained stale Lease ageing against a moving clock is exactly how
	// checkRemoteLeaseOnce models "no new evidence of life".
	haRemoteLeaseAgeSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_remote_lease_age_seconds",
		Help:      "Age in seconds of the newest Lease this Standby has read from the Active hub.",
	}, []string{"mode"})

	// haRemoteLeaseReadsTotal counts remote Lease reads by result.
	//
	// A Standby that has silently lost its read path is otherwise
	// indistinguishable from a healthy one from the outside: lastSeenLease is
	// deliberately not cleared on a failed read, so the cached view stays
	// populated and only the logs know. This is the metric that catches an
	// expired credential or a kubeconfig aimed at the wrong cluster — the
	// failure that presents as x509 errors looping until someone reads the logs.
	haRemoteLeaseReadsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_remote_lease_reads_total",
		Help:      "Count of reads of the Active hub's Lease from this Standby, by result.",
	}, []string{"result"})

	// haArmed reports whether this Standby has ever successfully read the
	// Active's Lease, and is therefore eligible to promote at all.
	//
	// The arming rule is a safety property — a Standby that has never seen the
	// Active alive must never conclude it died — but it has a failure mode with
	// no other signal: a hub misconfigured badly enough to never arm will never
	// fail over, and looks perfectly healthy until the day it is needed. 0 here
	// on a Standby means the HA pair is not actually protecting anything.
	haArmed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_armed",
		Help:      "1 if this Standby has read the Active hub's Lease at least once and could promote, 0 if not.",
	}, []string{"mode"})

	// haPruneResurrectedTotal counts objects the prune pass re-enqueued because
	// they exist on the Active with no mirror on the Standby.
	//
	// Prune is a backstop for drift the event path missed, so a backstop that
	// fires steadily is not reassurance — it is evidence the informer path is
	// dropping work. Zero is the healthy value, and nothing today would tell
	// you it is not zero.
	haPruneResurrectedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_prune_resurrected_total",
		Help:      "Count of Active-side objects the prune pass re-enqueued because the Standby had no mirror.",
	}, []string{"kind"})

	// haPruneLastRunTimestamp is the Unix timestamp of the last completed prune
	// pass. Its absence, or an age far past pruneInterval, means the drift
	// backstop is not running — which is silent, since a prune pass that never
	// happens produces no errors either.
	haPruneLastRunTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_prune_last_run_timestamp_seconds",
		Help:      "Unix timestamp of the last completed prune pass.",
	}, []string{"mode"})

	// haActivePublishErrorsTotal counts failures to write status.activeController.
	//
	// Publishing is best-effort everywhere by design: promotion logs and
	// continues past a failed publish rather than stranding the cluster with no
	// writer, and the periodic loop just retries. That is the right trade and it
	// is also why the failure is invisible — while being the exact field
	// worker-operator #467 reads to find the new Active. This is the difference
	// between "failover worked" and "failover worked but no worker noticed".
	haActivePublishErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "kubeslice_controller",
		Name:      "ha_active_publish_errors_total",
		Help:      "Count of failed attempts to publish status.activeController onto this hub's Cluster CRs.",
	})
)

// Bucket sets. Named rather than inlined because the promotion ones are shared
// by the total and the per-step histograms, which must stay comparable.
var (
	// syncLagBuckets is issue #298's specified set.
	syncLagBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30}

	// promotionBuckets reaches 60s deliberately. promote() can spend up to four
	// sequential promotionGracePeriod budgets (stop mirror, publish, kick, emit)
	// plus two promotionDialTimeout guard dials, so the interesting tail sits
	// far above DefBuckets' 10s ceiling — and a promotion pinned in the top
	// bucket is precisely the case worth seeing.
	promotionBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

	// detectionBuckets starts at 1s because detection cannot be faster than one
	// retryPeriod and is bounded below by leaseDuration + padding; sub-second
	// buckets would all be empty.
	detectionBuckets = []float64{1, 2, 5, 10, 15, 20, 30, 45, 60, 120}
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
	// abortMirrorNotStopped: the mirror did not confirm it stopped inside the
	// grace period, so proceeding would open the fence on a dual writer.
	abortMirrorNotStopped = "mirror_not_stopped"
	// abortLeaseAcquireFailed: this hub could not take the Lease on its own
	// cluster.
	abortLeaseAcquireFailed = "lease_acquire_failed"
	// abortNoRemoteClient: promote() was called with no client to the Active, so
	// nothing could have established that it was ever alive.
	abortNoRemoteClient = "no_remote_client"
)

// Outcomes recorded on haPromotionDurationSeconds.
const (
	outcomePromoted = "promoted"
	outcomeAborted  = "aborted"
)

// Steps recorded on haPromotionStepDurationSeconds. The values match the step
// names used in promote()'s own comments and log lines, so a slow step in a
// dashboard and a slow step in the logs are searchable with the same word.
const (
	stepStopMirror      = "stop_mirror"
	stepAcquireLease    = "acquire_lease"
	stepPublishActive   = "publish_active_controller"
	stepKickReconcilers = "kick_reconcilers"
	stepEmitPromoted    = "emit_event"
)

// Results recorded on haRemoteLeaseReadsTotal.
const (
	readResultOK    = "ok"
	readResultError = "error"
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		haLeaderStatus,
		haLeaseLastRenewTime,
		haLeaseRenewErrorsTotal,
		haSyncLagSeconds,
		haSyncErrorsTotal,
		haSyncQueueDepth,
		haFailoverTotal,
		haLastPromotionTimestamp,
		haPromotionDurationSeconds,
		haPromotionStepDurationSeconds,
		haFailoverDetectionSeconds,
		haPromotionsAbortedTotal,
		haRemoteLeaseAgeSeconds,
		haRemoteLeaseReadsTotal,
		haArmed,
		haPruneResurrectedTotal,
		haPruneLastRunTimestamp,
		haActivePublishErrorsTotal,
	)
}

// boolGauge maps a boolean onto the 1/0 a Prometheus gauge wants. Written once
// here because five call sites would otherwise each inline the same conditional.
func boolGauge(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// observeStep records the duration of one promotion step.
//
// Called explicitly after each step rather than deferred, because promote()'s
// steps are inline stretches of one function rather than separate calls, and a
// defer would fire at the end of the whole sequence instead of the end of the
// step. Every call site therefore sits immediately after that step's cancel().
func observeStep(step string, start time.Time) {
	haPromotionStepDurationSeconds.WithLabelValues(step).Observe(time.Since(start).Seconds())
}
