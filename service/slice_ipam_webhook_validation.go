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

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	apiGroupKubeSliceControllersIPAM = "controller.kubeslice.io"
)

// ValidateSliceIpamCreate validates SliceIpam creation
func ValidateSliceIpamCreate(ctx context.Context, sliceIpam *controllerv1alpha1.SliceIpam) (admission.Warnings, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Validating SliceIpam creation: %s/%s", sliceIpam.Namespace, sliceIpam.Name)

	// Validate slice name is not empty
	if err := validateSliceName(sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	// Validate slice subnet CIDR format and constraints
	if err := validateSliceSubnetIPAM(sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	// Validate subnet size constraints
	if err := validateSubnetSize(sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	// Validate that corresponding SliceConfig exists and uses Dynamic IPAM
	if err := validateSliceConfigExists(ctx, sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	// Validate no duplicate SliceIpam for the same slice
	if err := validateNoDuplicateSliceIpam(ctx, sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	return nil, nil
}

// ValidateSliceIpamUpdate validates SliceIpam updates
func ValidateSliceIpamUpdate(ctx context.Context, sliceIpam *controllerv1alpha1.SliceIpam, old runtime.Object) (admission.Warnings, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Validating SliceIpam update: %s/%s", sliceIpam.Namespace, sliceIpam.Name)

	oldSliceIpam := old.(*controllerv1alpha1.SliceIpam)

	// Validate immutable fields are not changed
	if err := validateImmutableFields(sliceIpam, oldSliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	// Validate subnet size if it's being changed
	if sliceIpam.Spec.SubnetSize != oldSliceIpam.Spec.SubnetSize {
		if err := validateSubnetSizeChange(ctx, sliceIpam, oldSliceIpam); err != nil {
			return nil, apierrors.NewInvalid(
				schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
				sliceIpam.Name,
				field.ErrorList{err},
			)
		}
	}

	// Validate subnet size constraints
	if err := validateSubnetSize(sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	return nil, nil
}

// ValidateSliceIpamDelete validates SliceIpam deletion
func ValidateSliceIpamDelete(ctx context.Context, sliceIpam *controllerv1alpha1.SliceIpam) (admission.Warnings, error) {
	logger := util.CtxLogger(ctx)
	logger.Infof("Validating SliceIpam deletion: %s/%s", sliceIpam.Namespace, sliceIpam.Name)

	// Validate that there are no active allocations
	if err := validateNoActiveAllocations(sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	// Check if corresponding SliceConfig still exists and is using this SliceIpam
	if err := validateSliceConfigDeletion(ctx, sliceIpam); err != nil {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: apiGroupKubeSliceControllersIPAM, Kind: "SliceIpam"},
			sliceIpam.Name,
			field.ErrorList{err},
		)
	}

	return nil, nil
}

// validateSliceName validates that slice name is not empty
func validateSliceName(sliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	if sliceIpam.Spec.SliceName == "" {
		return field.Required(
			field.NewPath("spec").Child("sliceName"),
			"slice name cannot be empty",
		)
	}
	return nil
}

// validateSliceSubnetIPAM validates the slice subnet CIDR format and constraints
func validateSliceSubnetIPAM(sliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	if sliceIpam.Spec.SliceSubnet == "" {
		return field.Required(
			field.NewPath("spec").Child("sliceSubnet"),
			"slice subnet cannot be empty",
		)
	}

	// Parse CIDR
	ip, ipNet, err := net.ParseCIDR(sliceIpam.Spec.SliceSubnet)
	if err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("sliceSubnet"),
			sliceIpam.Spec.SliceSubnet,
			fmt.Sprintf("invalid CIDR format: %v", err),
		)
	}

	// Validate it's a network address (not host address)
	if !ip.Equal(ipNet.IP) {
		return field.Invalid(
			field.NewPath("spec").Child("sliceSubnet"),
			sliceIpam.Spec.SliceSubnet,
			"CIDR must be a network address, not a host address",
		)
	}

	// Validate it's IPv4
	if ip.To4() == nil {
		return field.Invalid(
			field.NewPath("spec").Child("sliceSubnet"),
			sliceIpam.Spec.SliceSubnet,
			"only IPv4 CIDRs are supported",
		)
	}

	// Validate it's a private IP range (RFC 1918)
	if !isPrivateIP(ip) {
		return field.Invalid(
			field.NewPath("spec").Child("sliceSubnet"),
			sliceIpam.Spec.SliceSubnet,
			"CIDR must be in a private IP range (10.0.0.0/8, 172.16.0.0/12, or 192.168.0.0/16)",
		)
	}

	// Validate CIDR size is reasonable (between /8 and /28)
	size, _ := ipNet.Mask.Size()
	if size < 8 || size > 28 {
		return field.Invalid(
			field.NewPath("spec").Child("sliceSubnet"),
			sliceIpam.Spec.SliceSubnet,
			fmt.Sprintf("CIDR size must be between /8 and /28, got /%d", size),
		)
	}

	return nil
}

// validateSubnetSize validates the subnet size constraints
func validateSubnetSize(sliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	if sliceIpam.Spec.SubnetSize < 16 || sliceIpam.Spec.SubnetSize > 30 {
		return field.Invalid(
			field.NewPath("spec").Child("subnetSize"),
			sliceIpam.Spec.SubnetSize,
			fmt.Sprintf("subnet size must be between 16 and 30, got %d", sliceIpam.Spec.SubnetSize),
		)
	}

	// Validate that subnet size is larger than slice subnet
	_, ipNet, err := net.ParseCIDR(sliceIpam.Spec.SliceSubnet)
	if err == nil {
		sliceSize, _ := ipNet.Mask.Size()
		if sliceIpam.Spec.SubnetSize <= sliceSize {
			return field.Invalid(
				field.NewPath("spec").Child("subnetSize"),
				sliceIpam.Spec.SubnetSize,
				fmt.Sprintf("subnet size /%d must be larger than slice subnet size /%d", sliceIpam.Spec.SubnetSize, sliceSize),
			)
		}

		// Validate that at least one subnet can be allocated
		maxSubnets := 1 << uint(sliceIpam.Spec.SubnetSize-sliceSize)
		if maxSubnets < 1 {
			return field.Invalid(
				field.NewPath("spec").Child("subnetSize"),
				sliceIpam.Spec.SubnetSize,
				"subnet configuration would result in zero available subnets",
			)
		}
	}

	return nil
}

// validateSliceConfigExists validates that corresponding SliceConfig exists and uses Dynamic IPAM
func validateSliceConfigExists(ctx context.Context, sliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	key := types.NamespacedName{
		Name:      sliceIpam.Spec.SliceName,
		Namespace: sliceIpam.Namespace,
	}

	found, err := util.GetResourceIfExist(ctx, key, sliceConfig)
	if err != nil {
		return field.InternalError(
			field.NewPath("spec").Child("sliceName"),
			fmt.Errorf("failed to check if SliceConfig exists: %v", err),
		)
	}

	if !found {
		return field.Invalid(
			field.NewPath("spec").Child("sliceName"),
			sliceIpam.Spec.SliceName,
			"corresponding SliceConfig does not exist",
		)
	}

	// Validate that SliceConfig uses Dynamic IPAM
	if sliceConfig.Spec.SliceIpamType != "Dynamic" {
		return field.Invalid(
			field.NewPath("spec").Child("sliceName"),
			sliceIpam.Spec.SliceName,
			fmt.Sprintf("SliceConfig must use Dynamic IPAM type, got: %s", sliceConfig.Spec.SliceIpamType),
		)
	}

	// Validate that slice subnets match
	if sliceConfig.Spec.SliceSubnet != sliceIpam.Spec.SliceSubnet {
		return field.Invalid(
			field.NewPath("spec").Child("sliceSubnet"),
			sliceIpam.Spec.SliceSubnet,
			fmt.Sprintf("slice subnet must match SliceConfig subnet: %s", sliceConfig.Spec.SliceSubnet),
		)
	}

	return nil
}

// validateNoDuplicateSliceIpam validates that no duplicate SliceIpam exists for the same slice
func validateNoDuplicateSliceIpam(ctx context.Context, sliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	sliceIpamList := &controllerv1alpha1.SliceIpamList{}
	err := util.ListResources(ctx, sliceIpamList, client.InNamespace(sliceIpam.Namespace))
	if err != nil {
		return field.InternalError(
			field.NewPath("metadata").Child("name"),
			fmt.Errorf("failed to list existing SliceIpam resources: %v", err),
		)
	}

	for _, existingIpam := range sliceIpamList.Items {
		// Skip if it's the same resource (during update)
		if existingIpam.Name == sliceIpam.Name {
			continue
		}

		// Check if another SliceIpam exists for the same slice
		if existingIpam.Spec.SliceName == sliceIpam.Spec.SliceName {
			return field.Duplicate(
				field.NewPath("spec").Child("sliceName"),
				sliceIpam.Spec.SliceName,
			)
		}
	}

	return nil
}

// validateImmutableFields validates that immutable fields are not changed during update
func validateImmutableFields(sliceIpam *controllerv1alpha1.SliceIpam, oldSliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	// Slice name is immutable
	if sliceIpam.Spec.SliceName != oldSliceIpam.Spec.SliceName {
		return field.Forbidden(
			field.NewPath("spec").Child("sliceName"),
			"slice name is immutable",
		)
	}

	// Slice subnet is immutable
	if sliceIpam.Spec.SliceSubnet != oldSliceIpam.Spec.SliceSubnet {
		return field.Forbidden(
			field.NewPath("spec").Child("sliceSubnet"),
			"slice subnet is immutable",
		)
	}

	return nil
}

// validateSubnetSizeChange validates subnet size changes
func validateSubnetSizeChange(ctx context.Context, sliceIpam *controllerv1alpha1.SliceIpam, oldSliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	// If there are active allocations, subnet size cannot be changed
	if len(sliceIpam.Status.AllocatedSubnets) > 0 {
		// Count active allocations (not released)
		activeCount := 0
		for _, allocation := range sliceIpam.Status.AllocatedSubnets {
			if allocation.Status != controllerv1alpha1.SubnetStatusReleased {
				activeCount++
			}
		}

		if activeCount > 0 {
			return field.Forbidden(
				field.NewPath("spec").Child("subnetSize"),
				fmt.Sprintf("cannot change subnet size when there are %d active allocations", activeCount),
			)
		}
	}

	// Validate that new subnet size is compatible with existing allocations
	if sliceIpam.Spec.SubnetSize < oldSliceIpam.Spec.SubnetSize {
		return field.Invalid(
			field.NewPath("spec").Child("subnetSize"),
			sliceIpam.Spec.SubnetSize,
			"subnet size can only be increased, not decreased",
		)
	}

	return nil
}

// validateNoActiveAllocations validates that there are no active allocations before deletion
func validateNoActiveAllocations(sliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	activeCount := 0
	var activeClusters []string

	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status != controllerv1alpha1.SubnetStatusReleased {
			activeCount++
			activeClusters = append(activeClusters, allocation.ClusterName)
		}
	}

	if activeCount > 0 {
		return field.Forbidden(
			field.NewPath("metadata").Child("name"),
			fmt.Sprintf("cannot delete SliceIpam with %d active allocations for clusters: %v", activeCount, activeClusters),
		)
	}

	return nil
}

// validateSliceConfigDeletion validates SliceConfig state during SliceIpam deletion
func validateSliceConfigDeletion(ctx context.Context, sliceIpam *controllerv1alpha1.SliceIpam) *field.Error {
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	key := types.NamespacedName{
		Name:      sliceIpam.Spec.SliceName,
		Namespace: sliceIpam.Namespace,
	}

	found, err := util.GetResourceIfExist(ctx, key, sliceConfig)
	if err != nil {
		return field.InternalError(
			field.NewPath("spec").Child("sliceName"),
			fmt.Errorf("failed to check if SliceConfig exists: %v", err),
		)
	}

	// If SliceConfig exists and is not being deleted, warn
	if found && sliceConfig.DeletionTimestamp == nil {
		return field.Forbidden(
			field.NewPath("metadata").Child("name"),
			fmt.Sprintf("corresponding SliceConfig %s still exists and is not being deleted", sliceConfig.Name),
		)
	}

	return nil
}

// isPrivateIP checks if an IP is in a private range (RFC 1918)
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, ipNet, _ := net.ParseCIDR(cidr)
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}
