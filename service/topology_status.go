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
	"fmt"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// buildTopologyConvergedCondition aggregates the connectivity of a slice's
// WorkerSliceGateway objects into a single TopologyConverged condition.
//
// A slice is converged (ConditionTrue) when every gateway link reports
// ConnectionState == Connected, or when the slice has no gateway links at all.
// Otherwise it is ConditionFalse and the message identifies how many links are
// connected and names the first not-connected link (chosen deterministically by
// gateway name so the message is stable across reconciles). An empty
// ConnectionState is treated as Pending.
//
// LastTransitionTime is intentionally left unset: callers apply this condition
// via apimachinery meta.SetStatusCondition, which stamps the transition time
// only when the status actually changes.
func buildTopologyConvergedCondition(gateways []workerv1alpha1.WorkerSliceGateway, observedGeneration int64) metav1.Condition {
	cond := metav1.Condition{
		Type:               controllerv1alpha1.SliceConditionTypeTopologyConverged,
		ObservedGeneration: observedGeneration,
	}

	total := len(gateways)
	if total == 0 {
		cond.Status = metav1.ConditionTrue
		cond.Reason = controllerv1alpha1.SliceReasonNoGatewaysRequired
		cond.Message = "slice requires no gateway links"
		return cond
	}

	ready := 0
	notReadyFound := false
	firstNotReadyName := ""
	firstNotReadyState := ""
	for i := range gateways {
		state := gateways[i].Status.ConnectionState
		if state == workerv1alpha1.GatewayConnectionStateConnected {
			ready++
			continue
		}
		if state == "" {
			state = workerv1alpha1.GatewayConnectionStatePending
		}
		if !notReadyFound || gateways[i].Name < firstNotReadyName {
			notReadyFound = true
			firstNotReadyName = gateways[i].Name
			firstNotReadyState = state
		}
	}

	if ready == total {
		cond.Status = metav1.ConditionTrue
		cond.Reason = controllerv1alpha1.SliceReasonAllEdgesReady
		cond.Message = fmt.Sprintf("all %d gateway links connected", total)
		return cond
	}

	cond.Status = metav1.ConditionFalse
	cond.Reason = controllerv1alpha1.SliceReasonEdgesNotReady
	cond.Message = fmt.Sprintf("%d/%d gateway links connected; %s is %s", ready, total, firstNotReadyName, firstNotReadyState)
	return cond
}
