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

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ISecretService interface {
	DeleteSecret(ctx context.Context, namespace string, secretName string) (ctrl.Result, error)
}

type SecretService struct {
}

// DeleteSecret is a function to delete the secret
func (s *SecretService) DeleteSecret(ctx context.Context, namespace string, secretName string) (ctrl.Result, error) {
	nsResource := &corev1.Secret{}
	found, err := util.GetResourceIfExist(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: namespace,
	}, nsResource)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !found {
		return ctrl.Result{}, nil
	}

	//Load Event Recorder with project name and namespace
	eventRecorder := util.CtxEventRecorder(ctx).WithProject(util.GetProjectName(nsResource.Namespace)).WithNamespace(nsResource.Namespace)

	if found {
		err = util.DeleteResource(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
		})
		if err != nil {
			//Register an event for secret deletion failure
			util.RecordEvent(ctx, eventRecorder, nsResource, nil, ossEvents.EventSecretDeletionFailed)
			return ctrl.Result{}, err
		}
		//Register an event for secret deletion
		util.RecordEvent(ctx, eventRecorder, nsResource, nil, ossEvents.EventSecretDeleted)
	}
	return ctrl.Result{}, nil
}
