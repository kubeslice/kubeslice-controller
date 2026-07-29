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

func TestPublishOnce_NoopWhenNotLeader(t *testing.T) {
	c := clusterClient(t, newCluster("worker-1", "kubeslice-avesha"))
	p := testPublisher(t, c, stubLeadership{leader: false, identity: "hub-b"}, "https://hub-b.example.com:6443")

	require.NoError(t, p.PublishOnce(context.Background()))

	got := getCluster(t, c, "worker-1", "kubeslice-avesha")
	assert.Nil(t, got.Status.ActiveController,
		"a Standby must never write its own identity here — the mirror owns its copy, and a worker "+
			"tells the hubs apart by which one names itself")
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
