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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func kickObject(gvk schema.GroupVersionKind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

// kickClient builds a fake client that can list the mirrored CRD types.
func kickClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(clusterScheme(t)).WithObjects(objs...).Build()
}

func drain(ch <-chan event.GenericEvent) []string {
	var names []string
	for {
		select {
		case ev := <-ch:
			names = append(names, ev.Object.GetName())
		default:
			return names
		}
	}
}

func TestKick_DeliversOneEventPerObject(t *testing.T) {
	c := kickClient(t,
		newClusterObj("worker-1", "kubeslice-avesha"),
		newClusterObj("worker-2", "kubeslice-avesha"),
	)
	k := NewReconcileKicker(c, []schema.GroupVersionKind{GVKCluster}, testLog())

	require.NoError(t, k.Kick(context.Background()))

	got := drain(k.Source(GVKCluster))
	assert.ElementsMatch(t, []string{"worker-1", "worker-2"}, got,
		"every existing object must be re-enqueued; that is the whole point of the kick")
}

// TestKick_ChannelsAreDisjointPerType is the trap this component exists to
// avoid. Every source.Channel starts its own goroutine reading the channel it
// was handed; nine sources sharing one Go channel would mean nine goroutines
// competing for each value, so each type would receive an arbitrary subset of
// everything and most objects would reach the wrong controller. With a couple
// of objects in a test that still looks like it works, which is what makes it
// dangerous.
func TestKick_ChannelsAreDisjointPerType(t *testing.T) {
	c := kickClient(t,
		newClusterObj("worker-1", "kubeslice-avesha"),
		newProjectObj("avesha", "kubeslice-controller"),
	)
	k := NewReconcileKicker(c, []schema.GroupVersionKind{GVKCluster, GVKProject}, testLog())

	require.NoError(t, k.Kick(context.Background()))

	assert.Equal(t, []string{"worker-1"}, drain(k.Source(GVKCluster)),
		"the Cluster channel must carry only Clusters")
	assert.Equal(t, []string{"avesha"}, drain(k.Source(GVKProject)),
		"the Project channel must carry only Projects")
}

func TestKick_EmptyClusterIsNotAnError(t *testing.T) {
	k := NewReconcileKicker(kickClient(t), []schema.GroupVersionKind{GVKCluster}, testLog())

	require.NoError(t, k.Kick(context.Background()), "a hub with nothing to reconcile is fine")
	assert.Empty(t, drain(k.Source(GVKCluster)))
}

// TestKick_ContinuesAfterOneTypeFails: partial coverage beats none. The
// alternative is a promoted hub with nothing reconciled at all because one API
// call failed.
func TestKick_ContinuesAfterOneTypeFails(t *testing.T) {
	base := fake.NewClientBuilder().WithScheme(clusterScheme(t)).
		WithObjects(newProjectObj("avesha", "kubeslice-controller")).Build()
	c := interceptor.NewClient(base, interceptor.Funcs{
		List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if list.GetObjectKind().GroupVersionKind().Kind == "ClusterList" {
				return fmt.Errorf("simulated list failure")
			}
			return cl.List(ctx, list, opts...)
		},
	})
	k := NewReconcileKicker(c, []schema.GroupVersionKind{GVKCluster, GVKProject}, testLog())

	err := k.Kick(context.Background())
	require.Error(t, err, "the failing type must be reported")
	assert.Contains(t, err.Error(), "Cluster")
	assert.Equal(t, []string{"avesha"}, drain(k.Source(GVKProject)),
		"one type failing must not stop the others being kicked")
}

// TestKick_DoesNotBlockOnAFullChannel matters because of when the kick runs.
// Its consumers are controller-runtime sources, which only start draining once
// the manager is running, and main.go starts the promotion path before
// mgr.Start. A blocking send in that window would hang promotion on a channel
// nobody is reading — on a hub that has already taken leadership.
func TestKick_DoesNotBlockOnAFullChannel(t *testing.T) {
	var objs []client.Object
	for i := 0; i < DefaultKickChannelBuffer+25; i++ {
		objs = append(objs, newClusterObj(fmt.Sprintf("worker-%d", i), "kubeslice-avesha"))
	}
	k := NewReconcileKicker(kickClient(t, objs...), []schema.GroupVersionKind{GVKCluster}, testLog())

	done := make(chan error, 1)
	go func() { done <- k.Kick(context.Background()) }()

	select {
	case err := <-done:
		assert.NoError(t, err, "dropping events on a full channel is degradation, not failure")
	case <-time.After(3 * time.Second):
		t.Fatal("Kick blocked on a full channel; nothing drains these until the manager starts, " +
			"so this would hang promotion on an already-promoted hub")
	}
	assert.Len(t, drain(k.Source(GVKCluster)), DefaultKickChannelBuffer,
		"the channel should have filled to its buffer and the rest dropped")
}

// TestKick_RespectsContextCancellation must hold on every run, not most of
// them. Written first with ctx.Done() as a select case beside the send, it
// passed and failed at random: both cases are ready whenever the channel has
// room, and select chooses among ready cases uniformly. -shuffle surfaced it.
func TestKick_RespectsContextCancellation(t *testing.T) {
	var objs []client.Object
	for i := 0; i < 20; i++ {
		objs = append(objs, newClusterObj(fmt.Sprintf("worker-%d", i), "kubeslice-avesha"))
	}
	k := NewReconcileKicker(kickClient(t, objs...), []schema.GroupVersionKind{GVKCluster}, testLog())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for i := 0; i < 50; i++ {
		require.Error(t, k.Kick(ctx),
			"a cancelled context must abort the kick deterministically, not on a coin flip")
	}
	assert.Empty(t, drain(k.Source(GVKCluster)),
		"and nothing may have been delivered after cancellation")
}

func TestSource_UnknownTypeIsNil(t *testing.T) {
	k := NewReconcileKicker(kickClient(t), []schema.GroupVersionKind{GVKCluster}, testLog())
	assert.Nil(t, k.Source(GVKProject),
		"a type the kicker does not cover must yield nil, so the caller simply registers no watch")
}

// TestNilKicker_IsSafe covers the standalone path: main.go builds a kicker
// unconditionally, but a nil one must behave as "no kick" rather than panic.
func TestNilKicker_IsSafe(t *testing.T) {
	var k *ReconcileKicker
	assert.Nil(t, k.Source(GVKCluster))
	assert.NoError(t, k.Kick(context.Background()))
}

func TestReconciledGVKs_CoversEveryReconciledTypeAndNothingElse(t *testing.T) {
	got := ReconciledGVKs()
	assert.Len(t, got, 9, "there are nine reconcilers; each needs a channel")

	// Namespace is mirrored so contents have somewhere to land, but no
	// reconciler owns it, so kicking it would deliver events nothing handles.
	for _, g := range got {
		assert.NotEqual(t, "Namespace", g.Kind)
	}

	// Everything kicked must be something the mirror actually maintains,
	// otherwise a promoted hub would be kicking objects it never received.
	mirrored := map[schema.GroupVersionKind]bool{}
	for _, res := range CRDMirrorSet {
		mirrored[res.GVK] = true
	}
	for _, g := range got {
		assert.True(t, mirrored[g], "%s is kicked but not mirrored", g.Kind)
	}

	// And the returned slice must be a copy.
	got[0] = schema.GroupVersionKind{Kind: "Tampered"}
	assert.NotEqual(t, "Tampered", ReconciledGVKs()[0].Kind)
}

func newClusterObj(name, namespace string) client.Object {
	return kickObject(GVKCluster, namespace, name)
}

func newProjectObj(name, namespace string) client.Object {
	return kickObject(GVKProject, namespace, name)
}
