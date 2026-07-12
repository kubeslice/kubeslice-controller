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
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// TopologyEdge is a desired gateway connection between two clusters of a
// slice. ServerCluster hosts the gateway server side of the pair and
// ClientCluster dials in; for hub-and-spoke edges the hub is always the
// server so that spokes behind NAT never need to accept inbound connections.
type TopologyEdge struct {
	ServerCluster string
	ClientCluster string
}

// ResolveTopologyEdges computes the desired set of gateway connections for a
// slice from its member clusters and topology configuration. The result is
// deterministic. A nil topology or FullMesh mode yields every cluster pair in
// cluster-list order, matching existing full-mesh behavior. HubAndSpoke yields
// hub<->spoke edges (hub as server) for every hub, plus hub<->hub edges when
// more than one hub is configured.
func ResolveTopologyEdges(clusters []string, topology *controllerv1alpha1.TopologySpec) []TopologyEdge {
	edges := []TopologyEdge{}
	if topology == nil || topology.Mode != controllerv1alpha1.TopologyModeHubAndSpoke {
		for i := 0; i < len(clusters); i++ {
			for j := i + 1; j < len(clusters); j++ {
				edges = append(edges, TopologyEdge{ServerCluster: clusters[i], ClientCluster: clusters[j]})
			}
		}
		return edges
	}
	hubs := make(map[string]bool, len(topology.Hubs))
	for _, hub := range topology.Hubs {
		hubs[hub] = true
	}
	spokes := []string{}
	for _, cluster := range clusters {
		if !hubs[cluster] {
			spokes = append(spokes, cluster)
		}
	}
	for _, hub := range topology.Hubs {
		for _, spoke := range spokes {
			edges = append(edges, TopologyEdge{ServerCluster: hub, ClientCluster: spoke})
		}
	}
	for i := 0; i < len(topology.Hubs); i++ {
		for j := i + 1; j < len(topology.Hubs); j++ {
			edges = append(edges, TopologyEdge{ServerCluster: topology.Hubs[i], ClientCluster: topology.Hubs[j]})
		}
	}
	return edges
}
