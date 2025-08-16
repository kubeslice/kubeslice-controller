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

package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIpamAllocator_ValidateSliceSubnet(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name        string
		sliceSubnet string
		expectError bool
	}{
		{
			name:        "Valid IPv4 CIDR",
			sliceSubnet: "10.1.0.0/16",
			expectError: false,
		},
		{
			name:        "Valid IPv4 CIDR /24",
			sliceSubnet: "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "Invalid CIDR format",
			sliceSubnet: "10.1.0.0",
			expectError: true,
		},
		{
			name:        "Invalid IP address",
			sliceSubnet: "300.1.0.0/16",
			expectError: true,
		},
		{
			name:        "Empty string",
			sliceSubnet: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := allocator.ValidateSliceSubnet(tt.sliceSubnet)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIpamAllocator_CalculateMaxClusters(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name        string
		sliceSubnet string
		subnetSize  int
		expectedMax int
		expectError bool
	}{
		{
			name:        "16 to 24 subnets",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			expectedMax: 256,
			expectError: false,
		},
		{
			name:        "24 to 26 subnets",
			sliceSubnet: "192.168.1.0/24",
			subnetSize:  26,
			expectedMax: 4,
			expectError: false,
		},
		{
			name:        "16 to 20 subnets",
			sliceSubnet: "10.0.0.0/16",
			subnetSize:  20,
			expectedMax: 16,
			expectError: false,
		},
		{
			name:        "Invalid subnet size (same as slice)",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  16,
			expectedMax: 0,
			expectError: true,
		},
		{
			name:        "Invalid subnet size (smaller than slice)",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  8,
			expectedMax: 0,
			expectError: true,
		},
		{
			name:        "Invalid CIDR",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectedMax: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxClusters, err := allocator.CalculateMaxClusters(tt.sliceSubnet, tt.subnetSize)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedMax, maxClusters)
			}
		})
	}
}

func TestIpamAllocator_GenerateSubnetList(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name         string
		sliceSubnet  string
		subnetSize   int
		expectedLen  int
		firstSubnet  string
		secondSubnet string
		expectError  bool
	}{
		{
			name:         "16 to 24 subnets",
			sliceSubnet:  "10.1.0.0/16",
			subnetSize:   24,
			expectedLen:  256,
			firstSubnet:  "10.1.0.0/24",
			secondSubnet: "10.1.1.0/24",
			expectError:  false,
		},
		{
			name:         "24 to 26 subnets",
			sliceSubnet:  "192.168.1.0/24",
			subnetSize:   26,
			expectedLen:  4,
			firstSubnet:  "192.168.1.0/26",
			secondSubnet: "192.168.1.64/26",
			expectError:  false,
		},
		{
			name:        "Invalid subnet size",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  16,
			expectError: true,
		},
		{
			name:        "Invalid CIDR",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnets, err := allocator.GenerateSubnetList(tt.sliceSubnet, tt.subnetSize)
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, subnets, tt.expectedLen)
			if tt.expectedLen > 0 {
				require.Equal(t, tt.firstSubnet, subnets[0])
			}
			if tt.expectedLen > 1 {
				require.Equal(t, tt.secondSubnet, subnets[1])
			}
		})
	}
}

func TestIpamAllocator_FindNextAvailableSubnet(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name             string
		sliceSubnet      string
		subnetSize       int
		allocatedSubnets []string
		expectedSubnet   string
		expectError      bool
	}{
		{
			name:             "First subnet available",
			sliceSubnet:      "10.1.0.0/16",
			subnetSize:       24,
			allocatedSubnets: []string{},
			expectedSubnet:   "10.1.0.0/24",
			expectError:      false,
		},
		{
			name:             "Second subnet available",
			sliceSubnet:      "10.1.0.0/16",
			subnetSize:       24,
			allocatedSubnets: []string{"10.1.0.0/24"},
			expectedSubnet:   "10.1.1.0/24",
			expectError:      false,
		},
		{
			name:             "Multiple subnets allocated",
			sliceSubnet:      "192.168.1.0/24",
			subnetSize:       26,
			allocatedSubnets: []string{"192.168.1.0/26", "192.168.1.64/26"},
			expectedSubnet:   "192.168.1.128/26",
			expectError:      false,
		},
		{
			name:             "All subnets allocated",
			sliceSubnet:      "192.168.1.0/24",
			subnetSize:       26,
			allocatedSubnets: []string{"192.168.1.0/26", "192.168.1.64/26", "192.168.1.128/26", "192.168.1.192/26"},
			expectedSubnet:   "",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, err := allocator.FindNextAvailableSubnet(tt.sliceSubnet, tt.subnetSize, tt.allocatedSubnets)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedSubnet, subnet)
			}
		})
	}
}

func TestIpamAllocator_IsSubnetOverlapping(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name          string
		subnet1       string
		subnet2       string
		expectOverlap bool
		expectError   bool
	}{
		{
			name:          "Non-overlapping subnets",
			subnet1:       "10.1.0.0/24",
			subnet2:       "10.1.1.0/24",
			expectOverlap: false,
			expectError:   false,
		},
		{
			name:          "Overlapping subnets - same network",
			subnet1:       "10.1.0.0/24",
			subnet2:       "10.1.0.0/24",
			expectOverlap: true,
			expectError:   false,
		},
		{
			name:          "Overlapping subnets - subnet contains other",
			subnet1:       "10.1.0.0/16",
			subnet2:       "10.1.1.0/24",
			expectOverlap: true,
			expectError:   false,
		},
		{
			name:          "Overlapping subnets - other contains subnet",
			subnet1:       "10.1.1.0/24",
			subnet2:       "10.1.0.0/16",
			expectOverlap: true,
			expectError:   false,
		},
		{
			name:        "Invalid subnet1",
			subnet1:     "invalid",
			subnet2:     "10.1.1.0/24",
			expectError: true,
		},
		{
			name:        "Invalid subnet2",
			subnet1:     "10.1.0.0/24",
			subnet2:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overlap, err := allocator.IsSubnetOverlapping(tt.subnet1, tt.subnet2)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectOverlap, overlap)
			}
		})
	}
}

func TestIpamAllocator_OptimizeSubnetAllocations(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name           string
		sliceSubnet    string
		subnetSize     int
		activeSubnets  []string
		expectedResult []string
		expectError    bool
	}{
		{
			name:           "Optimize active subnets",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			activeSubnets:  []string{"10.1.2.0/24", "10.1.0.0/24", "10.1.1.0/24"},
			expectedResult: []string{"10.1.0.0/24", "10.1.1.0/24", "10.1.2.0/24"},
			expectError:    false,
		},
		{
			name:           "No active subnets",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			activeSubnets:  []string{},
			expectedResult: nil,
			expectError:    false,
		},
		{
			name:          "Invalid slice subnet",
			sliceSubnet:   "invalid",
			subnetSize:    24,
			activeSubnets: []string{"10.1.0.0/24"},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := allocator.OptimizeSubnetAllocations(tt.sliceSubnet, tt.subnetSize, tt.activeSubnets)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestIpamAllocator_GetSubnetUtilization(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name                string
		sliceSubnet         string
		subnetSize          int
		allocatedCount      int
		expectedUtilization float64
		expectError         bool
	}{
		{
			name:                "50% utilization",
			sliceSubnet:         "10.1.0.0/16",
			subnetSize:          24,
			allocatedCount:      128,
			expectedUtilization: 50.0,
			expectError:         false,
		},
		{
			name:                "100% utilization",
			sliceSubnet:         "192.168.1.0/24",
			subnetSize:          26,
			allocatedCount:      4,
			expectedUtilization: 100.0,
			expectError:         false,
		},
		{
			name:                "0% utilization",
			sliceSubnet:         "10.1.0.0/16",
			subnetSize:          24,
			allocatedCount:      0,
			expectedUtilization: 0.0,
			expectError:         false,
		},
		{
			name:           "Invalid slice subnet",
			sliceSubnet:    "invalid",
			subnetSize:     24,
			allocatedCount: 1,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utilization, err := allocator.GetSubnetUtilization(tt.sliceSubnet, tt.subnetSize, tt.allocatedCount)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedUtilization, utilization)
			}
		})
	}
}
