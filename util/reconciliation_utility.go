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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	LabelsKubeSliceController = map[string]string{
		"kubeslice-resource-owner": "controller",
	}
)
var (
	LabelName  = "kubeslice-controller-resource-name"
	LabelValue = "%s-%s"
)

// GetObjectKindis a function which return the kind of existing resource
func GetObjectKind(obj runtime.Object) string {
	kindPath := reflect.TypeOf(obj)
	kindPathName := kindPath.String()
	kindPathNameParts := strings.Split(kindPathName, ".")
	return kindPathNameParts[len(kindPathNameParts)-1]
}

// GetResourceIfExist is a function to get the given resource is in namespace
func GetResourceIfExist(ctx context.Context, namespacedName client.ObjectKey, object client.Object) (bool,
	error) {
	logger := CtxLogger(ctx)
	logger.Debugf("Fetching Object of kind %s with name %s in namespace %s", GetObjectKind(object),
		namespacedName.Name, namespacedName.Namespace)
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Get(ctx, namespacedName, object)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("Object of kind %s with name %s in namespace %s not found", GetObjectKind(object),
				namespacedName.Name, namespacedName.Namespace)
			return false, nil
		} else {
			logger.With(zap.Error(err)).Errorf("Error retrieving object of kind %s with name %s in namespace %s",
				GetObjectKind(object), namespacedName.Name, namespacedName.Namespace)
			return false, err
		}
	}
	return true, nil
}

// CreateResource is a function to create the given kind of resource if not exist in namepscae
func CreateResource(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Creating object kind %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Create(ctx, object)
	if err != nil {
		//logger.With(zap.Error(err)).Errorf("Failed to create resource: %v in namespace", object)
		return err
	}
	logger.Infof("Created object kind %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	return nil
}

// UpdateResource is a function to update resource
func UpdateResource(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Updating object kind %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Update(ctx, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to update resource: %v", object)
		return err
	}
	logger.Infof("Updated object kind %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	return err
}

// UpdateStatus is a function to update the status of given resource
func UpdateStatus(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Updating object status %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Status().Update(ctx, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to update status: %v", object)
		return err
	}
	logger.Infof("Updated object status %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	err = kubeSliceCtx.Get(ctx, client.ObjectKey{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("failed to fetch object after status update: %+v", object)
	}
	return nil
}

// DeleteResource is a function to delete the resource of given kind
func DeleteResource(ctx context.Context, object client.Object) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Deleting object kind %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.Delete(ctx, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to delete resource: %v", object)
		return err
	}
	logger.Infof("Deleted object kind %s with name %s in namespace %s", GetObjectKind(object), object.GetName(),
		object.GetNamespace())
	return nil
}

// ListResources is a function to list down the resources/objects of given kind
func ListResources(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	logger := CtxLogger(ctx)
	logger.Debugf("Listing objects of kind %s with options %v", GetObjectKind(list), opts)
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	err := kubeSliceCtx.List(ctx, list, opts...)
	return err
}

// AddFinalizer is a function to add specific conditions before deleting the resource
func AddFinalizer(ctx context.Context, object client.Object, finalizerName string) (ctrl.Result, error) {
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	logger := CtxLogger(ctx)
	logger.Debugf("Adding finalizer %s to %s", finalizerName, object.GetName())
	controllerutil.AddFinalizer(object, finalizerName)
	if err := kubeSliceCtx.Update(ctx, object); err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to add finalizer")
		return ctrl.Result{}, err
	}
	logger.Infof("Added finalizer %s to %s", finalizerName, object.GetName())
	err := kubeSliceCtx.Get(ctx, client.ObjectKey{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}, object)
	if err != nil {
		logger.With(zap.Error(err)).Errorf("failed to fetch object after adding finalizer: %+v", object)
	}
	return ctrl.Result{}, nil
}

// RemoveFinalizer is a function to discard the condition to delete the resource
func RemoveFinalizer(ctx context.Context, object client.Object, finalizerName string) (ctrl.Result, error) {
	kubeSliceCtx := GetKubeSliceControllerRequestContext(ctx)
	logger := CtxLogger(ctx)
	logger.Debugf("Removing finalizer %s from %s", finalizerName, object.GetName())
	controllerutil.RemoveFinalizer(object, finalizerName)
	if err := kubeSliceCtx.Update(ctx, object); err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to remove finalizer %s", finalizerName)
		return ctrl.Result{}, err
	}
	logger.Infof("Removed finalizer %s to %s", finalizerName, object.GetName())
	return ctrl.Result{}, nil
}

// IsReconciled is a function to requeue the reconcilation process
func IsReconciled(object ctrl.Result, err error) (bool, ctrl.Result, error) {
	if err != nil {
		zap.S().With(zap.Error(err)).Errorf("Error Occurred on Reconciliation. Need to requeue.")
		return true, object, err
	}
	if object.Requeue {
		zap.S().Debug("Need to requeue.")
		return true, object, nil
	} else {
		//zap.S().Debug("Requeue not needed.")
		return false, object, nil
	}
}

// ContainsString is a function to check if the given string is in given array
func ContainsString(strings []string, t string) bool {
	for _, item := range strings {
		if item == t {
			return true
		}
	}
	return false
}

// GetOwnerLabel is a function returns the label of object
func GetOwnerLabel(owner client.Object) map[string]string {
	label := map[string]string{}
	for key, value := range LabelsKubeSliceController {
		label[key] = value
	}
	label[LabelName] = fmt.Sprintf(LabelValue, GetObjectKind(owner), owner.GetName())
	return label
}

// EncodeToBase64 is a function to to encode the string
func EncodeToBase64(v interface{}) (string, error) {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	err := json.NewEncoder(encoder).Encode(v)
	if err != nil {
		return "", err
	}
	err = encoder.Close()
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
