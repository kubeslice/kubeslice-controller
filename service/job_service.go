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
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
)

type IJobService interface {
	CreateJob(ctx context.Context, namespace string, jobImage string, environment map[string]string) (ctrl.Result, error)
}

type JobService struct{}

// CreateJob is function to create the job in k8s
func (j *JobService) CreateJob(ctx context.Context, namespace string, jobImage string, environment map[string]string) (ctrl.Result, error) {
	name := fmt.Sprintf("%s-%s", "open-cert", uuid.NewUUID()[:8])
	tTLSecondsAfterFinished := int32(60 * 60 * 24)
	backoffLimit := int32(2)
	envValues := make([]v1.EnvVar, 0, len(environment))
	for key, value := range environment {
		envValue := v1.EnvVar{
			Name:  key,
			Value: value,
		}
		envValues = append(envValues, envValue)
	}

	logEnv := v1.EnvVar{
		Name:  "LOG_LEVEL",
		Value: util.LoglevelString,
	}
	envValues = append(envValues, logEnv)

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &tTLSecondsAfterFinished,
			BackoffLimit:            &backoffLimit,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "ovpn-cert-generator",
					},
					Name:      "ovpn-cert-job-pod",
					Namespace: namespace,
				},
				Spec: v1.PodSpec{
					RestartPolicy: "Never",
					Containers: []v1.Container{
						{
							Name:  "ovpn-cert-generator",
							Image: jobImage,
							Env: envValues,
						},
					},
					ServiceAccountName: JobServiceAccount,
				},
			},
		},
		Status: batchv1.JobStatus{},
	}
	if JobCredential != "" {
		job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, v1.LocalObjectReference{
			Name: JobCredential,
		})
	}
	err := util.CreateResource(ctx, job)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
