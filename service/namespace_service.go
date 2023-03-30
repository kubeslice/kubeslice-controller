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
	"fmt"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type INamespaceService interface {
	ReconcileProjectNamespace(ctx context.Context, namespace string, owner client.Object) (ctrl.Result, error)
	DeleteNamespace(ctx context.Context, namespace string) (ctrl.Result, error)
}

type NamespaceService struct {
}

// ReconcileProjectNamespace is a function to reconcile project namespace
func (n *NamespaceService) ReconcileProjectNamespace(ctx context.Context, namespace string, owner client.Object) (ctrl.Result, error) {
	nsResource := &corev1.Namespace{}
	found, err := util.GetResourceIfExist(ctx, client.ObjectKey{
		Name: namespace,
	}, nsResource)
	if err != nil {
		return ctrl.Result{}, err
	}
	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(ControllerNamespace)
	if !found {
		expectedNS := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespace,
				Labels: n.getResourceLabel(namespace, owner),
			},
		}
		err := util.CreateResource(ctx, expectedNS)
		expectedNS.Namespace = ControllerNamespace
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, expectedNS, nil, events.EventNamespaceCreationFailed)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, expectedNS, nil, events.EventNamespaceCreated)
	}
	return ctrl.Result{}, nil
}

// DeleteNamespace is a function deletes the namespace
func (n *NamespaceService) DeleteNamespace(ctx context.Context, namespace string) (ctrl.Result, error) {
	nsResource := &corev1.Namespace{}
	found, err := util.GetResourceIfExist(ctx, client.ObjectKey{
		Name: namespace,
	}, nsResource)
	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(namespace)).WithNamespace(ControllerNamespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		return ctrl.Result{}, err
	}
	if found {
		nsToBeDeleted := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		err := util.DeleteResource(ctx, nsToBeDeleted)
		nsToBeDeleted.Namespace = ControllerNamespace
		if err != nil {
			util.RecordEvent(ctx, eventRecorder, nsToBeDeleted, nil, events.EventNamespaceDeletionFailed)
			return ctrl.Result{}, err
		}
		util.RecordEvent(ctx, eventRecorder, nsToBeDeleted, nil, events.EventNamespaceDeleted)
	}
	return ctrl.Result{}, nil
}

func (n *NamespaceService) getResourceLabel(namespace string, owner client.Object) map[string]string {
	label := map[string]string{}
	for key, value := range util.LabelsKubeSliceController {
		label[key] = value
	}
	label[util.LabelName] = fmt.Sprintf(util.LabelValue, owner.GetObjectKind().GroupVersionKind().Kind, namespace)
	return label
}
