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

type ClusterHealthStatus string

const (
	ClusterHealthStatusNormal  = "Normal"
	ClusterHealthStatusWarning = "Warning"
)

type RegistrationStatus string

const (
	RegistrationStatusPending              = "Pending"
	RegistrationStatusInProgress           = "InProgress"
	RegistrationStatusFailed               = "Failed"
	RegistrationStatusRegistered           = "Registered"
	RegistrationStatusDeregisterInProgress = "DeregisterInProgress"
	RegistrationStatusDeregisterFailed     = "DeregisterFailed"
	RegistrationStatusDeregistered         = "Deregistered"
)

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	//NodeIP is the IP address of the Node - deprecated field use Plural NodeIPs
	//+kubebuilder:deprecatedversion:warning="controller/v1alpha1 NodeIP is deprecated...use NodeIPs"
	NodeIP  string   `json:"nodeIP,omitempty"`
	NodeIPs []string `json:"nodeIPs,omitempty"`
	// NetworkInterface is the network interface attached with the cluster.
	NetworkInterface string `json:"networkInterface,omitempty"`
	//put in an object
	ClusterProperty ClusterProperty `json:"clusterProperty,omitempty"`
}

type ClusterProperty struct {
	//Telemetry contains Telemetry information
	Telemetry Telemetry `json:"telemetry,omitempty"`
	//GeoLocation contains information regarding Geographical Location of the Cluster
	GeoLocation GeoLocation `json:"geoLocation,omitempty"`
	//Monitoring contains the Kubernetes Monitoring Dashboard
	Monitoring Monitoring `json:"monitoring,omitempty"`
}

// Telemetry defines the field of ClusterSpec
type Telemetry struct {
	//Enabled is the enable status of the Telemetry
	Enabled bool `json:"enabled,omitempty"`
	//TelemetryProvider is the Telemetry Provider information
	TelemetryProvider string `json:"telemetryProvider,omitempty"`
	//Endpoint is the Telemetry Endpoint
	Endpoint string `json:"endpoint,omitempty"`
}

// GeoLocation defines the field of ClusterSpec
type GeoLocation struct {
	//CloudProvider is the cloud service provider
	CloudProvider string `json:"cloudProvider,omitempty"`
	//CloudRegion is the region of the cloud
	CloudRegion string `json:"cloudRegion,omitempty"`
	//Latitude is the latitude of the cluster
	Latitude string `json:"latitude,omitempty"`
	//Longitude is the longitude of the cluster
	Longitude string `json:"longitude,omitempty"`
}

// Monitoring defines the field of ClusterSpec
type Monitoring struct {
	//KubernetesDashboard contains the information regarding Kubernetes Monitoring Dashboard
	KubernetesDashboard KubernetesDashboard `json:"kubernetesDashboard,omitempty"`
}

// KubernetesDashboard defines the field of ClusterSpec
type KubernetesDashboard struct {
	//Enabled is the enable status of the KubernetesDashboard
	Enabled bool `json:"enabled,omitempty"`
	//AccessToken is the Access Token to access the KubernetesDashboard
	AccessToken string `json:"accessToken,omitempty"`
	//IngressPrefix is the prefix of ingress gateway for KubernetesDashboard
	IngressPrefix string `json:"ingressPrefix,omitempty"`
	//Endpoint is the base endpoint to access the kubernetes dashboard
	Endpoint string `json:"endpoint,omitempty"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// SecretName is the name of the secret for the worker cluster.
	SecretName string `json:"secretName,omitempty"`
	//CniSubnet is the podip and service ip subnet of CNI
	CniSubnet []string `json:"cniSubnet,omitempty"`
	// Namespaces present in cluster
	Namespaces []NamespacesConfig `json:"namespaces,omitempty"`
	// ClusterHealth shows the health of the worker cluster
	ClusterHealth *ClusterHealth `json:"clusterHealth,omitempty"`
	// NodeIPs of the gateway node of worker cluster
	NodeIPs []string `json:"nodeIPs,omitempty"`
	// RegistrationStatus shows the status of cluster registration
	//+kubebuilder:validation:Enum:=Pending;InProgress;Failed;Registered;DeregisterInProgress;DeregisterFailed;Deregistered
	RegistrationStatus RegistrationStatus `json:"registrationStatus,omitempty"`
	// IsDeregisterInProgress is the flag to check if the cluster deregister is InProgress
	IsDeregisterInProgress bool `json:"isDeregisterInProgress,omitempty"`
	// NetworkPresent denotes if the networking components (NSM, Spire) are installed on a cluster
	//+kubebuilder:default:=false
	NetworkPresent bool `json:"networkPresent,omitempty"`

	// VCPURestriction is the restriction on the cluster disabling the creation of new pods
	VCPURestriction *VCPURestriction `json:"vCPURestriction,omitempty"`
}

type VCPURestriction struct {
	// EnforceRestrictions is the flag to check if the cluster is restricted
	EnforceRestrictions bool `json:"enforceRestrictions,omitempty"`
	// LastUpdatedTimestamp is the timestamp when the enforcement was updated
	LastUpdatedTimestamp metav1.Time `json:"lastUpdatedTimestamp,omitempty"`
}

type ClusterHealth struct {
	// ClusterHealthStatus shows the overall health status of the cluster
	//+kubebuilder:validation:Enum:=Normal;Warning
	ClusterHealthStatus ClusterHealthStatus `json:"clusterHealthStatus,omitempty"`
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

type NamespacesConfig struct {
	Name      string `json:"name,omitempty"`
	SliceName string `json:"sliceName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
