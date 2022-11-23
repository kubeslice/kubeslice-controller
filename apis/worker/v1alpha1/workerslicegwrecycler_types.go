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

// WorkerSliceGwRecyclerSpec defines the desired state of WorkerSliceGwRecycler
type WorkerSliceGwRecyclerSpec struct {
	GwPair GwPair `json:"gwPair,omitempty"`
	State string `json:"state,omitempty"`
	Request string `json:"request,omitempty"`
}

type GwPair struct {
	ClientID string `json:"clientId,omitempty"`
	ServerID string `json:"serverId,omitempty"`
}

// WorkerSliceGwRecyclerStatus defines the observed state of WorkerSliceGwRecycler
type WorkerSliceGwRecyclerStatus struct {
	Client ClientStatus `json:"client,omitempty"`
	ServersToRecycle []string `json:"serversToRecycle,omitempty"`
	RecycledServers []string `json:"recycledServers,omitempty"`
}
type ClientStatus struct {
	Response string `json:"response,omitempty"`
	RecycledClient string `json:"recycledClient,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WorkerSliceGwRecycler is the Schema for the workerslicegwrecyclers API
type WorkerSliceGwRecycler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerSliceGwRecyclerSpec   `json:"spec,omitempty"`
	Status WorkerSliceGwRecyclerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkerSliceGwRecyclerList contains a list of WorkerSliceGwRecycler
type WorkerSliceGwRecyclerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkerSliceGwRecycler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkerSliceGwRecycler{}, &WorkerSliceGwRecyclerList{})
}
