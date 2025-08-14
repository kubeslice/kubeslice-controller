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
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopologyService_ValidateTopologyConfiguration(t *testing.T) {
	ts := NewTopologyService()
	clusters := []string{"cluster-1", "cluster-2", "cluster-3"}

	tests := []struct {
		name           string
		topologyConfig *controllerv1alpha1.TopologyConfiguration
		clusters       []string
		expectedError  string
	}{
		{
			name:           "nil topology config should be valid",
			topologyConfig: nil,
			clusters:       clusters,
			expectedError:  "",
		},
		{
			name: "full mesh topology should be valid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.FULLMESH,
			},
			clusters:      clusters,
			expectedError: "",
		},
		{
			name: "hub-spoke topology without hub cluster should be invalid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.HUBSPOKE,
			},
			clusters:      clusters,
			expectedError: "hubCluster must be specified for hub-spoke topology",
		},
		{
			name: "hub-spoke topology with invalid hub cluster should be invalid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.HUBSPOKE,
				HubCluster:   "invalid-cluster",
			},
			clusters:      clusters,
			expectedError: "hub cluster 'invalid-cluster' is not present in the clusters list",
		},
		{
			name: "hub-spoke topology with valid hub cluster should be valid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.HUBSPOKE,
				HubCluster:   "cluster-1",
			},
			clusters:      clusters,
			expectedError: "",
		},
		{
			name: "custom topology without connectivity matrix should be invalid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
			},
			clusters:      clusters,
			expectedError: "connectivityMatrix must be specified for custom topology",
		},
		{
			name: "custom topology with invalid source cluster should be invalid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
				ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
					{
						SourceCluster:  "invalid-cluster",
						TargetClusters: []string{"cluster-2"},
					},
				},
			},
			clusters:      clusters,
			expectedError: "source cluster 'invalid-cluster' in connectivity matrix is not present in clusters list",
		},
		{
			name: "custom topology with invalid target cluster should be invalid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
				ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
					{
						SourceCluster:  "cluster-1",
						TargetClusters: []string{"invalid-cluster"},
					},
				},
			},
			clusters:      clusters,
			expectedError: "target cluster 'invalid-cluster' in connectivity matrix is not present in clusters list",
		},
		{
			name: "custom topology with source equal to target should be invalid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
				ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
					{
						SourceCluster:  "cluster-1",
						TargetClusters: []string{"cluster-1"},
					},
				},
			},
			clusters:      clusters,
			expectedError: "source cluster cannot be the same as target cluster: cluster-1",
		},
		{
			name: "valid custom topology should be valid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
				ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
					{
						SourceCluster:  "cluster-1",
						TargetClusters: []string{"cluster-2"},
					},
					{
						SourceCluster:  "cluster-2",
						TargetClusters: []string{"cluster-3"},
					},
				},
			},
			clusters:      clusters,
			expectedError: "",
		},
		{
			name: "VPN configuration with invalid cluster should be invalid",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.FULLMESH,
				ClusterVPNConfig: []controllerv1alpha1.ClusterVPNConfiguration{
					{
						ClusterName:       "invalid-cluster",
						VPNDeploymentType: controllerv1alpha1.VPNSERVER,
					},
				},
			},
			clusters:      clusters,
			expectedError: "cluster 'invalid-cluster' in VPN configuration is not present in clusters list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ts.ValidateTopologyConfiguration(tt.topologyConfig, tt.clusters)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

func TestTopologyService_GetConnectivityMatrix(t *testing.T) {
	ts := NewTopologyService()
	clusters := []string{"cluster-1", "cluster-2", "cluster-3"}

	tests := []struct {
		name           string
		topologyConfig *controllerv1alpha1.TopologyConfiguration
		clusters       []string
		expectedPairs  []ClusterPair
	}{
		{
			name:           "nil topology config should return full mesh",
			topologyConfig: nil,
			clusters:       clusters,
			expectedPairs: []ClusterPair{
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-2"},
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-3"},
				{SourceCluster: "cluster-2", DestinationCluster: "cluster-3"},
			},
		},
		{
			name: "full mesh topology should return all pairs",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.FULLMESH,
			},
			clusters: clusters,
			expectedPairs: []ClusterPair{
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-2"},
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-3"},
				{SourceCluster: "cluster-2", DestinationCluster: "cluster-3"},
			},
		},
		{
			name: "hub-spoke topology should return hub to all spokes",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.HUBSPOKE,
				HubCluster:   "cluster-1",
			},
			clusters: clusters,
			expectedPairs: []ClusterPair{
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-2"},
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-3"},
			},
		},
		{
			name: "custom topology should return specified connections",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
				ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
					{
						SourceCluster:  "cluster-1",
						TargetClusters: []string{"cluster-2"},
					},
					{
						SourceCluster:  "cluster-2",
						TargetClusters: []string{"cluster-3"},
					},
				},
			},
			clusters: clusters,
			expectedPairs: []ClusterPair{
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-2"},
				{SourceCluster: "cluster-2", DestinationCluster: "cluster-3"},
			},
		},
		{
			name: "custom topology should avoid duplicates",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
				ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
					{
						SourceCluster:  "cluster-1",
						TargetClusters: []string{"cluster-2"},
					},
					{
						SourceCluster:  "cluster-2",
						TargetClusters: []string{"cluster-1"},
					},
				},
			},
			clusters: clusters,
			expectedPairs: []ClusterPair{
				{SourceCluster: "cluster-1", DestinationCluster: "cluster-2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, err := ts.GetConnectivityMatrix(tt.topologyConfig, tt.clusters)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedPairs, pairs)
		})
	}
}

func TestTopologyService_GetVPNDeploymentType(t *testing.T) {
	ts := NewTopologyService()

	tests := []struct {
		name           string
		topologyConfig *controllerv1alpha1.TopologyConfiguration
		clusterName    string
		expectedType   controllerv1alpha1.VPNDeploymentType
	}{
		{
			name:           "nil topology config should return auto",
			topologyConfig: nil,
			clusterName:    "cluster-1",
			expectedType:   controllerv1alpha1.VPNAUTO,
		},
		{
			name: "cluster with explicit VPN config should return configured type",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.FULLMESH,
				ClusterVPNConfig: []controllerv1alpha1.ClusterVPNConfiguration{
					{
						ClusterName:       "cluster-1",
						VPNDeploymentType: controllerv1alpha1.VPNSERVER,
					},
				},
			},
			clusterName:  "cluster-1",
			expectedType: controllerv1alpha1.VPNSERVER,
		},
		{
			name: "cluster without explicit VPN config should return auto",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.FULLMESH,
				ClusterVPNConfig: []controllerv1alpha1.ClusterVPNConfiguration{
					{
						ClusterName:       "cluster-1",
						VPNDeploymentType: controllerv1alpha1.VPNSERVER,
					},
				},
			},
			clusterName:  "cluster-2",
			expectedType: controllerv1alpha1.VPNAUTO,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vpnType := ts.GetVPNDeploymentType(tt.topologyConfig, tt.clusterName)
			assert.Equal(t, tt.expectedType, vpnType)
		})
	}
}

// Test helper function for partial mesh connectivity validation
func TestPartialMeshConnectivity(t *testing.T) {
	ts := NewTopologyService()

	tests := []struct {
		name               string
		clusters           []string
		connectivityMatrix []controllerv1alpha1.ClusterConnectivity
		shouldBeValid      bool
	}{
		{
			name:     "connected partial mesh should be valid",
			clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
			connectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
				{
					SourceCluster:  "cluster-1",
					TargetClusters: []string{"cluster-2"},
				},
				{
					SourceCluster:  "cluster-2",
					TargetClusters: []string{"cluster-3"},
				},
			},
			shouldBeValid: true,
		},
		{
			name:     "disconnected partial mesh should be invalid",
			clusters: []string{"cluster-1", "cluster-2", "cluster-3", "cluster-4"},
			connectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
				{
					SourceCluster:  "cluster-1",
					TargetClusters: []string{"cluster-2"},
				},
				{
					SourceCluster:  "cluster-3",
					TargetClusters: []string{"cluster-4"},
				},
			},
			shouldBeValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topologyConfig := &controllerv1alpha1.TopologyConfiguration{
				TopologyType:       controllerv1alpha1.PARTIALMESH,
				ConnectivityMatrix: tt.connectivityMatrix,
			}

			err := ts.ValidateTopologyConfiguration(topologyConfig, tt.clusters)
			if tt.shouldBeValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "is not reachable")
			}
		})
	}
}
