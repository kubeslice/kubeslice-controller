/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VpnKeyRotationSpec defines the desired state of VpnKeyRotation
type VpnKeyRotationSpec struct {
	SliceName string `json:"sliceName,omitempty"`
	// ClusterGatewayMapping represents a map where key is cluster name and value is array of gateways present on that cluster.
	// This is used to avoid unnecessary reconciliation in worker-operator.
	ClusterGatewayMapping map[string][]string `json:"clusterGatewayMapping,omitempty"`
	// CertificateCreationTime is a time when certificate for all the gateway pairs is created/updated
	CertificateCreationTime *metav1.Time `json:"certificateCreationTime,omitempty"`
	// CertificateExpiryTime is a time when certificate for all the gateway pairs will expire
	CertificateExpiryTime *metav1.Time `json:"certificateExpiryTime,omitempty"`
	RotationInterval      int          `json:"rotationInterval,omitempty"`
	// clusters contains the list of clusters attached to this slice
	Clusters []string `json:"clusters,omitempty"`
	// RotationCount represent the number of times rotation has been already performed.
	RotationCount int `json:"rotationCount,omitempty"`
}

// VpnKeyRotationStatus defines the observed state of VpnKeyRotation
type VpnKeyRotationStatus struct {
	// This is map of gateway name to the current rotation state
	CurrentRotationState map[string]StatusOfKeyRotation `json:"currentRotationState,omitempty"`
	// This is circular array of last n number of rotation status.
	StatusHistory map[string][]StatusOfKeyRotation `json:"statusHistory,omitempty"`
}

// StatusOfKeyRotation represent per gateway status
type StatusOfKeyRotation struct {
	Status               string      `json:"status,omitempty"`
	LastUpdatedTimestamp metav1.Time `json:"lastUpdatedTimestamp,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VpnKeyRotation is the Schema for the vpnkeyrotations API
type VpnKeyRotation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpnKeyRotationSpec   `json:"spec,omitempty"`
	Status VpnKeyRotationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VpnKeyRotationList contains a list of VpnKeyRotation
type VpnKeyRotationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VpnKeyRotation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VpnKeyRotation{}, &VpnKeyRotationList{})
}

// status of key rotation updated by workers
const (
	SecretReadInProgress string = "READ_IN_PROGRESS"
	SecretUpdated        string = "SECRET_UPDATED"
	InProgress           string = "IN_PROGRESS"
	Complete             string = "COMPLETE"
	Error                string = "ERROR"
)
