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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidateSliceIpamCreate validates SliceIpam creation
func ValidateSliceIpamCreate(ctx context.Context, req admission.Request, obj runtime.Object) admission.Response {
	sliceIpam, ok := obj.(*v1alpha1.SliceIpam)
	if !ok {
		return admission.Errored(400, fmt.Errorf("expected SliceIpam object"))
	}

	// Validate slice subnet
	if err := validateSliceIpamSubnet(sliceIpam.Spec.SliceSubnet); err != nil {
		return admission.Denied(fmt.Sprintf("Invalid slice subnet: %v", err))
	}

	// Validate subnet size
	if err := validateIpamSubnetSize(sliceIpam.Spec.SliceSubnet, sliceIpam.Spec.SubnetSize); err != nil {
		return admission.Denied(fmt.Sprintf("Invalid subnet size: %v", err))
	}

	// Validate slice name
	if sliceIpam.Spec.SliceName == "" {
		return admission.Denied("Slice name cannot be empty")
	}

	return admission.Allowed("SliceIpam creation is valid")
}

// ValidateSliceIpamUpdate validates SliceIpam updates
func ValidateSliceIpamUpdate(ctx context.Context, req admission.Request, oldObj, newObj runtime.Object) admission.Response {
	oldSliceIpam, ok := oldObj.(*v1alpha1.SliceIpam)
	if !ok {
		return admission.Errored(400, fmt.Errorf("expected SliceIpam object"))
	}

	newSliceIpam, ok := newObj.(*v1alpha1.SliceIpam)
	if !ok {
		return admission.Errored(400, fmt.Errorf("expected SliceIpam object"))
	}

	// Prevent changes to immutable fields
	if oldSliceIpam.Spec.SliceName != newSliceIpam.Spec.SliceName {
		return admission.Denied("Slice name cannot be changed")
	}

	if oldSliceIpam.Spec.SliceSubnet != newSliceIpam.Spec.SliceSubnet {
		return admission.Denied("Slice subnet cannot be changed")
	}

	// Allow subnet size changes only if no subnets are allocated
	if oldSliceIpam.Spec.SubnetSize != newSliceIpam.Spec.SubnetSize {
		if len(oldSliceIpam.Status.AllocatedSubnets) > 0 {
			return admission.Denied("Subnet size cannot be changed when subnets are allocated")
		}
	}

	// Validate new subnet size if changed
	if oldSliceIpam.Spec.SubnetSize != newSliceIpam.Spec.SubnetSize {
		if err := validateIpamSubnetSize(newSliceIpam.Spec.SliceSubnet, newSliceIpam.Spec.SubnetSize); err != nil {
			return admission.Denied(fmt.Sprintf("Invalid subnet size: %v", err))
		}
	}

	return admission.Allowed("SliceIpam update is valid")
}

// ValidateSliceIpamDelete validates SliceIpam deletion
func ValidateSliceIpamDelete(ctx context.Context, req admission.Request, obj runtime.Object) admission.Response {
	sliceIpam, ok := obj.(*v1alpha1.SliceIpam)
	if !ok {
		return admission.Errored(400, fmt.Errorf("expected SliceIpam object"))
	}

	// Check if there are active subnet allocations
	activeAllocations := 0
	for _, allocation := range sliceIpam.Status.AllocatedSubnets {
		if allocation.Status == "InUse" {
			activeAllocations++
		}
	}

	if activeAllocations > 0 {
		return admission.Denied(fmt.Sprintf("Cannot delete SliceIpam with %d active subnet allocations", activeAllocations))
	}

	return admission.Allowed("SliceIpam deletion is valid")
}

// validateSliceIpamSubnet validates the slice subnet CIDR
func validateSliceIpamSubnet(sliceSubnet string) error {
	if sliceSubnet == "" {
		return fmt.Errorf("slice subnet cannot be empty")
	}

	_, _, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %v", err)
	}

	return nil
}

// validateIpamSubnetSize validates the subnet size
func validateIpamSubnetSize(sliceSubnet string, subnetSize int) error {
	if subnetSize < 16 || subnetSize > 30 {
		return fmt.Errorf("subnet size must be between 16 and 30")
	}

	_, sliceNet, err := net.ParseCIDR(sliceSubnet)
	if err != nil {
		return fmt.Errorf("invalid slice subnet: %v", err)
	}

	sliceOnes, _ := sliceNet.Mask.Size()
	if subnetSize <= sliceOnes {
		return fmt.Errorf("subnet size /%d must be larger than slice subnet size /%d", subnetSize, sliceOnes)
	}

	return nil
}
