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
	"net"
	"sort"
)

// IpamAllocator provides utilities for IP address management
type IpamAllocator struct{}

// NewIpamAllocator creates a new IPAM allocator
func NewIpamAllocator() *IpamAllocator {
	return &IpamAllocator{}
}

// ValidateSliceSubnet validates that the slice subnet is valid
func (ia *IpamAllocator) ValidateSliceSubnet(sliceSubnet string) error {
	_, _, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return fmt.Errorf("invalid slice subnet %s: %v", sliceSubnet, err)
	}
	return nil
}

// CalculateMaxClusters calculates the maximum number of clusters that can be supported
func (ia *IpamAllocator) CalculateMaxClusters(sliceSubnet string, subnetSize int) (int, error) {
	_, sliceNet, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return 0, fmt.Errorf("invalid slice subnet: %v", err)
	}

	sliceOnes, _ := sliceNet.Mask.Size()
	if subnetSize <= sliceOnes {
		return 0, fmt.Errorf("subnet size %d must be larger than slice subnet size %d", subnetSize, sliceOnes)
	}

	maxClusters := 1 << (subnetSize - sliceOnes)
	return maxClusters, nil
}

// GenerateSubnetList generates a list of all possible subnets from a slice subnet
func (ia *IpamAllocator) GenerateSubnetList(sliceSubnet string, subnetSize int) ([]string, error) {
	_, sliceNet, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return nil, fmt.Errorf("invalid slice subnet: %v", err)
	}

	sliceOnes, sliceBits := sliceNet.Mask.Size()
	if subnetSize <= sliceOnes {
		return nil, fmt.Errorf("subnet size %d must be larger than slice subnet size %d", subnetSize, sliceOnes)
	}

	numSubnets := 1 << (subnetSize - sliceOnes)
	subnets := make([]string, 0, numSubnets)

	// Calculate the increment between subnets
	increment := 1 << (sliceBits - subnetSize)

	// Start with the base IP of the slice network
	baseIP := make(net.IP, len(sliceNet.IP))
	copy(baseIP, sliceNet.IP)

	for i := 0; i < numSubnets; i++ {
		subnet := &net.IPNet{
			IP:   make(net.IP, len(baseIP)),
			Mask: net.CIDRMask(subnetSize, sliceBits),
		}
		copy(subnet.IP, baseIP)

		subnets = append(subnets, subnet.String())

		// Increment IP for next subnet
		ia.incrementIP(baseIP, increment)
	}

	return subnets, nil
}

// FindNextAvailableSubnet finds the next available subnet from a list of allocated subnets
func (ia *IpamAllocator) FindNextAvailableSubnet(sliceSubnet string, subnetSize int, allocatedSubnets []string) (string, error) {
	allSubnets, err := ia.GenerateSubnetList(sliceSubnet, subnetSize)
	if err != nil {
		return "", err
	}

	// Create a map for faster lookup
	allocated := make(map[string]bool)
	for _, subnet := range allocatedSubnets {
		allocated[subnet] = true
	}

	// Find first available subnet
	for _, subnet := range allSubnets {
		if !allocated[subnet] {
			return subnet, nil
		}
	}

	return "", fmt.Errorf("no available subnets")
}

// IsSubnetOverlapping checks if two subnets overlap
func (ia *IpamAllocator) IsSubnetOverlapping(subnet1, subnet2 string) (bool, error) {
	_, net1, err := net.ParseCIDR(subnet1)
	if err != nil {
		return false, fmt.Errorf("invalid subnet1: %v", err)
	}

	_, net2, err := net.ParseCIDR(subnet2)
	if err != nil {
		return false, fmt.Errorf("invalid subnet2: %v", err)
	}

	return net1.Contains(net2.IP) || net2.Contains(net1.IP), nil
}

// OptimizeSubnetAllocations optimizes subnet allocations by compacting released subnets
func (ia *IpamAllocator) OptimizeSubnetAllocations(sliceSubnet string, subnetSize int, activeSubnets []string) ([]string, error) {
	allSubnets, err := ia.GenerateSubnetList(sliceSubnet, subnetSize)
	if err != nil {
		return nil, err
	}

	// Create a map for active subnets
	active := make(map[string]bool)
	for _, subnet := range activeSubnets {
		active[subnet] = true
	}

	// Return optimized list (in this case, just the active subnets sorted)
	var optimized []string
	for _, subnet := range allSubnets {
		if active[subnet] {
			optimized = append(optimized, subnet)
		}
	}

	sort.Strings(optimized)
	return optimized, nil
}

// GetSubnetUtilization calculates the utilization percentage of subnets
func (ia *IpamAllocator) GetSubnetUtilization(sliceSubnet string, subnetSize int, allocatedCount int) (float64, error) {
	maxClusters, err := ia.CalculateMaxClusters(sliceSubnet, subnetSize)
	if err != nil {
		return 0, err
	}

	if maxClusters == 0 {
		return 0, nil
	}

	utilization := float64(allocatedCount) / float64(maxClusters) * 100
	return utilization, nil
}

// incrementIP increments an IP address by the given amount
func (ia *IpamAllocator) incrementIP(ip net.IP, increment int) {
	// Convert increment to big-endian bytes and add to IP
	for i := len(ip) - 1; i >= 0 && increment > 0; i-- {
		sum := int(ip[i]) + increment
		ip[i] = byte(sum & 0xFF)
		increment = sum >> 8
	}
}
