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
	"context"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IPAMService handles dynamic IP address allocation for slice overlay networks
type IPAMService struct {
	client client.Client
	mf     *metrics.MetricRecorder
}

// NewIPAMService creates a new IPAM service instance
func NewIPAMService(client client.Client, mf *metrics.MetricRecorder) *IPAMService {
	return &IPAMService{
		client: client,
		mf:     mf,
	}
}

// AllocateSubnetForCluster allocates a subnet for a specific cluster in a slice
func (s *IPAMService) AllocateSubnetForCluster(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig, clusterName string) (string, error) {
	logger := util.CtxLogger(ctx)

	// Check if dynamic IPAM is enabled
	if sliceConfig.Spec.SliceIpamType != "Dynamic" {
		// Fall back to static IPAM for backward compatibility
		return s.allocateStaticSubnet(ctx, sliceConfig, clusterName)
	}

	// Get or create IPAM pool for this slice
	pool, err := s.getOrCreateIPAMPool(ctx, sliceConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get/create IPAM pool: %w", err)
	}

	// Check if cluster already has an allocated subnet
	if existingSubnet, exists := pool.Status.AllocatedSubnets[clusterName]; exists {
		if existingSubnet.Status == "Active" {
			logger.Infof("Cluster %s already has allocated subnet %s", clusterName, existingSubnet.SubnetCIDR)
			return existingSubnet.SubnetCIDR, nil
		}
	}

	// Allocate a new subnet
	subnet, err := s.allocateSubnetFromPool(ctx, pool, clusterName, sliceConfig.Name, sliceConfig.Namespace)
	if err != nil {
		return "", fmt.Errorf("failed to allocate subnet: %w", err)
	}

	logger.Infof("Successfully allocated subnet %s for cluster %s in slice %s", subnet, clusterName, sliceConfig.Name)
	return subnet, nil
}

// DeallocateSubnetForCluster deallocates a subnet for a specific cluster
func (s *IPAMService) DeallocateSubnetForCluster(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig, clusterName string) error {
	logger := util.CtxLogger(ctx)

	// Check if dynamic IPAM is enabled
	if sliceConfig.Spec.SliceIpamType != "Dynamic" {
		// No deallocation needed for static IPAM
		return nil
	}

	// Get IPAM pool for this slice
	pool, err := s.getIPAMPool(ctx, sliceConfig.Name, sliceConfig.Namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pool doesn't exist, nothing to deallocate
			return nil
		}
		return fmt.Errorf("failed to get IPAM pool: %w", err)
	}

	// Deallocate the subnet
	err = s.deallocateSubnetFromPool(ctx, pool, clusterName)
	if err != nil {
		return fmt.Errorf("failed to deallocate subnet: %w", err)
	}

	logger.Infof("Successfully deallocated subnet for cluster %s in slice %s", clusterName, sliceConfig.Name)
	return nil
}

// getOrCreateIPAMPool gets an existing IPAM pool or creates a new one
func (s *IPAMService) getOrCreateIPAMPool(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) (*controllerv1alpha1.IPAMPool, error) {
	pool, err := s.getIPAMPool(ctx, sliceConfig.Name, sliceConfig.Namespace)
	if err == nil {
		return pool, nil
	}

	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Create new IPAM pool
	pool = &controllerv1alpha1.IPAMPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ipam-pool", sliceConfig.Name),
			Namespace: sliceConfig.Namespace,
			Labels: map[string]string{
				"slice-name": sliceConfig.Name,
				"ipam-type":  "dynamic",
			},
		},
		Spec: controllerv1alpha1.IPAMPoolSpec{
			SliceSubnet:        sliceConfig.Spec.SliceSubnet,
			PoolType:           "Dynamic",
			SubnetSize:         s.calculateOptimalSubnetSize(sliceConfig.Spec.MaxClusters),
			MaxClusters:        sliceConfig.Spec.MaxClusters,
			AllocationStrategy: "Efficient",
			AutoReclaim:        true,
			ReclaimDelay:       3600, // 1 hour
		},
	}

	// Initialize the pool
	err = s.InitializePool(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IPAM pool: %w", err)
	}

	// Create the pool
	err = s.client.Create(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("failed to create IPAM pool: %w", err)
	}

	return pool, nil
}

// getIPAMPool retrieves an existing IPAM pool
func (s *IPAMService) getIPAMPool(ctx context.Context, sliceName, namespace string) (*controllerv1alpha1.IPAMPool, error) {
	pool := &controllerv1alpha1.IPAMPool{}
	err := s.client.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf("%s-ipam-pool", sliceName),
		Namespace: namespace,
	}, pool)
	return pool, err
}

// InitializePool initializes the IPAM pool with available subnets
func (s *IPAMService) InitializePool(ctx context.Context, pool *controllerv1alpha1.IPAMPool) error {
	sliceSubnet := pool.Spec.SliceSubnet
	subnetSize := pool.Spec.SubnetSize

	// Parse the base slice subnet
	_, baseNet, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return fmt.Errorf("invalid slice subnet %s: %w", sliceSubnet, err)
	}

	// Parse the subnet size
	subnetSizeInt, err := strconv.Atoi(strings.TrimPrefix(subnetSize, "/"))
	if err != nil {
		return fmt.Errorf("invalid subnet size %s: %w", subnetSize, err)
	}

	// Calculate the number of subnets available
	baseMask, _ := baseNet.Mask.Size()
	subnetBits := subnetSizeInt - baseMask
	if subnetBits <= 0 {
		return fmt.Errorf("subnet size %s is too large for base subnet %s", subnetSize, sliceSubnet)
	}

	totalSubnets := int(math.Pow(2, float64(subnetBits)))
	if totalSubnets < pool.Spec.MaxClusters {
		return fmt.Errorf("subnet size %s only provides %d subnets, but max clusters is %d", subnetSize, totalSubnets, pool.Spec.MaxClusters)
	}

	// Generate available subnets
	availableSubnets := make([]string, 0, totalSubnets)
	for i := 0; i < totalSubnets; i++ {
		subnet := s.generateSubnet(baseNet, i, subnetSizeInt)
		availableSubnets = append(availableSubnets, subnet)
	}

	// Initialize pool status
	pool.Status = controllerv1alpha1.IPAMPoolStatus{
		AllocatedSubnets: make(map[string]controllerv1alpha1.ClusterSubnet),
		AvailableSubnets: availableSubnets,
		ReservedSubnets:  pool.Spec.ReservedSubnets,
		TotalSubnets:     totalSubnets,
		AllocatedCount:   0,
		AvailableCount:   totalSubnets,
	}

	return nil
}

// generateSubnet generates a subnet at the specified index
func (s *IPAMService) generateSubnet(baseNet *net.IPNet, index, subnetSize int) string {
	// Convert base network to IP
	baseIP := baseNet.IP.To4()
	if baseIP == nil {
		baseIP = baseNet.IP.To16()
	}

	// Calculate the offset for this subnet
	offset := index << (32 - subnetSize)

	// Create the new IP
	newIP := make(net.IP, len(baseIP))
	copy(newIP, baseIP)

	// Add the offset
	for i := len(newIP) - 1; i >= 0 && offset > 0; i-- {
		sum := int(newIP[i]) + (offset & 0xFF)
		newIP[i] = byte(sum & 0xFF)
		offset >>= 8
	}

	return fmt.Sprintf("%s/%d", newIP.String(), subnetSize)
}

// allocateSubnetFromPool allocates a subnet from the available pool
func (s *IPAMService) allocateSubnetFromPool(ctx context.Context, pool *controllerv1alpha1.IPAMPool, clusterName, sliceName, namespace string) (string, error) {
	if len(pool.Status.AvailableSubnets) == 0 {
		return "", fmt.Errorf("no available subnets in IPAM pool")
	}

	// Select subnet based on allocation strategy
	var selectedSubnet string
	switch pool.Spec.AllocationStrategy {
	case "Sequential":
		selectedSubnet = pool.Status.AvailableSubnets[0]
	case "Random":
		// For now, use sequential as random would require additional logic
		selectedSubnet = pool.Status.AvailableSubnets[0]
	case "Efficient":
		selectedSubnet = s.selectEfficientSubnet(pool)
	default:
		selectedSubnet = pool.Status.AvailableSubnets[0]
	}

	// Remove from available subnets
	pool.Status.AvailableSubnets = s.removeSubnetFromSlice(pool.Status.AvailableSubnets, selectedSubnet)

	// Add to allocated subnets
	now := metav1.Now()
	pool.Status.AllocatedSubnets[clusterName] = controllerv1alpha1.ClusterSubnet{
		ClusterName: clusterName,
		SubnetCIDR:  selectedSubnet,
		AllocatedAt: now,
		LastUsedAt:  &now,
		Status:      "Active",
		Labels: map[string]string{
			"slice-name": sliceName,
			"namespace":  namespace,
		},
	}

	// Update counts
	pool.Status.AllocatedCount++
	pool.Status.AvailableCount--
	pool.Status.LastAllocationTime = &now

	// Update the pool
	err := s.client.Status().Update(ctx, pool)
	if err != nil {
		return "", fmt.Errorf("failed to update IPAM pool: %w", err)
	}

	return selectedSubnet, nil
}

// selectEfficientSubnet selects the most efficient subnet based on current allocations
func (s *IPAMService) selectEfficientSubnet(pool *controllerv1alpha1.IPAMPool) string {
	if len(pool.Status.AvailableSubnets) == 0 {
		return ""
	}

	// For efficient allocation, prefer subnets that are closer to existing allocations
	// This helps maintain network locality
	if len(pool.Status.AllocatedSubnets) == 0 {
		return pool.Status.AvailableSubnets[0]
	}

	// Simple strategy: return the first available subnet
	// In a more sophisticated implementation, this could analyze network topology
	return pool.Status.AvailableSubnets[0]
}

// deallocateSubnetFromPool deallocates a subnet and returns it to the available pool
func (s *IPAMService) deallocateSubnetFromPool(ctx context.Context, pool *controllerv1alpha1.IPAMPool, clusterName string) error {
	clusterSubnet, exists := pool.Status.AllocatedSubnets[clusterName]
	if !exists {
		return nil // Nothing to deallocate
	}

	// Check if auto-reclaim is enabled
	if pool.Spec.AutoReclaim {
		// Mark for reclamation
		clusterSubnet.Status = "PendingReclamation"
		pool.Status.AllocatedSubnets[clusterName] = clusterSubnet

		// Schedule reclamation after delay
		go func() {
			time.Sleep(time.Duration(pool.Spec.ReclaimDelay) * time.Second)
			s.reclaimSubnet(context.Background(), pool, clusterName)
		}()
	} else {
		// Immediate deallocation
		subnet := clusterSubnet.SubnetCIDR

		// Remove from allocated subnets
		delete(pool.Status.AllocatedSubnets, clusterName)

		// Add back to available subnets
		pool.Status.AvailableSubnets = append(pool.Status.AvailableSubnets, subnet)

		// Update counts
		pool.Status.AllocatedCount--
		pool.Status.AvailableCount++

		// Sort available subnets for consistency
		sort.Strings(pool.Status.AvailableSubnets)
	}

	// Update the pool
	err := s.client.Status().Update(ctx, pool)
	if err != nil {
		return fmt.Errorf("failed to update IPAM pool: %w", err)
	}

	return nil
}

// reclaimSubnet reclaims a subnet that was marked for reclamation
func (s *IPAMService) reclaimSubnet(ctx context.Context, pool *controllerv1alpha1.IPAMPool, clusterName string) {
	clusterSubnet, exists := pool.Status.AllocatedSubnets[clusterName]
	if !exists || clusterSubnet.Status != "PendingReclamation" {
		return
	}

	subnet := clusterSubnet.SubnetCIDR

	// Remove from allocated subnets
	delete(pool.Status.AllocatedSubnets, clusterName)

	// Add back to available subnets
	pool.Status.AvailableSubnets = append(pool.Status.AvailableSubnets, subnet)

	// Update counts
	pool.Status.AllocatedCount--
	pool.Status.AvailableCount++

	// Update timestamps
	now := metav1.Now()
	pool.Status.LastReclamationTime = &now

	// Sort available subnets for consistency
	sort.Strings(pool.Status.AvailableSubnets)

	// Update the pool
	err := s.client.Status().Update(ctx, pool)
	if err != nil {
		util.CtxLogger(ctx).Errorf("Failed to reclaim subnet %s: %v", subnet, err)
	}
}

// removeSubnetFromSlice removes a subnet from a slice of subnets
func (s *IPAMService) removeSubnetFromSlice(subnets []string, subnet string) []string {
	// Create a copy to avoid modifying the original slice
	result := make([]string, len(subnets))
	copy(result, subnets)

	for i, sub := range result {
		if sub == subnet {
			return append(result[:i], result[i+1:]...)
		}
	}
	return result
}

// calculateOptimalSubnetSize calculates the optimal subnet size based on max clusters
func (s *IPAMService) calculateOptimalSubnetSize(maxClusters int) string {
	// Use the existing logic from util.FindCIDRByMaxClusters but adapt it for individual cluster subnets
	baseCidr := 24 // Start with /24 for individual clusters
	for i := 6; i >= 0; i-- {
		if float64(maxClusters) > math.Pow(2, float64(i)) {
			value := baseCidr + i
			return fmt.Sprintf("/%d", value)
		}
	}
	return "/24" // Default to /24
}

// allocateStaticSubnet provides backward compatibility with the existing static IPAM
func (s *IPAMService) allocateStaticSubnet(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig, clusterName string) (string, error) {
	// Use the existing static IPAM logic
	clusterCidr := util.FindCIDRByMaxClusters(sliceConfig.Spec.MaxClusters)

	// Find the cluster's position in the slice
	clusterIndex := -1
	for i, cluster := range sliceConfig.Spec.Clusters {
		if cluster == clusterName {
			clusterIndex = i
			break
		}
	}

	if clusterIndex == -1 {
		return "", fmt.Errorf("cluster %s not found in slice %s", clusterName, sliceConfig.Name)
	}

	// Generate subnet using existing logic
	subnet := util.GetClusterPrefixPool(sliceConfig.Spec.SliceSubnet, clusterIndex, clusterCidr)
	return subnet, nil
}

// GetClusterSubnet retrieves the allocated subnet for a cluster
func (s *IPAMService) GetClusterSubnet(ctx context.Context, sliceName, namespace, clusterName string) (string, error) {
	// Try dynamic IPAM first
	pool, err := s.getIPAMPool(ctx, sliceName, namespace)
	if err == nil {
		if clusterSubnet, exists := pool.Status.AllocatedSubnets[clusterName]; exists {
			return clusterSubnet.SubnetCIDR, nil
		}
	}

	// Fall back to static IPAM
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	err = s.client.Get(ctx, types.NamespacedName{
		Name:      sliceName,
		Namespace: namespace,
	}, sliceConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get slice config: %w", err)
	}

	return s.allocateStaticSubnet(ctx, sliceConfig, clusterName)
}

// ReconcileIPAMPool reconciles the IPAM pool for a slice
func (s *IPAMService) ReconcileIPAMPool(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) error {
	if sliceConfig.Spec.SliceIpamType != "Dynamic" {
		return nil // No reconciliation needed for static IPAM
	}

	pool, err := s.getIPAMPool(ctx, sliceConfig.Name, sliceConfig.Namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pool doesn't exist, create it
			_, err = s.getOrCreateIPAMPool(ctx, sliceConfig)
			return err
		}
		return fmt.Errorf("failed to get IPAM pool: %w", err)
	}

	// Check if pool needs updates
	needsUpdate := false

	// Update max clusters if changed
	if pool.Spec.MaxClusters != sliceConfig.Spec.MaxClusters {
		pool.Spec.MaxClusters = sliceConfig.Spec.MaxClusters
		needsUpdate = true
	}

	// Update slice subnet if changed
	if pool.Spec.SliceSubnet != sliceConfig.Spec.SliceSubnet {
		pool.Spec.SliceSubnet = sliceConfig.Spec.SliceSubnet
		needsUpdate = true
	}

	if needsUpdate {
		// Reinitialize the pool with new parameters
		err = s.InitializePool(ctx, pool)
		if err != nil {
			return fmt.Errorf("failed to reinitialize IPAM pool: %w", err)
		}

		err = s.client.Update(ctx, pool)
		if err != nil {
			return fmt.Errorf("failed to update IPAM pool: %w", err)
		}
	}

	return nil
}
