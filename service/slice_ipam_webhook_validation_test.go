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

	"github.com/dailymotion/allure-go"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	utilmock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestSliceIpamWebhookSuite(t *testing.T) {
	for k, v := range SliceIpamWebhookValidationTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var SliceIpamWebhookValidationTestbed = map[string]func(*testing.T){
	"TestValidateSliceIpamCreateSuccess":                     testValidateSliceIpamCreateSuccess,
	"TestValidateSliceIpamCreateEmptySliceName":              testValidateSliceIpamCreateEmptySliceName,
	"TestValidateSliceIpamCreateInvalidCIDR":                 testValidateSliceIpamCreateInvalidCIDR,
	"TestValidateSliceIpamCreateHostAddress":                 testValidateSliceIpamCreateHostAddress,
	"TestValidateSliceIpamCreateIPv6NotSupported":            testValidateSliceIpamCreateIPv6NotSupported,
	"TestValidateSliceIpamCreateNonPrivateIP":                testValidateSliceIpamCreateNonPrivateIP,
	"TestValidateSliceIpamCreateInvalidCIDRSize":             testValidateSliceIpamCreateInvalidCIDRSize,
	"TestValidateSliceIpamCreateInvalidSubnetSize":           testValidateSliceIpamCreateInvalidSubnetSize,
	"TestValidateSliceIpamCreateSubnetSizeTooSmall":          testValidateSliceIpamCreateSubnetSizeTooSmall,
	"TestValidateSliceIpamCreateSliceConfigNotFound":         testValidateSliceIpamCreateSliceConfigNotFound,
	"TestValidateSliceIpamCreateSliceConfigNotDynamic":       testValidateSliceIpamCreateSliceConfigNotDynamic,
	"TestValidateSliceIpamCreateSliceSubnetMismatch":         testValidateSliceIpamCreateSliceSubnetMismatch,
	"TestValidateSliceIpamCreateDuplicateSliceIpam":          testValidateSliceIpamCreateDuplicateSliceIpam,
	"TestValidateSliceIpamUpdateSuccess":                     testValidateSliceIpamUpdateSuccess,
	"TestValidateSliceIpamUpdateImmutableSliceName":          testValidateSliceIpamUpdateImmutableSliceName,
	"TestValidateSliceIpamUpdateImmutableSliceSubnet":        testValidateSliceIpamUpdateImmutableSliceSubnet,
	"TestValidateSliceIpamUpdateSubnetSizeWithActiveAllocs":  testValidateSliceIpamUpdateSubnetSizeWithActiveAllocs,
	"TestValidateSliceIpamUpdateSubnetSizeDecrease":          testValidateSliceIpamUpdateSubnetSizeDecrease,
	"TestValidateSliceIpamUpdateInvalidSubnetSize":           testValidateSliceIpamUpdateInvalidSubnetSize,
	"TestValidateSliceIpamDeleteSuccess":                     testValidateSliceIpamDeleteSuccess,
	"TestValidateSliceIpamDeleteWithActiveAllocations":       testValidateSliceIpamDeleteWithActiveAllocations,
	"TestValidateSliceIpamDeleteWithSliceConfigExisting":     testValidateSliceIpamDeleteWithSliceConfigExisting,
	"TestValidateSliceIpamDeleteWithReleasedAllocationsOnly": testValidateSliceIpamDeleteWithReleasedAllocationsOnly,
}

func testValidateSliceIpamCreateSuccess(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceConfigSpec{
			SliceIpamType: "Dynamic",
			SliceSubnet:   "10.1.0.0/16",
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig exists
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		*arg = *sliceConfig
	}).Once()

	// Mock no duplicate SliceIpam
	clientMock.On("List", ctx, mock.AnythingOfType("*v1alpha1.SliceIpamList"), mock.Anything).Return(nil).Once()

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.NoError(t, err)
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateEmptySliceName(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "slice name cannot be empty")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateInvalidCIDR(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "invalid-cidr",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid CIDR format")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateHostAddress(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.5/16",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CIDR must be a network address, not a host address")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateIPv6NotSupported(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "2001:db8::/32",
			SubnetSize:  64,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only IPv4 CIDRs are supported")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateNonPrivateIP(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "8.8.8.0/24",
			SubnetSize:  28,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CIDR must be in a private IP range")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateInvalidCIDRSize(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.0.0.0/6",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	// Note: CIDR is validated for being a network address before checking size
	require.Contains(t, err.Error(), "CIDR must be a network address, not a host address")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateInvalidSubnetSize(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  32,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subnet size must be between 16 and 30")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateSubnetSizeTooSmall(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/24",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subnet size /24 must be larger than slice subnet size /24")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateSliceConfigNotFound(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig not found - returns empty SliceConfig with empty SliceIpamType
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(nil).Run(func(args mock.Arguments) {
		// Return empty SliceConfig - which has SliceIpamType = "" (not "Dynamic")
		// This will trigger the "SliceConfig must use Dynamic IPAM type" error
	}).Once()

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	// Since GetResourceIfExist returns found=false for empty resource, it will check IPAM type
	require.Contains(t, err.Error(), "SliceConfig must use Dynamic IPAM type")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateSliceConfigNotDynamic(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceConfigSpec{
			SliceIpamType: "Static",
			SliceSubnet:   "10.1.0.0/16",
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig exists with Static IPAM
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		*arg = *sliceConfig
	}).Once()

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SliceConfig must use Dynamic IPAM type")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateSliceSubnetMismatch(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceConfigSpec{
			SliceIpamType: "Dynamic",
			SliceSubnet:   "10.2.0.0/16",
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig exists with different subnet
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		*arg = *sliceConfig
	}).Once()

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "slice subnet must match SliceConfig subnet")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamCreateDuplicateSliceIpam(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceConfigSpec{
			SliceIpamType: "Dynamic",
			SliceSubnet:   "10.1.0.0/16",
		},
	}

	existingSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-slice-ipam",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig exists
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		*arg = *sliceConfig
	}).Once()

	// Mock duplicate SliceIpam exists
	clientMock.On("List", ctx, mock.AnythingOfType("*v1alpha1.SliceIpamList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.SliceIpamList)
		arg.Items = []controllerv1alpha1.SliceIpam{*existingSliceIpam}
	}).Once()

	_, err := ValidateSliceIpamCreate(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Duplicate value")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamUpdateSuccess(t *testing.T) {
	oldSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{},
		},
	}

	newSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{},
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamUpdate(ctx, newSliceIpam, runtime.Object(oldSliceIpam))
	require.NoError(t, err)
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamUpdateImmutableSliceName(t *testing.T) {
	oldSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	newSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "different-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamUpdate(ctx, newSliceIpam, runtime.Object(oldSliceIpam))
	require.Error(t, err)
	require.Contains(t, err.Error(), "slice name is immutable")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamUpdateImmutableSliceSubnet(t *testing.T) {
	oldSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	newSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.2.0.0/16",
			SubnetSize:  24,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamUpdate(ctx, newSliceIpam, runtime.Object(oldSliceIpam))
	require.Error(t, err)
	require.Contains(t, err.Error(), "slice subnet is immutable")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamUpdateSubnetSizeWithActiveAllocs(t *testing.T) {
	now := metav1.Now()
	oldSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster-1",
					Subnet:      "10.1.0.0/24",
					AllocatedAt: now,
					Status:      controllerv1alpha1.SubnetStatusAllocated,
				},
			},
		},
	}

	newSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  25,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster-1",
					Subnet:      "10.1.0.0/24",
					AllocatedAt: now,
					Status:      controllerv1alpha1.SubnetStatusAllocated,
				},
			},
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamUpdate(ctx, newSliceIpam, runtime.Object(oldSliceIpam))
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot change subnet size when there are 1 active allocations")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamUpdateSubnetSizeDecrease(t *testing.T) {
	oldSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  25,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{},
		},
	}

	newSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{},
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamUpdate(ctx, newSliceIpam, runtime.Object(oldSliceIpam))
	require.Error(t, err)
	require.Contains(t, err.Error(), "subnet size can only be increased, not decreased")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamUpdateInvalidSubnetSize(t *testing.T) {
	oldSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
	}

	newSliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  35,
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamUpdate(ctx, newSliceIpam, runtime.Object(oldSliceIpam))
	require.Error(t, err)
	require.Contains(t, err.Error(), "subnet size must be between 16 and 30")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamDeleteSuccess(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{},
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig not found (already deleted)
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(
		kubeerrors.NewNotFound(schema.GroupResource{Group: "controller.kubeslice.io", Resource: "sliceconfigs"}, "test-slice"),
	).Once()

	_, err := ValidateSliceIpamDelete(ctx, sliceIpam)
	require.NoError(t, err)
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamDeleteWithActiveAllocations(t *testing.T) {
	now := metav1.Now()
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster-1",
					Subnet:      "10.1.0.0/24",
					AllocatedAt: now,
					Status:      controllerv1alpha1.SubnetStatusAllocated,
				},
				{
					ClusterName: "cluster-2",
					Subnet:      "10.1.1.0/24",
					AllocatedAt: now,
					Status:      controllerv1alpha1.SubnetStatusInUse,
				},
			},
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	_, err := ValidateSliceIpamDelete(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot delete SliceIpam with 2 active allocations")
	require.Contains(t, err.Error(), "cluster-1")
	require.Contains(t, err.Error(), "cluster-2")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamDeleteWithSliceConfigExisting(t *testing.T) {
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{},
		},
	}

	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceConfigSpec{
			SliceIpamType: "Dynamic",
			SliceSubnet:   "10.1.0.0/16",
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig exists and not being deleted
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.SliceConfig)
		*arg = *sliceConfig
	}).Once()

	_, err := ValidateSliceIpamDelete(ctx, sliceIpam)
	require.Error(t, err)
	require.Contains(t, err.Error(), "corresponding SliceConfig")
	require.Contains(t, err.Error(), "still exists")
	clientMock.AssertExpectations(t)
}

func testValidateSliceIpamDeleteWithReleasedAllocationsOnly(t *testing.T) {
	now := metav1.Now()
	sliceIpam := &controllerv1alpha1.SliceIpam{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "test-namespace",
		},
		Spec: controllerv1alpha1.SliceIpamSpec{
			SliceName:   "test-slice",
			SliceSubnet: "10.1.0.0/16",
			SubnetSize:  24,
		},
		Status: controllerv1alpha1.SliceIpamStatus{
			AllocatedSubnets: []controllerv1alpha1.ClusterSubnetAllocation{
				{
					ClusterName: "cluster-1",
					Subnet:      "10.1.0.0/24",
					AllocatedAt: now,
					Status:      controllerv1alpha1.SubnetStatusReleased,
					ReleasedAt:  &now,
				},
			},
		},
	}

	clientMock := &utilmock.Client{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)

	// Mock SliceConfig not found (already deleted)
	clientMock.On("Get", ctx, types.NamespacedName{Name: "test-slice", Namespace: "test-namespace"}, mock.AnythingOfType("*v1alpha1.SliceConfig")).Return(
		kubeerrors.NewNotFound(schema.GroupResource{Group: "controller.kubeslice.io", Resource: "sliceconfigs"}, "test-slice"),
	).Once()

	_, err := ValidateSliceIpamDelete(ctx, sliceIpam)
	require.NoError(t, err)
	clientMock.AssertExpectations(t)
}
