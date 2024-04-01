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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:Enum:=single-network;multi-network;no-network
type NetworkType string

const (
	// all workloads would be connected to the slice l3 overlay network
	SINGLENET NetworkType = "single-network"

	// workloads would be connected at l7 through network of envoy gateways.
	// And the gateways would be connected through slice l3 overlay
	MULTINET NetworkType = "multi-network"

	// slice without any connectivity between clusters
	NONET NetworkType = "no-network"
)

// SliceConfigSpec defines the desired state of SliceConfig
type SliceConfigSpec struct {
	//+kubebuilder:default:=single-network
	OverlayNetworkDeploymentMode NetworkType `json:"overlayNetworkDeploymentMode,omitempty"`
	SliceSubnet                  string      `json:"sliceSubnet,omitempty"`
	//+kubebuilder:default:=Application
	SliceType            string                      `json:"sliceType,omitempty"`
	SliceGatewayProvider *WorkerSliceGatewayProvider `json:"sliceGatewayProvider,omitempty"`
	//+kubebuilder:default:=Local
	SliceIpamType          string   `json:"sliceIpamType,omitempty"`
	Clusters               []string `json:"clusters,omitempty"`
	StandardQosProfileName string   `json:"standardQosProfileName,omitempty"` // FIXME: Add OneOf StandardQosProfileName vs QosProfileDetails
	// The custom QOS Profile Details
	QosProfileDetails         *QOSProfile               `json:"qosProfileDetails,omitempty"` // FIXME: Add OneOf StandardQosProfileName vs QosProfileDetails
	NamespaceIsolationProfile NamespaceIsolationProfile `json:"namespaceIsolationProfile,omitempty"`
	ExternalGatewayConfig     []ExternalGatewayConfig   `json:"externalGatewayConfig,omitempty"`
	//+kubebuilder:validation:Minimum=2
	//+kubebuilder:validation:Maximum=32
	//+kubebuilder:default:=16
	MaxClusters int `json:"maxClusters"`
	//+kubebuilder:validation:Minimum=30
	//+kubebuilder:validation:Maximum=90
	//+kubebuilder:default:=30
	RotationInterval int `json:"rotationInterval,omitempty"`
	// RenewBefore is used for renew now!
	RenewBefore *metav1.Time      `json:"renewBefore,omitempty"`
	VPNConfig   *VPNConfiguration `json:"vpnConfig,omitempty"`
}

// ExternalGatewayConfig is the configuration for external gateways like 'istio', etc/
type ExternalGatewayConfig struct {
	Ingress   ExternalGatewayConfigOptions `json:"ingress,omitempty"`
	Egress    ExternalGatewayConfigOptions `json:"egress,omitempty"`
	NsIngress ExternalGatewayConfigOptions `json:"nsIngress,omitempty"`
	//+kubebuilder:validation:Enum:=none;istio
	GatewayType string   `json:"gatewayType,omitempty"`
	Clusters    []string `json:"clusters,omitempty"`
}

type ExternalGatewayConfigOptions struct {
	Enabled bool `json:"enabled,omitempty"`
}

// WorkerSliceGatewayProvider defines the configuration for slicegateway
type WorkerSliceGatewayProvider struct {
	//+kubebuilder:default:=OpenVPN
	// +kubebuilder:validation:Required
	SliceGatewayType string `json:"sliceGatewayType"`

	//+kubebuilder:default:=Local
	// +kubebuilder:validation:Required
	SliceCaType string `json:"sliceCaType"`

	SliceGatewayServiceType []SliceGatewayServiceType `json:"sliceGatewayServiceType,omitempty"`
}

type SliceGatewayServiceType struct {
	// +kubebuilder:validation:Required
	Cluster string `json:"cluster"`
	// +kubebuilder:validation:Required
	//+kubebuilder:default:=NodePort
	//+kubebuilder:validation:Enum:=NodePort;LoadBalancer
	Type string `json:"type"`
	// +kubebuilder:validation:Required
	//+kubebuilder:default:=UDP
	//+kubebuilder:validation:Enum:=TCP;UDP
	Protocol string `json:"protocol"`
}

// QOSProfile is the QOS Profile configuration from backend
type QOSProfile struct {
	//+kubebuilder:default:=HTB
	// +kubebuilder:validation:Required
	QueueType string `json:"queueType"`

	// +kubebuilder:validation:Required
	Priority int `json:"priority"`

	//+kubebuilder:default:=BANDWIDTH_CONTROL
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

type NamespaceIsolationProfile struct {
	//+kubebuilder:default:=false
	//+kubebuilder:validation:Optional
	IsolationEnabled      bool                      `json:"isolationEnabled"`
	ApplicationNamespaces []SliceNamespaceSelection `json:"applicationNamespaces,omitempty"`
	AllowedNamespaces     []SliceNamespaceSelection `json:"allowedNamespaces,omitempty"`
}

type SliceNamespaceSelection struct {
	Namespace string   `json:"namespace,omitempty"`
	Clusters  []string `json:"clusters,omitempty"`
}

// VPNConfiguration defines the additional (optional) VPN Configuration to customise
type VPNConfiguration struct {
	//+kubebuilder:default:=AES-256-CBC
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Enum:=AES-256-CBC;AES-128-CBC
	Cipher string `json:"cipher"`
}

type KubesliceEvent struct {
	// Type of the event. Can be one of Error, Success or InProgress
	Type string `json:"type,omitempty"`
	// Trigger action. Examples - CLUSTER_OFFBOARDING, NAMESPCE_OFFBOARDING etc
	Action string `json:"action,omitempty"`
	// list of effected components on which action failed
	Components []string `json:"components,omitempty"`
	// Identifier of the component for which the action was triggered
	Identifier string `json:"identifier,omitempty"`
	// Reason message for the event
	Reason string `json:"reason,omitempty"`
	// Event name (from monitoring framework schema)
	Event string `json:"event"`
	// Timestamp of the event
	Timestamp metav1.Time `json:"timestamp,omitempty"`
	// Flag to determine if kubernetes event is already raised
	//+kubebuilder:default:=false
	IsEventRaised bool `json:"isEventRaised,omitempty"`
}

// SliceConfigStatus defines the observed state of SliceConfig
type SliceConfigStatus struct {
	KubesliceEvents []KubesliceEvent `json:"kubesliceEvents,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SliceConfig is the Schema for the sliceconfig API
type SliceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SliceConfigSpec   `json:"spec,omitempty"`
	Status SliceConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SliceConfigList contains a list of SliceConfig
type SliceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SliceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SliceConfig{}, &SliceConfigList{})
}
