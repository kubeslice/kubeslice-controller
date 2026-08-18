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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service"
)

// stubLeadership stands in for ClusterLeaderElector so the publisher can be
// exercised without a live Lease.
type stubLeadership struct {
	leader   bool
	identity string
}

func (s stubLeadership) IsLeader() bool   { return s.leader }
func (s stubLeadership) Identity() string { return s.identity }

// clusterScheme extends the shared testScheme with the controller CRDs the
// publisher writes to.
func clusterScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, controllerv1alpha1.AddToScheme(scheme))
	return scheme
}

func newCluster(name, namespace string) *controllerv1alpha1.Cluster {
	return &controllerv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
}

// clusterClient builds a fake client that honours the status subresource, which
// the publisher writes through exclusively.
func clusterClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(clusterScheme(t)).
		WithObjects(objs...).
		WithStatusSubresource(&controllerv1alpha1.Cluster{}).
		Build()
}

// countingClusterClient wraps clusterClient and counts status writes, so a test
// can assert that a converged pass writes nothing at all.
func countingClusterClient(t *testing.T, writes *int, objs ...client.Object) client.Client {
	t.Helper()
	base := fake.NewClientBuilder().
		WithScheme(clusterScheme(t)).
		WithObjects(objs...).
		WithStatusSubresource(&controllerv1alpha1.Cluster{}).
		Build()
	return interceptor.NewClient(base, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			*writes++
			return c.Status().Update(ctx, obj)
		},
	})
}

func testPublisher(t *testing.T, c client.Client, elector leadership, endpoint string) *ActivePublisher {
	t.Helper()
	return NewActivePublisher(c, elector, ActivePublisherOptions{
		Endpoint: endpoint,
		// A path that cannot exist, so the CA-bundle read fails predictably and
		// the tests that care about it opt in explicitly.
		CABundlePath: filepath.Join(t.TempDir(), "absent-ca.crt"),
		Log:          testLog(),
	})
}

func getCluster(t *testing.T, c client.Client, name, namespace string) *controllerv1alpha1.Cluster {
	t.Helper()
	got := &controllerv1alpha1.Cluster{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, got))
	return got
}

// TestPublishOnce_WritesWhileTheWriteFenceIsShut is the regression test for a
// bug live testing caught: promotion calls PublishOnce at step 7, between
// taking the Lease and opening the write fence, and IsLeader() reports false
// for that entire window because promote() holds the fence latch across its
// whole sequence. Gating PublishOnce on IsLeader() therefore made step 7 a
// guaranteed no-op that still logged success, and status.activeController kept
// naming the dead hub until the periodic loop happened to run.
//
// The gate belongs on the periodic loop (see publishOnce), not here.
func TestPublishOnce_WritesWhileTheWriteFenceIsShut(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: false, identity: "hub-b"}, "https://hub-b.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	require.NotNil(t, got.Status.ActiveController,
		"promotion must be able to publish before it opens the fence")
	assert.Equal(t, "hub-b", got.Status.ActiveController.ActiveIdentity)
	assert.Equal(t, "https://hub-b.example.com:6443", got.Status.ActiveController.Endpoint)
}

// TestPublishOnce_PeriodicLoopStillSkipsWhenNotLeader is the other half, and
// carries the invariant that matters: a Standby must never write its own
// identity here. The mirror owns a Standby's copy of the field, and a worker
// tells the two hubs apart by which one names itself — so the loop that runs
// continuously in standby mode has to stay gated, and a hub that has lost its
// Lease has to stop advertising itself.
//
// Ungating PublishOnce does not weaken that: the only caller is the promotion
// sequence, which reaches it after taking the Lease and setting mode Active, at
// which point the hub is not a Standby any more.
func TestPublishOnce_PeriodicLoopStillSkipsWhenNotLeader(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: false, identity: "hub-b"}, "https://hub-b.example.com:6443")

	leader, err := p.publishOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, leader, "the loop must report it did not hold leadership")

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	assert.Nil(t, got.Status.ActiveController,
		"a hub that does not hold leadership must not advertise itself from the periodic loop")
}

func TestPublishOnce_WritesActiveControllerToEveryCluster(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"), newCluster("worker-2", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))

	for _, name := range []string{"worker-1", "worker-2"} {
		got := getCluster(t, c, name, "kubeslice-avesha")
		require.NotNil(t, got.Status.ActiveController, "publisher must declare on %s", name)
		assert.Equal(t, "https://hub-a.example.com:6443", got.Status.ActiveController.Endpoint)
		assert.Equal(t, "hub-a", got.Status.ActiveController.ActiveIdentity)
		assert.False(t, got.Status.ActiveController.LastUpdated.IsZero(), "LastUpdated must be stamped")
	}
}

func TestPublishOnce_SecondPassWritesNothing(t *testing.T) {
	writes := 0
	c := countingClusterClient(t, &writes, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))
	assert.Equal(t, 1, writes, "first pass must publish")

	require.NoError(t, p.PublishOnce(context.Background()))
	assert.Equal(t, 1, writes,
		"a converged pass must not write; comparing LastUpdated would make every tick write to every Cluster CR")
}

func TestPublishOnce_RepublishesWhenIdentityChanges(t *testing.T) {
	existing := newCluster("worker-1", "kubeslice-avesha")
	existing.Status.ActiveController = &controllerv1alpha1.ActiveControllerInfo{
		Endpoint:       "https://hub-a.example.com:6443",
		ActiveIdentity: "hub-a",
		LastUpdated:    metav1.NewTime(time.Now().Add(-time.Hour)),
	}
	c := clusterClient(t, existing)
	// hub-b has been promoted and now publishes about itself.
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-b"}, "https://hub-b.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	require.NotNil(t, got.Status.ActiveController)
	assert.Equal(t, "hub-b", got.Status.ActiveController.ActiveIdentity,
		"a promoted hub must overwrite the previous holder's declaration")
	assert.Equal(t, "https://hub-b.example.com:6443", got.Status.ActiveController.Endpoint)
}

func TestPublishOnce_RefusesShippedPlaceholderEndpoint(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, PlaceholderControllerEndpoint)

	require.NoError(t, p.PublishOnce(context.Background()),
		"refusing to publish must not be an error — a misconfigured endpoint must not stop the hub reconciling")

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	assert.Nil(t, got.Status.ActiveController,
		"publishing the shipped placeholder would advertise an unreachable failover target")
}

func TestPublishOnce_RefusesEmptyEndpoint(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "")

	require.NoError(t, p.PublishOnce(context.Background()))

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	assert.Nil(t, got.Status.ActiveController)
}

// TestPlaceholderMatchesServiceDefault pins the duplicated literal to its source.
// main.go overwrites service.ControllerEndpoint with the flag value at startup,
// so the publisher cannot read the default at runtime and must carry its own copy.
// If the shipped default ever changes, this fails instead of the publisher
// silently starting to advertise it.
func TestPlaceholderMatchesServiceDefault(t *testing.T) {
	assert.Equal(t, service.ControllerEndpoint, PlaceholderControllerEndpoint,
		"pkg/ha's placeholder copy has drifted from service.ControllerEndpoint's shipped default")
}

func TestNewActivePublisher_ReadsAndEncodesCABundle(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caPath, []byte("-----BEGIN CERTIFICATE-----\nfake\n"), 0o600))

	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := NewActivePublisher(c, stubLeadership{leader: true, identity: "hub-a"}, ActivePublisherOptions{
		Endpoint:     "https://hub-a.example.com:6443",
		CABundlePath: caPath,
		Log:          testLog(),
	})
	require.NoError(t, p.PublishOnce(context.Background()))

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	require.NotNil(t, got.Status.ActiveController)
	decoded, err := base64.StdEncoding.DecodeString(got.Status.ActiveController.CABundle)
	require.NoError(t, err, "caBundle must be base64-encoded PEM")
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nfake\n", string(decoded))
}

func TestNewActivePublisher_PublishesWithoutUnreadableCABundle(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	require.NotNil(t, got.Status.ActiveController,
		"an unreadable CA bundle must not block publication — endpoint and identity are what select a hub")
	assert.Empty(t, got.Status.ActiveController.CABundle)
}

func TestPublishOnce_ReturnsErrorWhenListFails(t *testing.T) {
	base := fake.NewClientBuilder().WithScheme(clusterScheme(t)).Build()
	c := interceptor.NewClient(base, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			return fmt.Errorf("simulated API server down")
		},
	})
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	assert.Error(t, p.PublishOnce(context.Background()),
		"a failed list must surface so the caller can retry, not be swallowed as success")
}

func TestPublishOnce_ContinuesAfterOneClusterFails(t *testing.T) {
	base := fake.NewClientBuilder().
		WithScheme(clusterScheme(t)).
		WithObjects(newCluster("worker-1", "kubeslice-avesha"), newCluster("worker-2", "kubeslice-avesha")).
		WithStatusSubresource(&controllerv1alpha1.Cluster{}).
		Build()
	c := interceptor.NewClient(base, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if obj.GetName() == "worker-1" {
				return fmt.Errorf("simulated conflict")
			}
			return c.Status().Update(ctx, obj)
		},
	})
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	assert.Error(t, p.PublishOnce(context.Background()), "the failing cluster must be reported")

	got := getCluster(t, c, "worker-2", "kubeslice-avesha")
	assert.NotNil(t, got.Status.ActiveController,
		"one cluster failing must not stop the others being published")
}

// lateLeadership becomes the leader only after IsLeader has been asked a few
// times, standing in for an Active whose Lease renewal lands a second or two
// after start-up.
type lateLeadership struct {
	mu     sync.Mutex
	checks int
	after  int
}

func (l *lateLeadership) IsLeader() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.checks++
	return l.checks > l.after
}
func (l *lateLeadership) Identity() string { return "hub-a" }

// TestStart_PublishesPromptlyWhenLeadershipArrivesLate pins the fix for a defect
// found in live testing: Start's first pass runs before the elector has acquired
// its Lease, so it skipped, and the next attempt was a full publish interval
// away — a fresh Active took 31s to advertise itself. While not the leader the
// loop must poll on the much shorter leadership interval instead.
// ---- read-after-write verification (the silently-pruned-field defect) ----

// pruningClusterClient emulates the API server behaviour that made this defect
// invisible: a Cluster CRD without status.activeController in its schema. The
// status write is ACCEPTED, and the field is silently dropped. Reads therefore
// never show it. Also counts Gets, so a test can assert the read-back stops once
// the field is confirmed.
func pruningClusterClient(t *testing.T, gets *int, prune bool, objs ...client.Object) client.Client {
	t.Helper()
	base := fake.NewClientBuilder().
		WithScheme(clusterScheme(t)).
		WithObjects(objs...).
		WithStatusSubresource(&controllerv1alpha1.Cluster{}).
		Build()
	return interceptor.NewClient(base, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if prune {
				if cl, ok := obj.(*controllerv1alpha1.Cluster); ok {
					cl.Status.ActiveController = nil // the API server drops the unknown field
				}
			}
			return c.SubResource(sub).Update(ctx, obj, opts...)
		},
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if gets != nil {
				*gets++
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
}

// TestPublishOnce_ReportsWhenTheFieldIsSilentlyPruned is the regression this
// defect deserves. Against a chart-installed hub the publisher logged success
// every 30s for hours while nothing persisted and no worker could discover the
// Active. An accepted write is not evidence.
func TestPublishOnce_ReportsWhenTheFieldIsSilentlyPruned(t *testing.T) {
	c := pruningClusterClient(t, nil, true, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	err := p.PublishOnce(context.Background())
	require.Error(t, err, "a write that did not persist must not be reported as a successful publish")
	assert.Contains(t, err.Error(), "did not persist")
	assert.Contains(t, err.Error(), "CRD",
		"the error has to name the likely cause, or an operator cannot act on it")
	assert.False(t, p.persistenceVerified.Load())
}

// One error per pass, not one per Cluster CR: every cluster shares the same CRD
// schema, so the rest would repeat the same message.
func TestPublishOnce_PruningReportsOncePerPassNotPerCluster(t *testing.T) {
	c := pruningClusterClient(t, nil, true,
		newCluster("worker-1", "kubeslice-avesha"),
		newCluster("worker-2", "kubeslice-avesha"),
		newCluster("worker-3", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	err := p.PublishOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, strings.Count(err.Error(), "did not persist"),
		"three clusters with the same broken schema must produce one report, not three")
}

// A hub restarting onto Cluster CRs that already carry its own declaration
// never writes, so it never reaches the read-back. Reading the value back off
// the API server is evidence enough on its own.
func TestPublishOnce_AlreadyConvergedCountsAsVerified(t *testing.T) {
	existing := newCluster("worker-1", "kubeslice-avesha")
	existing.Status.ActiveController = &controllerv1alpha1.ActiveControllerInfo{
		Endpoint:       "https://hub-a.example.com:6443",
		ActiveIdentity: "hub-a",
	}
	c := clusterClient(t, existing)
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))
	assert.True(t, p.persistenceVerified.Load(),
		"a field that already reads back as the desired value has demonstrably persisted")
}

func TestPublishOnce_VerifiesOnceThenStopsReadingBack(t *testing.T) {
	gets := 0
	c := pruningClusterClient(t, &gets, false, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: true, identity: "hub-a"}, "https://hub-a.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))
	assert.True(t, p.persistenceVerified.Load(), "a field that read back correctly must be recorded as verified")
	afterFirst := gets
	assert.Greater(t, afterFirst, 0, "the first pass has to read back to confirm anything")

	// Change the identity so the next pass genuinely writes again, and confirm it
	// does not pay for another read-back: the schema is a settled fact by now.
	p.elector = stubLeadership{leader: true, identity: "hub-b"}
	require.NoError(t, p.PublishOnce(context.Background()))
	assert.Equal(t, afterFirst, gets,
		"verification is once per process; re-reading every pass would double read traffic to prove a settled fact")
}

func TestStart_PublishesPromptlyWhenLeadershipArrivesLate(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := NewActivePublisher(c, &lateLeadership{after: 3}, ActivePublisherOptions{
		Endpoint: "https://hub-a.example.com:6443",
		// A publish interval far longer than the test's patience: if the loop
		// waits this out before retrying, the test fails.
		Interval:               time.Hour,
		LeadershipPollInterval: 5 * time.Millisecond,
		CABundlePath:           filepath.Join(t.TempDir(), "absent-ca.crt"),
		Log:                    testLog(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = p.Start(ctx) }()

	require.Eventually(t, func() bool {
		return getCluster(t, c, "worker-1", "kubeslice-avesha").Status.ActiveController != nil
	}, 2*time.Second, 5*time.Millisecond,
		"a hub that becomes leader after start-up must publish on the leadership poll interval, "+
			"not wait out a full publish interval")
}

func TestStart_ReturnsNilOnContextCancel(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := NewActivePublisher(c, stubLeadership{leader: true, identity: "hub-a"}, ActivePublisherOptions{
		Endpoint: "https://hub-a.example.com:6443",
		Interval: 10 * time.Millisecond,
		Log:      testLog(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Start(ctx) }()

	require.Eventually(t, func() bool {
		return getCluster(t, c, "worker-1", "kubeslice-avesha").Status.ActiveController != nil
	}, time.Second, 5*time.Millisecond, "Start must publish immediately, not wait for the first tick")

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err, "graceful shutdown must not be reported as an error")
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}
