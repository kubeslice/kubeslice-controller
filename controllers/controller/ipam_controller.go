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

package controller

import (
	"context"
	"fmt"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/events"
	"github.com/kubeslice/kubeslice-controller/metrics"
	"github.com/kubeslice/kubeslice-controller/service"
	"github.com/kubeslice/kubeslice-controller/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IPAMPoolReconciler reconciles IPAM pools
type IPAMPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	ipam   *service.IPAMService
	mf     *metrics.MetricRecorder
}

// NewIPAMPoolReconciler creates a new IPAM pool reconciler
func NewIPAMPoolReconciler(client client.Client, scheme *runtime.Scheme, mf *metrics.MetricRecorder) *IPAMPoolReconciler {
	return &IPAMPoolReconciler{
		Client: client,
		Scheme: scheme,
		ipam:   service.NewIPAMService(client, mf),
		mf:     mf,
	}
}

//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=ipampools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=ipampools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=ipampools/finalizers,verbs=update
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=sliceconfigs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=controller.kubeslice.io,resources=sliceconfigs/status,verbs=get;update;patch

// Reconcile reconciles IPAM pools
func (r *IPAMPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling IPAM pool")

	// Load context with event recorder
	eventRecorder := util.CtxEventRecorder(ctx)

	// Get the IPAM pool
	ipamPool := &controllerv1alpha1.IPAMPool{}
	err := r.Get(ctx, req.NamespacedName, ipamPool)
	if err != nil {
		if errors.IsNotFound(err) {
			// IPAM pool not found, nothing to reconcile
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get IPAM pool")
		return ctrl.Result{}, err
	}

	// Check if IPAM pool is being deleted
	if !ipamPool.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, ipamPool)
	}

	// Reconcile the IPAM pool
	err = r.reconcileIPAMPool(ctx, ipamPool)
	if err != nil {
		logger.Error(err, "Failed to reconcile IPAM pool")
		util.RecordEvent(ctx, eventRecorder, ipamPool, nil, events.EventIPAMPoolReconciliationFailed)
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Record successful reconciliation
	util.RecordEvent(ctx, eventRecorder, ipamPool, nil, events.EventIPAMPoolReconciled)
	logger.Info("IPAM pool reconciled successfully")

	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// reconcileIPAMPool reconciles a single IPAM pool
func (r *IPAMPoolReconciler) reconcileIPAMPool(ctx context.Context, ipamPool *controllerv1alpha1.IPAMPool) error {
	logger := util.CtxLogger(ctx)

	// Check if the IPAM pool is properly initialized
	if ipamPool.Status.TotalSubnets == 0 {
		logger.Info("Initializing IPAM pool")
		err := r.ipam.InitializePool(ctx, ipamPool)
		if err != nil {
			return fmt.Errorf("failed to initialize IPAM pool: %w", err)
		}

		// Update the pool status
		err = r.Status().Update(ctx, ipamPool)
		if err != nil {
			return fmt.Errorf("failed to update IPAM pool status: %w", err)
		}
	}

	// Check for stale allocations (clusters that are no longer in the slice)
	err := r.cleanupStaleAllocations(ctx, ipamPool)
	if err != nil {
		return fmt.Errorf("failed to cleanup stale allocations: %w", err)
	}

	// Update metrics
	r.updateMetrics(ipamPool)

	return nil
}

// cleanupStaleAllocations removes allocations for clusters that are no longer in the slice
func (r *IPAMPoolReconciler) cleanupStaleAllocations(ctx context.Context, ipamPool *controllerv1alpha1.IPAMPool) error {
	logger := util.CtxLogger(ctx)

	// Extract slice name from pool name (format: {sliceName}-ipam-pool)
	sliceName := ipamPool.Name[:len(ipamPool.Name)-len("-ipam-pool")]

	// Get the slice config to check current clusters
	sliceConfig := &controllerv1alpha1.SliceConfig{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      sliceName,
		Namespace: ipamPool.Namespace,
	}, sliceConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// Slice config not found, clean up all allocations
			logger.Info("Slice config not found, cleaning up all allocations")
			for clusterName := range ipamPool.Status.AllocatedSubnets {
				err := r.ipam.DeallocateSubnetForCluster(ctx, sliceConfig, clusterName)
				if err != nil {
					logger.Error(err, "Failed to deallocate subnet for cluster", "cluster", clusterName)
				}
			}
			return nil
		}
		return fmt.Errorf("failed to get slice config: %w", err)
	}

	// Check for clusters that are no longer in the slice
	currentClusters := make(map[string]bool)
	for _, cluster := range sliceConfig.Spec.Clusters {
		currentClusters[cluster] = true
	}

	// Find stale allocations
	for clusterName := range ipamPool.Status.AllocatedSubnets {
		if !currentClusters[clusterName] {
			logger.Info("Deallocating subnet for removed cluster", "cluster", clusterName)
			err := r.ipam.DeallocateSubnetForCluster(ctx, sliceConfig, clusterName)
			if err != nil {
				logger.Error(err, "Failed to deallocate subnet for removed cluster", "cluster", clusterName)
			}
		}
	}

	return nil
}

// updateMetrics updates metrics for the IPAM pool
func (r *IPAMPoolReconciler) updateMetrics(ipamPool *controllerv1alpha1.IPAMPool) {

	// log the metrics for debugging
	logger := log.Log.WithName("IPAMPoolReconciler")
	logger.Info("IPAM Pool metrics",
		"pool_name", ipamPool.Name,
		"total_subnets", ipamPool.Status.TotalSubnets,
		"allocated_count", ipamPool.Status.AllocatedCount,
		"available_count", ipamPool.Status.AvailableCount)
}

// handleDeletion handles the deletion of an IPAM pool
func (r *IPAMPoolReconciler) handleDeletion(ctx context.Context, ipamPool *controllerv1alpha1.IPAMPool) (ctrl.Result, error) {
	logger := util.CtxLogger(ctx)
	logger.Info("Handling IPAM pool deletion")

	// Clean up all allocations
	for clusterName := range ipamPool.Status.AllocatedSubnets {
		logger.Info("Deallocating subnet during pool deletion", "cluster", clusterName)
		// Create a minimal slice config for deallocation
		sliceConfig := &controllerv1alpha1.SliceConfig{
			Spec: controllerv1alpha1.SliceConfigSpec{
				SliceIpamType: "Dynamic",
			},
		}
		err := r.ipam.DeallocateSubnetForCluster(ctx, sliceConfig, clusterName)
		if err != nil {
			logger.Error(err, "Failed to deallocate subnet during pool deletion", "cluster", clusterName)
		}
	}

	// Remove finalizers if any
	if len(ipamPool.Finalizers) > 0 {
		ipamPool.Finalizers = nil
		err := r.Update(ctx, ipamPool)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the manager
func (r *IPAMPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controllerv1alpha1.IPAMPool{}).
		Complete(r)
}
