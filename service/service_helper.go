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
	"time"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RemoveWorkerFinalizers removes the finalizer specified by workerFinalizerName from the
// object's finalizers list.
// If the workerFinalizerName is not found in the initial finalizers list, all finalizers
// are removed and the function returns the reconciliation result for no-requeue.
// If the finalizers list is empty after the removal, the function returns the
// reconciliation result for no-requeue.
// If the finalizers list is non-empty after the removal, the function returns the
// reconciliation result for delayed requeue.
func RemoveWorkerFinalizers(ctx context.Context, object client.Object, workerFinalizerName string) ctrl.Result {
	logger := util.CtxLogger(ctx)
	finalizers := object.GetFinalizers()
	additionalFinalizers := make([]string, 0)
	result := ctrl.Result{}
	logger.Debugf("Attempting to remove finalizer %s from %v", workerFinalizerName, finalizers)
	if util.IsInSlice(finalizers, workerFinalizerName) {
		for _, finalizer := range finalizers {
			if finalizer != workerFinalizerName {
				additionalFinalizers = append(additionalFinalizers, finalizer)
			}
		}
		if len(additionalFinalizers) > 0 {
			if time.Now().Sub(object.GetDeletionTimestamp().Time) < KubesliceWorkerDeleteRequeueTime*time.Minute {
				result.Requeue = true
				result.RequeueAfter = KubesliceWorkerDeleteRequeueTime * time.Minute
				logger.Debugf("Found additional finalizers: %v. Requeuing with Reconciliation Result: %+v", additionalFinalizers, result)
				return result
			} else {
				logger.Debugf("Cleaning up additional finalizers: %v because deletion grace period has exceeded", additionalFinalizers)
			}
		} else {
			logger.Debugf("No additional finalizers found.")
		}
	}
	object.SetFinalizers(make([]string, 0))
	if err := util.UpdateResource(ctx, object); err != nil {
		logger.With(zap.Error(err)).Errorf("Failed to cleanup finalizers")
	}
	return result
}

// get Slice gateway service type for each cluster registered with given slice
func getSliceGwSvcTypes(sliceConfig *v1alpha1.SliceConfig) map[string]*v1alpha1.SliceGatewayServiceType {
	var sliceGwSvcTypeMap = make(map[string]*v1alpha1.SliceGatewayServiceType)
	for _, gwSvctype := range sliceConfig.Spec.SliceGatewayProvider.SliceGatewayServiceType {
		if gwSvctype.Cluster == "*" {
			for _, cluster := range sliceConfig.Spec.Clusters {
				sliceGwSvcTypeMap[cluster] = &gwSvctype
			}
		} else {
			sliceGwSvcTypeMap[gwSvctype.Cluster] = &gwSvctype
		}
	}
	return sliceGwSvcTypeMap
}
