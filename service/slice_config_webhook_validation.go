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

package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateSliceConfigCreate is a function to verify the creation of slice config
func ValidateSliceConfigCreate(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) error {
	var allErrs field.ErrorList
	if err := validateProjectNamespace(ctx, sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	} else {
		if err = validateSliceSubnet(sliceConfig); err != nil {
			allErrs = append(allErrs, err)
		}
		if err = validateClusters(ctx, sliceConfig); err != nil {
			allErrs = append(allErrs, err)
		}
		if err = validateQosProfile(ctx, sliceConfig); err != nil {
			allErrs = append(allErrs, err)
		}
		if err = validateExternalGatewayConfig(sliceConfig); err != nil {
			allErrs = append(allErrs, err)
		}
		if err = validateApplicationNamespaces(ctx, sliceConfig); err != nil {
			allErrs = append(allErrs, err)
		}
		if err := validateAllowedNamespaces(sliceConfig); err != nil {
			allErrs = append(allErrs, err)
		}
		// TODO: merge validateApplicationNamespaces & validateAllowedNamespaces in one function
		if err = validateNamespaceIsolationProfile(sliceConfig); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "SliceConfig"}, sliceConfig.Name, allErrs)
}

// ValidateSliceConfigUpdate is function to verify the update of slice config
func ValidateSliceConfigUpdate(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) error {
	var allErrs field.ErrorList
	if err := preventUpdate(ctx, sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateClusters(ctx, sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateQosProfile(ctx, sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateExternalGatewayConfig(sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateApplicationNamespaces(ctx, sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateAllowedNamespaces(sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateNamespaceIsolationProfile(sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "SliceConfig"}, sliceConfig.Name, allErrs)
}

// ValidateSliceConfigDelete is function to validate the deletion of sliceConfig
func ValidateSliceConfigDelete(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) error {
	var allErrs field.ErrorList
	if err := checkNamespaceDeboardingStatus(ctx, sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateIfServiceExportConfigExists(ctx, sliceConfig); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "controller.kubeslice.io", Kind: "SliceConfig"}, sliceConfig.Name, allErrs)
}

// checkNamespaceDeboardingStatus checks if the namespace is deboarding
func checkNamespaceDeboardingStatus(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	workerSlices := &workerv1alpha1.WorkerSliceConfigList{}
	ownerLabel := map[string]string{
		"original-slice-name": sliceConfig.Name,
	}
	err := util.ListResources(ctx, workerSlices, client.MatchingLabels(ownerLabel), client.InNamespace(sliceConfig.Namespace))
	if err == nil && len(workerSlices.Items) > 0 {
		for _, slice := range workerSlices.Items {
			if len(slice.Spec.NamespaceIsolationProfile.ApplicationNamespaces) > 0 {
				applicationNamespacesErr := &field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "Field: ApplicationNamespaces",
					BadValue: fmt.Sprintf("Number of ApplicationNamespaces: %d ", len(slice.Spec.NamespaceIsolationProfile.ApplicationNamespaces)),
					Detail:   fmt.Sprint("Please deboard the namespaces before deletion of slice."),
				}
				return applicationNamespacesErr
			} else {
				if len(slice.Status.OnboardedAppNamespaces) > 0 {
					onboardNamespaceErr := &field.Error{
						Type:     field.ErrorTypeInternal,
						Field:    "Field: OnboardedAppNamespaces",
						BadValue: fmt.Sprintf("Number of onboarded Application namespaces: %d", len(slice.Status.OnboardedAppNamespaces)),
						Detail:   fmt.Sprint("Deboarding of namespaces is in progress, please try after some time."),
					}
					return onboardNamespaceErr
				}
			}

		}
	}
	return nil
}

// validateSliceSubnet is function to validate the the subnet of slice
func validateSliceSubnet(sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	if !util.IsPrivateSubnet(sliceConfig.Spec.SliceSubnet) {
		return field.Invalid(field.NewPath("Spec").Child("sliceSubnet"), sliceConfig.Spec.SliceSubnet, "must be a private subnet")
	}
	if !util.HasPrefix(sliceConfig.Spec.SliceSubnet, "16") {
		return field.Invalid(field.NewPath("Spec").Child("sliceSubnet"), sliceConfig.Spec.SliceSubnet, "prefix must be 16")
	}
	if !util.HasLastTwoOctetsZero(sliceConfig.Spec.SliceSubnet) {
		return field.Invalid(field.NewPath("Spec").Child("sliceSubnet"), sliceConfig.Spec.SliceSubnet, "third and fourth octets must be 0")
	}
	return nil
}

// validateProjectNamespace is a function to verify the namespace of project
func validateProjectNamespace(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	namespace := &corev1.Namespace{}
	exist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: sliceConfig.Namespace}, namespace)
	if !exist || !checkForProjectNamespace(namespace) {
		return field.Invalid(field.NewPath("metadata").Child("namespace"), sliceConfig.Namespace, "SliceConfig must be applied on project namespace")
	}
	return nil
}

// validateIfServiceExportConfigExists is a function to validate if ServiceExportConfig exists for the given SliceConfig
func validateIfServiceExportConfigExists(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	serviceExports := &controllerv1alpha1.ServiceExportConfigList{}
	err := getServiceExportBySliceName(ctx, sliceConfig.Namespace, sliceConfig.Name, serviceExports)
	if err == nil && len(serviceExports.Items) > 0 {
		return field.Forbidden(field.NewPath("ServiceExportConfig"), "The SliceConfig can only be deleted after all the service export configs are deleted for the slice.")
	}
	return nil
}

// getServiceExportBySliceName is a function to get the service export configs by slice name
func getServiceExportBySliceName(ctx context.Context, namespace string, sliceName string, serviceExports *controllerv1alpha1.ServiceExportConfigList) error {
	label := map[string]string{
		"original-slice-name": sliceName,
	}
	err := util.ListResources(ctx, serviceExports, client.InNamespace(namespace), client.MatchingLabels(label))
	return err
}

// checkForProjectNamespace is a function to check namespace is in decided format
func checkForProjectNamespace(namespace *corev1.Namespace) bool {
	return namespace.Labels[util.LabelName] == fmt.Sprintf(util.LabelValue, "Project", namespace.Name)
}

// validateClusters is function to validate the cluster specification
func validateClusters(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	if duplicate, value := util.CheckDuplicateInArray(sliceConfig.Spec.Clusters); duplicate {
		return field.Duplicate(field.NewPath("Spec").Child("Clusters"), strings.Join(value, ", "))
	}
	for _, clusterName := range sliceConfig.Spec.Clusters {
		cluster := controllerv1alpha1.Cluster{}
		exist, _ := util.GetResourceIfExist(ctx, client.ObjectKey{Name: clusterName, Namespace: sliceConfig.Namespace}, &cluster)
		if !exist {
			return field.Invalid(field.NewPath("Spec").Child("Clusters"), clusterName, "cluster is not registered")
		}
		if cluster.Spec.NetworkInterface == "" {
			return field.Required(field.NewPath("Spec").Child("Clusters").Child("NetworkInterface"), "for cluster "+clusterName)
		}
		if cluster.Spec.NodeIP == "" {
			return field.Required(field.NewPath("Spec").Child("Clusters").Child("NodeIP"), "for cluster "+clusterName)
		}
		if len(cluster.Status.CniSubnet) == 0 {
			return field.NotFound(field.NewPath("Status").Child("CniSubnet"), "in cluster "+clusterName+". Possible cause: Slice Operator installation is pending on the cluster.")
		}
		for _, cniSubnet := range cluster.Status.CniSubnet {
			if util.OverlapIP(cniSubnet, sliceConfig.Spec.SliceSubnet) {
				return field.Invalid(field.NewPath("Spec").Child("SliceSubnet"), sliceConfig.Spec.SliceSubnet, "must not overlap with CniSubnet "+cniSubnet+" of cluster "+clusterName)
			}
		}
	}
	return nil
}

// preventUpdate is a function to stop/avoid the update of config of slice
func preventUpdate(ctx context.Context, sc *controllerv1alpha1.SliceConfig) *field.Error {
	sliceConfig := controllerv1alpha1.SliceConfig{}
	_, _ = util.GetResourceIfExist(ctx, client.ObjectKey{Name: sc.Name, Namespace: sc.Namespace}, &sliceConfig)
	if sliceConfig.Spec.SliceSubnet != sc.Spec.SliceSubnet {
		return field.Invalid(field.NewPath("Spec").Child("SliceSubnet"), sc.Spec.SliceSubnet, "cannot be updated")
	}
	if sliceConfig.Spec.SliceType != sc.Spec.SliceType {
		return field.Invalid(field.NewPath("Spec").Child("SliceType"), sc.Spec.SliceType, "cannot be updated")
	}
	if sliceConfig.Spec.SliceGatewayProvider.SliceGatewayType != sc.Spec.SliceGatewayProvider.SliceGatewayType {
		return field.Invalid(field.NewPath("Spec").Child("SliceGatewayProvider").Child("SliceGatewayType"), sc.Spec.SliceGatewayProvider.SliceGatewayType, "cannot be updated")
	}
	if sliceConfig.Spec.SliceGatewayProvider.SliceCaType != sc.Spec.SliceGatewayProvider.SliceCaType {
		return field.Invalid(field.NewPath("Spec").Child("SliceGatewayProvider").Child("SliceCaType"), sc.Spec.SliceGatewayProvider.SliceCaType, "cannot be updated")
	}
	if sliceConfig.Spec.SliceIpamType != sc.Spec.SliceIpamType {
		return field.Invalid(field.NewPath("Spec").Child("SliceIpamType"), sc.Spec.SliceIpamType, "cannot be updated")
	}

	return nil
}

// validateQosProfile is a function to validate the Qos(quality of service)profile of slice
func validateQosProfile(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	if len(sliceConfig.Spec.StandardQosProfileName) > 0 && len(sliceConfig.Spec.QosProfileDetails.QueueType) > 0 {
		return field.Invalid(field.NewPath("Spec").Child("StandardQosProfileName"), sliceConfig.Spec.StandardQosProfileName, "StandardQosProfileName cannot be set when QosProfileDetails is set")
	}
	if len(sliceConfig.Spec.StandardQosProfileName) <= 0 && len(sliceConfig.Spec.QosProfileDetails.QueueType) <= 0 {
		return field.Invalid(field.NewPath("Spec").Child("StandardQosProfileName"), sliceConfig.Spec.StandardQosProfileName, "Either StandardQosProfileName or QosProfileDetails is required")
	}
	if len(sliceConfig.Spec.StandardQosProfileName) > 0 {
		existsQosConfigFromStandardQosProfileName(ctx, sliceConfig.Namespace, sliceConfig.Spec.StandardQosProfileName)
	}
	if sliceConfig.Spec.QosProfileDetails.BandwidthCeilingKbps < sliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps {
		return field.Invalid(field.NewPath("Spec").Child("QosProfileDetails").Child("BandwidthGuaranteedKbps"), sliceConfig.Spec.QosProfileDetails.BandwidthGuaranteedKbps, "BandwidthGuaranteedKbps cannot be greater than BandwidthCeilingKbps")
	}

	return nil
}

// validateExternalGatewayConfig is a function to validate the external gateway
func validateExternalGatewayConfig(sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	count := 0
	var allClusters []string
	for _, config := range sliceConfig.Spec.ExternalGatewayConfig {
		if util.ContainsString(config.Clusters, "*") {
			count++
			if len(config.Clusters) > 1 {
				return field.Invalid(field.NewPath("Spec").Child("ExternalGatewayConfig").Child("Clusters"), strings.Join(config.Clusters, ", "), "other clusters are not allowed when * is present")
			}
		}
		for _, cluster := range config.Clusters {
			allClusters = append(allClusters, cluster)
			if cluster != "*" && !util.ContainsString(sliceConfig.Spec.Clusters, cluster) {
				return field.Invalid(field.NewPath("Spec").Child("ExternalGatewayConfig").Child("Clusters"), cluster, "cluster is not participating in slice config")
			}
		}
	}
	if count > 1 {
		return field.Invalid(field.NewPath("Spec").Child("ExternalGatewayConfig").Child("Clusters"), "*", "* is not allowed in more than one external gateways")
	}
	if duplicate, value := util.CheckDuplicateInArray(allClusters); duplicate {
		return field.Duplicate(field.NewPath("Spec").Child("ExternalGatewayConfig").Child("Clusters"), strings.Join(value, ", "))
	}
	return nil
}

// validateApplicationNamespaces is function to validate the application namespaces
func validateApplicationNamespaces(ctx context.Context, sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	for _, applicationNamespace := range sliceConfig.Spec.NamespaceIsolationProfile.ApplicationNamespaces {
		/* check duplicate values of clusters */
		if len(applicationNamespace.Namespace) > 0 && len(applicationNamespace.Clusters) == 0 {
			return field.Required(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("ApplicationNamespaces").Child("Clusters"), "clusters")
		}
		if len(applicationNamespace.Namespace) == 0 && len(applicationNamespace.Clusters) > 0 {
			return field.Required(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("ApplicationNamespaces").Child("Namespace"), "Namespace")
		}
		if duplicate, value := util.CheckDuplicateInArray(applicationNamespace.Clusters); duplicate {
			return field.Duplicate(field.NewPath("Spec").Child("NamespaceIsolationProfile.ApplicationNamespaces").Child("Clusters"), strings.Join(value, ", "))
		}
		if applicationNamespace.Clusters[0] == "*" {
			for _, clusterName := range sliceConfig.Spec.Clusters {
				err := validateGrantedClusterNamespaces(ctx, clusterName, applicationNamespace.Namespace, sliceConfig.Name, sliceConfig)
				if err != nil {
					return err
				}
			}
		} else {
			for _, clusterName := range applicationNamespace.Clusters {
				err := validateGrantedClusterNamespaces(ctx, clusterName, applicationNamespace.Namespace, sliceConfig.Name, sliceConfig)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateClusterNamespaces is a function to validate the namespaces present is cluster
func validateGrantedClusterNamespaces(ctx context.Context, clusterName string, applicationNamespace string, sliceName string, sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	cluster := controllerv1alpha1.Cluster{}
	_, _ = util.GetResourceIfExist(ctx, client.ObjectKey{Name: clusterName, Namespace: sliceConfig.Namespace}, &cluster)
	for _, clusterNamespace := range cluster.Status.Namespaces {
		if applicationNamespace == clusterNamespace.Name && len(clusterNamespace.SliceName) > 0 && clusterNamespace.SliceName != sliceName {
			return field.Invalid(field.NewPath("Spec").Child("NamespaceIsolationProfile.ApplicationNamespaces"), applicationNamespace, "The given namespace: "+applicationNamespace+" in cluster "+clusterName+" is already acquired by other slice: "+clusterNamespace.SliceName)
		}
	}
	return nil
}

// validateAllowedNamespaces is a function to validate the namespaces present in slice config
func validateAllowedNamespaces(sliceConfig *controllerv1alpha1.SliceConfig) *field.Error {
	for _, allowedNamespace := range sliceConfig.Spec.NamespaceIsolationProfile.AllowedNamespaces {
		/* check duplicate values of clusters */
		if duplicate, value := util.CheckDuplicateInArray(allowedNamespace.Clusters); duplicate {
			return field.Duplicate(field.NewPath("Spec").Child("NamespaceIsolationProfile.AllowedNamespaces").Child("Clusters"), strings.Join(value, ", "))
		}
	}
	return nil
}

// validateNamespaceIsolationProfile checks for validation errors in NamespaceIsolationProfile.
// Checks if the participating clusters are valid and if the namespaces are configured correctly.
func validateNamespaceIsolationProfile(s *controllerv1alpha1.SliceConfig) *field.Error {
	if len(s.Spec.NamespaceIsolationProfile.ApplicationNamespaces) == 0 && len(s.Spec.NamespaceIsolationProfile.AllowedNamespaces) == 0 {
		return nil
	}

	// for each namespace in applicationNamespaces, check if the clusters are valid
	participatingClusters := s.Spec.Clusters
	var checkedApplicationNs []string

	for _, nsSelection := range s.Spec.NamespaceIsolationProfile.ApplicationNamespaces {
		validNamespace, _ := regexp.MatchString("^[a-zA-Z0-9-]+$", nsSelection.Namespace)
		if validNamespace == false {
			return field.Invalid(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("ApplicationNamespaces").Child("Namespace"), nsSelection.Namespace, "Namespaces cannot contain special characteres")
		}
		//check if the clusters are already specified for a namespace
		if util.ContainsString(checkedApplicationNs, nsSelection.Namespace) {
			return field.Duplicate(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("ApplicationNamespaces").Child("Namespace"), nsSelection.Namespace)
		}
		checkedApplicationNs = append(checkedApplicationNs, nsSelection.Namespace)

		if util.ContainsString(nsSelection.Clusters, "*") {
			if len(nsSelection.Clusters) > 1 {
				return field.Invalid(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("ApplicationNamespaces").Child("Clusters"), strings.Join(nsSelection.Clusters, ", "), "Other clusters are not allowed when * is present")
			}
		}

		//check if the cluster is valid
		for _, cluster := range nsSelection.Clusters {
			if cluster != "*" && !util.ContainsString(participatingClusters, cluster) {
				return field.Invalid(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("ApplicationNamespaces").Child("Clusters"), cluster, "Cluster is not participating in slice config")
			}
		}
	}

	// for each namespace in AllowedNamespaces, check if the clusters are valid
	var checkedAllowedNs []string
	for _, nsSelection := range s.Spec.NamespaceIsolationProfile.AllowedNamespaces {
		if len(nsSelection.Namespace) == 0 && len(nsSelection.Clusters) > 0 {
			return field.Required(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("AllowedNamespaces").Child("Namespace"), nsSelection.Namespace)
		}
		validNamespace, _ := regexp.MatchString("^[a-zA-Z0-9-]+$", nsSelection.Namespace)
		if validNamespace == false {
			return field.Invalid(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("AllowedNamespaces").Child("Namespace"), nsSelection.Namespace, "Namespaces cannot contain special characteres")
		}
		if len(nsSelection.Namespace) > 0 && len(nsSelection.Clusters) == 0 {
			return field.Required(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("AllowedNamespaces").Child("Clusters"), "clusters")
		}
		//check if the clusters are already specified for a namespace
		if util.ContainsString(checkedAllowedNs, nsSelection.Namespace) {
			return field.Duplicate(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("AllowedNamespaces").Child("Namespace"), nsSelection.Namespace)
		}
		checkedAllowedNs = append(checkedAllowedNs, nsSelection.Namespace)

		if util.ContainsString(nsSelection.Clusters, "*") {
			if len(nsSelection.Clusters) > 1 {
				return field.Invalid(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("AllowedNamespaces").Child("Clusters"), strings.Join(nsSelection.Clusters, ", "), "Other clusters are not allowed when * is present")
			}
		}
		//check if the cluster is valid
		for _, cluster := range nsSelection.Clusters {
			if cluster != "*" && !util.ContainsString(participatingClusters, cluster) {
				return field.Invalid(field.NewPath("Spec").Child("NamespaceIsolationProfile").Child("AllowedNamespaces").Child("Clusters"), cluster, "Cluster is not participating in slice config")
			}
		}
	}
	return nil
}

func existsQosConfigFromStandardQosProfileName(ctx context.Context, namespace string, qosProfileName string) bool {
	NamespacedName := client.ObjectKey{
		Name:      qosProfileName,
		Namespace: namespace,
	}
	sliceQosConfig := &controllerv1alpha1.SliceQoSConfig{}
	found, err := util.GetResourceIfExist(ctx, NamespacedName, sliceQosConfig)
	if err != nil {
		return false
	}
	return found
}
