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

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	k8sUuid "k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Loglevel zapcore.Level
var LoglevelString string

// Client is interface for k8s
type Client interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
	Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	Status() client.StatusWriter
}
type KubeSliceControllerContextKey struct {
}

// kubeSliceControllerRequestContext is a schema for request context
type KubeSliceControllerRequestContext struct {
	Client
	Scheme *runtime.Scheme
	Log    *zap.SugaredLogger
}

// kubeSliceControllerContext is instance of kubeSliceControllerContextKey
var KubeSliceControllerContext = &KubeSliceControllerContextKey{}

// PrepareKubeSliceControllersRequestContext is a function to create the context for kube slice
func PrepareKubeSliceControllersRequestContext(ctx context.Context, client Client,
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

	ctxVal := &KubeSliceControllerRequestContext{
		Client: client,
		Scheme: scheme,
		Log:    log,
	}
	newCtx := context.WithValue(ctx, KubeSliceControllerContext, ctxVal)
	return newCtx
}
