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
)

func TestSliceConfigTopologyBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name           string
		topologyConfig *controllerv1alpha1.TopologyConfiguration
		expectedType   controllerv1alpha1.TopologyType
		description    string
	}{
		{
			name:           "nil topology config should default to full mesh",
			topologyConfig: nil,
			expectedType:   controllerv1alpha1.FULLMESH,
			description:    "Verifies backward compatibility when no topology config is provided",
		},
		{
			name:           "empty topology config should default to full mesh",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{},
			expectedType:   controllerv1alpha1.FULLMESH,
			description:    "Verifies default topology type is set to full mesh",
		},
		{
			name: "explicit full mesh topology should be preserved",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.FULLMESH,
			},
			expectedType: controllerv1alpha1.FULLMESH,
			description:  "Verifies explicit full mesh topology configuration is preserved",
		},
		{
			name: "hub-spoke topology should be preserved",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.HUBSPOKE,
				HubCluster:   "cluster-1",
			},
			expectedType: controllerv1alpha1.HUBSPOKE,
			description:  "Verifies hub-spoke topology configuration is preserved",
		},
		{
			name: "custom topology should be preserved",
			topologyConfig: &controllerv1alpha1.TopologyConfiguration{
				TopologyType: controllerv1alpha1.CUSTOM,
				ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
					{
						SourceCluster:  "cluster-1",
						TargetClusters: []string{"cluster-2"},
					},
				},
			},
			expectedType: controllerv1alpha1.CUSTOM,
			description:  "Verifies custom topology configuration is preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a slice config with the topology config
			sliceConfig := &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					TopologyConfig: tt.topologyConfig,
				},
			}

			// Apply the webhook default logic
			sliceConfig.Default()

			// Check that topology configuration is properly defaulted
			if sliceConfig.Spec.TopologyConfig == nil {
				t.Fatal("TopologyConfig should not be nil after applying defaults")
			}

			assert.Equal(t, tt.expectedType, sliceConfig.Spec.TopologyConfig.TopologyType, tt.description)
		})
	}
}

func TestTopologyConfigurationValidation(t *testing.T) {
	tests := []struct {
		name         string
		sliceConfig  *controllerv1alpha1.SliceConfig
		expectError  bool
		errorMessage string
		description  string
	}{
		{
			name: "valid full mesh topology",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
					TopologyConfig: &controllerv1alpha1.TopologyConfiguration{
						TopologyType: controllerv1alpha1.FULLMESH,
					},
				},
			},
			expectError: false,
			description: "Valid full mesh topology should pass validation",
		},
		{
			name: "valid hub-spoke topology",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
					TopologyConfig: &controllerv1alpha1.TopologyConfiguration{
						TopologyType: controllerv1alpha1.HUBSPOKE,
						HubCluster:   "cluster-1",
					},
				},
			},
			expectError: false,
			description: "Valid hub-spoke topology should pass validation",
		},
		{
			name: "invalid hub-spoke topology - missing hub cluster",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
					TopologyConfig: &controllerv1alpha1.TopologyConfiguration{
						TopologyType: controllerv1alpha1.HUBSPOKE,
					},
				},
			},
			expectError:  true,
			errorMessage: "hubCluster must be specified",
			description:  "Hub-spoke topology without hub cluster should fail validation",
		},
		{
			name: "invalid hub-spoke topology - hub cluster not in clusters list",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
					TopologyConfig: &controllerv1alpha1.TopologyConfiguration{
						TopologyType: controllerv1alpha1.HUBSPOKE,
						HubCluster:   "invalid-cluster",
					},
				},
			},
			expectError:  true,
			errorMessage: "is not present in the clusters list",
			description:  "Hub-spoke topology with invalid hub cluster should fail validation",
		},
		{
			name: "valid custom topology",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
					TopologyConfig: &controllerv1alpha1.TopologyConfiguration{
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
				},
			},
			expectError: false,
			description: "Valid custom topology should pass validation",
		},
		{
			name: "invalid custom topology - missing connectivity matrix",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
					TopologyConfig: &controllerv1alpha1.TopologyConfiguration{
						TopologyType: controllerv1alpha1.CUSTOM,
					},
				},
			},
			expectError:  true,
			errorMessage: "connectivityMatrix must be specified",
			description:  "Custom topology without connectivity matrix should fail validation",
		},
		{
			name: "invalid custom topology - invalid source cluster",
			sliceConfig: &controllerv1alpha1.SliceConfig{
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters: []string{"cluster-1", "cluster-2", "cluster-3"},
					TopologyConfig: &controllerv1alpha1.TopologyConfiguration{
						TopologyType: controllerv1alpha1.CUSTOM,
						ConnectivityMatrix: []controllerv1alpha1.ClusterConnectivity{
							{
								SourceCluster:  "invalid-cluster",
								TargetClusters: []string{"cluster-2"},
							},
						},
					},
				},
			},
			expectError:  true,
			errorMessage: "is not present in clusters list",
			description:  "Custom topology with invalid source cluster should fail validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation function directly
			fieldError := validateTopologyConfiguration(tt.sliceConfig)

			if tt.expectError {
				assert.NotNil(t, fieldError, tt.description)
				if tt.errorMessage != "" {
					assert.Contains(t, fieldError.Error(), tt.errorMessage, tt.description)
				}
			} else {
				assert.Nil(t, fieldError, tt.description)
			}
		})
	}
}

func TestVPNDeploymentTypeConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		vpnConfigs  []controllerv1alpha1.ClusterVPNConfiguration
		clusterName string
		expected    controllerv1alpha1.VPNDeploymentType
	}{
		{
			name:        "no VPN config should return auto",
			vpnConfigs:  nil,
			clusterName: "cluster-1",
			expected:    controllerv1alpha1.VPNAUTO,
		},
		{
			name: "cluster with server config should return server",
			vpnConfigs: []controllerv1alpha1.ClusterVPNConfiguration{
				{
					ClusterName:       "cluster-1",
					VPNDeploymentType: controllerv1alpha1.VPNSERVER,
				},
			},
			clusterName: "cluster-1",
			expected:    controllerv1alpha1.VPNSERVER,
		},
		{
			name: "cluster with client config should return client",
			vpnConfigs: []controllerv1alpha1.ClusterVPNConfiguration{
				{
					ClusterName:       "cluster-1",
					VPNDeploymentType: controllerv1alpha1.VPNCLIENT,
				},
			},
			clusterName: "cluster-1",
			expected:    controllerv1alpha1.VPNCLIENT,
		},
		{
			name: "cluster not in config should return auto",
			vpnConfigs: []controllerv1alpha1.ClusterVPNConfiguration{
				{
					ClusterName:       "cluster-2",
					VPNDeploymentType: controllerv1alpha1.VPNSERVER,
				},
			},
			clusterName: "cluster-1",
			expected:    controllerv1alpha1.VPNAUTO,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := NewTopologyService()
			topologyConfig := &controllerv1alpha1.TopologyConfiguration{
				ClusterVPNConfig: tt.vpnConfigs,
			}

			result := ts.GetVPNDeploymentType(topologyConfig, tt.clusterName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
