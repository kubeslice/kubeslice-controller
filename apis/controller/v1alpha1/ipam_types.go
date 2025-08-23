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

// IPAMPoolSpec defines the desired state of IPAMPool
type IPAMPoolSpec struct {
	// SliceSubnet is the base CIDR block for the slice overlay network
	// +kubebuilder:validation:Required
	SliceSubnet string `json:"sliceSubnet"`

	// PoolType defines the type of IPAM pool (Dynamic or Static)
	// +kubebuilder:validation:Enum=Dynamic;Static
	// +kubebuilder:default=Dynamic
	PoolType string `json:"poolType"`

	// SubnetSize defines the size of individual cluster subnets (e.g., "/24", "/26")
	// +kubebuilder:validation:Required
	SubnetSize string `json:"subnetSize"`

	// MaxClusters is the maximum number of clusters that can join this slice
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:default=16
	MaxClusters int `json:"maxClusters"`

	// ReservedSubnets is a list of subnets that are reserved and cannot be allocated
	// +optional
	ReservedSubnets []string `json:"reservedSubnets,omitempty"`

	// AllocationStrategy defines how IP addresses are allocated
	// +kubebuilder:validation:Enum=Sequential;Random;Efficient
	// +kubebuilder:default=Efficient
	AllocationStrategy string `json:"allocationStrategy"`

	// AutoReclaim enables automatic reclamation of unused subnets when clusters leave
	// +kubebuilder:default=true
	AutoReclaim bool `json:"autoReclaim"`

	// ReclaimDelay is the delay before reclaiming an unused subnet (in seconds)
	// +kubebuilder:validation:Minimum=300
	// +kubebuilder:default=3600
	ReclaimDelay int `json:"reclaimDelay"`
}

// IPAMPoolStatus defines the observed state of IPAMPool
type IPAMPoolStatus struct {
	// AllocatedSubnets tracks which subnets are allocated to which clusters
	// +optional
	AllocatedSubnets map[string]ClusterSubnet `json:"allocatedSubnets,omitempty"`

	// AvailableSubnets is a list of available subnets for allocation
	// +optional
	AvailableSubnets []string `json:"availableSubnets,omitempty"`

	// ReservedSubnets is a list of subnets that are reserved
	// +optional
	ReservedSubnets []string `json:"reservedSubnets,omitempty"`

	// TotalSubnets is the total number of subnets in the pool
	TotalSubnets int `json:"totalSubnets"`

	// AllocatedCount is the number of allocated subnets
	AllocatedCount int `json:"allocatedCount"`

	// AvailableCount is the number of available subnets
	AvailableCount int `json:"availableCount"`

	// LastAllocationTime is the timestamp of the last allocation
	// +optional
	LastAllocationTime *metav1.Time `json:"lastAllocationTime,omitempty"`

	// LastReclamationTime is the timestamp of the last reclamation
	// +optional
	LastReclamationTime *metav1.Time `json:"lastReclamationTime,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ClusterSubnet represents a subnet allocated to a specific cluster
type ClusterSubnet struct {
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`

	// SubnetCIDR is the allocated subnet CIDR
	SubnetCIDR string `json:"subnetCIDR"`

	// AllocatedAt is when the subnet was allocated
	AllocatedAt metav1.Time `json:"allocatedAt"`

	// LastUsedAt is when the subnet was last used
	// +optional
	LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`

	// Status indicates the current status of the subnet
	// +kubebuilder:validation:Enum=Active;Inactive;PendingReclamation
	Status string `json:"status"`

	// Labels are additional labels for the subnet
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// IPAMPool is the Schema for the ipampools API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SliceSubnet",type="string",JSONPath=".spec.sliceSubnet"
// +kubebuilder:printcolumn:name="PoolType",type="string",JSONPath=".spec.poolType"
// +kubebuilder:printcolumn:name="Allocated",type="integer",JSONPath=".status.allocatedCount"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableCount"
// +kubebuilder:printcolumn:name="Total",type="integer",JSONPath=".status.totalSubnets"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type IPAMPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPAMPoolSpec   `json:"spec,omitempty"`
	Status IPAMPoolStatus `json:"status,omitempty"`
}

// IPAMPoolList contains a list of IPAMPool
// +kubebuilder:object:root=true
type IPAMPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPAMPool `json:"items"`
}

// IPAMAllocation represents a request for IP allocation
type IPAMAllocation struct {
	// ClusterName is the name of the cluster requesting allocation
	ClusterName string `json:"clusterName"`

	// SliceName is the name of the slice
	SliceName string `json:"sliceName"`

	// Namespace is the namespace of the slice
	Namespace string `json:"namespace"`

	// RequestedSubnetSize is the requested subnet size (optional, uses pool default if not specified)
	// +optional
	RequestedSubnetSize *string `json:"requestedSubnetSize,omitempty"`

	// Labels are additional labels for the allocation
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// IPAMDeallocation represents a request for IP deallocation
type IPAMDeallocation struct {
	// ClusterName is the name of the cluster to deallocate
	ClusterName string `json:"clusterName"`

	// SliceName is the name of the slice
	SliceName string `json:"sliceName"`

	// Namespace is the namespace of the slice
	Namespace string `json:"namespace"`

	// Force forces deallocation even if the subnet is still in use
	// +optional
	Force bool `json:"force,omitempty"`
}

func init() {
	SchemeBuilder.Register(&IPAMPool{}, &IPAMPoolList{})
}

