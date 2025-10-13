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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Allocated;InUse;Released
type SubnetAllocationStatus string

const (
	SubnetStatusAllocated SubnetAllocationStatus = "Allocated"
	SubnetStatusInUse     SubnetAllocationStatus = "InUse"
	SubnetStatusReleased  SubnetAllocationStatus = "Released"
)

// SliceIpamSpec defines the desired state of SliceIpam
type SliceIpamSpec struct {
	// SliceName is the name of the slice for which IPAM is being managed
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SliceName string `json:"sliceName"`

	// SliceSubnet is the CIDR block allocated to the slice
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}\/([0-9]|[1-2][0-9]|3[0-2])$`
	SliceSubnet string `json:"sliceSubnet"`

	// SubnetSize defines the size of subnets to be allocated to clusters
	// +kubebuilder:validation:Minimum=16
	// +kubebuilder:validation:Maximum=30
	// +kubebuilder:default=24
	SubnetSize int `json:"subnetSize,omitempty"`
}

// ClusterSubnetAllocation represents the subnet allocation for a specific cluster
type ClusterSubnetAllocation struct {
	// ClusterName is the name of the cluster
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName"`

	// Subnet is the CIDR block allocated to the cluster
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}\/([0-9]|[1-2][0-9]|3[0-2])$`
	Subnet string `json:"subnet"`

	// AllocatedAt is the timestamp when the subnet was allocated
	// +kubebuilder:validation:Required
	AllocatedAt metav1.Time `json:"allocatedAt"`

	// Status represents the current status of the subnet allocation
	// +kubebuilder:default=Allocated
	Status SubnetAllocationStatus `json:"status,omitempty"`

	// ReleasedAt is the timestamp when the subnet was released (only set when status="Released")
	ReleasedAt *metav1.Time `json:"releasedAt,omitempty"`
}

// SliceIpamStatus defines the observed state of SliceIpam
type SliceIpamStatus struct {
	// AllocatedSubnets contains the list of subnet allocations for clusters
	AllocatedSubnets []ClusterSubnetAllocation `json:"allocatedSubnets,omitempty"`

	// AvailableSubnets is the number of available subnets that can be allocated
	// Note: omitempty removed to ensure 0 is displayed when pool is exhausted
	AvailableSubnets int `json:"availableSubnets"`

	// TotalSubnets is the total number of subnets that can be created from the slice subnet
	TotalSubnets int `json:"totalSubnets,omitempty"`

	// LastUpdated is the timestamp when the status was last updated
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Slice Name",type=string,JSONPath=`.spec.sliceName`
//+kubebuilder:printcolumn:name="Slice Subnet",type=string,JSONPath=`.spec.sliceSubnet`
//+kubebuilder:printcolumn:name="Subnet Size",type=integer,JSONPath=`.spec.subnetSize`
//+kubebuilder:printcolumn:name="Available Subnets",type=integer,JSONPath=`.status.availableSubnets`
//+kubebuilder:printcolumn:name="Total Subnets",type=integer,JSONPath=`.status.totalSubnets`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SliceIpam is the Schema for the sliceipams API
type SliceIpam struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SliceIpamSpec   `json:"spec,omitempty"`
	Status SliceIpamStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SliceIpamList contains a list of SliceIpam
type SliceIpamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SliceIpam `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SliceIpam{}, &SliceIpamList{})
}
