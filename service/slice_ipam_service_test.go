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
	"net"
	"testing"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestSliceIpamService_generateSubnets(t *testing.T) {
	service := &SliceIpamService{}

	tests := []struct {
		name         string
		sliceSubnet  string
		subnetSize   int
		expectedLen  int
		expectedErr  bool
		firstSubnet  string
		secondSubnet string
	}{
		{
			name:         "Valid /16 to /24 subnets",
			sliceSubnet:  "10.1.0.0/16",
			subnetSize:   24,
			expectedLen:  256,
			expectedErr:  false,
			firstSubnet:  "10.1.0.0/24",
			secondSubnet: "10.1.1.0/24",
		},
		{
			name:         "Valid /24 to /26 subnets",
			sliceSubnet:  "192.168.1.0/24",
			subnetSize:   26,
			expectedLen:  4,
			expectedErr:  false,
			firstSubnet:  "192.168.1.0/26",
			secondSubnet: "192.168.1.64/26",
		},
		{
			name:        "Invalid subnet size",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  16,
			expectedErr: true,
		},
		{
			name:        "Invalid CIDR",
			sliceSubnet: "invalid",
			subnetSize:  24,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, sliceNet, err := net.ParseCIDR(tt.sliceSubnet)
			if err != nil && !tt.expectedErr {
				t.Fatalf("Failed to parse CIDR: %v", err)
			}
			if err != nil && tt.expectedErr {
				return // Expected error in parsing
			}

			subnets, err := service.generateSubnets(sliceNet, tt.subnetSize)

			if tt.expectedErr {
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

func TestSliceIpamService_allocateNextAvailableSubnet(t *testing.T) {
	service := &SliceIpamService{}

	sliceIpam := &v1alpha1.SliceIpam{
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster1",
					Subnet:      "10.1.0.0/24",
					Status:      "InUse",
				},
				{
					ClusterName: "cluster2",
					Subnet:      "10.1.1.0/24",
					Status:      "Released",
				},
			},
		},
	}

	// Test - should return the first available subnet (10.1.1.0/24 is released, so it should be available)
	subnet, err := service.allocateNextAvailableSubnet(sliceIpam)

	require.NoError(t, err)
	require.Equal(t, "10.1.1.0/24", subnet) // Released subnet should be available
}

func TestSliceIpamService_incrementIP(t *testing.T) {
	service := &SliceIpamService{}

	tests := []struct {
		name      string
		ip        string
		increment int
		expected  string
	}{
		{
			name:      "Simple increment",
			ip:        "10.1.0.0",
			increment: 256,
			expected:  "10.1.1.0",
		},
		{
			name:      "Large increment",
			ip:        "10.1.0.0",
			increment: 512,
			expected:  "10.1.2.0",
		},
		{
			name:      "Cross byte boundary",
			ip:        "10.1.255.0",
			increment: 256,
			expected:  "10.2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip).To4()
			service.incrementIP(ip, tt.increment)
			result := net.IP(ip).String()
			require.Equal(t, tt.expected, result)
		})
	}
}
