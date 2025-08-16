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
	"testing"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSliceConfigService_handleSliceConfigIPAM(t *testing.T) {
	tests := []struct {
		name        string
		sliceConfig *v1alpha1.SliceConfig
		expectCall  bool
		expectError bool
	}{
		{
			name: "Dynamic IPAM - should create SliceIpam",
			sliceConfig: &v1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-test",
				},
				Spec: v1alpha1.SliceConfigSpec{
					SliceIpamType: "Dynamic",
					SliceSubnet:   "10.1.0.0/16",
				},
			},
			expectCall:  true,
			expectError: false,
		},
		{
			name: "Local IPAM - should skip",
			sliceConfig: &v1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "kubeslice-test",
				},
				Spec: v1alpha1.SliceConfigSpec{
					SliceIpamType: "Local",
					SliceSubnet:   "10.1.0.0/16",
				},
			},
			expectCall:  false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockIpamService := &mocks.ISliceIpamService{}

			service := &SliceConfigService{
				sipam: mockIpamService,
			}

			if tt.expectCall {
				mockIpamService.On("CreateSliceIpam", mock.Anything, tt.sliceConfig).Return(nil)
			}

			err := service.handleSliceConfigIPAM(context.Background(), tt.sliceConfig)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			mockIpamService.AssertExpectations(t)
		})
	}
}

func TestSliceConfigService_handleClusterIPAMAllocation(t *testing.T) {
	mockIpamService := &mocks.ISliceIpamService{}

	service := &SliceConfigService{
		sipam: mockIpamService,
	}

	expectedSubnet := "10.1.0.0/24"
	mockIpamService.On("AllocateSubnetForCluster", mock.Anything, "test-slice", "cluster1", "kubeslice-test").Return(expectedSubnet, nil)

	subnet, err := service.handleClusterIPAMAllocation(context.Background(), "test-slice", "cluster1", "kubeslice-test")

	require.NoError(t, err)
	require.Equal(t, expectedSubnet, subnet)
	mockIpamService.AssertExpectations(t)
}

func TestSliceConfigService_handleClusterIPAMRelease(t *testing.T) {
	mockIpamService := &mocks.ISliceIpamService{}

	service := &SliceConfigService{
		sipam: mockIpamService,
	}

	mockIpamService.On("ReleaseSubnetForCluster", mock.Anything, "test-slice", "cluster1", "kubeslice-test").Return(nil)

	err := service.handleClusterIPAMRelease(context.Background(), "test-slice", "cluster1", "kubeslice-test")

	require.NoError(t, err)
	mockIpamService.AssertExpectations(t)
}

func TestSliceConfigService_cleanupSliceIPAM(t *testing.T) {
	mockIpamService := &mocks.ISliceIpamService{}

	service := &SliceConfigService{
		sipam: mockIpamService,
	}

	mockIpamService.On("DeleteSliceIpam", mock.Anything, "test-slice", "kubeslice-test").Return(nil)

	err := service.cleanupSliceIPAM(context.Background(), "test-slice", "kubeslice-test")

	require.NoError(t, err)
	mockIpamService.AssertExpectations(t)
}
