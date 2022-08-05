//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppPod) DeepCopyInto(out *AppPod) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppPod.
func (in *AppPod) DeepCopy() *AppPod {
	if in == nil {
		return nil
	}
	out := new(AppPod)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClientStatus) DeepCopyInto(out *ClientStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClientStatus.
func (in *ClientStatus) DeepCopy() *ClientStatus {
	if in == nil {
		return nil
	}
	out := new(ClientStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComponentStatus) DeepCopyInto(out *ComponentStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComponentStatus.
func (in *ComponentStatus) DeepCopy() *ComponentStatus {
	if in == nil {
		return nil
	}
	out := new(ComponentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalGatewayConfig) DeepCopyInto(out *ExternalGatewayConfig) {
	*out = *in
	out.Ingress = in.Ingress
	out.Egress = in.Egress
	out.NsIngress = in.NsIngress
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalGatewayConfig.
func (in *ExternalGatewayConfig) DeepCopy() *ExternalGatewayConfig {
	if in == nil {
		return nil
	}
	out := new(ExternalGatewayConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalGatewayConfigOptions) DeepCopyInto(out *ExternalGatewayConfigOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalGatewayConfigOptions.
func (in *ExternalGatewayConfigOptions) DeepCopy() *ExternalGatewayConfigOptions {
	if in == nil {
		return nil
	}
	out := new(ExternalGatewayConfigOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewayCredentials) DeepCopyInto(out *GatewayCredentials) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewayCredentials.
func (in *GatewayCredentials) DeepCopy() *GatewayCredentials {
	if in == nil {
		return nil
	}
	out := new(GatewayCredentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GwPair) DeepCopyInto(out *GwPair) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GwPair.
func (in *GwPair) DeepCopy() *GwPair {
	if in == nil {
		return nil
	}
	out := new(GwPair)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceConfig) DeepCopyInto(out *NamespaceConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceConfig.
func (in *NamespaceConfig) DeepCopy() *NamespaceConfig {
	if in == nil {
		return nil
	}
	out := new(NamespaceConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceIsolationProfile) DeepCopyInto(out *NamespaceIsolationProfile) {
	*out = *in
	if in.ApplicationNamespaces != nil {
		in, out := &in.ApplicationNamespaces, &out.ApplicationNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllowedNamespaces != nil {
		in, out := &in.AllowedNamespaces, &out.AllowedNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceIsolationProfile.
func (in *NamespaceIsolationProfile) DeepCopy() *NamespaceIsolationProfile {
	if in == nil {
		return nil
	}
	out := new(NamespaceIsolationProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QOSProfile) DeepCopyInto(out *QOSProfile) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QOSProfile.
func (in *QOSProfile) DeepCopy() *QOSProfile {
	if in == nil {
		return nil
	}
	out := new(QOSProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceDiscoveryEndpoint) DeepCopyInto(out *ServiceDiscoveryEndpoint) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceDiscoveryEndpoint.
func (in *ServiceDiscoveryEndpoint) DeepCopy() *ServiceDiscoveryEndpoint {
	if in == nil {
		return nil
	}
	out := new(ServiceDiscoveryEndpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceDiscoveryPort) DeepCopyInto(out *ServiceDiscoveryPort) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceDiscoveryPort.
func (in *ServiceDiscoveryPort) DeepCopy() *ServiceDiscoveryPort {
	if in == nil {
		return nil
	}
	out := new(ServiceDiscoveryPort)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceGatewayConfig) DeepCopyInto(out *SliceGatewayConfig) {
	*out = *in
	if in.NodeIps != nil {
		in, out := &in.NodeIps, &out.NodeIps
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodePorts != nil {
		in, out := &in.NodePorts, &out.NodePorts
		*out = make([]int, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceGatewayConfig.
func (in *SliceGatewayConfig) DeepCopy() *SliceGatewayConfig {
	if in == nil {
		return nil
	}
	out := new(SliceGatewayConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SliceHealth) DeepCopyInto(out *SliceHealth) {
	*out = *in
	if in.ComponentStatuses != nil {
		in, out := &in.ComponentStatuses, &out.ComponentStatuses
		*out = make([]ComponentStatus, len(*in))
		copy(*out, *in)
	}
	in.LastUpdated.DeepCopyInto(&out.LastUpdated)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SliceHealth.
func (in *SliceHealth) DeepCopy() *SliceHealth {
	if in == nil {
		return nil
	}
	out := new(SliceHealth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerServiceImport) DeepCopyInto(out *WorkerServiceImport) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerServiceImport.
func (in *WorkerServiceImport) DeepCopy() *WorkerServiceImport {
	if in == nil {
		return nil
	}
	out := new(WorkerServiceImport)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerServiceImport) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerServiceImportList) DeepCopyInto(out *WorkerServiceImportList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WorkerServiceImport, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerServiceImportList.
func (in *WorkerServiceImportList) DeepCopy() *WorkerServiceImportList {
	if in == nil {
		return nil
	}
	out := new(WorkerServiceImportList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerServiceImportList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerServiceImportSpec) DeepCopyInto(out *WorkerServiceImportSpec) {
	*out = *in
	if in.SourceClusters != nil {
		in, out := &in.SourceClusters, &out.SourceClusters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ServiceDiscoveryEndpoints != nil {
		in, out := &in.ServiceDiscoveryEndpoints, &out.ServiceDiscoveryEndpoints
		*out = make([]ServiceDiscoveryEndpoint, len(*in))
		copy(*out, *in)
	}
	if in.ServiceDiscoveryPorts != nil {
		in, out := &in.ServiceDiscoveryPorts, &out.ServiceDiscoveryPorts
		*out = make([]ServiceDiscoveryPort, len(*in))
		copy(*out, *in)
	}
	if in.Aliases != nil {
		in, out := &in.Aliases, &out.Aliases
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerServiceImportSpec.
func (in *WorkerServiceImportSpec) DeepCopy() *WorkerServiceImportSpec {
	if in == nil {
		return nil
	}
	out := new(WorkerServiceImportSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerServiceImportStatus) DeepCopyInto(out *WorkerServiceImportStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerServiceImportStatus.
func (in *WorkerServiceImportStatus) DeepCopy() *WorkerServiceImportStatus {
	if in == nil {
		return nil
	}
	out := new(WorkerServiceImportStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceConfig) DeepCopyInto(out *WorkerSliceConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceConfig.
func (in *WorkerSliceConfig) DeepCopy() *WorkerSliceConfig {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerSliceConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceConfigList) DeepCopyInto(out *WorkerSliceConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WorkerSliceConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceConfigList.
func (in *WorkerSliceConfigList) DeepCopy() *WorkerSliceConfigList {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerSliceConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceConfigSpec) DeepCopyInto(out *WorkerSliceConfigSpec) {
	*out = *in
	out.SliceGatewayProvider = in.SliceGatewayProvider
	out.QosProfileDetails = in.QosProfileDetails
	in.NamespaceIsolationProfile.DeepCopyInto(&out.NamespaceIsolationProfile)
	if in.Octet != nil {
		in, out := &in.Octet, &out.Octet
	if in.IpamClusterOctet != nil {
		in, out := &in.IpamClusterOctet, &out.IpamClusterOctet
		*out = new(int)
		**out = **in
	}
	out.ExternalGatewayConfig = in.ExternalGatewayConfig
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceConfigSpec.
func (in *WorkerSliceConfigSpec) DeepCopy() *WorkerSliceConfigSpec {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceConfigStatus) DeepCopyInto(out *WorkerSliceConfigStatus) {
	*out = *in
	if in.ConnectedAppPods != nil {
		in, out := &in.ConnectedAppPods, &out.ConnectedAppPods
		*out = make([]AppPod, len(*in))
		copy(*out, *in)
	}
	if in.OnboardedAppNamespaces != nil {
		in, out := &in.OnboardedAppNamespaces, &out.OnboardedAppNamespaces
		*out = make([]NamespaceConfig, len(*in))
		copy(*out, *in)
	}
	if in.SliceHealth != nil {
		in, out := &in.SliceHealth, &out.SliceHealth
		*out = new(SliceHealth)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceConfigStatus.
func (in *WorkerSliceConfigStatus) DeepCopy() *WorkerSliceConfigStatus {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGateway) DeepCopyInto(out *WorkerSliceGateway) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGateway.
func (in *WorkerSliceGateway) DeepCopy() *WorkerSliceGateway {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGateway)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerSliceGateway) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGatewayList) DeepCopyInto(out *WorkerSliceGatewayList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WorkerSliceGateway, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGatewayList.
func (in *WorkerSliceGatewayList) DeepCopy() *WorkerSliceGatewayList {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGatewayList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerSliceGatewayList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGatewayProvider) DeepCopyInto(out *WorkerSliceGatewayProvider) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGatewayProvider.
func (in *WorkerSliceGatewayProvider) DeepCopy() *WorkerSliceGatewayProvider {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGatewayProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGatewaySpec) DeepCopyInto(out *WorkerSliceGatewaySpec) {
	*out = *in
	out.GatewayCredentials = in.GatewayCredentials
	in.LocalGatewayConfig.DeepCopyInto(&out.LocalGatewayConfig)
	in.RemoteGatewayConfig.DeepCopyInto(&out.RemoteGatewayConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGatewaySpec.
func (in *WorkerSliceGatewaySpec) DeepCopy() *WorkerSliceGatewaySpec {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGatewaySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGatewayStatus) DeepCopyInto(out *WorkerSliceGatewayStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGatewayStatus.
func (in *WorkerSliceGatewayStatus) DeepCopy() *WorkerSliceGatewayStatus {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGatewayStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGwRecycler) DeepCopyInto(out *WorkerSliceGwRecycler) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGwRecycler.
func (in *WorkerSliceGwRecycler) DeepCopy() *WorkerSliceGwRecycler {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGwRecycler)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerSliceGwRecycler) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGwRecyclerList) DeepCopyInto(out *WorkerSliceGwRecyclerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WorkerSliceGwRecycler, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGwRecyclerList.
func (in *WorkerSliceGwRecyclerList) DeepCopy() *WorkerSliceGwRecyclerList {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGwRecyclerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerSliceGwRecyclerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGwRecyclerSpec) DeepCopyInto(out *WorkerSliceGwRecyclerSpec) {
	*out = *in
	out.GwPair = in.GwPair
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGwRecyclerSpec.
func (in *WorkerSliceGwRecyclerSpec) DeepCopy() *WorkerSliceGwRecyclerSpec {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGwRecyclerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerSliceGwRecyclerStatus) DeepCopyInto(out *WorkerSliceGwRecyclerStatus) {
	*out = *in
	out.Client = in.Client
	if in.ServersToRecycle != nil {
		in, out := &in.ServersToRecycle, &out.ServersToRecycle
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.RecycledServers != nil {
		in, out := &in.RecycledServers, &out.RecycledServers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerSliceGwRecyclerStatus.
func (in *WorkerSliceGwRecyclerStatus) DeepCopy() *WorkerSliceGwRecyclerStatus {
	if in == nil {
		return nil
	}
	out := new(WorkerSliceGwRecyclerStatus)
	in.DeepCopyInto(out)
	return out
}
