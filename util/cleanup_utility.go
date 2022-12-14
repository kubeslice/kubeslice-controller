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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateResource is a function to update resource
func CleanupUpdateResource(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Updating %s %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Update(ctx, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to update resource: %v", object)
		return err
	}
	logger.Infof("%s Updated %s %s in namespace %s", Tick, GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	return err
}

// UpdateStatus is a function to update the status of given resource
func CleanupUpdateStatus(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Updating status of %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Status().Update(ctx, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("%s Failed to update status: %v", Err, object)
		return err
	}
	logger.Infof("%s Updated status of %s %s in namespace %s", Tick, GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	err = kubeSliceCtx.Get(ctx, client.ObjectKey{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("%s Failed to fetch object after status update: %+v", Err, object)
	}
	return nil
}

// DeleteResource is a function to delete the resource of given kind
func CleanupDeleteResource(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Deleting %s %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Delete(ctx, object)
	if err != nil {
		return err
	}
	logger.Infof("%s Deleted %s %s in namespace %s", Recycle, GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	return nil
}
