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
	"errors"
	"testing"

	"github.com/dailymotion/allure-go"
	"github.com/kubeslice/kubeslice-controller/util"
	utilMock "github.com/kubeslice/kubeslice-controller/util/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestJobServiceSuite(t *testing.T) {
	for k, v := range JobServiceTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var JobServiceTestbed = map[string]func(*testing.T){
	"Test_CreateJob_ThrowsError": Test_CreateJob_ReturnsError,
	"Test_CreateJob_CallsCreate": Test_CreateJob_CallsCreate,
}

func Test_CreateJob_CallsCreate(t *testing.T) {
	clientMock := &utilMock.Client{}
	clientMock.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
	jobService := JobService{}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "jobservicecontroller", nil)

	result, err := jobService.CreateJob(ctx, "", "", nil)
	expectedResult := ctrl.Result{}
	require.NoError(t, nil)
	require.Equal(t, result, expectedResult)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}
func Test_CreateJob_ReturnsError(t *testing.T) {
	clientMock := &utilMock.Client{}
	error := errors.New("testinternalerror")
	clientMock.On("Create", mock.Anything, mock.Anything).Return(error).Once()
	jobService := JobService{}
	ctx := util.PrepareKubeSliceControllersRequestContext(context.Background(), clientMock, nil, "jobservicecontroller", nil)
	result, err := jobService.CreateJob(ctx, "", "", nil)
	expectedResult := ctrl.Result{}
	require.Error(t, err)
	require.Equal(t, result, expectedResult)
	require.Equal(t, err, error)
	clientMock.AssertExpectations(t)
}
