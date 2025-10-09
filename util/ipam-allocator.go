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
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"sync"
	"time"
)

// Custom error types for better error handling
var (
	ErrEmptySliceSubnet   = errors.New("slice subnet cannot be empty")
	ErrInvalidCIDRFormat  = errors.New("invalid CIDR format")
	ErrIPv6NotSupported   = errors.New("only IPv4 subnets are supported")
	ErrNonPrivateSubnet   = errors.New("slice subnet must be from private IP ranges")
	ErrSubnetTooSmall     = errors.New("slice subnet is too small, minimum /28 required")
	ErrSubnetTooLarge     = errors.New("slice subnet is too large, maximum /8 allowed")
	ErrNonNetworkAddress  = errors.New("slice subnet must be a network address")
	ErrInvalidSubnetSize  = errors.New("subnet size must be between 16 and 30")
	ErrSubnetSizeTooLarge = errors.New("subnet size must be larger than slice subnet size")
	ErrNoAvailableSubnets = errors.New("no available subnets")
	ErrNegativeGrowthRate = errors.New("growth rate cannot be negative")
	ErrEmptyClusterName   = errors.New("cluster name cannot be empty")
	ErrDuplicateCluster   = errors.New("duplicate cluster name")
	ErrEmptySubnet        = errors.New("subnet cannot be empty")
	ErrOverlappingSubnets = errors.New("overlapping subnets detected")
)

// ClusterSubnetAllocation represents the subnet allocation for a specific cluster
// This is a local definition to avoid import cycles
type ClusterSubnetAllocation struct {
	ClusterName string
	Subnet      string
	AllocatedAt time.Time
	Status      string
	ReleasedAt  *time.Time
}

const (
	// DefaultCacheTTL is the default time-to-live for cache entries
	DefaultCacheTTL = 5 * time.Minute
	// MaxAllocationAttempts is the maximum number of attempts to find an available subnet
	MaxAllocationAttempts = 1000
	// MinSubnetSize is the minimum allowed subnet size (CIDR prefix length)
	MinSubnetSize = 16
	// MaxSubnetSize is the maximum allowed subnet size (CIDR prefix length)
	MaxSubnetSize = 30
	// MinSliceSubnetSize is the minimum allowed slice subnet size (CIDR prefix length)
	MinSliceSubnetSize = 8
	// MaxSliceSubnetSize is the maximum allowed slice subnet size (CIDR prefix length)
	MaxSliceSubnetSize = 28
	// IPv4AddressBits is the number of bits in an IPv4 address
	IPv4AddressBits = 32
)

// IpamAllocator manages dynamic IP allocation for KubeSlice
type IpamAllocator struct {
	cache map[string]*AllocationCache
	mutex sync.RWMutex
}

// AllocationCache stores cached allocation information for a slice
type AllocationCache struct {
	SliceSubnet      string
	SubnetSize       int
	AllocatedSubnets map[string]bool
	LastUpdated      time.Time
	TTL              time.Duration
}

// NewIpamAllocator creates a new IPAM allocator instance
func NewIpamAllocator() *IpamAllocator {
	return &IpamAllocator{
		cache: make(map[string]*AllocationCache),
		mutex: sync.RWMutex{},
	}
}

// ValidateSliceSubnet validates if the provided CIDR is valid for slice allocation
func (ia *IpamAllocator) ValidateSliceSubnet(sliceSubnet string) error {
	if sliceSubnet == "" {
		return ErrEmptySliceSubnet
	}

	ip, ipNet, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCIDRFormat, err)
	}

	// Check if it's IPv4
	if ip.To4() == nil {
		return ErrIPv6NotSupported
	}

	// Check if it's a private subnet
	if !isPrivateIPAMSubnet(sliceSubnet) {
		return ErrNonPrivateSubnet
	}

	// Get the subnet size from CIDR
	ones, bits := ipNet.Mask.Size()
	if bits != IPv4AddressBits {
		return ErrInvalidCIDRFormat
	}

	// Check if the subnet is too small for meaningful allocation
	if ones > MaxSliceSubnetSize {
		return ErrSubnetTooSmall
	}

	// Check if the subnet is too large (administrative limit)
	if ones < MinSliceSubnetSize {
		return ErrSubnetTooLarge
	}

	// Ensure the IP is the network address
	if !ip.Equal(ipNet.IP) {
		return ErrNonNetworkAddress
	}

	return nil
}

// CalculateMaxClusters calculates the maximum number of clusters that can be allocated subnets
func (ia *IpamAllocator) CalculateMaxClusters(sliceSubnet string, subnetSize int) (int, error) {
	if err := ia.ValidateSliceSubnet(sliceSubnet); err != nil {
		return 0, fmt.Errorf("invalid slice subnet: %v", err)
	}

	if subnetSize < MinSubnetSize || subnetSize > MaxSubnetSize {
		return 0, ErrInvalidSubnetSize
	}

	_, ipNet, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return 0, err
	}

	sliceOnes, _ := ipNet.Mask.Size()

	// Check if we can subdivide
	if subnetSize <= sliceOnes {
		return 0, ErrSubnetSizeTooLarge
	}

	// Calculate number of possible subnets
	bitsForSubnets := subnetSize - sliceOnes
	maxClusters := int(math.Pow(2, float64(bitsForSubnets)))

	return maxClusters, nil
}

// GenerateSubnetList generates all possible subnets from the slice subnet
func (ia *IpamAllocator) GenerateSubnetList(sliceSubnet string, subnetSize int) ([]string, error) {
	maxClusters, err := ia.CalculateMaxClusters(sliceSubnet, subnetSize)
	if err != nil {
		return nil, err
	}

	_, ipNet, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return nil, err
	}

	subnets := make([]string, 0, maxClusters)
	baseIP := ipNet.IP.To4()

	// Calculate step size: how many IP addresses between each subnet
	stepSize := 1 << (IPv4AddressBits - subnetSize)

	for i := 0; i < maxClusters; i++ {
		subnetIP := make(net.IP, len(baseIP))
		copy(subnetIP, baseIP)

		// Calculate the subnet IP by adding the offset
		// Each subnet starts at baseIP + i * stepSize
		offset := uint32(i * stepSize)

		// Apply the offset to the IP address
		ipInt := uint32(subnetIP[0])<<24 + uint32(subnetIP[1])<<16 + uint32(subnetIP[2])<<8 + uint32(subnetIP[3])
		ipInt += uint32(offset)

		subnetIP[0] = byte(ipInt >> 24)
		subnetIP[1] = byte(ipInt >> 16)
		subnetIP[2] = byte(ipInt >> 8)
		subnetIP[3] = byte(ipInt)

		subnet := fmt.Sprintf("%s/%d", subnetIP.String(), subnetSize)
		subnets = append(subnets, subnet)
	}

	return subnets, nil
}

// FindNextAvailableSubnet finds the next available subnet sequentially
func (ia *IpamAllocator) FindNextAvailableSubnet(sliceSubnet string, subnetSize int, allocatedSubnets []string) (string, error) {
	subnets, err := ia.GenerateSubnetList(sliceSubnet, subnetSize)
	if err != nil {
		return "", err
	}

	// Create a map for O(1) lookup of allocated subnets
	allocated := make(map[string]bool)
	for _, subnet := range allocatedSubnets {
		allocated[subnet] = true
	}

	// Find the first available subnet
	for _, subnet := range subnets {
		if !allocated[subnet] {
			return subnet, nil
		}
	}

	return "", ErrNoAvailableSubnets
}

// FindNextAvailableSubnetWithReclamation finds the next available subnet with support for reclaiming released subnets
func (ia *IpamAllocator) FindNextAvailableSubnetWithReclamation(sliceSubnet string, subnetSize int, allocations []ClusterSubnetAllocation, reclaimAfter time.Duration) (string, bool, error) {
	subnets, err := ia.GenerateSubnetList(sliceSubnet, subnetSize)
	if err != nil {
		return "", false, err
	}

	// Create maps for different allocation states
	activeAllocated := make(map[string]bool)
	reclaimableSubnets := make(map[string]ClusterSubnetAllocation)

	currentTime := time.Now()

	for _, allocation := range allocations {
		switch allocation.Status {
		case "Allocated", "InUse":
			activeAllocated[allocation.Subnet] = true
		case "Released":
			// Check if subnet can be reclaimed based on release time
			if allocation.ReleasedAt != nil {
				if allocation.ReleasedAt.After(currentTime.Add(-reclaimAfter)) {
					// Too recent to reclaim, treat as allocated
					activeAllocated[allocation.Subnet] = true
				} else {
					// Available for reclamation
					reclaimableSubnets[allocation.Subnet] = allocation
				}
			} else {
				// ReleasedAt is nil, treat as reclaimable immediately (edge case)
				reclaimableSubnets[allocation.Subnet] = allocation
			}
		}
	}

	// First priority: Try to reclaim a released subnet (reuse existing allocation)
	for _, subnet := range subnets {
		if _, canReclaim := reclaimableSubnets[subnet]; canReclaim {
			return subnet, true, nil // true indicates this is a reclaimed subnet
		}
	}

	// Second priority: Find a completely new subnet
	for _, subnet := range subnets {
		if !activeAllocated[subnet] && reclaimableSubnets[subnet].Subnet == "" {
			return subnet, false, nil // false indicates this is a new subnet
		}
	}

	return "", false, ErrNoAvailableSubnets
}

// FindOptimalSubnet finds the optimal subnet considering cluster hints and fragmentation
func (ia *IpamAllocator) FindOptimalSubnet(sliceSubnet string, subnetSize int, allocatedSubnets []string, clusterHint string) (string, error) {
	subnets, err := ia.GenerateSubnetList(sliceSubnet, subnetSize)
	if err != nil {
		return "", err
	}

	// Create a map for O(1) lookup of allocated subnets
	allocated := make(map[string]bool)
	for _, subnet := range allocatedSubnets {
		allocated[subnet] = true
	}

	// If cluster hint is provided, try to find a subnet that has some correlation
	if clusterHint != "" {
		preferredIndex := ia.calculatePreferredIndex(clusterHint, len(subnets))

		// Search around the preferred index
		for offset := 0; offset < len(subnets); offset++ {
			// Try both directions from the preferred index
			for _, direction := range []int{1, -1} {
				idx := (preferredIndex + direction*offset + len(subnets)) % len(subnets)
				if !allocated[subnets[idx]] {
					return subnets[idx], nil
				}
			}
		}
	}

	// Fallback to sequential allocation if hint-based allocation fails
	return ia.FindNextAvailableSubnet(sliceSubnet, subnetSize, allocatedSubnets)
}

// calculatePreferredIndex calculates a preferred starting index based on cluster hint
func (ia *IpamAllocator) calculatePreferredIndex(clusterHint string, totalSubnets int) int {
	// Simple hash function to distribute clusters
	hash := 0
	for _, char := range clusterHint {
		hash = hash*31 + int(char)
	}
	return int(math.Abs(float64(hash))) % totalSubnets
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

	// Check if either network contains the other's network address
	return net1.Contains(net2.IP) || net2.Contains(net1.IP), nil
}

// GetSubnetUtilization calculates the utilization percentage of the slice subnet
func (ia *IpamAllocator) GetSubnetUtilization(sliceSubnet string, subnetSize int, allocatedCount int) (float64, error) {
	maxClusters, err := ia.CalculateMaxClusters(sliceSubnet, subnetSize)
	if err != nil {
		return 0, err
	}

	if maxClusters == 0 {
		return 0, errors.New("no clusters can be allocated")
	}

	return float64(allocatedCount) / float64(maxClusters) * 100, nil
}

// CompactAllocations suggests a compacted allocation to minimize fragmentation
func (ia *IpamAllocator) CompactAllocations(sliceSubnet string, subnetSize int, activeSubnets []string) ([]string, error) {
	subnets, err := ia.GenerateSubnetList(sliceSubnet, subnetSize)
	if err != nil {
		return nil, err
	}

	if len(activeSubnets) == 0 {
		return []string{}, nil
	}

	// Sort active subnets to maintain order
	sort.Strings(activeSubnets)

	// Find the first len(activeSubnets) subnets from the generated list
	compactedSubnets := make([]string, 0, len(activeSubnets))
	for i := 0; i < len(activeSubnets) && i < len(subnets); i++ {
		compactedSubnets = append(compactedSubnets, subnets[i])
	}

	return compactedSubnets, nil
}

// PredictAllocationNeeds predicts future allocation requirements based on growth rate
func (ia *IpamAllocator) PredictAllocationNeeds(sliceSubnet string, subnetSize int, currentAllocations int, growthRate float64) (int, error) {
	maxClusters, err := ia.CalculateMaxClusters(sliceSubnet, subnetSize)
	if err != nil {
		return 0, err
	}

	if growthRate < 0 {
		return 0, ErrNegativeGrowthRate
	}

	// Simple prediction: current + (current * growth_rate)
	predicted := int(float64(currentAllocations) * (1 + growthRate))

	// Ensure we don't exceed maximum capacity
	if predicted > maxClusters {
		predicted = maxClusters
	}

	return predicted, nil
}

// ValidateAllocationConsistency validates that allocations are consistent and non-overlapping
func (ia *IpamAllocator) ValidateAllocationConsistency(allocations []ClusterSubnetAllocation) error {
	if len(allocations) == 0 {
		return nil
	}

	// Check for duplicate cluster names
	clusterNames := make(map[string]bool)
	subnets := make([]string, 0, len(allocations))

	for _, allocation := range allocations {
		if allocation.ClusterName == "" {
			return ErrEmptyClusterName
		}

		if clusterNames[allocation.ClusterName] {
			return fmt.Errorf("%w: %s", ErrDuplicateCluster, allocation.ClusterName)
		}
		clusterNames[allocation.ClusterName] = true

		if allocation.Subnet == "" {
			return fmt.Errorf("%w for cluster %s", ErrEmptySubnet, allocation.ClusterName)
		}

		// Validate subnet format - use simple CIDR validation for cluster subnets
		_, _, err := net.ParseCIDR(allocation.Subnet)
		if err != nil {
			return fmt.Errorf("invalid subnet %s for cluster %s: %v", allocation.Subnet, allocation.ClusterName, err)
		}

		subnets = append(subnets, allocation.Subnet)
	}

	// Check for overlapping subnets
	for i := 0; i < len(subnets); i++ {
		for j := i + 1; j < len(subnets); j++ {
			overlapping, err := ia.IsSubnetOverlapping(subnets[i], subnets[j])
			if err != nil {
				return fmt.Errorf("error checking subnet overlap: %v", err)
			}
			if overlapping {
				return fmt.Errorf("%w: %s and %s", ErrOverlappingSubnets, subnets[i], subnets[j])
			}
		}
	}

	return nil
}

// GetCachedAllocation retrieves cached allocation information for a slice
func (ia *IpamAllocator) GetCachedAllocation(sliceName string) (*AllocationCache, bool) {
	if sliceName == "" {
		return nil, false
	}

	ia.mutex.RLock()
	defer ia.mutex.RUnlock()

	cache, exists := ia.cache[sliceName]
	if !exists {
		return nil, false
	}

	// Check if cache has expired
	if time.Since(cache.LastUpdated) > cache.TTL {
		// Cache expired, but don't remove it here to avoid deadlock
		// Let the caller handle cache invalidation
		return nil, false
	}

	return cache, true
}

// UpdateCache updates the cache with new allocation information
func (ia *IpamAllocator) UpdateCache(sliceName string, cache *AllocationCache) {
	if sliceName == "" {
		return
	}

	ia.mutex.Lock()
	defer ia.mutex.Unlock()

	if cache == nil {
		delete(ia.cache, sliceName)
		return
	}

	cache.LastUpdated = time.Now()
	if cache.TTL == 0 {
		cache.TTL = DefaultCacheTTL
	}

	ia.cache[sliceName] = cache
}

// InvalidateCache removes cached allocation information for a slice
func (ia *IpamAllocator) InvalidateCache(sliceName string) {
	if sliceName == "" {
		return
	}

	ia.mutex.Lock()
	defer ia.mutex.Unlock()

	delete(ia.cache, sliceName)
}

// CleanupExpiredCache removes expired entries from the cache
func (ia *IpamAllocator) CleanupExpiredCache() {
	ia.mutex.Lock()
	defer ia.mutex.Unlock()

	now := time.Now()
	for sliceName, cache := range ia.cache {
		if now.Sub(cache.LastUpdated) > cache.TTL {
			delete(ia.cache, sliceName)
		}
	}
}

// GetCacheStats returns statistics about the cache
func (ia *IpamAllocator) GetCacheStats() map[string]interface{} {
	ia.mutex.RLock()
	defer ia.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_entries": len(ia.cache),
		"entries":       make(map[string]interface{}),
	}

	entries := make(map[string]interface{})
	for sliceName, cache := range ia.cache {
		entries[sliceName] = map[string]interface{}{
			"slice_subnet":    cache.SliceSubnet,
			"subnet_size":     cache.SubnetSize,
			"allocated_count": len(cache.AllocatedSubnets),
			"last_updated":    cache.LastUpdated,
			"ttl":             cache.TTL.String(),
			"expires_at":      cache.LastUpdated.Add(cache.TTL),
		}
	}

	stats["entries"] = entries
	return stats
}

// isPrivateIPAMSubnet checks if the given CIDR is within private IP ranges
func isPrivateIPAMSubnet(cidr string) bool {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	// Define private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",     // Class A private range
		"172.16.0.0/12",  // Class B private range
		"192.168.0.0/16", // Class C private range
	}

	// Check if the subnet's network address falls within any private range
	for _, privateRange := range privateRanges {
		_, privateNet, err := net.ParseCIDR(privateRange)
		if err != nil {
			continue
		}

		// Check if the subnet's network address is contained within the private range
		if privateNet.Contains(ipNet.IP) {
			return true
		}
	}

	return false
}
