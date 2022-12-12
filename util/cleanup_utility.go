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
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetObjectKind is a function which return the kind of existing resource
func CleanupGetObjectKind(obj runtime.Object) string {
	kindPath := reflect.TypeOf(obj)
	kindPathName := kindPath.String()
	kindPathNameParts := strings.Split(kindPathName, ".")
	return kindPathNameParts[len(kindPathNameParts)-1]
}

// GetResourceIfExist is a function to get the given resource is in namespace
func CleanupGetResourceIfExist(ctx context.Context, namespacedName client.ObjectKey, object client.Object) (bool,
	error) {
	logger := CtxLogger(ctx)
	logger.Debugf("Fetching %s %s in namespace %s", CleanupGetObjectKind(object),
		namespacedName.Name, namespacedName.Namespace)
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Get(ctx, namespacedName, object)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("%s %s in namespace %s not found", CleanupGetObjectKind(object),
				namespacedName.Name, namespacedName.Namespace)
			return false, nil
		} else {
			logger.With(zap.Error(err)).Errorf("%s Error retrieving object of kind %s with name %s in namespace %s", Err,
				CleanupGetObjectKind(object), namespacedName.Name, namespacedName.Namespace)
			return false, err
		}
	}
	return true, nil
}

// UpdateResource is a function to update resource
func CleanupUpdateResource(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Updating %s %s in namespace %s", CleanupGetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Update(ctx, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to update resource: %v", object)
		return err
	}
	logger.Infof("%s Updated %s %s in namespace %s", Tick, CleanupGetObjectKind(object), object.GetName(),
		object.GetNamespace())
	return err
}

// UpdateStatus is a function to update the status of given resource
func CleanupUpdateStatus(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Updating status of %s with name %s in namespace %s", CleanupGetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Status().Update(ctx, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("%s Failed to update status: %v", Err, object)
		return err
	}
	logger.Infof("%s Updated status of %s %s in namespace %s", Tick, CleanupGetObjectKind(object), object.GetName(),
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
	logger.Debugf("Deleting %s %s in namespace %s", CleanupGetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Delete(ctx, object)
	if err != nil {
		return err
	}
	logger.Infof("%s Deleted %s %s in namespace %s", Recycle, CleanupGetObjectKind(object), object.GetName(),
		object.GetNamespace())
	return nil
}

// ListResources is a function to list down the resources/objects of given kind
func CleanupListResources(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Listing %s with options %v", CleanupGetObjectKind(list), opts)

	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.List(ctx, list, opts...)
	return err
}

// GetOwnerLabel is a function returns the label of object
func CleanupGetOwnerLabel(completeResourceName string) map[string]string {
	label := map[string]string{}
	for key, value := range LabelsKubeSliceController {
		label[key] = value
	}
	lenCompleteResourceName := len(completeResourceName)
	i := 0
	j := 0
	//resourceName = fmt.Sprintf(LabelValue, CleanupGetObjectKind(owner), owner.GetName())
	if lenCompleteResourceName > 63 {
		noOfLabels := lenCompleteResourceName / 63
		label["kubeslice-controller-resource-name"] = completeResourceName[j:63]
		j = 63
		lenCompleteResourceName = lenCompleteResourceName - 63
		for i = 1; i <= noOfLabels; i++ {
			if lenCompleteResourceName < 63 {
				break
			}
			label["kubeslice-controller-resource-name-"+fmt.Sprint(i)] = completeResourceName[j : 63*(i+1)]
			lenCompleteResourceName = lenCompleteResourceName - 63
			j = 63 * (i + 1)
		}
		label["kubeslice-controller-resource-name-"+fmt.Sprint(i)] = completeResourceName[j:]
	} else {
		label["kubeslice-controller-resource-name"] = completeResourceName
	}
	return label
}
