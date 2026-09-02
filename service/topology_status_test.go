/*
 * 	Copyright (c) 2026 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
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
	"strings"
	"testing"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func gw(name, state string) workerv1alpha1.WorkerSliceGateway {
	g := workerv1alpha1.WorkerSliceGateway{}
	g.Name = name
	g.Status.ConnectionState = state
	return g
}

func TestBuildTopologyConvergedCondition(t *testing.T) {
	cases := []struct {
		name           string
		gateways       []workerv1alpha1.WorkerSliceGateway
		wantStatus     metav1.ConditionStatus
		wantReason     string
		msgContains    []string
		msgNotContains []string
	}{
		{
			name:        "no gateways is trivially converged",
			gateways:    nil,
			wantStatus:  metav1.ConditionTrue,
			wantReason:  controllerv1alpha1.SliceReasonNoGatewaysRequired,
			msgContains: []string{"no gateway links"},
		},
		{
			name: "all connected is converged",
			gateways: []workerv1alpha1.WorkerSliceGateway{
				gw("slice-hub-spoke1", workerv1alpha1.GatewayConnectionStateConnected),
				gw("slice-spoke1-hub", workerv1alpha1.GatewayConnectionStateConnected),
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  controllerv1alpha1.SliceReasonAllEdgesReady,
			msgContains: []string{"all", "2"},
		},
		{
			name: "one not connected is not converged and is named",
			gateways: []workerv1alpha1.WorkerSliceGateway{
				gw("slice-hub-spoke1", workerv1alpha1.GatewayConnectionStateConnected),
				gw("slice-hub-spoke2", workerv1alpha1.GatewayConnectionStateNotConnected),
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  controllerv1alpha1.SliceReasonEdgesNotReady,
			msgContains: []string{"1/2", "slice-hub-spoke2", "NotConnected"},
		},
		{
			name: "empty connection state is reported as Pending",
			gateways: []workerv1alpha1.WorkerSliceGateway{
				gw("slice-hub-spoke1", ""),
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  controllerv1alpha1.SliceReasonEdgesNotReady,
			msgContains: []string{"0/1", "Pending"},
		},
		{
			name: "first not-ready is chosen deterministically by name",
			gateways: []workerv1alpha1.WorkerSliceGateway{
				gw("slice-z", workerv1alpha1.GatewayConnectionStateNotConnected),
				gw("slice-a", workerv1alpha1.GatewayConnectionStateNotConnected),
				gw("slice-m", workerv1alpha1.GatewayConnectionStateConnected),
			},
			wantStatus:     metav1.ConditionFalse,
			wantReason:     controllerv1alpha1.SliceReasonEdgesNotReady,
			msgContains:    []string{"1/3", "slice-a"},
			msgNotContains: []string{"slice-z"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cond := buildTopologyConvergedCondition(tc.gateways, 7)
			if cond.Type != controllerv1alpha1.SliceConditionTypeTopologyConverged {
				t.Fatalf("type = %q, want %q", cond.Type, controllerv1alpha1.SliceConditionTypeTopologyConverged)
			}
			if cond.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q (msg=%q)", cond.Status, tc.wantStatus, cond.Message)
			}
			if cond.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", cond.Reason, tc.wantReason)
			}
			if cond.ObservedGeneration != 7 {
				t.Fatalf("observedGeneration = %d, want 7", cond.ObservedGeneration)
			}
			for _, sub := range tc.msgContains {
				if !strings.Contains(cond.Message, sub) {
					t.Fatalf("message %q does not contain %q", cond.Message, sub)
				}
			}
			for _, sub := range tc.msgNotContains {
				if strings.Contains(cond.Message, sub) {
					t.Fatalf("message %q should not contain %q", cond.Message, sub)
				}
			}
		})
	}
}
