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

package service

import (
	"fmt"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// ITopologyService defines the interface for topology management
type ITopologyService interface {
	// ValidateTopologyConfiguration validates the topology configuration
	ValidateTopologyConfiguration(topologyConfig *controllerv1alpha1.TopologyConfiguration, clusters []string) error

	// GetConnectivityMatrix returns the connectivity matrix based on topology configuration
	GetConnectivityMatrix(topologyConfig *controllerv1alpha1.TopologyConfiguration, clusters []string) ([]ClusterPair, error)

	// GetVPNDeploymentType returns the VPN deployment type for a specific cluster
	GetVPNDeploymentType(topologyConfig *controllerv1alpha1.TopologyConfiguration, clusterName string) controllerv1alpha1.VPNDeploymentType
}

// TopologyService implements ITopologyService
type TopologyService struct{}

// NewTopologyService creates a new topology service
func NewTopologyService() *TopologyService {
	return &TopologyService{}
}

// ValidateTopologyConfiguration validates the topology configuration
func (t *TopologyService) ValidateTopologyConfiguration(topologyConfig *controllerv1alpha1.TopologyConfiguration, clusters []string) error {
	if topologyConfig == nil {
		// If no topology config is provided, default to full-mesh (backward compatibility)
		return nil
	}

	clusterSet := make(map[string]bool)
	for _, cluster := range clusters {
		clusterSet[cluster] = true
	}

	switch topologyConfig.TopologyType {
	case controllerv1alpha1.HUBSPOKE:
		if topologyConfig.HubCluster == "" {
			return fmt.Errorf("hubCluster must be specified for hub-spoke topology")
		}
		if !clusterSet[topologyConfig.HubCluster] {
			return fmt.Errorf("hub cluster '%s' is not present in the clusters list", topologyConfig.HubCluster)
		}

	case controllerv1alpha1.CUSTOM, controllerv1alpha1.PARTIALMESH:
		if len(topologyConfig.ConnectivityMatrix) == 0 {
			return fmt.Errorf("connectivityMatrix must be specified for %s topology", topologyConfig.TopologyType)
		}

		// Validate connectivity matrix
		for _, connectivity := range topologyConfig.ConnectivityMatrix {
			if !clusterSet[connectivity.SourceCluster] {
				return fmt.Errorf("source cluster '%s' in connectivity matrix is not present in clusters list", connectivity.SourceCluster)
			}
			for _, targetCluster := range connectivity.TargetClusters {
				if !clusterSet[targetCluster] {
					return fmt.Errorf("target cluster '%s' in connectivity matrix is not present in clusters list", targetCluster)
				}
				if connectivity.SourceCluster == targetCluster {
					return fmt.Errorf("source cluster cannot be the same as target cluster: %s", targetCluster)
				}
			}
		}

		// Partial mesh reachability validation (DFS)
		if topologyConfig.TopologyType == controllerv1alpha1.PARTIALMESH {
			adjacency := make(map[string]map[string]bool)
			for _, cluster := range clusters {
				adjacency[cluster] = make(map[string]bool)
			}
			for _, connectivity := range topologyConfig.ConnectivityMatrix {
				for _, target := range connectivity.TargetClusters {
					adjacency[connectivity.SourceCluster][target] = true
					adjacency[target][connectivity.SourceCluster] = true
				}
			}
			visited := make(map[string]bool)
			if len(clusters) > 0 {
				var dfsVisit func(string)
				dfsVisit = func(cluster string) {
					visited[cluster] = true
					for neighbor := range adjacency[cluster] {
						if !visited[neighbor] {
							dfsVisit(neighbor)
						}
					}
				}
				dfsVisit(clusters[0])
			}
			for _, cluster := range clusters {
				if !visited[cluster] {
					return fmt.Errorf("cluster '%s' is not reachable in the partial mesh topology", cluster)
				}
			}
		}
	}

	// Validate cluster VPN configurations
	for _, vpnConfig := range topologyConfig.ClusterVPNConfig {
		if !clusterSet[vpnConfig.ClusterName] {
			return fmt.Errorf("cluster '%s' in VPN configuration is not present in clusters list", vpnConfig.ClusterName)
		}
	}

	return nil
}

// GetConnectivityMatrix returns the connectivity matrix based on topology configuration
func (t *TopologyService) GetConnectivityMatrix(topologyConfig *controllerv1alpha1.TopologyConfiguration, clusters []string) ([]ClusterPair, error) {
	if topologyConfig == nil || topologyConfig.TopologyType == controllerv1alpha1.FULLMESH {
		return t.getFullMeshConnectivity(clusters), nil
	}

	switch topologyConfig.TopologyType {
	case controllerv1alpha1.HUBSPOKE:
		return t.getHubSpokeConnectivity(topologyConfig.HubCluster, clusters), nil

	case controllerv1alpha1.PARTIALMESH, controllerv1alpha1.CUSTOM:
		return t.getCustomConnectivity(topologyConfig.ConnectivityMatrix), nil

	default:
		return t.getFullMeshConnectivity(clusters), nil
	}
}

// GetVPNDeploymentType returns the VPN deployment type for a specific cluster
func (t *TopologyService) GetVPNDeploymentType(topologyConfig *controllerv1alpha1.TopologyConfiguration, clusterName string) controllerv1alpha1.VPNDeploymentType {
	if topologyConfig == nil {
		return controllerv1alpha1.VPNAUTO
	}

	for _, vpnConfig := range topologyConfig.ClusterVPNConfig {
		if vpnConfig.ClusterName == clusterName {
			return vpnConfig.VPNDeploymentType
		}
	}

	return controllerv1alpha1.VPNAUTO
}

// getFullMeshConnectivity generates full mesh connectivity matrix
func (t *TopologyService) getFullMeshConnectivity(clusters []string) []ClusterPair {
	var pairs []ClusterPair
	noClusters := len(clusters)

	for i := 0; i < noClusters; i++ {
		for j := i + 1; j < noClusters; j++ {
			pairs = append(pairs, ClusterPair{
				SourceCluster:      clusters[i],
				DestinationCluster: clusters[j],
			})
		}
	}

	return pairs
}

// getHubSpokeConnectivity generates hub-spoke connectivity matrix
func (t *TopologyService) getHubSpokeConnectivity(hubCluster string, clusters []string) []ClusterPair {
	var pairs []ClusterPair

	for _, cluster := range clusters {
		if cluster != hubCluster {
			pairs = append(pairs, ClusterPair{
				SourceCluster:      hubCluster,
				DestinationCluster: cluster,
			})
		}
	}

	return pairs
}

// getCustomConnectivity generates connectivity matrix from custom configuration
func (t *TopologyService) getCustomConnectivity(connectivityMatrix []controllerv1alpha1.ClusterConnectivity) []ClusterPair {
	var pairs []ClusterPair
	seenPairs := make(map[string]bool)

	for _, connectivity := range connectivityMatrix {
		for _, targetCluster := range connectivity.TargetClusters {
			// Create a unique key for the pair to avoid duplicates
			key1 := fmt.Sprintf("%s-%s", connectivity.SourceCluster, targetCluster)
			key2 := fmt.Sprintf("%s-%s", targetCluster, connectivity.SourceCluster)

			// Only add the pair if we haven't seen it before (in either direction)
			if !seenPairs[key1] && !seenPairs[key2] {
				pairs = append(pairs, ClusterPair{
					SourceCluster:      connectivity.SourceCluster,
					DestinationCluster: targetCluster,
				})
				seenPairs[key1] = true
				seenPairs[key2] = true
			}
		}
	}

	return pairs
}
