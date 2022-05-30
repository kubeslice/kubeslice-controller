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

	vutil "github.com/kubeslice/apis/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
)

var Loglevel zapcore.Level
var LoglevelString string

// type kubeSliceControllerContextKey struct {
// }

// kubeSliceControllerRequestContext is a schema for request context
type kubeSliceControllerRequestContext struct {
	Client
	Scheme *runtime.Scheme
	Log    *zap.SugaredLogger
}

// kubeSliceControllerContext is instance of kubeSliceControllerContextKey
//var kubeSliceControllerContext = &kubeSliceControllerContextKey{}

// PrepareKubeSliceControllersRequestContext is a function to create the context for kube slice
/* func PrepareKubeSliceControllersRequestContext(ctx context.Context, client Client,
	scheme *runtime.Scheme, controllerName string) context.Context {
	uuid := k8sUuid.NewUUID()[:8]

	var log *zap.SugaredLogger

	if Loglevel == zap.DebugLevel {
		log = zap.S().With(
			zap.String("RequestId", string(uuid)),
			zap.String("Controller", controllerName),
		)
	} else {
		log = zap.S()
	}

	ctxVal := &kubeSliceControllerRequestContext{
		Client: client,
		Scheme: scheme,
		Log:    log,
	}
	newCtx := context.WithValue(ctx, kubeSliceControllerContext, ctxVal)
	return newCtx
} */

// GetKubeSliceControllerRequestContext is a function to get the request context
func GetKubeSliceControllerRequestContext(ctx context.Context) *vutil.KubeSliceControllerRequestContext {
	if ctx.Value(vutil.KubeSliceControllerContext) != nil {
		return ctx.Value(vutil.KubeSliceControllerContext).(*vutil.KubeSliceControllerRequestContext)
	}
	return nil
}

// CtxLogger is a function to get the logs
func CtxLogger(ctx context.Context) *zap.SugaredLogger {
	logg := GetKubeSliceControllerRequestContext(ctx).Log
	return logg
}
