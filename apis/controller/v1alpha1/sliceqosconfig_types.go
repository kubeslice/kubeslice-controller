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

// SliceQoSConfigSpec defines the desired state of SliceQoSConfig
type SliceQoSConfigSpec struct {
	//+kubebuilder:validation:Enum:=HTB
	// +kubebuilder:validation:Required
	QueueType string `json:"queueType"`

	// +kubebuilder:validation:Required
	Priority int `json:"priority"`

	//+kubebuilder:validation:Enum:=BANDWIDTH_CONTROL
	// +kubebuilder:validation:Required
	TcType string `json:"tcType"`

	//+kubebuilder:validation:Required
	BandwidthCeilingKbps int `json:"bandwidthCeilingKbps"` //FIXME: Need research for unlimited

	//+kubebuilder:validation:Required
	BandwidthGuaranteedKbps int `json:"bandwidthGuaranteedKbps"`

	//+kubebuilder:validation:Enum:=Default;AF11;AF12;AF13;AF21;AF22;AF23;AF31;AF32;AF33;AF41;AF42;AF43;EF
	//+kubebuilder:validation:Required
	DscpClass string `json:"dscpClass"`
}

// SliceQoSConfigStatus defines the observed state of SliceQoSConfig
type SliceQoSConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SliceQoSConfig is the Schema for the sliceqosconfigs API
type SliceQoSConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SliceQoSConfigSpec   `json:"spec,omitempty"`
	Status SliceQoSConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SliceQoSConfigList contains a list of SliceQoSConfig
type SliceQoSConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SliceQoSConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SliceQoSConfig{}, &SliceQoSConfigList{})
}
