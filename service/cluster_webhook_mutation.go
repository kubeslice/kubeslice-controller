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
	"encoding/json"
	"net/http"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	v1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

/* MutateClusterSpec function mutates the req object for cluster */
func MutateClusterSpec(ctx context.Context, r *controllerv1alpha1.Cluster, req v1.AdmissionRequest) admission.Response {
	err := json.Unmarshal(req.Object.Raw, r)
	//mutate nodeIp
	if len(r.Spec.NodeIPs) == 0 {
		r.Spec.NodeIPs = append(r.Spec.NodeIPs, r.Spec.NodeIP)
	}
	if req.Operation == v1.Create {
		r.ResourceVersion = ""
	}
	marshaledcluster, err := json.Marshal(r)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledcluster)
}
