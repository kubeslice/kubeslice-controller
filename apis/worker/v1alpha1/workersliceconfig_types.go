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

type ComponentHealthStatus string

const (
	ComponentHealthStatusNormal  = "Normal"
	ComponentHealthStatusWarning = "Warning"
	ComponentHealthStatusError   = "Error"
)

type SliceHealthStatus string

const (
	SliceHealthStatusNormal  = "Normal"
	SliceHealthStatusWarning = "Warning"
)

// WorkerSliceConfigSpec defines the desired state of Slice
type WorkerSliceConfigSpec struct {
	SliceName   string `json:"sliceName,omitempty"`
	SliceSubnet string `json:"sliceSubnet,omitempty"`
	//+kubebuilder:default:=Application
	SliceType            string                     `json:"sliceType,omitempty"`
	SliceGatewayProvider WorkerSliceGatewayProvider `json:"sliceGatewayProvider,omitempty"`
	//+kubebuilder:default:=Local
	SliceIpamType             string                    `json:"sliceIpamType,omitempty"`
	QosProfileDetails         QOSProfile                `json:"qosProfileDetails,omitempty"`
	NamespaceIsolationProfile NamespaceIsolationProfile `json:"namespaceIsolationProfile,omitempty"`
	IpamClusterOctet          int                       `json:"ipamClusterOctet,omitempty"`
	//+kubebuilder:validation:Required
	Octet                 *int                  `json:"octet"`
	ClusterSubnetCIDR     string                `json:"clusterSubnetCIDR,omitempty"`
	ExternalGatewayConfig ExternalGatewayConfig `json:"externalGatewayConfig,omitempty"`
}

// WorkerSliceGatewayProvider defines the configuration for slicegateway
type WorkerSliceGatewayProvider struct {
	//+kubebuilder:default:=OpenVPN
	SliceGatewayType string `json:"sliceGatewayType,omitempty"`
	//+kubebuilder:default:=Local
	SliceCaType string `json:"sliceCaType,omitempty"`
	//+kubebuilder:default:=NodePort
	//+kubebuilder:validation:Enum:=NodePort;LoadBalancer
	SliceGatewayServiceType string `json:"sliceGatewayServiceType,omitempty"`
	//+kubebuilder:default:=UDP
	//+kubebuilder:validation:Enum:=TCP;UDP
	SliceGatewayProtocol string `json:"sliceGatewayProtocol,omitempty"`
}

// QOSProfile is the QOS Profile configuration from backend
type QOSProfile struct {
	//+kubebuilder:default:=HTB
	QueueType               string `json:"queueType,omitempty"`
	Priority                int    `json:"priority,omitempty"`
	TcType                  string `json:"tcType,omitempty"`
	BandwidthCeilingKbps    int    `json:"bandwidthCeilingKbps,omitempty"`
	BandwidthGuaranteedKbps int    `json:"bandwidthGuaranteedKbps,omitempty"`
	//+kubebuilder:validation:Enum:=Default;AF11;AF12;AF13;AF21;AF22;AF23;AF31;AF32;AF33;AF41;AF42;AF43;EF
	DscpClass string `json:"dscpClass,omitempty"`
}

type NamespaceIsolationProfile struct {
	//+kubebuilder:default:=false
	//+kubebuilder:validation:Optional
	IsolationEnabled      bool     `json:"isolationEnabled"`
	ApplicationNamespaces []string `json:"applicationNamespaces,omitempty"`
	AllowedNamespaces     []string `json:"allowedNamespaces,omitempty"`
}

type ExternalGatewayConfig struct {
	Ingress   ExternalGatewayConfigOptions `json:"ingress,omitempty"`
	Egress    ExternalGatewayConfigOptions `json:"egress,omitempty"`
	NsIngress ExternalGatewayConfigOptions `json:"nsIngress,omitempty"`
	//+kubebuilder:validation:Enum:=none;istio
	GatewayType string `json:"gatewayType,omitempty"`
}

type ExternalGatewayConfigOptions struct {
	Enabled bool `json:"enabled,omitempty"`
}

// AppPod defines the app pods connected to slice
type AppPod struct {
	// PodName is App Pod Name
	PodName string `json:"podName,omitempty"`
	// PodNamespace is App Pod Namespace
	PodNamespace string `json:"podNamespace,omitempty"`
	// PodIP is App Pod IP
	PodIP string `json:"podIp,omitempty"`
	// NsmIP is the nsm ip of App
	NsmIP string `json:"nsmIp,omitempty"`
	// NsmInterface is the nsm interface of App
	NsmInterface string `json:"nsmInterface,omitempty"`
	// PeerIp is the nsm peer ip of gateway
	NsmPeerIP string `json:"nsmPeerIp,omitempty"`
}

// WorkerSliceConfigStatus defines the observed state of Slice
type WorkerSliceConfigStatus struct {
	ConnectedAppPods       []AppPod          `json:"connectedAppPods,omitempty"`
	OnboardedAppNamespaces []NamespaceConfig `json:"onboardedAppNamespaces,omitempty"`
	// SliceHealth shows the health of the slice in worker cluster
	SliceHealth *SliceHealth `json:"sliceHealth,omitempty"`
}

type SliceHealth struct {
	// SliceHealthStatus shows the overall health status of the slice
	//+kubebuilder:validation:Enum:=Normal;Warning
	SliceHealthStatus SliceHealthStatus `json:"sliceHealthStatus,omitempty"`
	// ComponentStatuses shows the health status of individual components in the cluster
	ComponentStatuses []ComponentStatus `json:"componentStatuses,omitempty"`
	// LastUpdated is the timestamp when healthstatus was updated
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

type ComponentStatus struct {
	// Component name
	Component string `json:"component"`
	//+kubebuilder:validation:Enum:=Normal;Warning;Error
	ComponentHealthStatus ComponentHealthStatus `json:"componentHealthStatus"`
}

type NamespaceConfig struct {
	Name string `json:"name,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WorkerSliceConfig is the Schema for the slice API
type WorkerSliceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerSliceConfigSpec   `json:"spec,omitempty"`
	Status WorkerSliceConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkerSliceConfigList contains a list of Slice
type WorkerSliceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkerSliceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkerSliceConfig{}, &WorkerSliceConfigList{})
}
