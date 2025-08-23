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
	"net"
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewIPAMService(t *testing.T) {
	mf := &metrics.MetricRecorder{}
	service := NewIPAMService(nil, mf)

	assert.NotNil(t, service)
	assert.Equal(t, mf, service.mf)
}

func TestCalculateOptimalSubnetSize(t *testing.T) {
	mf := &metrics.MetricRecorder{}
	service := NewIPAMService(nil, mf)

	// Test with different max cluster values
	// The function calculates: baseCidr(24) + i where i is the first value where maxClusters > 2^i
	tests := []struct {
		maxClusters int
		expected    string
	}{
		{2, "/24"},   // 2 > 2^0 (1), so 24 + 0 = 24
		{4, "/25"},   // 4 > 2^1 (2), so 24 + 1 = 25
		{8, "/26"},   // 8 > 2^2 (4), so 24 + 2 = 26
		{16, "/27"},  // 16 > 2^3 (8), so 24 + 3 = 27
		{32, "/28"},  // 32 > 2^4 (16), so 24 + 4 = 28
		{64, "/29"},  // 64 > 2^5 (32), so 24 + 5 = 29
		{128, "/30"}, // 128 > 2^6 (64), so 24 + 6 = 30
	}

	for _, test := range tests {
		result := service.calculateOptimalSubnetSize(test.maxClusters)
		assert.Equal(t, test.expected, result)
	}
}

func TestGenerateSubnet(t *testing.T) {
	mf := &metrics.MetricRecorder{}
	service := NewIPAMService(nil, mf)

	// Test subnet generation
	_, baseNet, err := net.ParseCIDR("10.1.0.0/16")
	assert.NoError(t, err)

	// Test first subnet
	subnet1 := service.generateSubnet(baseNet, 0, 24)
	assert.Equal(t, "10.1.0.0/24", subnet1)

	// Test second subnet
	subnet2 := service.generateSubnet(baseNet, 1, 24)
	assert.Equal(t, "10.1.1.0/24", subnet2)

	// Test third subnet
	subnet3 := service.generateSubnet(baseNet, 2, 24)
	assert.Equal(t, "10.1.2.0/24", subnet3)
}

func TestRemoveSubnetFromSlice(t *testing.T) {
	mf := &metrics.MetricRecorder{}
	service := NewIPAMService(nil, mf)

	subnets := []string{"10.1.0.0/24", "10.1.1.0/24", "10.1.2.0/24"}

	// Test removing middle subnet
	result := service.removeSubnetFromSlice(subnets, "10.1.1.0/24")
	expected := []string{"10.1.0.0/24", "10.1.2.0/24"}
	assert.Equal(t, expected, result)

	// Test removing first subnet
	result = service.removeSubnetFromSlice(subnets, "10.1.0.0/24")
	expected = []string{"10.1.1.0/24", "10.1.2.0/24"}
	assert.Equal(t, expected, result)

	// Test removing last subnet
	result = service.removeSubnetFromSlice(subnets, "10.1.2.0/24")
	expected = []string{"10.1.0.0/24", "10.1.1.0/24"}
	assert.Equal(t, expected, result)

	// Test removing non-existent subnet
	result = service.removeSubnetFromSlice(subnets, "10.1.3.0/24")
	assert.Equal(t, subnets, result)
}

func TestAllocateStaticSubnet(t *testing.T) {
	mf := &metrics.MetricRecorder{}
	service := NewIPAMService(nil, mf)

	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "default",
		},
		Spec: controllerv1alpha1.SliceConfigSpec{
			SliceSubnet: "10.1.0.0/16",
			MaxClusters: 4,
			Clusters:    []string{"cluster-1", "cluster-2"},
		},
	}

	// Test allocation for existing cluster
	subnet, err := service.allocateStaticSubnet(context.Background(), sliceConfig, "cluster-1")
	assert.NoError(t, err)
	assert.NotEmpty(t, subnet)

	// Test allocation for non-existent cluster
	_, err = service.allocateStaticSubnet(context.Background(), sliceConfig, "cluster-3")
	assert.Error(t, err)
}
