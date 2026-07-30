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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// failingReadClient returns a client whose Get always fails with a transport-
// style error, simulating an API server that is unreachable rather than one
// that answers "not found".
func failingReadClient(t *testing.T) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(testScheme(t)).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			return fmt.Errorf("simulated API server unreachable")
		},
	}).Build()
}

// TestSelfHealthy_NotFoundCountsAsHealthy is the guard's most important test.
// On a first-ever promotion there is no Lease on this hub's own cluster yet, so
// the self-health read returns NotFound — and NotFound means the API server
// ANSWERED, which is exactly the thing being tested. Treating it as unhealthy
// would block every real first failover while looking perfectly reasonable in
// review.
func TestSelfHealthy_NotFoundCountsAsHealthy(t *testing.T) {
	// An empty local cluster: the HA Lease does not exist yet.
	e := NewClusterLeaderElector(fakeClient(t), fakeClient(t), Options{Mode: ModeStandby, Log: testLog()})

	assert.True(t, e.selfHealthy(context.Background()),
		"NotFound means the API server answered — on a first-ever promotion no local Lease exists, "+
			"and treating that as unhealthy would block every real first failover")
}

func TestSelfHealthy_ExistingLeaseIsHealthy(t *testing.T) {
	local := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-b", time.Now()))
	e := NewClusterLeaderElector(local, fakeClient(t), Options{Mode: ModeStandby, Log: testLog()})

	assert.True(t, e.selfHealthy(context.Background()))
}

func TestSelfHealthy_UnreachableIsUnhealthy(t *testing.T) {
	e := NewClusterLeaderElector(failingReadClient(t), fakeClient(t), Options{Mode: ModeStandby, Log: testLog()})

	assert.False(t, e.selfHealthy(context.Background()),
		"a transport error against our own API server means we may be the broken one, not the Active")
}

func TestActiveStillAlive_FreshLeaseAborts(t *testing.T) {
	remote := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now()))
	e := NewClusterLeaderElector(fakeClient(t), remote, Options{Mode: ModeStandby, Log: testLog()})

	assert.True(t, e.activeStillAlive(context.Background()),
		"the Active renewed between polls; this is the polling race the final dial exists to catch")
}

// TestActiveStillAlive_RefreshesCacheOnAbort: aborting must leave the elector
// better informed, not latched on the stale verdict it just disproved.
func TestActiveStillAlive_RefreshesCacheOnAbort(t *testing.T) {
	remote := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now()))
	e := NewClusterLeaderElector(fakeClient(t), remote, Options{Mode: ModeStandby, Log: testLog()})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))

	require.True(t, e.activeStillAlive(context.Background()))

	candidate, err := e.checkRemoteLeaseOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, candidate, "the refreshed view must clear candidacy on the next tick")
}

func TestActiveStillAlive_UnreachableProceeds(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{Mode: ModeStandby, Log: testLog()})

	assert.False(t, e.activeStillAlive(context.Background()),
		"an unreachable Active is not evidence of life; the final dial buys almost nothing on this path")
}

func TestActiveStillAlive_StaleLeaseProceeds(t *testing.T) {
	remote := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour)))
	e := NewClusterLeaderElector(fakeClient(t), remote, Options{Mode: ModeStandby, Log: testLog()})

	assert.False(t, e.activeStillAlive(context.Background()),
		"reachable but still stale confirms the verdict rather than refuting it")
}

func TestGuardsAllowPromotion_BothPass(t *testing.T) {
	e := NewClusterLeaderElector(fakeClient(t), failingReadClient(t), Options{Mode: ModeStandby, Log: testLog()})

	assert.True(t, e.guardsAllowPromotion(context.Background()),
		"own API server reachable (NotFound) and Active unreachable: promotion may proceed")
}

func TestGuardsAllowPromotion_SelfUnhealthyBlocks(t *testing.T) {
	// Both sides unreachable — the classic "it's me, not them" case.
	e := NewClusterLeaderElector(failingReadClient(t), failingReadClient(t), Options{Mode: ModeStandby, Log: testLog()})

	assert.False(t, e.guardsAllowPromotion(context.Background()),
		"if this hub cannot reach its own API server, the evidence is equally consistent with "+
			"this hub being the broken one")
}

func TestGuardsAllowPromotion_LiveActiveBlocks(t *testing.T) {
	remote := fakeClient(t, newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now()))
	e := NewClusterLeaderElector(fakeClient(t), remote, Options{Mode: ModeStandby, Log: testLog()})

	assert.False(t, e.guardsAllowPromotion(context.Background()))
}

// TestGuardsAbort_DoesNotDisarm: an abort must change nothing except the
// promotion attempt itself. Clearing the cached Lease would silently disarm the
// elector, and an already-gone Active can never re-arm it — turning one
// transient guard failure into a hub that will never fail over again.
func TestGuardsAbort_DoesNotDisarm(t *testing.T) {
	e := NewClusterLeaderElector(failingReadClient(t), failingReadClient(t), Options{Mode: ModeStandby, Log: testLog()})
	e.lastSeenLease = newLease(DefaultLeaseName, DefaultLeaseNamespace, "hub-a", time.Now().Add(-time.Hour))

	require.False(t, e.guardsAllowPromotion(context.Background()))

	assert.NotNil(t, e.lastSeenLease, "aborting must not clear the cached lease")
	candidate, _ := e.checkRemoteLeaseOnce(context.Background())
	assert.True(t, candidate, "the next tick must re-evaluate and still see a candidate")
}

// TestPromotionDialTimeout_IsApplied proves the bound exists at all. main.go
// builds the remote client with no timeout, so an unbounded guard would hang
// for the OS TCP timeout — minutes, far outside the failover budget.
func TestPromotionDialTimeout_IsApplied(t *testing.T) {
	blocked := make(chan struct{})
	defer close(blocked)

	hanging := fake.NewClientBuilder().WithScheme(testScheme(t)).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-blocked:
				return nil
			}
		},
	}).Build()

	e := NewClusterLeaderElector(hanging, hanging, Options{
		Mode:                 ModeStandby,
		PromotionDialTimeout: 50 * time.Millisecond,
		Log:                  testLog(),
	})

	done := make(chan bool, 1)
	go func() { done <- e.selfHealthy(context.Background()) }()

	select {
	case healthy := <-done:
		assert.False(t, healthy, "a timed-out read is not proof of a healthy self")
	case <-time.After(2 * time.Second):
		t.Fatal("selfHealthy did not respect PromotionDialTimeout — an unbounded read would " +
			"hang until the OS TCP timeout")
	}
}
