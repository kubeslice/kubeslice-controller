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
	"time"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const SliceIpamFinalizer = "controller.kubeslice.io/slice-ipam-finalizer"

type ISliceIpamService interface {
	ReconcileSliceIpam(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	AllocateSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) (string, error)
	ReleaseSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) error
	GetClusterSubnet(ctx context.Context, sliceName, clusterName, namespace string) (string, error)
	CreateSliceIpam(ctx context.Context, sliceConfig *v1alpha1.SliceConfig) error
	DeleteSliceIpam(ctx context.Context, sliceName, namespace string) error
}

// SliceIpamService follows existing service struct pattern
type SliceIpamService struct {
	mf        metrics.IMetricRecorder // Standard metrics field
	allocator *util.IpamAllocator
}

// NewSliceIpamService creates a new SliceIpam service instance
func NewSliceIpamService(mf metrics.IMetricRecorder) *SliceIpamService {
	return &SliceIpamService{
		mf:        mf,
		allocator: util.NewIpamAllocator(),
	}
}

// ReconcileSliceIpam reconciles SliceIpam resources following existing service patterns
func (s *SliceIpamService) ReconcileSliceIpam(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Starting reconciliation for SliceIpam %s", req.NamespacedName)

	// Get SliceIpam resource
	sliceIpam := &v1alpha1.SliceIpam{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, sliceIpam)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("SliceIpam %s not found, may have been deleted", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Load metrics with project name and namespace
	s.mf.WithProject(util.GetProjectName(sliceIpam.Namespace)).
		WithNamespace(sliceIpam.Namespace)

	// Handle deletion
	if sliceIpam.DeletionTimestamp != nil {
		return s.handleSliceIpamDeletion(ctx, sliceIpam)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(sliceIpam, SliceIpamFinalizer) {
		controllerutil.AddFinalizer(sliceIpam, SliceIpamFinalizer)
		return ctrl.Result{}, util.UpdateResource(ctx, sliceIpam)
	}

	// Reconcile logic
	return s.reconcileSliceIpamState(ctx, sliceIpam)
}

// AllocateSubnetForCluster allocates a subnet for a specific cluster in a slice
func (s *SliceIpamService) AllocateSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) (string, error) {
	logger := util.CtxLogger(ctx)
	startTime := time.Now()

	logger.Infof("Allocating subnet for cluster %s in slice %s", clusterName, sliceName)

	// Get SliceIpam resource
	sliceIpam := &v1alpha1.SliceIpam{}
	key := types.NamespacedName{Name: sliceName, Namespace: namespace}
	found, err := util.GetResourceIfExist(ctx, key, sliceIpam)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("SliceIpam %s not found", sliceName)
	}

	// Initialize status counters if not set (for backward compatibility or if status update failed)
	if sliceIpam.Status.TotalSubnets == 0 {
		totalSubnets, err := s.allocator.CalculateMaxClusters(sliceIpam.Spec.SliceSubnet, sliceIpam.Spec.SubnetSize)
		if err != nil {
			logger.Errorf("Failed to calculate total subnets: %v", err)
		} else {
			sliceIpam.Status.TotalSubnets = totalSubnets
			sliceIpam.Status.AvailableSubnets = totalSubnets - len(sliceIpam.Status.AllocatedSubnets)
		}
	}

	// Check if cluster already has allocation
	var releasedIndex = -1
	var releasedSubnet string
	for i, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.ClusterName == clusterName {
			if allocation.Status == "Allocated" || allocation.Status == "InUse" {
				logger.Infof("Cluster %s already has subnet %s allocated", clusterName, allocation.Subnet)
				return allocation.Subnet, nil
			}
			// Track if there's a Released entry for this cluster
			if allocation.Status == "Released" {
				releasedIndex = i
				releasedSubnet = allocation.Subnet
				logger.Infof("Found released subnet %s for cluster %s at index %d", releasedSubnet, clusterName, releasedIndex)
			}
		}
	}

	// Find next available subnet
	allocatedSubnets := make([]string, 0, len(sliceIpam.Status.AllocatedSubnets))
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status == "Allocated" || allocation.Status == "InUse" {
			allocatedSubnets = append(allocatedSubnets, allocation.Subnet)
		}
	}

	subnet, err := s.allocator.FindNextAvailableSubnet(
		sliceIpam.Spec.SliceSubnet,
		sliceIpam.Spec.SubnetSize,
		allocatedSubnets,
	)
	if err != nil {
		return "", fmt.Errorf("failed to find available subnet: %v", err)
	}

	// If we found a Released entry for this cluster, update it (regardless of subnet match)
	// This prevents duplicate entries for the same cluster
	if releasedIndex >= 0 {
		logger.Infof("Reusing released entry for cluster %s - updating from %s to %s", clusterName, releasedSubnet, subnet)
		sliceIpam.Status.AllocatedSubnets[releasedIndex].Subnet = subnet
		sliceIpam.Status.AllocatedSubnets[releasedIndex].Status = "Allocated"
		sliceIpam.Status.AllocatedSubnets[releasedIndex].AllocatedAt = metav1.Now()
		sliceIpam.Status.AllocatedSubnets[releasedIndex].ReleasedAt = nil
	} else {
		// Add new allocation to status
		logger.Infof("Allocating new subnet %s for cluster %s - appending new entry", subnet, clusterName)
		allocation := v1alpha1.ClusterSubnetAllocation{
			ClusterName: clusterName,
			Subnet:      subnet,
			AllocatedAt: metav1.Now(),
			Status:      "Allocated",
		}
		sliceIpam.Status.AllocatedSubnets = append(sliceIpam.Status.AllocatedSubnets, allocation)
	}

	// Update counters
	if sliceIpam.Status.AvailableSubnets > 0 {
		sliceIpam.Status.AvailableSubnets--
	}
	sliceIpam.Status.LastUpdated = metav1.Now()

	// Update status subresource (use UpdateStatus instead of UpdateResource)
	if err := util.UpdateStatus(ctx, sliceIpam); err != nil {
		// Record failure metric
		s.recordAllocationMetrics(sliceName, clusterName, namespace, time.Since(startTime), "failure")
		return "", fmt.Errorf("failed to update SliceIpam status: %v", err)
	}

	// Record success metrics
	s.recordAllocationMetrics(sliceName, clusterName, namespace, time.Since(startTime), "success")

	logger.Infof("Allocated subnet %s to cluster %s", subnet, clusterName)
	return subnet, nil
}

// ReleaseSubnetForCluster releases a subnet allocation for a specific cluster
func (s *SliceIpamService) ReleaseSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) error {
	logger := util.CtxLogger(ctx)
	logger.Infof("Releasing subnet for cluster %s in slice %s", clusterName, sliceName)

	// Get SliceIpam resource
	sliceIpam := &v1alpha1.SliceIpam{}
	key := types.NamespacedName{Name: sliceName, Namespace: namespace}
	found, err := util.GetResourceIfExist(ctx, key, sliceIpam)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("SliceIpam %s not found", sliceName)
	}

	// Find and update allocation
	found = false
	now := metav1.Now()
	for i, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.ClusterName == clusterName {
			if allocation.Status == "Allocated" || allocation.Status == "InUse" {
				sliceIpam.Status.AllocatedSubnets[i].Status = "Released"
				sliceIpam.Status.AllocatedSubnets[i].ReleasedAt = &now
				sliceIpam.Status.AvailableSubnets++
				found = true
				logger.Infof("Released subnet %s for cluster %s", allocation.Subnet, clusterName)
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("no active subnet allocation found for cluster %s", clusterName)
	}

	sliceIpam.Status.LastUpdated = metav1.Now()

	// Update status subresource (use UpdateStatus instead of UpdateResource)
	if err := util.UpdateStatus(ctx, sliceIpam); err != nil {
		return fmt.Errorf("failed to update SliceIpam status: %v", err)
	}

	// Record release metrics
	s.recordReleaseMetrics(sliceName, clusterName, namespace)

	return nil
}

// GetClusterSubnet retrieves the subnet allocated to a specific cluster
func (s *SliceIpamService) GetClusterSubnet(ctx context.Context, sliceName, clusterName, namespace string) (string, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Getting subnet for cluster %s in slice %s", clusterName, sliceName)

	// Get SliceIpam resource
	sliceIpam := &v1alpha1.SliceIpam{}
	key := types.NamespacedName{Name: sliceName, Namespace: namespace}
	found, err := util.GetResourceIfExist(ctx, key, sliceIpam)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("SliceIpam %s not found", sliceName)
	}

	// Find cluster allocation
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.ClusterName == clusterName {
			if allocation.Status == "Allocated" || allocation.Status == "InUse" {
				return allocation.Subnet, nil
			}
		}
	}

	return "", fmt.Errorf("no active subnet allocation found for cluster %s", clusterName)
}

// CreateSliceIpam creates a new SliceIpam resource from a SliceConfig
func (s *SliceIpamService) CreateSliceIpam(ctx context.Context, sliceConfig *v1alpha1.SliceConfig) error {
	logger := util.CtxLogger(ctx)
	logger.Infof("Creating SliceIpam for slice %s", sliceConfig.Name)

	// Check if SliceIpam already exists
	sliceIpam := &v1alpha1.SliceIpam{}
	key := types.NamespacedName{Name: sliceConfig.Name, Namespace: sliceConfig.Namespace}
	found, err := util.GetResourceIfExist(ctx, key, sliceIpam)
	if err != nil {
		return fmt.Errorf("failed to check if SliceIpam exists: %v", err)
	}
	if found {
		logger.Infof("SliceIpam %s already exists, skipping creation", sliceConfig.Name)
		return nil
	}

	// Use default subnet size of 24 if not specified
	subnetSize := 24

	// Calculate total subnets available
	totalSubnets, err := s.allocator.CalculateMaxClusters(
		sliceConfig.Spec.SliceSubnet,
		subnetSize,
	)
	if err != nil {
		return fmt.Errorf("failed to calculate max clusters: %v", err)
	}

	// Create SliceIpam resource
	sliceIpam = &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceConfig.Name,
			Namespace: sliceConfig.Namespace,
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   sliceConfig.Name,
			SliceSubnet: sliceConfig.Spec.SliceSubnet,
			SubnetSize:  subnetSize,
		},
		Status: v1alpha1.SliceIpamStatus{
			AllocatedSubnets: []v1alpha1.ClusterSubnetAllocation{},
			AvailableSubnets: totalSubnets,
			TotalSubnets:     totalSubnets,
			LastUpdated:      metav1.Now(),
		},
	}

	// Create the resource
	if err := util.CreateResource(ctx, sliceIpam); err != nil {
		return fmt.Errorf("failed to create SliceIpam: %v", err)
	}

	// Initialize status subresource after creation
	// Note: In Kubernetes, status subresources are not set during create, must update separately
	if err := util.UpdateStatus(ctx, sliceIpam); err != nil {
		logger.Errorf("Failed to initialize SliceIpam status: %v, will retry on next reconciliation", err)
		// Don't fail the entire create if status update fails - it will be retried
	}

	logger.Infof("Created SliceIpam %s with %d total subnets", sliceConfig.Name, totalSubnets)
	return nil
}

// DeleteSliceIpam deletes a SliceIpam resource
func (s *SliceIpamService) DeleteSliceIpam(ctx context.Context, sliceName, namespace string) error {
	logger := util.CtxLogger(ctx)
	logger.Infof("Deleting SliceIpam %s", sliceName)

	// Get SliceIpam resource
	sliceIpam := &v1alpha1.SliceIpam{}
	key := types.NamespacedName{Name: sliceName, Namespace: namespace}
	found, err := util.GetResourceIfExist(ctx, key, sliceIpam)
	if err != nil {
		return err
	}
	if !found {
		logger.Infof("SliceIpam %s not found, may already be deleted", sliceName)
		return nil
	}

	// Delete the resource
	if err := util.DeleteResource(ctx, sliceIpam); err != nil {
		return fmt.Errorf("failed to delete SliceIpam: %v", err)
	}

	logger.Infof("Deleted SliceIpam %s", sliceName)
	return nil
}

// handleSliceIpamDeletion handles the deletion of SliceIpam resources
func (s *SliceIpamService) handleSliceIpamDeletion(ctx context.Context, sliceIpam *v1alpha1.SliceIpam) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Handling deletion for SliceIpam %s", sliceIpam.Name)

	// Check for active allocations
	activeAllocations := 0
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status == "Allocated" || allocation.Status == "InUse" {
			activeAllocations++
		}
	}

	if activeAllocations > 0 {
		logger.Warnf("SliceIpam %s has %d active allocations, waiting for cleanup", sliceIpam.Name, activeAllocations)
		// Requeue after some time to check again
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(sliceIpam, SliceIpamFinalizer)
	if err := util.UpdateResource(ctx, sliceIpam); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %v", err)
	}

	logger.Infof("Successfully handled deletion for SliceIpam %s", sliceIpam.Name)
	return ctrl.Result{}, nil
}

// reconcileSliceIpamState reconciles the state of a SliceIpam resource
func (s *SliceIpamService) reconcileSliceIpamState(ctx context.Context, sliceIpam *v1alpha1.SliceIpam) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Reconciling state for SliceIpam %s", sliceIpam.Name)

	// Validate slice subnet
	if err := s.allocator.ValidateSliceSubnet(sliceIpam.Spec.SliceSubnet); err != nil {
		logger.Errorf("Invalid slice subnet %s: %v", sliceIpam.Spec.SliceSubnet, err)
		return ctrl.Result{}, err
	}

	// Calculate and update total subnets if not set
	if sliceIpam.Status.TotalSubnets == 0 {
		totalSubnets, err := s.allocator.CalculateMaxClusters(
			sliceIpam.Spec.SliceSubnet,
			sliceIpam.Spec.SubnetSize,
		)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to calculate max clusters: %v", err)
		}

		sliceIpam.Status.TotalSubnets = totalSubnets
		sliceIpam.Status.AvailableSubnets = totalSubnets - len(sliceIpam.Status.AllocatedSubnets)
		sliceIpam.Status.LastUpdated = metav1.Now()

		if err := util.UpdateStatus(ctx, sliceIpam); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update SliceIpam status: %v", err)
		}
	}

	// Sync with SliceConfig to ensure consistency (Issue 3 & 4 fix)
	if err := s.syncWithSliceConfig(ctx, sliceIpam); err != nil {
		logger.Errorf("Failed to sync with SliceConfig: %v", err)
		// Don't fail reconciliation, just log and continue
	}

	// Cleanup expired allocations (older than 24 hours)
	s.cleanupExpiredAllocations(ctx, sliceIpam)

	// Record current IPAM state metrics
	s.recordIPAMStateMetrics(sliceIpam)

	logger.Infof("Successfully reconciled SliceIpam %s", sliceIpam.Name)
	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

// syncWithSliceConfig syncs SliceIpam status with the corresponding SliceConfig
// This implements bidirectional sync (Issue 3) and automatic status population (Issue 4)
func (s *SliceIpamService) syncWithSliceConfig(ctx context.Context, sliceIpam *v1alpha1.SliceIpam) error {
	logger := util.CtxLogger(ctx)
	logger.Infof("Syncing SliceIpam %s with SliceConfig", sliceIpam.Name)

	// Get the corresponding SliceConfig
	sliceConfig := &v1alpha1.SliceConfig{}
	key := types.NamespacedName{Name: sliceIpam.Name, Namespace: sliceIpam.Namespace}
	found, err := util.GetResourceIfExist(ctx, key, sliceConfig)
	if err != nil {
		return fmt.Errorf("failed to get SliceConfig: %v", err)
	}
	if !found {
		logger.Warnf("SliceConfig %s not found, skipping sync", sliceIpam.Name)
		return nil
	}

	// Verify SliceIpamType is Dynamic
	if sliceConfig.Spec.SliceIpamType != "Dynamic" {
		logger.Warnf("SliceConfig %s is not using Dynamic IPAM, skipping sync", sliceIpam.Name)
		return nil
	}

	// Get current cluster list from SliceConfig
	configClusters := make(map[string]bool)
	for _, cluster := range sliceConfig.Spec.Clusters {
		configClusters[cluster] = true
	}

	// Get allocated clusters from SliceIpam status
	allocatedClusters := make(map[string]bool)
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status == "Allocated" || allocation.Status == "InUse" {
			allocatedClusters[allocation.ClusterName] = true
		}
	}

	needsUpdate := false

	// Check for clusters removed from SliceConfig
	for clusterName := range allocatedClusters {
		if !configClusters[clusterName] {
			logger.Infof("Cluster %s removed from SliceConfig, releasing subnet", clusterName)
			// Mark allocation as released
			for i, allocation := range sliceIpam.Status.AllocatedSubnets {
				if allocation.ClusterName == clusterName &&
					(allocation.Status == "Allocated" || allocation.Status == "InUse") {
					now := metav1.Now()
					sliceIpam.Status.AllocatedSubnets[i].Status = "Released"
					sliceIpam.Status.AllocatedSubnets[i].ReleasedAt = &now
					sliceIpam.Status.AvailableSubnets++
					needsUpdate = true
					logger.Infof("Released subnet %s for removed cluster %s", allocation.Subnet, clusterName)
					break
				}
			}
		}
	}

	// Update status counters
	if needsUpdate {
		sliceIpam.Status.LastUpdated = metav1.Now()
		if err := util.UpdateStatus(ctx, sliceIpam); err != nil {
			return fmt.Errorf("failed to update SliceIpam after sync: %v", err)
		}
		logger.Infof("Successfully synced SliceIpam %s with SliceConfig changes", sliceIpam.Name)
	}

	return nil
}

// cleanupExpiredAllocations removes old released allocations
func (s *SliceIpamService) cleanupExpiredAllocations(ctx context.Context, sliceIpam *v1alpha1.SliceIpam) {
	logger := util.CtxLogger(ctx)

	// Remove allocations that have been released for more than 24 hours
	cutoffTime := time.Now().Add(-24 * time.Hour)
	var keepAllocations []v1alpha1.ClusterSubnetAllocation
	cleaned := 0

	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status == "Released" && allocation.ReleasedAt != nil {
			if allocation.ReleasedAt.Time.Before(cutoffTime) {
				cleaned++
				continue
			}
		}
		keepAllocations = append(keepAllocations, allocation)
	}

	if cleaned > 0 {
		sliceIpam.Status.AllocatedSubnets = keepAllocations
		sliceIpam.Status.LastUpdated = metav1.Now()

		if err := util.UpdateStatus(ctx, sliceIpam); err != nil {
			logger.Errorf("Failed to cleanup expired allocations: %v", err)
		} else {
			logger.Infof("Cleaned up %d expired allocations for SliceIpam %s", cleaned, sliceIpam.Name)
			// Record cleanup metric
			s.recordCleanupMetrics(sliceIpam.Name, sliceIpam.Namespace, cleaned)
		}
	}
}

// recordAllocationMetrics records metrics for subnet allocation operations
func (s *SliceIpamService) recordAllocationMetrics(sliceName, clusterName, namespace string, duration time.Duration, status string) {
	// Set context for metrics
	mr := s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	// Record allocation latency using prometheus labels directly
	if metrics.IPAMAllocationLatency != nil {
		labels := prometheus.Labels{
			"cluster":                    clusterName,
			"slice_name":                 sliceName,
			"slice_project":              util.GetProjectName(namespace),
			"slice_cluster":              util.ClusterController,
			"slice_namespace":            namespace,
			"slice_reporting_controller": util.InstanceController,
		}
		metrics.IPAMAllocationLatency.With(labels).Observe(duration.Seconds())
	}

	// Record allocation counter
	mr.RecordCounterMetric(metrics.IPAMAllocationsTotal, map[string]string{
		"cluster": clusterName,
		"status":  status,
	})
}

// recordReleaseMetrics records metrics for subnet release operations
func (s *SliceIpamService) recordReleaseMetrics(sliceName, clusterName, namespace string) {
	// Set context for metrics
	mr := s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	// Record release counter
	mr.RecordCounterMetric(metrics.IPAMReleasesTotal, map[string]string{
		"cluster": clusterName,
	})
}

// recordCleanupMetrics records metrics for cleanup operations
func (s *SliceIpamService) recordCleanupMetrics(sliceName, namespace string, count int) {
	// Set context for metrics
	mr := s.mf.WithProject(util.GetProjectName(namespace)).
		WithNamespace(namespace).
		WithSlice(sliceName)

	// Record cleanup counter
	// Note: In Prometheus, counters can only be incremented by 1, so we increment multiple times
	for i := 0; i < count; i++ {
		mr.RecordCounterMetric(metrics.IPAMCleanupsTotal, map[string]string{})
	}
}

// recordIPAMStateMetrics records gauge metrics for IPAM state
func (s *SliceIpamService) recordIPAMStateMetrics(sliceIpam *v1alpha1.SliceIpam) {
	// Set context for metrics
	mr := s.mf.WithProject(util.GetProjectName(sliceIpam.Namespace)).
		WithNamespace(sliceIpam.Namespace).
		WithSlice(sliceIpam.Name)

	// Count allocated subnets
	allocatedCount := 0
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status == "Allocated" || allocation.Status == "InUse" {
			allocatedCount++
		}
	}

	// Record total subnets
	mr.RecordGaugeMetric(metrics.IPAMTotalSubnets, map[string]string{}, float64(sliceIpam.Status.TotalSubnets))

	// Record available subnets
	mr.RecordGaugeMetric(metrics.IPAMAvailableSubnets, map[string]string{}, float64(sliceIpam.Status.AvailableSubnets))

	// Record allocated subnets
	mr.RecordGaugeMetric(metrics.IPAMAllocatedSubnets, map[string]string{}, float64(allocatedCount))

	// Calculate and record utilization rate
	var utilizationRate float64
	if sliceIpam.Status.TotalSubnets > 0 {
		utilizationRate = (float64(allocatedCount) / float64(sliceIpam.Status.TotalSubnets)) * 100
	}
	mr.RecordGaugeMetric(metrics.IPAMUtilizationRate, map[string]string{}, utilizationRate)

	// Calculate and record fragmentation rate
	// Convert allocations to util type
	utilAllocations := make([]util.ClusterSubnetAllocation, len(sliceIpam.Status.AllocatedSubnets))
	for i, allocation := range sliceIpam.Status.AllocatedSubnets {
		utilAllocations[i] = util.ClusterSubnetAllocation{
			ClusterName: allocation.ClusterName,
			Subnet:      allocation.Subnet,
			AllocatedAt: allocation.AllocatedAt.Time,
			Status:      string(allocation.Status),
		}
		if allocation.ReleasedAt != nil {
			releasedAt := allocation.ReleasedAt.Time
			utilAllocations[i].ReleasedAt = &releasedAt
		}
	}

	fragmentation, err := s.allocator.CalculateFragmentation(
		sliceIpam.Spec.SliceSubnet,
		sliceIpam.Spec.SubnetSize,
		utilAllocations,
	)
	if err == nil {
		mr.RecordGaugeMetric(metrics.IPAMFragmentationRate, map[string]string{}, fragmentation)
	}
}
