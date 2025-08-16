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
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SliceIpamSpec defines the desired state of SliceIpam
type SliceIpamSpec struct {
	// SliceName is the name of the slice this IPAM manages
	SliceName string `json:"sliceName"`
	// SliceSubnet is the overall CIDR block for the slice
	SliceSubnet string `json:"sliceSubnet"`
	// SubnetSize is the size of subnets to allocate to clusters (e.g., 24 for /24 subnets)
	//+kubebuilder:validation:Minimum=16
	//+kubebuilder:validation:Maximum=30
	//+kubebuilder:default:=24
	SubnetSize int `json:"subnetSize,omitempty"`
}

// ClusterSubnetAllocation represents a subnet allocated to a cluster
type ClusterSubnetAllocation struct {
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`
	// Subnet is the CIDR block allocated to this cluster
	Subnet string `json:"subnet"`
	// AllocatedAt is the timestamp when this subnet was allocated
	AllocatedAt metav1.Time `json:"allocatedAt"`
	// Status indicates the current status of this allocation
	//+kubebuilder:validation:Enum:=Allocated;InUse;Released
	//+kubebuilder:default:=Allocated
	Status string `json:"status,omitempty"`
}

// SliceIpamStatus defines the observed state of SliceIpam
type SliceIpamStatus struct {
	// AllocatedSubnets contains the list of subnets allocated to clusters
	AllocatedSubnets []ClusterSubnetAllocation `json:"allocatedSubnets,omitempty"`
	// AvailableSubnets is the count of available subnets that can be allocated
	AvailableSubnets int `json:"availableSubnets,omitempty"`
	// TotalSubnets is the total number of subnets that can be created from the slice subnet
	TotalSubnets int `json:"totalSubnets,omitempty"`
	// LastUpdated is the timestamp when the status was last updated
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:webhook:path=/validate-controller-kubeslice-io-v1alpha1-sliceipam,mutating=false,failurePolicy=fail,sideEffects=None,groups=controller.kubeslice.io,resources=sliceipams,verbs=create;update;delete,versions=v1alpha1,name=vsliceipam.kb.io,admissionReviewVersions=v1

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

// SetupWebhookWithManager sets up the webhook with the manager
func (r *SliceIpam) SetupWebhookWithManager(mgr ctrl.Manager,
	validateCreate func(ctx context.Context, req admission.Request, obj runtime.Object) admission.Response,
	validateUpdate func(ctx context.Context, req admission.Request, oldObj, newObj runtime.Object) admission.Response,
	validateDelete func(ctx context.Context, req admission.Request, obj runtime.Object) admission.Response) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&sliceIpamValidator{
			validateCreate: validateCreate,
			validateUpdate: validateUpdate,
			validateDelete: validateDelete,
		}).
		Complete()
}

type sliceIpamValidator struct {
	validateCreate func(ctx context.Context, req admission.Request, obj runtime.Object) admission.Response
	validateUpdate func(ctx context.Context, req admission.Request, oldObj, newObj runtime.Object) admission.Response
	validateDelete func(ctx context.Context, req admission.Request, obj runtime.Object) admission.Response
}

func (v *sliceIpamValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	req := admission.Request{}
	resp := v.validateCreate(ctx, req, obj)
	if !resp.Allowed {
		return fmt.Errorf("validation failed: %s", resp.Result.Message)
	}
	return nil
}

func (v *sliceIpamValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	req := admission.Request{}
	resp := v.validateUpdate(ctx, req, oldObj, newObj)
	if !resp.Allowed {
		return fmt.Errorf("validation failed: %s", resp.Result.Message)
	}
	return nil
}

func (v *sliceIpamValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	req := admission.Request{}
	resp := v.validateDelete(ctx, req, obj)
	if !resp.Allowed {
		return fmt.Errorf("validation failed: %s", resp.Result.Message)
	}
	return nil
}
