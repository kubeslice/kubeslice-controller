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

package ha

import (
	"context"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	coordinationv1 "k8s.io/api/coordination/v1"

	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/util"
)

// PromotedToActiveEmitter returns a PromotionHooks.EmitPromotedEvent that
// records the promotion against the Lease the new Active just acquired.
//
// The Lease is the right object to attach it to: it *is* the leadership record,
// there is exactly one, and it lives in the controller's own namespace — so the
// Event lands beside the controller that emitted it, discoverable with a plain
// `kubectl get events -n <controller namespace>`.
//
// Not kubeslice-system. Issue #297 asks for the Event on that namespace, but
// it does not exist on a hub cluster: per ADR #293 Decision 1 it is a *worker*
// namespace (worker-operator, NSM, gateways, DNS), and the hub's is
// kubeslice-controller / $KUBESLICE_CONTROLLER_MANAGER_NAMESPACE. The trap is
// that a constant with exactly the wrong meaning is sitting in the vendor tree
// (kubeslice-monitoring's logger.ControlPlaneNamespace) waiting to be reached
// for. The recorder derives the Event's namespace from the involved object, so
// passing the Lease is what puts it in the right place.
//
// recorder.RecordEvent is called directly, never util.RecordEvent. That
// helper's first statement is util.CtxLogger(ctx), which nil-panics on any
// context that has not been through PrepareKubeSliceControllersRequestContext —
// and promotion runs on main.go's signal-handler context, which has not. This
// crashed a live Standby once during #295; the fix is not to reach for the
// helper out of habit.
func PromotedToActiveEmitter(recorder events.EventRecorder) func(context.Context, *coordinationv1.Lease) error {
	if recorder == nil {
		return nil
	}
	return func(ctx context.Context, lease *coordinationv1.Lease) error {
		if lease == nil {
			// Nothing to attach the Event to. Promotion has already succeeded by
			// this point, so this is worth reporting but not worth failing over.
			return nil
		}
		return recorder.RecordEvent(ctx, &events.Event{
			Object:            lease,
			ReportingInstance: util.InstanceController,
			Name:              ossEvents.EventHAPromotedToActive,
		})
	}
}
