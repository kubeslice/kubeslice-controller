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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSliceIpamSubnet(t *testing.T) {
	tests := []struct {
		name        string
		sliceSubnet string
		expectError bool
	}{
		{
			name:        "Valid subnet",
			sliceSubnet: "10.1.0.0/16",
			expectError: false,
		},
		{
			name:        "Invalid subnet",
			sliceSubnet: "invalid",
			expectError: true,
		},
		{
			name:        "Empty subnet",
			sliceSubnet: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSliceIpamSubnet(tt.sliceSubnet)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateIpamSubnetSize(t *testing.T) {
	tests := []struct {
		name        string
		sliceSubnet string
		subnetSize  int
		expectError bool
	}{
		{
			name:        "Valid subnet size",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  24,
			expectError: false,
		},
		{
			name:        "Invalid subnet size - too small",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  16,
			expectError: true,
		},
		{
			name:        "Invalid subnet size - out of range",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  15,
			expectError: true,
		},
		{
			name:        "Invalid subnet size - too large",
			sliceSubnet: "10.1.0.0/16",
			subnetSize:  31,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIpamSubnetSize(tt.sliceSubnet, tt.subnetSize)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
