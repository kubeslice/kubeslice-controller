/*
 * Copyright (c) 2026 Avesha, Inc. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package service

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test_VpnCertRotationJobFailure_NoRequeue verifies that a failed cert-rotation Job
// causes the reconciler to requeue (not silently stop) and resets jobCreationInProgress
// so the job is recreated on the next reconcile.
func Test_VpnCertRotationJobFailure_NoRequeue(t *testing.T) {
	ctx, clientMock, vpnService, _, _ := setupTestCase()

	// Certs are expired: CertificateExpiryTime is in the past.
	expiredTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	vpnConfig := &controllerv1alpha1.VpnKeyRotation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-slice", Namespace: "test-ns"},
		Spec: controllerv1alpha1.VpnKeyRotationSpec{
			SliceName:               "test-slice",
			CertificateCreationTime: &expiredTime,
			CertificateExpiryTime:   &expiredTime,
			RotationInterval:        30,
		},
	}
	sliceConfig := &controllerv1alpha1.SliceConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-slice", Namespace: "test-ns"},
	}

	// Simulate: jobs were already triggered in a previous reconcile.
	vpnService.jobCreationInProgress.Store(true)

	// The cert-rotation Job is in Failed state.
	clientMock.
		On("List", mock.Anything, mock.AnythingOfType("*v1.JobList"), mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			jobList := args.Get(1).(*batchv1.JobList)
			jobList.Items = []batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "cert-job-1",
						Labels: map[string]string{"SLICE_NAME": "test-slice"},
					},
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{
							{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
						},
					},
				},
			}
		}).Once()
	clientMock.On("Create", mock.Anything, mock.AnythingOfType("*v1.Event")).Return(nil).Maybe()

	result, returnedConfig, err := vpnService.reconcileVpnKeyRotationConfig(ctx, vpnConfig, sliceConfig)

	assert.Nil(t, err)
	assert.Nil(t, returnedConfig)
	assert.Equal(t, 30*time.Second, result.RequeueAfter, "must requeue so rotation is retried after job failure")
	assert.False(t, vpnService.jobCreationInProgress.Load(), "jobCreationInProgress must be cleared so the failed job is recreated")
}
