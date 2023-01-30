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
	"github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IWorkerSliceGatewayRecyclerService interface {
	ListWorkerSliceGatewayRecyclers(ctx context.Context, ownerLabel map[string]string, namespace string) ([]v1alpha1.WorkerSliceGwRecycler, error)
	DeleteWorkerSliceGatewayRecyclersByLabel(ctx context.Context, label map[string]string, namespace string) error
}

// WorkerSliceGatewayRecyclerService is a schema for interfaces JobService, WorkerSliceConfigService, SecretService
type WorkerSliceGatewayRecyclerService struct{}

// DeleteWorkerSliceGatewayRecyclersByLabel is a function to delete worker slice gateway by label
func (s *WorkerSliceGatewayRecyclerService) DeleteWorkerSliceGatewayRecyclersByLabel(ctx context.Context, label map[string]string, namespace string) error {
	gateways, err := s.ListWorkerSliceGatewayRecyclers(ctx, label, namespace)
	if err != nil {
		return err
	}
	for _, gateway := range gateways {
		err = util.DeleteResource(ctx, &gateway)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListWorkerSliceGatewayRecyclers is a function to list down the established gateways
func (s *WorkerSliceGatewayRecyclerService) ListWorkerSliceGatewayRecyclers(ctx context.Context, ownerLabel map[string]string,
	namespace string) ([]v1alpha1.WorkerSliceGwRecycler, error) {
	gatewayRecyclers := &v1alpha1.WorkerSliceGwRecyclerList{}
	err := util.ListResources(ctx, gatewayRecyclers, client.MatchingLabels(ownerLabel), client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return gatewayRecyclers.Items, nil
}
