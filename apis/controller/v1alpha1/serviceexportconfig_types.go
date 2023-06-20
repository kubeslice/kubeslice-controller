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

// ServiceExportConfigSpec defines the desired state of ServiceExportConfig
type ServiceExportConfigSpec struct {
	//ServiceName is the name of the service
	// +kubebuilder:validation:Required
	ServiceName      string `json:"serviceName"`
	ServiceNamespace string `json:"serviceNamespace,omitempty"`
	// clusterId is the id of the cluster where the service is available.
	// +kubebuilder:validation:Required
	SourceCluster string `json:"sourceCluster"`
	// The name of the slice.
	// +kubebuilder:validation:Required
	SliceName string `json:"sliceName"`
	// the service discovery endpoint array
	ServiceDiscoveryEndpoints []ServiceDiscoveryEndpoint `json:"serviceDiscoveryEndpoints,omitempty"`
	// The ports for the given service.
	ServiceDiscoveryPorts []ServiceDiscoveryPort `json:"serviceDiscoveryPorts,omitempty"`
	// Alias names for the exported service. The service could be addressed by the alias names
	// in addition to the slice.local name.
	Aliases []string `json:"aliases,omitempty"`
}

type ServiceDiscoveryEndpoint struct {
	// The name of the pod.
	PodName string `json:"podName,omitempty"`
	// The ID of the cluster.
	Cluster string `json:"cluster,omitempty"`
	// The NSM IP address.
	NsmIp string `json:"nsmIp,omitempty"`
	// the dns_name of the service
	DnsName string `json:"dnsName,omitempty"`
	// port of the service
	Port int32 `json:"port,omitempty"`
}

type ServiceDiscoveryPort struct {
	// The name of the port.
	Name string `json:"name,omitempty"`
	// The port number.
	Port int32 `json:"port,omitempty"`
	// The protocol.
	Protocol string `json:"protocol,omitempty"`
}

type ServiceExportConfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ServiceExportConfig is the Schema for the serviceexportconfigs API
type ServiceExportConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceExportConfigSpec   `json:"spec,omitempty"`
	Status ServiceExportConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceExportConfigList contains a list of ServiceExportConfig
type ServiceExportConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceExportConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceExportConfig{}, &ServiceExportConfigList{})
}
