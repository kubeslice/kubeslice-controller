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
	"net"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"github.com/kubeslice/kubeslice-controller/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ISliceIpamService interface {
	ReconcileSliceIpam(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	AllocateSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) (string, error)
	ReleaseSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) error
	GetClusterSubnet(ctx context.Context, sliceName, clusterName, namespace string) (string, error)
	CreateSliceIpam(ctx context.Context, sliceConfig *v1alpha1.SliceConfig) error
	DeleteSliceIpam(ctx context.Context, sliceName, namespace string) error
}

// SliceIpamService implements ISliceIpamService interface
type SliceIpamService struct {
	mf metrics.IMetricRecorder
}

const SliceIpamFinalizer = "controller.kubeslice.io/slice-ipam-finalizer"

// ReconcileSliceIpam reconciles a SliceIpam resource
func (s *SliceIpamService) ReconcileSliceIpam(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Started Reconciliation of SliceIpam %v", req.NamespacedName)

	sliceIpam := &v1alpha1.SliceIpam{}
	found, err := util.GetResourceIfExist(ctx, req.NamespacedName, sliceIpam)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		logger.Infof("SliceIpam %v not found, returning from reconciler loop.", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Handle finalizers
	if sliceIpam.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(sliceIpam.GetFinalizers(), SliceIpamFinalizer) {
			if shouldReturn, result, reconErr := util.IsReconciled(util.AddFinalizer(ctx, sliceIpam, SliceIpamFinalizer)); shouldReturn {
				return result, reconErr
			}
		}
	} else {
		logger.Debug("Starting delete for SliceIpam", req.NamespacedName)
		if shouldReturn, result, reconErr := util.IsReconciled(s.cleanUpSliceIpamResources(ctx, sliceIpam)); shouldReturn {
			return result, reconErr
		}
		if shouldReturn, result, reconErr := util.IsReconciled(util.RemoveFinalizer(ctx, sliceIpam, SliceIpamFinalizer)); shouldReturn {
			return result, reconErr
		}
		return ctrl.Result{}, nil
	}

	// Update status
	return s.updateSliceIpamStatus(ctx, sliceIpam)
}

// AllocateSubnetForCluster allocates a subnet for a cluster
func (s *SliceIpamService) AllocateSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) (string, error) {
	logger := util.CtxLogger(ctx)

	sliceIpam := &v1alpha1.SliceIpam{}
	namespacedName := types.NamespacedName{
		Name:      sliceName + "-ipam",
		Namespace: namespace,
	}

	found, err := util.GetResourceIfExist(ctx, namespacedName, sliceIpam)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("SliceIpam not found for slice %s", sliceName)
	}

	// Check if cluster already has an allocation
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.ClusterName == clusterName {
			if allocation.Status == "Released" {
				// Reactivate the allocation
				allocation.Status = "InUse"
				allocation.AllocatedAt = metav1.Now()
				_, err := s.updateSliceIpamStatus(ctx, sliceIpam)
				return allocation.Subnet, err
			}
			return allocation.Subnet, nil
		}
	}

	// Allocate new subnet
	subnet, err := s.allocateNextAvailableSubnet(sliceIpam)
	if err != nil {
		return "", err
	}

	// Add allocation to status
	allocation := v1alpha1.ClusterSubnetAllocation{
		ClusterName: clusterName,
		Subnet:      subnet,
		AllocatedAt: metav1.Now(),
		Status:      "InUse",
	}
	sliceIpam.Status.AllocatedSubnets = append(sliceIpam.Status.AllocatedSubnets, allocation)

	_, err = s.updateSliceIpamStatus(ctx, sliceIpam)
	if err != nil {
		return "", err
	}

	logger.Infof("Allocated subnet %s to cluster %s in slice %s", subnet, clusterName, sliceName)
	return subnet, nil
}

// ReleaseSubnetForCluster releases a subnet allocated to a cluster
func (s *SliceIpamService) ReleaseSubnetForCluster(ctx context.Context, sliceName, clusterName, namespace string) error {
	logger := util.CtxLogger(ctx)

	sliceIpam := &v1alpha1.SliceIpam{}
	namespacedName := types.NamespacedName{
		Name:      sliceName + "-ipam",
		Namespace: namespace,
	}

	found, err := util.GetResourceIfExist(ctx, namespacedName, sliceIpam)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("SliceIpam not found for slice %s", sliceName)
	}

	// Find and mark allocation as released
	for i, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.ClusterName == clusterName {
			sliceIpam.Status.AllocatedSubnets[i].Status = "Released"
			_, err = s.updateSliceIpamStatus(ctx, sliceIpam)
			if err != nil {
				return err
			}
			logger.Infof("Released subnet %s from cluster %s in slice %s", allocation.Subnet, clusterName, sliceName)
			return nil
		}
	}

	return fmt.Errorf("no subnet allocation found for cluster %s in slice %s", clusterName, sliceName)
}

// GetClusterSubnet returns the subnet allocated to a cluster
func (s *SliceIpamService) GetClusterSubnet(ctx context.Context, sliceName, clusterName, namespace string) (string, error) {
	sliceIpam := &v1alpha1.SliceIpam{}
	namespacedName := types.NamespacedName{
		Name:      sliceName + "-ipam",
		Namespace: namespace,
	}

	found, err := util.GetResourceIfExist(ctx, namespacedName, sliceIpam)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("SliceIpam not found for slice %s", sliceName)
	}

	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.ClusterName == clusterName && allocation.Status != "Released" {
			return allocation.Subnet, nil
		}
	}

	return "", fmt.Errorf("no active subnet allocation found for cluster %s in slice %s", clusterName, sliceName)
}

// CreateSliceIpam creates a new SliceIpam resource for a slice
func (s *SliceIpamService) CreateSliceIpam(ctx context.Context, sliceConfig *v1alpha1.SliceConfig) error {
	logger := util.CtxLogger(ctx)

	sliceIpam := &v1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceConfig.Name + "-ipam",
			Namespace: sliceConfig.Namespace,
			Labels: map[string]string{
				util.LabelProjectName: util.GetProjectName(sliceConfig.Namespace),
				util.LabelManagedBy:   "kubeslice-controller",
			},
		},
		Spec: v1alpha1.SliceIpamSpec{
			SliceName:   sliceConfig.Name,
			SliceSubnet: sliceConfig.Spec.SliceSubnet,
			SubnetSize:  24, // Default to /24 subnets
		},
	}

	err := util.CreateResource(ctx, sliceIpam)
	if err != nil {
		return err
	}

	logger.Infof("Created SliceIpam %s for slice %s", sliceIpam.Name, sliceConfig.Name)
	return nil
}

// DeleteSliceIpam deletes a SliceIpam resource
func (s *SliceIpamService) DeleteSliceIpam(ctx context.Context, sliceName, namespace string) error {
	logger := util.CtxLogger(ctx)

	sliceIpam := &v1alpha1.SliceIpam{}
	namespacedName := types.NamespacedName{
		Name:      sliceName + "-ipam",
		Namespace: namespace,
	}

	found, err := util.GetResourceIfExist(ctx, namespacedName, sliceIpam)
	if err != nil {
		return err
	}
	if !found {
		logger.Infof("SliceIpam %s not found, nothing to delete", namespacedName)
		return nil
	}

	err = util.DeleteResource(ctx, sliceIpam)
	if err != nil {
		return err
	}

	logger.Infof("Deleted SliceIpam %s for slice %s", sliceIpam.Name, sliceName)
	return nil
}

// allocateNextAvailableSubnet finds the next available subnet to allocate
func (s *SliceIpamService) allocateNextAvailableSubnet(sliceIpam *v1alpha1.SliceIpam) (string, error) {
	_, sliceNet, err := net.ParseCIDR(sliceIpam.Spec.SliceSubnet)
	if err != nil {
		return "", fmt.Errorf("invalid slice subnet: %v", err)
	}

	// Get all allocated subnets
	allocatedSubnets := make(map[string]bool)
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status != "Released" {
			allocatedSubnets[allocation.Subnet] = true
		}
	}

	// Generate subnets and find first available
	subnets, err := s.generateSubnets(sliceNet, sliceIpam.Spec.SubnetSize)
	if err != nil {
		return "", err
	}

	for _, subnet := range subnets {
		if !allocatedSubnets[subnet] {
			return subnet, nil
		}
	}

	return "", fmt.Errorf("no available subnets in slice %s", sliceIpam.Spec.SliceName)
}

// generateSubnets generates all possible subnets from the slice subnet
func (s *SliceIpamService) generateSubnets(sliceNet *net.IPNet, subnetSize int) ([]string, error) {
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
		s.incrementIP(baseIP, increment)
	}

	return subnets, nil
}

// incrementIP increments an IP address by the given amount
func (s *SliceIpamService) incrementIP(ip net.IP, increment int) {
	// Convert increment to big-endian bytes and add to IP
	for i := len(ip) - 1; i >= 0 && increment > 0; i-- {
		sum := int(ip[i]) + increment
		ip[i] = byte(sum & 0xFF)
		increment = sum >> 8
	}
}

// updateSliceIpamStatus updates the status of SliceIpam
func (s *SliceIpamService) updateSliceIpamStatus(ctx context.Context, sliceIpam *v1alpha1.SliceIpam) (ctrl.Result, error) {
	// Calculate available and total subnets
	_, sliceNet, err := net.ParseCIDR(sliceIpam.Spec.SliceSubnet)
	if err != nil {
		return ctrl.Result{}, err
	}

	sliceOnes, _ := sliceNet.Mask.Size()
	totalSubnets := 1 << (sliceIpam.Spec.SubnetSize - sliceOnes)

	activeAllocations := 0
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status != "Released" {
			activeAllocations++
		}
	}

	sliceIpam.Status.TotalSubnets = totalSubnets
	sliceIpam.Status.AvailableSubnets = totalSubnets - activeAllocations
	sliceIpam.Status.LastUpdated = metav1.Now()

	err = util.UpdateResource(ctx, sliceIpam)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// cleanUpSliceIpamResources cleans up resources when SliceIpam is deleted
func (s *SliceIpamService) cleanUpSliceIpamResources(ctx context.Context, sliceIpam *v1alpha1.SliceIpam) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Cleaning up SliceIpam resources for %s", sliceIpam.Name)

	// Mark all allocations as released
	for i := range sliceIpam.Status.AllocatedSubnets {
		sliceIpam.Status.AllocatedSubnets[i].Status = "Released"
	}

	err := util.UpdateResource(ctx, sliceIpam)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
