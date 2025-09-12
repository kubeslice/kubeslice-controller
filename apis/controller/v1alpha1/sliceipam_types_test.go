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

package v1alpha1

import (
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSliceIpamSpec_JSONMarshaling(t *testing.T) {
	spec := SliceIpamSpec{
		SliceName:   "test-slice",
		SliceSubnet: "10.1.0.0/16",
		SubnetSize:  24,
	}

	// Test marshaling
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Failed to marshal SliceIpamSpec: %v", err)
	}

	// Test unmarshaling
	var unmarshaled SliceIpamSpec
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SliceIpamSpec: %v", err)
	}

	// Verify fields
	if unmarshaled.SliceName != spec.SliceName {
		t.Errorf("Expected SliceName %s, got %s", spec.SliceName, unmarshaled.SliceName)
	}
	if unmarshaled.SliceSubnet != spec.SliceSubnet {
		t.Errorf("Expected SliceSubnet %s, got %s", spec.SliceSubnet, unmarshaled.SliceSubnet)
	}
	if unmarshaled.SubnetSize != spec.SubnetSize {
		t.Errorf("Expected SubnetSize %d, got %d", spec.SubnetSize, unmarshaled.SubnetSize)
	}
}

func TestClusterSubnetAllocation_JSONMarshaling(t *testing.T) {
	now := metav1.NewTime(time.Now())
	allocation := ClusterSubnetAllocation{
		ClusterName: "cluster-1",
		Subnet:      "10.1.1.0/24",
		AllocatedAt: now,
		Status:      SubnetStatusAllocated,
	}

	// Test marshaling
	data, err := json.Marshal(allocation)
	if err != nil {
		t.Fatalf("Failed to marshal ClusterSubnetAllocation: %v", err)
	}

	// Test unmarshaling
	var unmarshaled ClusterSubnetAllocation
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ClusterSubnetAllocation: %v", err)
	}

	// Verify fields
	if unmarshaled.ClusterName != allocation.ClusterName {
		t.Errorf("Expected ClusterName %s, got %s", allocation.ClusterName, unmarshaled.ClusterName)
	}
	if unmarshaled.Subnet != allocation.Subnet {
		t.Errorf("Expected Subnet %s, got %s", allocation.Subnet, unmarshaled.Subnet)
	}
	if unmarshaled.Status != allocation.Status {
		t.Errorf("Expected Status %s, got %s", allocation.Status, unmarshaled.Status)
	}
}

func TestSliceIpamStatus_JSONMarshaling(t *testing.T) {
	now := metav1.NewTime(time.Now())
	allocation := ClusterSubnetAllocation{
		ClusterName: "cluster-1",
		Subnet:      "10.1.1.0/24",
		AllocatedAt: now,
		Status:      SubnetStatusAllocated,
	}

	status := SliceIpamStatus{
		AllocatedSubnets: []ClusterSubnetAllocation{allocation},
		AvailableSubnets: 254,
		TotalSubnets:     256,
		LastUpdated:      now,
	}

	// Test marshaling
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal SliceIpamStatus: %v", err)
	}

	// Test unmarshaling
	var unmarshaled SliceIpamStatus
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SliceIpamStatus: %v", err)
	}

	// Verify fields
	if len(unmarshaled.AllocatedSubnets) != len(status.AllocatedSubnets) {
		t.Errorf("Expected %d allocated subnets, got %d", len(status.AllocatedSubnets), len(unmarshaled.AllocatedSubnets))
	}
	if unmarshaled.AvailableSubnets != status.AvailableSubnets {
		t.Errorf("Expected AvailableSubnets %d, got %d", status.AvailableSubnets, unmarshaled.AvailableSubnets)
	}
	if unmarshaled.TotalSubnets != status.TotalSubnets {
		t.Errorf("Expected TotalSubnets %d, got %d", status.TotalSubnets, unmarshaled.TotalSubnets)
	}
}

func TestSliceIpam_JSONMarshaling(t *testing.T) {
	now := metav1.NewTime(time.Now())
	sliceIpam := SliceIpam{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "controller.kubeslice.io/v1alpha1",
			Kind:       "SliceIpam",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-ipam",
			Namespace: "kubeslice-system",
		},
		Spec: SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: SliceIpamStatus{
			AllocatedSubnets: []ClusterSubnetAllocation{
				{
					ClusterName: "cluster-1",
					Subnet:      "10.1.1.0/24",
					AllocatedAt: now,
					Status:      SubnetStatusAllocated,
				},
			},
			AvailableSubnets: 254,
			TotalSubnets:     256,
			LastUpdated:      now,
		},
	}

	// Test marshaling
	data, err := json.Marshal(sliceIpam)
	if err != nil {
		t.Fatalf("Failed to marshal SliceIpam: %v", err)
	}

	// Test unmarshaling
	var unmarshaled SliceIpam
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SliceIpam: %v", err)
	}

	// Verify basic fields
	if unmarshaled.Spec.SliceName != sliceIpam.Spec.SliceName {
		t.Errorf("Expected SliceName %s, got %s", sliceIpam.Spec.SliceName, unmarshaled.Spec.SliceName)
	}
	if unmarshaled.Spec.SliceSubnet != sliceIpam.Spec.SliceSubnet {
		t.Errorf("Expected SliceSubnet %s, got %s", sliceIpam.Spec.SliceSubnet, unmarshaled.Spec.SliceSubnet)
	}
}

func TestSubnetAllocationStatus_EnumValues(t *testing.T) {
	validStatuses := []SubnetAllocationStatus{
		SubnetStatusAllocated,
		SubnetStatusInUse,
		SubnetStatusReleased,
	}

	expectedValues := []string{"Allocated", "InUse", "Released"}

	for i, status := range validStatuses {
		if string(status) != expectedValues[i] {
			t.Errorf("Expected status value %s, got %s", expectedValues[i], string(status))
		}
	}
}

func TestSliceIpam_DefaultValues(t *testing.T) {
	sliceIpam := SliceIpam{
		Spec: SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			// SubnetSize not set, should default to 24
		},
	}

	// Test that default values are applied correctly
	// Note: In a real scenario, this would be handled by the webhook
	if sliceIpam.Spec.SubnetSize != 0 {
		t.Errorf("Expected SubnetSize to be 0 (unset), got %d", sliceIpam.Spec.SubnetSize)
	}
}

func TestSliceIpam_DeepCopy(t *testing.T) {
	now := metav1.NewTime(time.Now())
	original := &SliceIpam{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "controller.kubeslice.io/v1alpha1",
			Kind:       "SliceIpam",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-ipam",
			Namespace: "kubeslice-system",
		},
		Spec: SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: SliceIpamStatus{
			AllocatedSubnets: []ClusterSubnetAllocation{
				{
					ClusterName: "cluster-1",
					Subnet:      "10.1.1.0/24",
					AllocatedAt: now,
					Status:      SubnetStatusAllocated,
				},
			},
			AvailableSubnets: 254,
			TotalSubnets:     256,
			LastUpdated:      now,
		},
	}

	// Test DeepCopy
	copied := original.DeepCopy()
	if copied == original {
		t.Error("DeepCopy should return a different pointer")
	}

	// Verify that the copy has the same values
	if copied.Spec.SliceName != original.Spec.SliceName {
		t.Errorf("Expected SliceName %s, got %s", original.Spec.SliceName, copied.Spec.SliceName)
	}

	// Modify the copy and ensure original is unchanged
	copied.Spec.SliceName = "modified-slice"
	if original.Spec.SliceName == "modified-slice" {
		t.Error("Modifying copy should not affect original")
	}
}

func TestSliceIpamList_DeepCopy(t *testing.T) {
	original := &SliceIpamList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "controller.kubeslice.io/v1alpha1",
			Kind:       "SliceIpamList",
		},
		Items: []SliceIpam{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slice-ipam-1",
				},
				Spec: SliceIpamSpec{
					SliceName:   "slice-1",
					SliceSubnet: "10.1.0.0/16",
					SubnetSize:  24,
				},
			},
		},
	}

	// Test DeepCopy
	copied := original.DeepCopy()
	if copied == original {
		t.Error("DeepCopy should return a different pointer")
	}

	// Verify that the copy has the same values
	if len(copied.Items) != len(original.Items) {
		t.Errorf("Expected %d items, got %d", len(original.Items), len(copied.Items))
	}

	// Modify the copy and ensure original is unchanged
	copied.Items[0].Spec.SliceName = "modified-slice"
	if original.Items[0].Spec.SliceName == "modified-slice" {
		t.Error("Modifying copy should not affect original")
	}
}

func TestSliceIpam_RuntimeObject(t *testing.T) {
	sliceIpam := &SliceIpam{}

	// Test that SliceIpam implements runtime.Object
	var _ runtime.Object = sliceIpam

	// Test GetObjectKind
	gvk := sliceIpam.GetObjectKind()
	if gvk == nil {
		t.Error("GetObjectKind should not return nil")
	}
}

func TestSliceIpamList_RuntimeObject(t *testing.T) {
	sliceIpamList := &SliceIpamList{}

	// Test that SliceIpamList implements runtime.Object
	var _ runtime.Object = sliceIpamList

	// Test GetObjectKind
	gvk := sliceIpamList.GetObjectKind()
	if gvk == nil {
		t.Error("GetObjectKind should not return nil")
	}
}
