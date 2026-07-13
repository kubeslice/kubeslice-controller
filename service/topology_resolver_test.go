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

package service

import (
	"reflect"
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

func TestResolveTopologyEdges(t *testing.T) {
	cases := []struct {
		name     string
		clusters []string
		topology *controllerv1alpha1.TopologySpec
		want     []TopologyEdge
	}{
		{
			name:     "nil topology is full mesh in cluster order",
			clusters: []string{"a", "b", "c"},
			topology: nil,
			want: []TopologyEdge{
				{ServerCluster: "a", ClientCluster: "b"},
				{ServerCluster: "a", ClientCluster: "c"},
				{ServerCluster: "b", ClientCluster: "c"},
			},
		},
		{
			name:     "explicit FullMesh is full mesh",
			clusters: []string{"a", "b", "c"},
			topology: &controllerv1alpha1.TopologySpec{Mode: controllerv1alpha1.TopologyModeFullMesh},
			want: []TopologyEdge{
				{ServerCluster: "a", ClientCluster: "b"},
				{ServerCluster: "a", ClientCluster: "c"},
				{ServerCluster: "b", ClientCluster: "c"},
			},
		},
		{
			name:     "hub and spoke: hub is server, no spoke-to-spoke",
			clusters: []string{"worker-1", "worker-2", "worker-3"},
			topology: &controllerv1alpha1.TopologySpec{Mode: controllerv1alpha1.TopologyModeHubAndSpoke, Hubs: []string{"worker-1"}},
			want: []TopologyEdge{
				{ServerCluster: "worker-1", ClientCluster: "worker-2"},
				{ServerCluster: "worker-1", ClientCluster: "worker-3"},
			},
		},
		{
			name:     "hub is server even when not first in cluster list",
			clusters: []string{"worker-1", "worker-2", "worker-3"},
			topology: &controllerv1alpha1.TopologySpec{Mode: controllerv1alpha1.TopologyModeHubAndSpoke, Hubs: []string{"worker-2"}},
			want: []TopologyEdge{
				{ServerCluster: "worker-2", ClientCluster: "worker-1"},
				{ServerCluster: "worker-2", ClientCluster: "worker-3"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveTopologyEdges(tc.clusters, tc.topology)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s:\n got  %v\n want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestTopologyEdgeSetContains(t *testing.T) {
	// hub-and-spoke edges: hub=worker-1, spokes worker-2/worker-3
	set := NewTopologyEdgeSet(ResolveTopologyEdges(
		[]string{"worker-1", "worker-2", "worker-3"},
		&controllerv1alpha1.TopologySpec{Mode: controllerv1alpha1.TopologyModeHubAndSpoke, Hubs: []string{"worker-1"}},
	))
	// desired hub<->spoke edges, both directions
	if !set.Contains("worker-1", "worker-2") || !set.Contains("worker-2", "worker-1") {
		t.Fatal("expected worker-1<->worker-2 to be a desired edge (either direction)")
	}
	if !set.Contains("worker-1", "worker-3") {
		t.Fatal("expected worker-1<->worker-3 to be a desired edge")
	}
	// spoke<->spoke is NOT desired
	if set.Contains("worker-2", "worker-3") || set.Contains("worker-3", "worker-2") {
		t.Fatal("did not expect worker-2<->worker-3 (spoke-to-spoke) to be a desired edge")
	}
}
