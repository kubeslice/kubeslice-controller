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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// haMetricNames is every metric this package publishes, fully qualified.
//
// Written out by hand rather than derived from the collectors, so that the list
// is an independent statement of the contract: a metric renamed in metrics.go
// without a matching change here fails, which is the point, because the names
// are what dashboards, alerts and the runbook are written against.
var haMetricNames = []string{
	"kubeslice_controller_ha_leader_status",
	"kubeslice_controller_ha_lease_last_renew_time_seconds",
	"kubeslice_controller_ha_lease_renew_errors_total",
	"kubeslice_controller_ha_sync_lag_seconds",
	"kubeslice_controller_ha_sync_errors_total",
	"kubeslice_controller_ha_sync_queue_depth",
	"kubeslice_controller_ha_failover_total",
	"kubeslice_controller_ha_failover_detection_seconds",
	"kubeslice_controller_ha_promotions_aborted_total",
	"kubeslice_controller_ha_last_promotion_timestamp_seconds",
	"kubeslice_controller_ha_promotion_duration_seconds",
	"kubeslice_controller_ha_promotion_step_duration_seconds",
	"kubeslice_controller_ha_remote_lease_age_seconds",
	"kubeslice_controller_ha_remote_lease_reads_total",
	"kubeslice_controller_ha_armed",
	"kubeslice_controller_ha_prune_resurrected_total",
	"kubeslice_controller_ha_prune_last_run_timestamp_seconds",
	"kubeslice_controller_ha_active_publish_errors_total",
}

// touchEveryVec gives each labelled metric one child. A *Vec with no children
// collects nothing at all, so without this the registry checks below would pass
// vacuously for exactly the metrics most likely to be mis-registered.
func touchEveryVec() {
	haSyncLagSeconds.WithLabelValues("SliceConfig", "create").Observe(0.2)
	haSyncErrorsTotal.WithLabelValues("SliceConfig", "update").Add(0)
	haPromotionsAbortedTotal.WithLabelValues(abortLeaseLive).Add(0)
	haPromotionDurationSeconds.WithLabelValues(outcomePromoted).Observe(0.3)
	haPromotionStepDurationSeconds.WithLabelValues(stepStopMirror).Observe(0.1)
	haRemoteLeaseReadsTotal.WithLabelValues(readResultOK).Add(0)
	haPruneResurrectedTotal.WithLabelValues("SliceConfig").Add(0)
	haLeaseLastRenewTime.WithLabelValues(string(ModeActive)).Set(1)
	haLastPromotionTimestamp.WithLabelValues(string(ModeActive)).Set(1)
	haRemoteLeaseAgeSeconds.WithLabelValues(string(ModeStandby)).Set(1)
	haArmed.WithLabelValues(string(ModeStandby)).Set(1)
	haPruneLastRunTimestamp.WithLabelValues(string(ModeStandby)).Set(1)
}

// histogramSampleCount reports how many observations one series of a histogram
// has recorded. testutil.ToFloat64 cannot read histograms, and the observation
// count — not the series count — is what the promotion tests need.
func histogramSampleCount(t *testing.T, c prometheus.Collector, wantLabels map[string]string) uint64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 128)
	c.Collect(ch)
	close(ch)

	for m := range ch {
		var pb dto.Metric
		require.NoError(t, m.Write(&pb))
		if pb.Histogram == nil {
			continue
		}
		got := map[string]string{}
		for _, l := range pb.GetLabel() {
			got[l.GetName()] = l.GetValue()
		}
		match := true
		for k, v := range wantLabels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return pb.Histogram.GetSampleCount()
		}
	}
	return 0
}

// histogramUpperBounds returns the bucket boundaries a histogram was built with,
// read back off a collected sample rather than off the source slice — the point
// being to prove the histogram actually carries them.
func histogramUpperBounds(t *testing.T, c prometheus.Collector) []float64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 128)
	c.Collect(ch)
	close(ch)

	for m := range ch {
		var pb dto.Metric
		require.NoError(t, m.Write(&pb))
		if pb.Histogram == nil {
			continue
		}
		var bounds []float64
		for _, b := range pb.Histogram.GetBucket() {
			bounds = append(bounds, b.GetUpperBound())
		}
		return bounds
	}
	return nil
}

// TestHAMetrics_RegisteredOnControllerRuntimeRegistry is issue #298's first
// acceptance criterion, and it pins the bug it was written against: these
// metrics used to be registered with prometheus.MustRegister, i.e. onto the
// client library's default registry, which controller-runtime's metrics server
// does not serve. They were being collected and published on a port that
// appears in no manifest, so /metrics never carried a single one of them.
func TestHAMetrics_RegisteredOnControllerRuntimeRegistry(t *testing.T) {
	touchEveryVec()

	families, err := ctrlmetrics.Registry.Gather()
	require.NoError(t, err)

	present := map[string]*dto.MetricFamily{}
	for _, f := range families {
		present[f.GetName()] = f
	}

	for _, name := range haMetricNames {
		assert.Contains(t, present, name,
			"%s must be registered on ctrlmetrics.Registry — that is the only registry "+
				"controller-runtime's metrics server serves", name)
	}
}

// TestHAMetrics_ServedOverHTTPWithHelpAndType is acceptance criteria #1 and #4
// taken literally: the metrics must be visible on a /metrics endpoint, with HELP
// and TYPE comments in the output.
//
// The handler here is constructed exactly as controller-runtime's own metrics
// server constructs it — promhttp.HandlerFor over ctrlmetrics.Registry, see
// pkg/metrics/server/server.go — so this exercises the real exposition path
// rather than a re-implementation of it, without needing a manager or a cluster.
func TestHAMetrics_ServedOverHTTPWithHelpAndType(t *testing.T) {
	touchEveryVec()

	srv := httptest.NewServer(promhttp.HandlerFor(ctrlmetrics.Registry, promhttp.HandlerOpts{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := string(body)

	for _, name := range haMetricNames {
		assert.Contains(t, out, "# HELP "+name, "%s must appear with a HELP comment", name)
		assert.Contains(t, out, "# TYPE "+name, "%s must appear with a TYPE comment", name)
	}
}

// TestHAMetrics_HaveHelpAndType is acceptance criterion #4: HELP and TYPE must
// appear in /metrics output. Both are emitted by the exposition format from the
// collector's own description, so asserting they are non-empty here is the same
// guarantee without needing to stand up an HTTP server.
func TestHAMetrics_HaveHelpAndType(t *testing.T) {
	touchEveryVec()

	families, err := ctrlmetrics.Registry.Gather()
	require.NoError(t, err)

	checked := 0
	for _, f := range families {
		name := f.GetName()
		if !contains(haMetricNames, name) {
			continue
		}
		checked++
		assert.NotEmpty(t, f.GetHelp(), "%s must carry a HELP string", name)
		assert.NotEqual(t, dto.MetricType(-1), f.GetType(), "%s must carry a TYPE", name)
	}
	assert.Equal(t, len(haMetricNames), checked, "every HA metric must have been reachable to check")
}

// TestHAMetrics_LintClean runs Prometheus' own naming linter. It is what catches
// the mistakes a human reviewer reliably misses: a counter without the _total
// suffix, a unit that disagrees with the name, a gauge measuring seconds called
// something else.
func TestHAMetrics_LintClean(t *testing.T) {
	touchEveryVec()

	for _, c := range []prometheus.Collector{
		haLeaderStatus,
		haLeaseLastRenewTime,
		haLeaseRenewErrorsTotal,
		haSyncLagSeconds,
		haSyncErrorsTotal,
		haSyncQueueDepth,
		haFailoverTotal,
		haFailoverDetectionSeconds,
		haPromotionsAbortedTotal,
		haLastPromotionTimestamp,
		haPromotionDurationSeconds,
		haPromotionStepDurationSeconds,
		haRemoteLeaseAgeSeconds,
		haRemoteLeaseReadsTotal,
		haArmed,
		haPruneResurrectedTotal,
		haPruneLastRunTimestamp,
		haActivePublishErrorsTotal,
	} {
		problems, err := testutil.CollectAndLint(c)
		require.NoError(t, err)
		assert.Empty(t, problems, "metric naming/lint problems: %+v", problems)
	}
}

func TestHASyncMetrics_RecordAndLabel(t *testing.T) {
	haSyncLagSeconds.Reset()
	haSyncErrorsTotal.Reset()

	haSyncLagSeconds.WithLabelValues("SliceConfig", "create").Observe(0.5)
	haSyncErrorsTotal.WithLabelValues("SliceConfig", "update").Inc()
	haSyncErrorsTotal.WithLabelValues("SliceConfig", "update").Inc()

	assert.Equal(t, 1, testutil.CollectAndCount(haSyncLagSeconds))
	assert.Equal(t, float64(2), testutil.ToFloat64(haSyncErrorsTotal.WithLabelValues("SliceConfig", "update")))
}

// TestHASyncLagSeconds_UsesTheSpecifiedBuckets pins the buckets to issue #298's
// table. The previous value was prometheus.DefBuckets, which spends five of its
// eleven buckets below 100ms — a range a cross-cluster mirror never operates in
// — and stops at 10s, below the lag worth alerting on.
func TestHASyncLagSeconds_UsesTheSpecifiedBuckets(t *testing.T) {
	haSyncLagSeconds.Reset()
	haSyncLagSeconds.WithLabelValues("SliceConfig", "create").Observe(0.2)

	assert.Equal(t, []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		histogramUpperBounds(t, haSyncLagSeconds))
}

// TestHAPromotionHistograms_ReachPastTenSeconds guards the reason promotion has
// its own bucket set: a promotion can spend four sequential grace periods plus
// two guard dials, so a ceiling of 10s would collapse every slow promotion —
// precisely the ones worth investigating — into +Inf.
func TestHAPromotionHistograms_ReachPastTenSeconds(t *testing.T) {
	haPromotionDurationSeconds.Reset()
	haPromotionDurationSeconds.WithLabelValues(outcomePromoted).Observe(0.1)

	bounds := histogramUpperBounds(t, haPromotionDurationSeconds)
	require.NotEmpty(t, bounds)
	assert.Equal(t, float64(60), bounds[len(bounds)-1],
		"the top bucket must be well above a single grace period")
}

func TestHALeaderStatus_TracksLeadershipTransitions(t *testing.T) {
	// Standalone is always the leader, and the gauge is published at
	// construction so the series exists before any transition happens.
	NewClusterLeaderElector(fakeClient(t), nil, Options{Mode: ModeStandalone, Log: testLog()})
	assert.Equal(t, float64(1), testutil.ToFloat64(haLeaderStatus),
		"standalone must publish 1 immediately — it is unconditionally the leader")

	e := NewClusterLeaderElector(fakeClient(t), nil, Options{Mode: ModeStandby, Identity: "hub-b", Log: testLog()})
	assert.Equal(t, float64(0), testutil.ToFloat64(haLeaderStatus),
		"a standby must publish 0 at construction, not leave the series absent")

	e.setLeader(true)
	assert.Equal(t, float64(1), testutil.ToFloat64(haLeaderStatus))
	e.setLeader(false)
	assert.Equal(t, float64(0), testutil.ToFloat64(haLeaderStatus))
}

// TestRoleScopedGauges_AbsentOnTheWrongRole is the regression test for a defect
// that only a live pair exposed. These gauges were plain prometheus.Gauge and
// were simply never Set() on the role they do not describe — which does not make
// them absent. **A registered plain Gauge always collects, reporting 0.** So a
// real Active published `ha_armed 0` (making "unarmed Standby" alerts fire on
// every Active) and `ha_prune_last_run_timestamp_seconds 0` (making the
// "backstop is not running" alert fire forever), and any hub that had not
// promoted published `ha_last_promotion_timestamp_seconds 0`, i.e. 1970.
//
// A *Vec with no children is what actually collects nothing, so each of these is
// now labelled by `mode` and only its own role creates the child. Asserting on
// the gathered output rather than on values, because the whole property under
// test is a series NOT being there.
func TestRoleScopedGauges_AbsentOnTheWrongRole(t *testing.T) {
	for _, v := range []*prometheus.GaugeVec{
		haArmed, haRemoteLeaseAgeSeconds, haLeaseLastRenewTime,
		haLastPromotionTimestamp, haPruneLastRunTimestamp,
	} {
		v.Reset()
	}

	// An Active that renews: publishes its renew time, and nothing Standby-scoped.
	active := NewClusterLeaderElector(fakeClient(t), nil, Options{
		Mode: ModeActive, Identity: "hub-a", Log: testLog(),
	})
	require.NoError(t, active.renewOnce(context.Background()))

	assert.Equal(t, 1, testutil.CollectAndCount(haLeaseLastRenewTime),
		"an Active must publish its own lease renew time")
	assert.Equal(t, 0, testutil.CollectAndCount(haArmed),
		"an Active must publish NO ha_armed series — it has no remote hub to be armed against")
	assert.Equal(t, 0, testutil.CollectAndCount(haRemoteLeaseAgeSeconds),
		"an Active must publish NO remote lease age")
	assert.Equal(t, 0, testutil.CollectAndCount(haPruneLastRunTimestamp),
		"an Active runs no prune pass and must publish no timestamp for it")
	assert.Equal(t, 0, testutil.CollectAndCount(haLastPromotionTimestamp),
		"a hub that has not promoted must publish no promotion timestamp, not a zero one")

	// A Standby: the mirror image.
	haLeaseLastRenewTime.Reset()
	standby := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	_, _ = standby.checkRemoteLeaseOnce(context.Background())

	assert.Equal(t, 1, testutil.CollectAndCount(haArmed),
		"a Standby must publish ha_armed so that 0 is alertable")
	assert.Equal(t, 0, testutil.CollectAndCount(haLeaseLastRenewTime),
		"a Standby holds no lease of its own and must publish no renew time")
}

// TestPromote_DropsStandbyScopedSeries covers the other half: a promoted hub has
// stopped watching a remote, so a frozen remote-lease age left behind would look
// exactly like a Standby whose Active is healthy.
func TestPromote_DropsStandbyScopedSeries(t *testing.T) {
	haRemoteLeaseAgeSeconds.Reset()

	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))
	_, _ = e.checkRemoteLeaseOnce(context.Background())
	require.Equal(t, 1, testutil.CollectAndCount(haRemoteLeaseAgeSeconds),
		"precondition: the Standby was publishing a remote lease age")

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	require.True(t, promoted)

	assert.Equal(t, 0, testutil.CollectAndCount(haRemoteLeaseAgeSeconds),
		"promotion must drop the remote lease age — there is no remote to age any more")
	assert.Equal(t, 1, testutil.CollectAndCount(haLastPromotionTimestamp),
		"and must start publishing when it promoted")
}

func TestHAArmed_ZeroUntilTheActiveLeaseIsRead(t *testing.T) {
	ctx := context.Background()

	// A standby whose remote reads always fail never arms.
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	assert.Equal(t, float64(0), testutil.ToFloat64(haArmed.WithLabelValues(string(ModeStandby))),
		"construction must publish 0 so a never-arming standby is visible")

	_, _ = e.checkRemoteLeaseOnce(ctx)
	assert.Equal(t, float64(0), testutil.ToFloat64(haArmed.WithLabelValues(string(ModeStandby))),
		"a failed read must not arm the hub")

	// One successful read arms it.
	remote := fakeClient(t)
	lease := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now())
	require.NoError(t, remote.Create(ctx, lease))
	armed := NewClusterLeaderElector(fakeClient(t), remote, Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	_, err := armed.checkRemoteLeaseOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(haArmed.WithLabelValues(string(ModeStandby))))
}

func TestHARemoteLeaseReads_CountedByResult(t *testing.T) {
	ctx := context.Background()
	haRemoteLeaseReadsTotal.Reset()

	remote := fakeClient(t)
	lease := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now())
	require.NoError(t, remote.Create(ctx, lease))
	ok := NewClusterLeaderElector(fakeClient(t), remote, Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	_, err := ok.checkRemoteLeaseOnce(ctx)
	require.NoError(t, err)

	failing := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode: ModeStandby, Identity: "hub-c", Log: testLog(),
	})
	_, _ = failing.checkRemoteLeaseOnce(ctx)

	assert.Equal(t, float64(1), testutil.ToFloat64(haRemoteLeaseReadsTotal.WithLabelValues(readResultOK)))
	assert.Equal(t, float64(1), testutil.ToFloat64(haRemoteLeaseReadsTotal.WithLabelValues(readResultError)))
}

// TestHARemoteLeaseAge_ClimbsWhileReadsFail is the property that makes this
// gauge a leading indicator rather than a lagging one. A failed read leaves the
// cached Lease in place by design, so the age must keep climbing against the
// clock — a gauge frozen at its last good value would report a healthy Active
// throughout the outage.
func TestHARemoteLeaseAge_ClimbsWhileReadsFail(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-90*time.Second))

	_, _ = e.checkRemoteLeaseOnce(context.Background())

	age := testutil.ToFloat64(haRemoteLeaseAgeSeconds.WithLabelValues(string(ModeStandby)))
	assert.Greater(t, age, float64(85), "the age must reflect the retained stale lease, not the failed read")
}

func TestRemoteLeaseAge(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	_, ok := remoteLeaseAge(nil, now)
	assert.False(t, ok, "no lease read yet is not an age of zero")

	noRenew := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", now)
	noRenew.Spec.RenewTime = nil
	_, ok = remoteLeaseAge(noRenew, now)
	assert.False(t, ok, "a lease with no renewTime has no measurable age")

	past := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", now.Add(-30*time.Second))
	age, ok := remoteLeaseAge(past, now)
	require.True(t, ok)
	assert.Equal(t, 30*time.Second, age)

	// Clock skew: the Active's clock ahead of ours must clamp to zero rather
	// than publish a negative age that would make an alert flap.
	future := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", now.Add(10*time.Second))
	age, ok = remoteLeaseAge(future, now)
	require.True(t, ok)
	assert.Equal(t, time.Duration(0), age)
}

// TestPromote_RecordsDurationTimestampAndSteps covers the metrics the promotion
// path exists to produce: when it happened, how long it took in total, and how
// long each bounded step took.
func TestPromote_RecordsDurationTimestampAndSteps(t *testing.T) {
	haPromotionDurationSeconds.Reset()
	haPromotionStepDurationSeconds.Reset()
	haLastPromotionTimestamp.Reset()

	before := testutil.ToFloat64(haFailoverTotal)
	e := standbyReadyToPromote(t)
	rec := newPromotionRecorder()
	e.SetPromotionHooks(rec.hooks(e))

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	require.True(t, promoted)

	assert.Equal(t, before+1, testutil.ToFloat64(haFailoverTotal))
	assert.Greater(t, testutil.ToFloat64(haLastPromotionTimestamp.WithLabelValues(string(ModeActive))), float64(0),
		"a completed promotion must stamp when it happened")

	assert.Equal(t, uint64(1),
		histogramSampleCount(t, haPromotionDurationSeconds, map[string]string{"outcome": outcomePromoted}),
		"a successful promotion must be recorded as such, not as an abort")
	assert.Equal(t, uint64(0),
		histogramSampleCount(t, haPromotionDurationSeconds, map[string]string{"outcome": outcomeAborted}))

	// Every step of the sequence that ran must have timed itself.
	for _, step := range []string{stepStopMirror, stepAcquireLease, stepPublishActive, stepKickReconcilers, stepEmitPromoted} {
		assert.Equal(t, uint64(1),
			histogramSampleCount(t, haPromotionStepDurationSeconds, map[string]string{"step": step}),
			"step %s must record its own duration", step)
	}
}

// TestPromote_AbortIsLabelledAbortedAndCounted checks the other half of the
// outcome label. A guard refusal is not a failure and not a promotion; it has to
// be visible as its own thing, in both the duration histogram and the reason
// counter.
func TestPromote_AbortIsLabelledAbortedAndCounted(t *testing.T) {
	haPromotionDurationSeconds.Reset()
	haPromotionsAbortedTotal.Reset()

	// A standby whose Active is alive: the final-dial guard must refuse.
	ctx := context.Background()
	remote := fakeClient(t)
	live := newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now())
	require.NoError(t, remote.Create(ctx, live))

	e := NewClusterLeaderElector(fakeClient(t), remote, Options{
		Mode: ModeStandby, Identity: "hub-b", Log: testLog(),
	})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))

	promoted, err := e.promote(ctx)
	require.NoError(t, err, "declining to promote is a correct outcome, not an error")
	require.False(t, promoted)

	assert.Equal(t, uint64(1),
		histogramSampleCount(t, haPromotionDurationSeconds, map[string]string{"outcome": outcomeAborted}))
	assert.Equal(t, uint64(0),
		histogramSampleCount(t, haPromotionDurationSeconds, map[string]string{"outcome": outcomePromoted}))
	assert.Equal(t, float64(1),
		testutil.ToFloat64(haPromotionsAbortedTotal.WithLabelValues(abortLeaseLive)))
}

// TestPromote_ConcurrentRejectionIsNotTimed keeps the near-instant rejections
// out of the histogram. promote() returns in nanoseconds when the latch is
// already held, and a pile of those labelled "aborted" would drag the aborted
// quantiles to zero and hide the aborts that spent a whole grace period first.
func TestPromote_ConcurrentRejectionIsNotTimed(t *testing.T) {
	haPromotionDurationSeconds.Reset()
	haPromotionsAbortedTotal.Reset()

	e := standbyReadyToPromote(t)
	e.promoting.Store(true) // pretend a concurrent tick holds the latch

	promoted, err := e.promote(context.Background())
	require.NoError(t, err)
	require.False(t, promoted)

	assert.Equal(t, uint64(0),
		histogramSampleCount(t, haPromotionDurationSeconds, map[string]string{"outcome": outcomeAborted}),
		"a rejected concurrent attempt is not an attempt and must not be timed")
	assert.Equal(t, float64(1),
		testutil.ToFloat64(haPromotionsAbortedTotal.WithLabelValues(abortAlreadyPromoting)),
		"it must still be counted, so the rejection is not invisible")
}

func TestHALeaseRenewMetrics(t *testing.T) {
	ctx := context.Background()
	haLeaseLastRenewTime.Reset()
	before := testutil.ToFloat64(haLeaseRenewErrorsTotal)

	e := NewClusterLeaderElector(fakeClient(t), nil, Options{
		Mode: ModeActive, Identity: "hub-a", Log: testLog(),
	})
	require.NoError(t, e.renewOnce(ctx))
	assert.Greater(t, testutil.ToFloat64(haLeaseLastRenewTime.WithLabelValues(string(ModeActive))), float64(0),
		"a successful renewal must stamp when it happened")
	assert.Equal(t, before, testutil.ToFloat64(haLeaseRenewErrorsTotal))

	failing := NewClusterLeaderElector(failingReadClient(t), nil, Options{
		Mode: ModeActive, Identity: "hub-a", Log: testLog(),
	})
	require.Error(t, failing.renewOnce(ctx))
	assert.Equal(t, before+1, testutil.ToFloat64(haLeaseRenewErrorsTotal))
}

// TestHAPruneMetrics_CountResurrectionsAndStampTheRun covers the drift
// backstop's two signals. Prune re-enqueuing work is not reassurance: it means
// the informer path missed something, so zero is the healthy value and a steady
// non-zero rate is the actual finding.
func TestHAPruneMetrics_CountResurrectionsAndStampTheRun(t *testing.T) {
	ctx := context.Background()
	haPruneResurrectedTotal.Reset()
	haPruneLastRunTimestamp.Reset()

	missing := syncKey{GVK: testGVK, Namespace: "proj-a", Name: "sc-missing"}
	s := buildSyncer(t, newStubRemote())
	// Active has an object with no mirror on the Standby — the reverse diff.
	s.remoteList = stubRemoteList([]syncKey{missing}, nil)

	s.pruneOnce(ctx)

	assert.Equal(t, float64(1),
		testutil.ToFloat64(haPruneResurrectedTotal.WithLabelValues(testGVK.Kind)),
		"an Active object with no mirror must be counted, not silently re-enqueued")
	assert.Greater(t, testutil.ToFloat64(haPruneLastRunTimestamp.WithLabelValues(string(ModeStandby))), float64(0),
		"a completed pass must stamp when it ran, so a stalled backstop is visible")
}

// TestHAPruneLastRun_StampedEvenWhenAKindWasSkipped is the deliberate choice
// documented at the call site: a failed list skips that kind, but the pass did
// run, and the skip is already counted on ha_sync_errors_total. Not stamping
// would make a partially-degraded prune indistinguishable from one that never
// executed at all.
func TestHAPruneLastRun_StampedEvenWhenAKindWasSkipped(t *testing.T) {
	haPruneLastRunTimestamp.Reset()

	s := buildSyncer(t, newStubRemote())
	s.remoteList = stubRemoteList(nil, fmt.Errorf("simulated transient list failure"))

	s.pruneOnce(context.Background())

	assert.Greater(t, testutil.ToFloat64(haPruneLastRunTimestamp.WithLabelValues(string(ModeStandby))), float64(0))
}

// TestHASyncQueueDepth_RisesOnEnqueueAndFallsOnDrain is the distinction from
// ha_sync_lag_seconds: lag is only observed for items that completed, so a
// syncer falling behind reports healthy lag off the few that finish. Depth is
// what shows the backlog.
func TestHASyncQueueDepth_RisesOnEnqueueAndFallsOnDrain(t *testing.T) {
	ctx := context.Background()
	s := buildSyncer(t, newStubRemote())

	s.enqueue(syncKey{GVK: testGVK, Namespace: "proj-a", Name: "one"})
	s.enqueue(syncKey{GVK: testGVK, Namespace: "proj-a", Name: "two"})
	assert.Equal(t, float64(2), testutil.ToFloat64(haSyncQueueDepth))

	key, _ := s.queue.Get()
	s.processOnce(ctx, key)
	assert.Equal(t, float64(1), testutil.ToFloat64(haSyncQueueDepth),
		"the gauge must fall as the backlog drains, not only ever rise")
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
