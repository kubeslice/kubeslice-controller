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

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// handleSliceConfigIPAM manages IPAM resources for SliceConfig
func (s *SliceConfigService) handleSliceConfigIPAM(ctx context.Context, sliceConfig *v1alpha1.SliceConfig) error {
	// Check if SliceIpamType is set to Dynamic
	if sliceConfig.Spec.SliceIpamType != "Dynamic" {
		return nil
	}

	// Create SliceIpam resource if it doesn't exist
	err := s.sipam.CreateSliceIpam(ctx, sliceConfig)
	if err != nil {
		return err
	}

	return nil
}

// handleClusterIPAMAllocation allocates subnet for a cluster joining a slice
func (s *SliceConfigService) handleClusterIPAMAllocation(ctx context.Context, sliceName, clusterName, namespace string) (string, error) {
	subnet, err := s.sipam.AllocateSubnetForCluster(ctx, sliceName, clusterName, namespace)
	if err != nil {
		return "", err
	}

	return subnet, nil
}

// handleClusterIPAMRelease releases subnet when a cluster leaves a slice
func (s *SliceConfigService) handleClusterIPAMRelease(ctx context.Context, sliceName, clusterName, namespace string) error {
	err := s.sipam.ReleaseSubnetForCluster(ctx, sliceName, clusterName, namespace)
	if err != nil {
		return err
	}

	return nil
}

// cleanupSliceIPAM cleans up IPAM resources when a slice is deleted
func (s *SliceConfigService) cleanupSliceIPAM(ctx context.Context, sliceName, namespace string) error {
	err := s.sipam.DeleteSliceIpam(ctx, sliceName, namespace)
	if err != nil {
		return err
	}

	return nil
}
