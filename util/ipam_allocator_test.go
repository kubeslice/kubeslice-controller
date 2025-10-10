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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewIpamAllocator tests the constructor for IPAM allocator
func TestNewIpamAllocator(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "create-new-allocator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocator := NewIpamAllocator()
			if allocator == nil {
				t.Fatal("Expected non-nil allocator")
			}
			if allocator.cache == nil {
				t.Fatal("Expected non-nil cache")
			}
			if len(allocator.cache) != 0 {
				t.Fatal("Expected empty cache")
			}
		})
	}
}

// TestValidateSliceSubnet comprehensively tests subnet validation
func TestValidateSliceSubnet(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name        string
		subnet      string
		expectError bool
		expectedErr error
	}{
		// Valid cases
		{
			name:        "valid-private-subnet-10",
			subnet:      "10.1.0.0/16",
			expectError: false,
		},
		{
			name:        "valid-private-subnet-172",
			subnet:      "172.16.0.0/16",
			expectError: false,
		},
		{
			name:        "valid-private-subnet-192",
			subnet:      "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "valid-minimum-size",
			subnet:      "10.0.0.0/28",
			expectError: false,
		},
		{
			name:        "valid-maximum-size",
			subnet:      "10.0.0.0/8",
			expectError: false,
		},
		// Error cases
		{
			name:        "empty-subnet",
			subnet:      "",
			expectError: true,
			expectedErr: ErrEmptySliceSubnet,
		},
		{
			name:        "invalid-cidr-format",
			subnet:      "invalid-cidr",
			expectError: true,
			expectedErr: ErrInvalidCIDRFormat,
		},
		{
			name:        "invalid-cidr-no-prefix",
			subnet:      "10.1.0.0",
			expectError: true,
			expectedErr: ErrInvalidCIDRFormat,
		},
		{
			name:        "ipv6-subnet",
			subnet:      "2001:db8::/32",
			expectError: true,
			expectedErr: ErrIPv6NotSupported,
		},
		{
			name:        "public-subnet",
			subnet:      "8.8.8.0/24",
			expectError: true,
			expectedErr: ErrNonPrivateSubnet,
		},
		{
			name:        "subnet-too-small",
			subnet:      "10.0.0.0/29",
			expectError: true,
			expectedErr: ErrSubnetTooSmall,
		},
		{
			name:        "subnet-too-large",
			subnet:      "10.0.0.0/7",
			expectError: true,
			expectedErr: ErrSubnetTooLarge,
		},
		{
			name:        "non-network-address",
			subnet:      "10.0.0.1/24",
			expectError: true,
			expectedErr: ErrNonNetworkAddress,
		},
		{
			name:        "private-subnet-edge-10",
			subnet:      "10.255.255.0/28",
			expectError: false,
		},
		{
			name:        "private-subnet-edge-172",
			subnet:      "172.31.255.0/28",
			expectError: false,
		},
		{
			name:        "private-subnet-edge-192",
			subnet:      "192.168.255.0/28",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := allocator.ValidateSliceSubnet(tt.subnet)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedErr != nil && !strings.Contains(err.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestCalculateMaxClusters tests cluster capacity calculation
func TestCalculateMaxClusters(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name         string
		sliceSubnet  string
		subnetSize   int
		expectError  bool
		expectedMax  int
		expectedErr  error
	}{
		{
			name:        "valid-16-to-24",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			expectError: false,
			expectedMax: 256,
		},
		{
			name:        "valid-16-to-20",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  20,
			expectError: false,
			expectedMax: 16,
		},
		{
			name:        "valid-8-to-16",
			sliceSubnet: "10.0.0.0/8",
			subnetSize:  16,
			expectError: false,
			expectedMax: 256,
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid-subnet",
			subnetSize:  24,
			expectError: true,
		},
		{
			name:        "subnet-size-too-small",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  15,
			expectError: true,
			expectedErr: ErrInvalidSubnetSize,
		},
		{
			name:        "subnet-size-too-large",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  31,
			expectError: true,
			expectedErr: ErrInvalidSubnetSize,
		},
		{
			name:        "subnet-size-larger-than-slice",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  16,
			expectError: true,
			expectedErr: ErrSubnetSizeTooLarge,
		},
		{
			name:        "subnet-size-equal-to-slice",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  16,
			expectError: true,
			expectedErr: ErrSubnetSizeTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			max, err := allocator.CalculateMaxClusters(tt.sliceSubnet, tt.subnetSize)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedErr != nil && !strings.Contains(err.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error containing %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if max != tt.expectedMax {
					t.Errorf("Expected max %d, got %d", tt.expectedMax, max)
				}
			}
		})
	}
}

// TestGenerateSubnetList tests subnet list generation
func TestGenerateSubnetList(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name          string
		sliceSubnet   string
		subnetSize    int
		expectError   bool
		expectedCount int
		expectedFirst string
		expectedLast  string
	}{
		{
			name:          "valid-16-to-24",
			sliceSubnet:   "10.1.0.0/16",
			subnetSize:    24,
			expectError:   false,
			expectedCount: 256,
			expectedFirst: "10.1.0.0/24",
			expectedLast:  "10.1.255.0/24",
		},
		{
			name:          "valid-24-to-26",
			sliceSubnet:   "192.168.1.0/24",
			subnetSize:    26,
			expectError:   false,
			expectedCount: 4,
			expectedFirst: "192.168.1.0/26",
			expectedLast:  "192.168.1.192/26",
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectError: true,
		},
		{
			name:        "invalid-subnet-size",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  31,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnets, err := allocator.GenerateSubnetList(tt.sliceSubnet, tt.subnetSize)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(subnets) != tt.expectedCount {
					t.Errorf("Expected %d subnets, got %d", tt.expectedCount, len(subnets))
				}
				if len(subnets) > 0 {
					if subnets[0] != tt.expectedFirst {
						t.Errorf("Expected first subnet %s, got %s", tt.expectedFirst, subnets[0])
					}
					if subnets[len(subnets)-1] != tt.expectedLast {
						t.Errorf("Expected last subnet %s, got %s", tt.expectedLast, subnets[len(subnets)-1])
					}
				}
			}
		})
	}
}

// TestFindNextAvailableSubnet tests sequential subnet allocation
func TestFindNextAvailableSubnet(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name              string
		sliceSubnet       string
		subnetSize        int
		allocatedSubnets  []string
		expectError       bool
		expectedSubnet    string
	}{
		{
			name:             "first-available",
			sliceSubnet:      "10.1.0.0/16",
			subnetSize:       24,
			allocatedSubnets: []string{},
			expectError:      false,
			expectedSubnet:   "10.1.0.0/24",
		},
		{
			name:             "second-available",
			sliceSubnet:      "10.1.0.0/16",
			subnetSize:       24,
			allocatedSubnets: []string{"10.1.0.0/24"},
			expectError:      false,
			expectedSubnet:   "10.1.1.0/24",
		},
		{
			name:             "skip-allocated",
			sliceSubnet:      "10.1.0.0/16",
			subnetSize:       24,
			allocatedSubnets: []string{"10.1.0.0/24", "10.1.1.0/24"},
			expectError:      false,
			expectedSubnet:   "10.1.2.0/24",
		},
		{
			name:             "no-available-subnets",
			sliceSubnet:      "192.168.1.0/24",
			subnetSize:       26,
			allocatedSubnets: []string{"192.168.1.0/26", "192.168.1.64/26", "192.168.1.128/26", "192.168.1.192/26"},
			expectError:      true,
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, err := allocator.FindNextAvailableSubnet(tt.sliceSubnet, tt.subnetSize, tt.allocatedSubnets)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if subnet != tt.expectedSubnet {
					t.Errorf("Expected subnet %s, got %s", tt.expectedSubnet, subnet)
				}
			}
		})
	}
}

// TestFindNextAvailableSubnetWithReclamation tests subnet allocation with reclamation
func TestFindNextAvailableSubnetWithReclamation(t *testing.T) {
	allocator := NewIpamAllocator()

	now := time.Now()
	tests := []struct {
		name            string
		sliceSubnet     string
		subnetSize      int
		allocations     []ClusterSubnetAllocation
		reclaimAfter    time.Duration
		expectError     bool
		expectedSubnet  string
		expectedReclaim bool
	}{
		{
			name:        "new-allocation",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			allocations: []ClusterSubnetAllocation{},
			reclaimAfter: time.Hour,
			expectError: false,
			expectedSubnet: "10.1.0.0/24",
			expectedReclaim: false,
		},
		{
			name:        "reclaim-old-released",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			allocations: []ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "10.1.0.0/24",
					Status:      "Released",
					ReleasedAt:  &[]time.Time{now.Add(-2 * time.Hour)}[0],
				},
			},
			reclaimAfter: time.Hour,
			expectError: false,
			expectedSubnet: "10.1.0.0/24",
			expectedReclaim: true,
		},
		{
			name:        "do-not-reclaim-recent-released",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			allocations: []ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "10.1.0.0/24",
					Status:      "Released",
					ReleasedAt:  &[]time.Time{now.Add(-30 * time.Minute)}[0],
				},
			},
			reclaimAfter: time.Hour,
			expectError: false,
			expectedSubnet: "10.1.1.0/24",
			expectedReclaim: false,
		},
		{
			name:        "active-allocation",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			allocations: []ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "10.1.0.0/24",
					Status:      "Allocated",
				},
			},
			reclaimAfter: time.Hour,
			expectError: false,
			expectedSubnet: "10.1.1.0/24",
			expectedReclaim: false,
		},
		{
			name:        "in-use-allocation",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			allocations: []ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "10.1.0.0/24",
					Status:      "InUse",
				},
			},
			reclaimAfter: time.Hour,
			expectError: false,
			expectedSubnet: "10.1.1.0/24",
			expectedReclaim: false,
		},
		{
			name:        "reclaim-released-without-time",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			allocations: []ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "10.1.0.0/24",
					Status:      "Released",
					ReleasedAt:  nil, // Edge case
				},
			},
			reclaimAfter: time.Hour,
			expectError: false,
			expectedSubnet: "10.1.0.0/24",
			expectedReclaim: true,
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid",
			subnetSize:  24,
			allocations: []ClusterSubnetAllocation{},
			reclaimAfter: time.Hour,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, reclaimed, err := allocator.FindNextAvailableSubnetWithReclamation(tt.sliceSubnet, tt.subnetSize, tt.allocations, tt.reclaimAfter)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if subnet != tt.expectedSubnet {
					t.Errorf("Expected subnet %s, got %s", tt.expectedSubnet, subnet)
				}
				if reclaimed != tt.expectedReclaim {
					t.Errorf("Expected reclaimed %v, got %v", tt.expectedReclaim, reclaimed)
				}
			}
		})
	}
}

// TestFindOptimalSubnet tests optimal subnet allocation with hints
func TestFindOptimalSubnet(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name             string
		sliceSubnet      string
		subnetSize       int
		allocatedSubnets []string
		clusterHint      string
		expectError      bool
		expectedSubnet   string
	}{
		{
			name:           "no-hint-first-available",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedSubnets: []string{},
			clusterHint:    "",
			expectError:    false,
			expectedSubnet: "10.1.0.0/24",
		},
		{
			name:           "with-hint",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedSubnets: []string{},
			clusterHint:    "cluster-a",
			expectError:    false,
			// Result will depend on hash function
		},
		{
			name:           "hint-with-conflicts",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedSubnets: []string{"10.1.0.0/24", "10.1.1.0/24"},
			clusterHint:    "cluster-b",
			expectError:    false,
			// Should fallback to available subnet
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid",
			subnetSize:  24,
			clusterHint: "cluster-a",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, err := allocator.FindOptimalSubnet(tt.sliceSubnet, tt.subnetSize, tt.allocatedSubnets, tt.clusterHint)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.expectedSubnet != "" && subnet != tt.expectedSubnet {
					t.Errorf("Expected subnet %s, got %s", tt.expectedSubnet, subnet)
				}
				// Verify the returned subnet is valid and not in allocated list
				if subnet != "" {
					for _, allocated := range tt.allocatedSubnets {
						if subnet == allocated {
							t.Errorf("Returned already allocated subnet: %s", subnet)
						}
					}
				}
			}
		})
	}
}

// TestIsSubnetOverlapping tests subnet overlap detection
func TestIsSubnetOverlapping(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name        string
		subnet1     string
		subnet2     string
		expectError bool
		expected    bool
	}{
		{
			name:     "no-overlap",
			subnet1:  "10.1.0.0/24",
			subnet2:  "10.1.1.0/24",
			expected: false,
		},
		{
			name:     "identical-subnets",
			subnet1:  "10.1.0.0/24",
			subnet2:  "10.1.0.0/24",
			expected: true,
		},
		{
			name:     "subnet1-contains-subnet2",
			subnet1:  "10.1.0.0/16",
			subnet2:  "10.1.1.0/24",
			expected: true,
		},
		{
			name:     "subnet2-contains-subnet1",
			subnet1:  "10.1.1.0/24",
			subnet2:  "10.1.0.0/16",
			expected: true,
		},
		{
			name:        "invalid-subnet1",
			subnet1:     "invalid",
			subnet2:     "10.1.0.0/24",
			expectError: true,
		},
		{
			name:        "invalid-subnet2",
			subnet1:     "10.1.0.0/24",
			subnet2:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overlapping, err := allocator.IsSubnetOverlapping(tt.subnet1, tt.subnet2)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if overlapping != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, overlapping)
				}
			}
		})
	}
}

// TestGetSubnetUtilization tests utilization calculation
func TestGetSubnetUtilization(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name            string
		sliceSubnet     string
		subnetSize      int
		allocatedCount  int
		expectError     bool
		expectedUtil    float64
	}{
		{
			name:           "zero-allocation",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedCount: 0,
			expectError:    false,
			expectedUtil:   0.0,
		},
		{
			name:           "half-utilization",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedCount: 128,
			expectError:    false,
			expectedUtil:   50.0,
		},
		{
			name:           "full-utilization",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedCount: 256,
			expectError:    false,
			expectedUtil:   100.0,
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectError: true,
		},
		{
			name:           "max-clusters-zero",
			sliceSubnet:    "10.0.0.0/30", // Very small slice that can't accommodate any subnets of size 24
			subnetSize:     24,
			allocatedCount: 0,
			expectError:    true,
		},
		{
			name:           "allocation-exceeds-capacity",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedCount: 500, // More than the 256 possible subnets
			expectError:    false,
			expectedUtil:   195.3125, // 500/256 * 100 = 195.3125%
		},
		{
			name:           "negative-allocated-count",
			sliceSubnet:    "10.1.0.0/16",
			subnetSize:     24,
			allocatedCount: -5,
			expectError:    false,
			expectedUtil:   -1.953125, // -5/256 * 100 = -1.953125%
		},
		{
			name:        "invalid-subnet-size-negative",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  -1,
			expectError: true,
		},
		{
			name:        "invalid-subnet-size-zero",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  0,
			expectError: true,
		},
		{
			name:        "invalid-subnet-size-too-large",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  33,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			util, err := allocator.GetSubnetUtilization(tt.sliceSubnet, tt.subnetSize, tt.allocatedCount)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if util != tt.expectedUtil {
					t.Errorf("Expected utilization %f, got %f", tt.expectedUtil, util)
				}
			}
		})
	}
}

// TestCompactAllocations tests allocation compaction
func TestCompactAllocations(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name            string
		sliceSubnet     string
		subnetSize      int
		activeSubnets   []string
		expectError     bool
		expectedCount   int
	}{
		{
			name:          "empty-allocations",
			sliceSubnet:   "10.1.0.0/16",
			subnetSize:    24,
			activeSubnets: []string{},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:          "compact-three-subnets",
			sliceSubnet:   "10.1.0.0/16",
			subnetSize:    24,
			activeSubnets: []string{"10.1.5.0/24", "10.1.10.0/24", "10.1.2.0/24"},
			expectError:   false,
			expectedCount: 3,
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compacted, err := allocator.CompactAllocations(tt.sliceSubnet, tt.subnetSize, tt.activeSubnets)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(compacted) != tt.expectedCount {
					t.Errorf("Expected %d compacted subnets, got %d", tt.expectedCount, len(compacted))
				}
				// Verify compacted subnets are sequential from the beginning
				if len(compacted) > 0 {
					expectedSubnets, _ := allocator.GenerateSubnetList(tt.sliceSubnet, tt.subnetSize)
					for i, subnet := range compacted {
						if i < len(expectedSubnets) && subnet != expectedSubnets[i] {
							t.Errorf("Expected compacted subnet[%d] to be %s, got %s", i, expectedSubnets[i], subnet)
						}
					}
				}
			}
		})
	}
}

// TestPredictAllocationNeeds tests allocation prediction
func TestPredictAllocationNeeds(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name               string
		sliceSubnet        string
		subnetSize         int
		currentAllocations int
		growthRate         float64
		expectError        bool
		expectedPrediction int
	}{
		{
			name:               "no-growth",
			sliceSubnet:        "10.1.0.0/16",
			subnetSize:         24,
			currentAllocations: 10,
			growthRate:         0.0,
			expectError:        false,
			expectedPrediction: 10,
		},
		{
			name:               "fifty-percent-growth",
			sliceSubnet:        "10.1.0.0/16",
			subnetSize:         24,
			currentAllocations: 10,
			growthRate:         0.5,
			expectError:        false,
			expectedPrediction: 15,
		},
		{
			name:               "growth-exceeds-capacity",
			sliceSubnet:        "192.168.1.0/24",
			subnetSize:         26,
			currentAllocations: 3,
			growthRate:         2.0,
			expectError:        false,
			expectedPrediction: 4, // Capped at max capacity
		},
		{
			name:               "negative-growth",
			sliceSubnet:        "10.1.0.0/16",
			subnetSize:         24,
			currentAllocations: 10,
			growthRate:         -0.1,
			expectError:        true,
		},
		{
			name:        "invalid-slice-subnet",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicted, err := allocator.PredictAllocationNeeds(tt.sliceSubnet, tt.subnetSize, tt.currentAllocations, tt.growthRate)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if predicted != tt.expectedPrediction {
					t.Errorf("Expected prediction %d, got %d", tt.expectedPrediction, predicted)
				}
			}
		})
	}
}

// TestValidateAllocationConsistency tests allocation consistency validation
func TestValidateAllocationConsistency(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name        string
		allocations []ClusterSubnetAllocation
		expectError bool
		expectedErr error
	}{
		{
			name:        "empty-allocations",
			allocations: []ClusterSubnetAllocation{},
			expectError: false,
		},
		{
			name: "valid-allocations",
			allocations: []ClusterSubnetAllocation{
				{ClusterName: "cluster1", Subnet: "10.1.0.0/24"},
				{ClusterName: "cluster2", Subnet: "10.1.1.0/24"},
			},
			expectError: false,
		},
		{
			name: "empty-cluster-name",
			allocations: []ClusterSubnetAllocation{
				{ClusterName: "", Subnet: "10.1.0.0/24"},
			},
			expectError: true,
			expectedErr: ErrEmptyClusterName,
		},
		{
			name: "duplicate-cluster-name",
			allocations: []ClusterSubnetAllocation{
				{ClusterName: "cluster1", Subnet: "10.1.0.0/24"},
				{ClusterName: "cluster1", Subnet: "10.1.1.0/24"},
			},
			expectError: true,
			expectedErr: ErrDuplicateCluster,
		},
		{
			name: "empty-subnet",
			allocations: []ClusterSubnetAllocation{
				{ClusterName: "cluster1", Subnet: ""},
			},
			expectError: true,
			expectedErr: ErrEmptySubnet,
		},
		{
			name: "invalid-subnet-format",
			allocations: []ClusterSubnetAllocation{
				{ClusterName: "cluster1", Subnet: "invalid-subnet"},
			},
			expectError: true,
		},
		{
			name: "overlapping-subnets",
			allocations: []ClusterSubnetAllocation{
				{ClusterName: "cluster1", Subnet: "10.1.0.0/16"},
				{ClusterName: "cluster2", Subnet: "10.1.1.0/24"},
			},
			expectError: true,
			expectedErr: ErrOverlappingSubnets,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := allocator.ValidateAllocationConsistency(tt.allocations)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedErr != nil && !strings.Contains(err.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error containing %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestCacheOperations tests all cache-related operations
func TestCacheOperations(t *testing.T) {
	allocator := NewIpamAllocator()

	t.Run("get-cached-allocation-empty-slice-name", func(t *testing.T) {
		cache, found := allocator.GetCachedAllocation("")
		if found {
			t.Error("Expected not found for empty slice name")
		}
		if cache != nil {
			t.Error("Expected nil cache for empty slice name")
		}
	})

	t.Run("get-cached-allocation-non-existent", func(t *testing.T) {
		cache, found := allocator.GetCachedAllocation("non-existent")
		if found {
			t.Error("Expected not found for non-existent slice")
		}
		if cache != nil {
			t.Error("Expected nil cache for non-existent slice")
		}
	})

	t.Run("update-cache-empty-slice-name", func(t *testing.T) {
		cache := &AllocationCache{
			SliceSubnet:      "10.0.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: make(map[string]bool),
			TTL:              DefaultCacheTTL,
		}
		allocator.UpdateCache("", cache)
		// Should not crash, silently ignored
	})

	t.Run("update-cache-nil-cache", func(t *testing.T) {
		// First add a cache entry
		validCache := &AllocationCache{
			SliceSubnet:      "10.0.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: make(map[string]bool),
			TTL:              DefaultCacheTTL,
		}
		allocator.UpdateCache("test-slice", validCache)

		// Verify it exists
		_, found := allocator.GetCachedAllocation("test-slice")
		if !found {
			t.Error("Expected cache entry to exist")
		}

		// Delete with nil cache
		allocator.UpdateCache("test-slice", nil)

		// Verify it's deleted
		_, found = allocator.GetCachedAllocation("test-slice")
		if found {
			t.Error("Expected cache entry to be deleted")
		}
	})

	t.Run("update-cache-with-default-ttl", func(t *testing.T) {
		cache := &AllocationCache{
			SliceSubnet:      "10.0.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: make(map[string]bool),
			TTL:              0, // Should use default
		}
		allocator.UpdateCache("test-slice", cache)

		retrieved, found := allocator.GetCachedAllocation("test-slice")
		if !found {
			t.Error("Expected cache entry to be found")
		}
		if retrieved.TTL != DefaultCacheTTL {
			t.Errorf("Expected TTL %v, got %v", DefaultCacheTTL, retrieved.TTL)
		}
	})

	t.Run("get-cached-allocation-expired", func(t *testing.T) {
		cache := &AllocationCache{
			SliceSubnet:      "10.0.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: make(map[string]bool),
			LastUpdated:      time.Now().Add(-time.Hour), // Expired
			TTL:              time.Minute,
		}

		// Manually add expired cache
		allocator.cache["expired-slice"] = cache

		retrieved, found := allocator.GetCachedAllocation("expired-slice")
		if found {
			t.Error("Expected expired cache to not be found")
		}
		if retrieved != nil {
			t.Error("Expected nil cache for expired entry")
		}
	})

	t.Run("invalidate-cache-empty-slice-name", func(t *testing.T) {
		allocator.InvalidateCache("")
		// Should not crash, silently ignored
	})

	t.Run("invalidate-cache-valid", func(t *testing.T) {
		cache := &AllocationCache{
			SliceSubnet:      "10.0.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: make(map[string]bool),
			TTL:              DefaultCacheTTL,
		}
		allocator.UpdateCache("test-slice", cache)

		// Verify it exists
		_, found := allocator.GetCachedAllocation("test-slice")
		if !found {
			t.Error("Expected cache entry to exist")
		}

		// Invalidate
		allocator.InvalidateCache("test-slice")

		// Verify it's gone
		_, found = allocator.GetCachedAllocation("test-slice")
		if found {
			t.Error("Expected cache entry to be invalidated")
		}
	})

	t.Run("cleanup-expired-cache", func(t *testing.T) {
		// Add fresh cache
		freshCache := &AllocationCache{
			SliceSubnet:      "10.0.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: make(map[string]bool),
			LastUpdated:      time.Now(),
			TTL:              time.Hour,
		}
		allocator.UpdateCache("fresh-slice", freshCache)

		// Add expired cache
		expiredCache := &AllocationCache{
			SliceSubnet:      "10.1.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: make(map[string]bool),
			LastUpdated:      time.Now().Add(-2 * time.Hour),
			TTL:              time.Hour,
		}
		allocator.cache["expired-slice"] = expiredCache

		// Cleanup
		allocator.CleanupExpiredCache()

		// Fresh should remain
		_, found := allocator.GetCachedAllocation("fresh-slice")
		if !found {
			t.Error("Expected fresh cache to remain")
		}

		// Expired should be gone
		if _, exists := allocator.cache["expired-slice"]; exists {
			t.Error("Expected expired cache to be cleaned up")
		}
	})

	t.Run("get-cache-stats", func(t *testing.T) {
		// Clear cache first
		allocator.cache = make(map[string]*AllocationCache)

		// Add test cache entries
		cache1 := &AllocationCache{
			SliceSubnet:      "10.0.0.0/16",
			SubnetSize:       24,
			AllocatedSubnets: map[string]bool{"10.0.1.0/24": true},
			TTL:              DefaultCacheTTL,
		}
		allocator.UpdateCache("slice1", cache1)

		cache2 := &AllocationCache{
			SliceSubnet:      "10.1.0.0/16",
			SubnetSize:       26,
			AllocatedSubnets: map[string]bool{"10.1.0.0/26": true, "10.1.0.64/26": true},
			TTL:              time.Hour,
		}
		allocator.UpdateCache("slice2", cache2)

		stats := allocator.GetCacheStats()

		// Verify total entries
		if totalEntries, ok := stats["total_entries"].(int); !ok || totalEntries != 2 {
			t.Errorf("Expected 2 total entries, got %v", stats["total_entries"])
		}

		// Verify entries structure
		if entries, ok := stats["entries"].(map[string]interface{}); !ok {
			t.Error("Expected entries to be a map")
		} else {
			if _, exists := entries["slice1"]; !exists {
				t.Error("Expected slice1 in entries")
			}
			if _, exists := entries["slice2"]; !exists {
				t.Error("Expected slice2 in entries")
			}

			// Verify slice1 entry details
			if slice1Entry, ok := entries["slice1"].(map[string]interface{}); ok {
				if sliceSubnet := slice1Entry["slice_subnet"]; sliceSubnet != "10.0.0.0/16" {
					t.Errorf("Expected slice_subnet 10.0.0.0/16, got %v", sliceSubnet)
				}
				if subnetSize := slice1Entry["subnet_size"]; subnetSize != 24 {
					t.Errorf("Expected subnet_size 24, got %v", subnetSize)
				}
				if allocatedCount := slice1Entry["allocated_count"]; allocatedCount != 1 {
					t.Errorf("Expected allocated_count 1, got %v", allocatedCount)
				}
			}
		}
	})
}

// TestConcurrentCacheOperations tests cache operations under concurrent access
func TestConcurrentCacheOperations(t *testing.T) {
	allocator := NewIpamAllocator()

	t.Run("concurrent-cache-updates", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		numOperations := 100

		// Test concurrent updates
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					sliceName := fmt.Sprintf("slice-%d", id)
					cache := &AllocationCache{
						SliceSubnet:      fmt.Sprintf("10.%d.0.0/16", id),
						SubnetSize:       24,
						AllocatedSubnets: make(map[string]bool),
						TTL:              DefaultCacheTTL,
					}
					allocator.UpdateCache(sliceName, cache)
				}
			}(i)
		}

		wg.Wait()

		// Verify final state
		stats := allocator.GetCacheStats()
		if totalEntries := stats["total_entries"].(int); totalEntries != numGoroutines {
			t.Errorf("Expected %d cache entries, got %d", numGoroutines, totalEntries)
		}
	})

	t.Run("concurrent-reads-and-writes", func(t *testing.T) {
		var wg sync.WaitGroup

		// Writers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					sliceName := fmt.Sprintf("slice-%d", id)
					cache := &AllocationCache{
						SliceSubnet:      fmt.Sprintf("10.%d.0.0/16", id),
						SubnetSize:       24,
						AllocatedSubnets: make(map[string]bool),
						TTL:              DefaultCacheTTL,
					}
					allocator.UpdateCache(sliceName, cache)
				}
			}(i)
		}

		// Readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					sliceName := fmt.Sprintf("slice-%d", id%5)
					allocator.GetCachedAllocation(sliceName)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestPrivateIPAMSubnet tests the private subnet validation helper function
func TestPrivateIPAMSubnet(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		expected bool
	}{
		{
			name:     "valid-class-a-private",
			cidr:     "10.0.0.0/8",
			expected: true,
		},
		{
			name:     "valid-class-a-private-subset",
			cidr:     "10.1.0.0/16",
			expected: true,
		},
		{
			name:     "valid-class-b-private",
			cidr:     "172.16.0.0/12",
			expected: true,
		},
		{
			name:     "valid-class-b-private-subset",
			cidr:     "172.20.0.0/16",
			expected: true,
		},
		{
			name:     "valid-class-c-private",
			cidr:     "192.168.0.0/16",
			expected: true,
		},
		{
			name:     "valid-class-c-private-subset",
			cidr:     "192.168.1.0/24",
			expected: true,
		},
		{
			name:     "public-subnet-google-dns",
			cidr:     "8.8.8.0/24",
			expected: false,
		},
		{
			name:     "public-subnet-cloudflare-dns",
			cidr:     "1.1.1.0/24",
			expected: false,
		},
		{
			name:     "invalid-cidr",
			cidr:     "invalid-cidr",
			expected: false,
		},
		{
			name:     "edge-of-class-a",
			cidr:     "10.255.255.0/24",
			expected: true,
		},
		{
			name:     "edge-of-class-b",
			cidr:     "172.31.255.0/24",
			expected: true,
		},
		{
			name:     "outside-class-b-lower",
			cidr:     "172.15.0.0/16",
			expected: false,
		},
		{
			name:     "outside-class-b-upper",
			cidr:     "172.32.0.0/16",
			expected: false,
		},
		{
			name:     "localhost-range",
			cidr:     "127.0.0.0/8",
			expected: false,
		},
		{
			name:     "link-local-range",
			cidr:     "169.254.0.0/16",
			expected: false,
		},
		{
			name:     "multicast-range",
			cidr:     "224.0.0.0/4",
			expected: false,
		},
		{
			name:     "empty-cidr",
			cidr:     "",
			expected: false,
		},
		{
			name:     "cidr-without-prefix",
			cidr:     "192.168.1.1",
			expected: false,
		},
		{
			name:     "invalid-ip-in-cidr",
			cidr:     "256.256.256.256/24",
			expected: false,
		},
		{
			name:     "class-a-boundary-lower",
			cidr:     "10.0.0.0/32",
			expected: true,
		},
		{
			name:     "class-a-boundary-upper",
			cidr:     "10.255.255.255/32",
			expected: true,
		},
		{
			name:     "class-b-boundary-lower",
			cidr:     "172.16.0.0/32",
			expected: true,
		},
		{
			name:     "class-b-boundary-upper",
			cidr:     "172.31.255.255/32",
			expected: true,
		},
		{
			name:     "class-c-boundary-lower",
			cidr:     "192.168.0.0/32",
			expected: true,
		},
		{
			name:     "class-c-boundary-upper",
			cidr:     "192.168.255.255/32",
			expected: true,
		},
		{
			name:     "apipa-range",
			cidr:     "169.254.1.0/24",
			expected: false,
		},
		{
			name:     "carrier-grade-nat",
			cidr:     "100.64.0.0/10",
			expected: false,
		},
		{
			name:     "ipv6-private-equivalent",
			cidr:     "fc00::/7",
			expected: false,
		},
		{
			name:     "malformed-cidr-with-letters",
			cidr:     "10.0.0.a/24",
			expected: false,
		},
		{
			name:     "out-of-range-octets",
			cidr:     "300.1.1.1/24",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPrivateIPAMSubnet(tt.cidr)
			if result != tt.expected {
				t.Errorf("Expected %v for CIDR %s, got %v", tt.expected, tt.cidr, result)
			}
		})
	}
}

// TestCalculatePreferredIndex tests the preferred index calculation helper
func TestCalculatePreferredIndex(t *testing.T) {
	allocator := NewIpamAllocator()

	tests := []struct {
		name         string
		clusterHint  string
		totalSubnets int
	}{
		{
			name:         "empty-hint",
			clusterHint:  "",
			totalSubnets: 100,
		},
		{
			name:         "single-char-hint",
			clusterHint:  "a",
			totalSubnets: 100,
		},
		{
			name:         "multi-char-hint",
			clusterHint:  "cluster-name",
			totalSubnets: 256,
		},
		{
			name:         "numeric-hint",
			clusterHint:  "123",
			totalSubnets: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := allocator.calculatePreferredIndex(tt.clusterHint, tt.totalSubnets)
			
			// Verify index is within bounds
			if index < 0 || index >= tt.totalSubnets {
				t.Errorf("Index %d out of bounds [0, %d)", index, tt.totalSubnets)
			}

			// Verify consistency - same input should give same output
			index2 := allocator.calculatePreferredIndex(tt.clusterHint, tt.totalSubnets)
			if index != index2 {
				t.Errorf("Inconsistent index calculation: %d vs %d", index, index2)
			}
		})
	}
}

// TestAllocationScenarios tests comprehensive allocation scenarios
func TestAllocationScenarios(t *testing.T) {
	allocator := NewIpamAllocator()

	t.Run("full-lifecycle-scenario", func(t *testing.T) {
		sliceSubnet := "10.1.0.0/16"
		subnetSize := 24

		// Validate slice subnet
		err := allocator.ValidateSliceSubnet(sliceSubnet)
		if err != nil {
			t.Fatalf("Unexpected validation error: %v", err)
		}

		// Calculate max clusters
		maxClusters, err := allocator.CalculateMaxClusters(sliceSubnet, subnetSize)
		if err != nil {
			t.Fatalf("Unexpected error calculating max clusters: %v", err)
		}

		// Allocate multiple subnets
		allocated := make([]string, 0)
		for i := 0; i < 5; i++ {
			subnet, err := allocator.FindNextAvailableSubnet(sliceSubnet, subnetSize, allocated)
			if err != nil {
				t.Fatalf("Unexpected error finding subnet: %v", err)
			}
			allocated = append(allocated, subnet)
		}

		// Verify allocations don't overlap
		allocations := make([]ClusterSubnetAllocation, len(allocated))
		for i, subnet := range allocated {
			allocations[i] = ClusterSubnetAllocation{
				ClusterName: fmt.Sprintf("cluster-%d", i),
				Subnet:      subnet,
				Status:      "Allocated",
			}
		}

		err = allocator.ValidateAllocationConsistency(allocations)
		if err != nil {
			t.Fatalf("Allocation consistency validation failed: %v", err)
		}

		// Check utilization
		utilization, err := allocator.GetSubnetUtilization(sliceSubnet, subnetSize, len(allocated))
		if err != nil {
			t.Fatalf("Unexpected error calculating utilization: %v", err)
		}

		expectedUtilization := float64(len(allocated)) / float64(maxClusters) * 100
		if utilization != expectedUtilization {
			t.Errorf("Expected utilization %f, got %f", expectedUtilization, utilization)
		}

		// Test compaction
		compacted, err := allocator.CompactAllocations(sliceSubnet, subnetSize, allocated)
		if err != nil {
			t.Fatalf("Unexpected error compacting allocations: %v", err)
		}
		if len(compacted) != len(allocated) {
			t.Errorf("Expected %d compacted subnets, got %d", len(allocated), len(compacted))
		}

		// Test prediction
		predicted, err := allocator.PredictAllocationNeeds(sliceSubnet, subnetSize, len(allocated), 0.5)
		if err != nil {
			t.Fatalf("Unexpected error predicting needs: %v", err)
		}
		expectedPredicted := int(float64(len(allocated)) * 1.5)
		if predicted != expectedPredicted {
			t.Errorf("Expected prediction %d, got %d", expectedPredicted, predicted)
		}
	})

	t.Run("reclamation-scenario", func(t *testing.T) {
		sliceSubnet := "192.168.1.0/24"
		subnetSize := 26

		now := time.Now()
		allocations := []ClusterSubnetAllocation{
			{
				ClusterName: "active-cluster",
				Subnet:      "192.168.1.0/26",
				Status:      "Allocated",
			},
			{
				ClusterName: "released-old",
				Subnet:      "192.168.1.64/26",
				Status:      "Released",
				ReleasedAt:  &[]time.Time{now.Add(-2 * time.Hour)}[0],
			},
			{
				ClusterName: "released-recent",
				Subnet:      "192.168.1.128/26",
				Status:      "Released",
				ReleasedAt:  &[]time.Time{now.Add(-30 * time.Minute)}[0],
			},
		}

		// Should reclaim the old released subnet
		subnet, reclaimed, err := allocator.FindNextAvailableSubnetWithReclamation(
			sliceSubnet, subnetSize, allocations, time.Hour)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !reclaimed {
			t.Error("Expected reclamation to occur")
		}
		if subnet != "192.168.1.64/26" {
			t.Errorf("Expected reclaimed subnet 192.168.1.64/26, got %s", subnet)
		}

		// Update allocations to mark the old one as active
		for i := range allocations {
			if allocations[i].Subnet == "192.168.1.64/26" {
				allocations[i].Status = "Allocated"
				allocations[i].ReleasedAt = nil
			}
		}

		// Should get new subnet since recent released is not reclaimable yet
		subnet, reclaimed, err = allocator.FindNextAvailableSubnetWithReclamation(
			sliceSubnet, subnetSize, allocations, time.Hour)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if reclaimed {
			t.Error("Expected no reclamation")
		}
		if subnet != "192.168.1.192/26" {
			t.Errorf("Expected new subnet 192.168.1.192/26, got %s", subnet)
		}
	})
}

// TestErrorConditions tests various error conditions comprehensively
func TestErrorConditions(t *testing.T) {
	allocator := NewIpamAllocator()

	t.Run("error-propagation", func(t *testing.T) {
		// Test that errors from CalculateMaxClusters propagate through GenerateSubnetList
		_, err := allocator.GenerateSubnetList("invalid-cidr", 24)
		if err == nil {
			t.Error("Expected error for invalid CIDR")
		}

		// Test that errors from GenerateSubnetList propagate through FindNextAvailableSubnet
		_, err = allocator.FindNextAvailableSubnet("invalid-cidr", 24, []string{})
		if err == nil {
			t.Error("Expected error for invalid CIDR")
		}

		// Test that errors propagate through FindOptimalSubnet
		_, err = allocator.FindOptimalSubnet("invalid-cidr", 24, []string{}, "cluster-hint")
		if err == nil {
			t.Error("Expected error for invalid CIDR")
		}
	})

	t.Run("edge-case-errors", func(t *testing.T) {
		// Test GetSubnetUtilization with zero max clusters (edge case)
		// This would require a subnet configuration that results in zero max clusters
		// which should be caught by validation, but let's test the error path
		_, err := allocator.GetSubnetUtilization("10.1.0.0/16", 15, 10) // subnet size <= slice size
		if err == nil {
			t.Error("Expected error for invalid subnet size")
		}
		
		// Test private IP validation edge case
		result := isPrivateIPAMSubnet("192.168.0.0/16") // class C network in correct range
		if !result {
			t.Error("Expected true for valid class C private range")
		}
	})

	t.Run("validation-consistency-error-branches", func(t *testing.T) {
		// Test more branches in ValidateAllocationConsistency
		allocations := []ClusterSubnetAllocation{
			{ClusterName: "cluster1", Subnet: "192.168.0.0/24"},
			{ClusterName: "cluster2", Subnet: "192.168.0.0/25"}, // Overlaps with first
		}
		
		err := allocator.ValidateAllocationConsistency(allocations)
		if err == nil {
			t.Error("Expected error for overlapping subnets")
		}
	})

	t.Run("get-subnet-utilization-error-branch", func(t *testing.T) {
		// Test branch where maxClusters is 0
		util, err := allocator.GetSubnetUtilization("invalid", 24, 0)
		if err == nil {
			t.Error("Expected error for invalid subnet")
		}
		if util != 0 {
			t.Errorf("Expected 0 utilization for error case, got %f", util)
		}
	})
}

// TestBoundaryConditions tests boundary conditions and edge cases
func TestBoundaryConditions(t *testing.T) {
	allocator := NewIpamAllocator()

	t.Run("boundary-subnet-sizes", func(t *testing.T) {
		// Test minimum subnet size with a larger slice subnet
		_, err := allocator.CalculateMaxClusters("10.0.0.0/8", MinSubnetSize)
		if err != nil {
			t.Errorf("Expected no error for minimum subnet size %d, got: %v", MinSubnetSize, err)
		}

		// Test maximum subnet size
		_, err = allocator.CalculateMaxClusters("10.0.0.0/16", MaxSubnetSize)
		if err != nil {
			t.Errorf("Expected no error for maximum subnet size %d, got: %v", MaxSubnetSize, err)
		}

		// Test below minimum
		_, err = allocator.CalculateMaxClusters("10.0.0.0/16", MinSubnetSize-1)
		if err == nil {
			t.Error("Expected error for subnet size below minimum")
		}

		// Test above maximum
		_, err = allocator.CalculateMaxClusters("10.0.0.0/16", MaxSubnetSize+1)
		if err == nil {
			t.Error("Expected error for subnet size above maximum")
		}
	})

	t.Run("boundary-slice-sizes", func(t *testing.T) {
		// Test minimum slice size
		err := allocator.ValidateSliceSubnet("10.0.0.0/28")
		if err != nil {
			t.Errorf("Expected no error for minimum slice size /28, got: %v", err)
		}

		// Test maximum slice size
		err = allocator.ValidateSliceSubnet("10.0.0.0/8")
		if err != nil {
			t.Errorf("Expected no error for maximum slice size /8, got: %v", err)
		}

		// Test below minimum (too small)
		err = allocator.ValidateSliceSubnet("10.0.0.0/29")
		if err == nil {
			t.Error("Expected error for slice size below minimum")
		}

		// Test above maximum (too large)
		err = allocator.ValidateSliceSubnet("10.0.0.0/7")
		if err == nil {
			t.Error("Expected error for slice size above maximum")
		}
	})
}
