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

package util

import (
	"context"

	apiutil "github.com/kubeslice/apis/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
)

var Loglevel zapcore.Level
var LoglevelString string

type kubeSliceControllerRequestContext struct {
	Client
	Scheme *runtime.Scheme
	Log    *zap.SugaredLogger
}

// GetKubeSliceControllerRequestContext is a function to get the request context
func GetKubeSliceControllerRequestContext(ctx context.Context) *apiutil.KubeSliceControllerRequestContext {
	if ctx.Value(apiutil.KubeSliceControllerContext) != nil {
		return ctx.Value(apiutil.KubeSliceControllerContext).(*apiutil.KubeSliceControllerRequestContext)
	}
	return nil
}

// CtxLogger is a function to get the logs
func CtxLogger(ctx context.Context) *zap.SugaredLogger {
	logg := GetKubeSliceControllerRequestContext(ctx).Log
	return logg
}
