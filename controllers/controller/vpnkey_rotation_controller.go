/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"github.com/kubeslice/kubeslice-controller/pkg/ha"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

// VpnKeyRotationReconciler reconciles a VpnKeyRotation object
type VpnKeyRotationReconciler struct {
	// PromotionKick, when set, delivers one event per existing object after a
	// promotion. The HA write fence drops reconcile requests rather than
	// requeuing them, so flipping it reconciles nothing that already existed;
	// this is what wakes that state up. Nil outside HA, which registers no
	// extra watch and leaves behaviour unchanged.
	PromotionKick <-chan event.GenericEvent
	client.Client
	Scheme                *runtime.Scheme
	VpnKeyRotationService service.IVpnKeyRotationService
	Log                   *zap.SugaredLogger
	EventRecorder         *events.EventRecorder
	// LeaderElector gates mutating reconciles on cross-cluster leadership. It is
	// nil-safe: a nil elector (HA not wired) behaves as standalone. See ADR #293.
	LeaderElector *ha.ClusterLeaderElector
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpnKeyRotationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.VpnKeyRotation{})
	// Registered only when set. source.Channel rejects a nil channel when the
	// manager starts it, so an unconditional watch would break every caller that
	// does not wire the kick — the envtest suite among them.
	if r.PromotionKick != nil {
		b = b.WatchesRawSource(source.Channel(r.PromotionKick, &handler.EnqueueRequestForObject{}))
	}
	return b.Complete(r)
}

// Reconcile is a function to reconcile the VpnKeyRotation, VpnKeyRotationReconciler implements it
func (r *VpnKeyRotationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// HA write fence: only the Active hub (or a standalone controller) writes.
	// A Standby evaluates this on every call and no-ops.
	if r.LeaderElector != nil && !r.LeaderElector.IsLeader() {
		r.Log.Info("standby mode, skipping reconcile")
		return ctrl.Result{}, nil
	}
	kubeSliceCtx := util.PrepareKubeSliceControllersRequestContext(ctx, r.Client, r.Scheme, "VpnKeyRotationController", r.EventRecorder)
	return r.VpnKeyRotationService.ReconcileVpnKeyRotation(kubeSliceCtx, req)
}
