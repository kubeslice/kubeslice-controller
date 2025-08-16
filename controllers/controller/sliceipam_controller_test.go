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

package controller

import (
	"context"
	"testing"

	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSliceIpamReconciler_Reconcile(t *testing.T) {
	// Setup
	mockService := &mocks.ISliceIpamService{}

	reconciler := &SliceIpamReconciler{
		SliceIpamService: mockService,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-slice-ipam",
			Namespace: "kubeslice-test",
		},
	}

	// Mock the service call
	mockService.On("ReconcileSliceIpam", mock.Anything, req).Return(ctrl.Result{}, nil)

	// Test
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)
	mockService.AssertExpectations(t)
}
