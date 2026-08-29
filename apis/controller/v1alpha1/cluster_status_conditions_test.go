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

package v1alpha1

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const clustersCRDPath = "../../../config/crd/bases/controller.kubeslice.io_clusters.yaml"

// The worker operator reports its hub-connection health as conditions on its own
// Cluster CR. That only works if this repository declares the field, in two
// separate places, and both have failed independently before:
//
//   - the Go type, because the controller reconciles Cluster status with a typed
//     client — a round-trip through a struct that has no Conditions field drops
//     whatever the worker wrote, which is how status.activeController was being
//     erased from the other direction;
//   - the generated CRD, because a structural schema prunes any field it does not
//     describe, silently, on a write the API server still reports as accepted.
//
// The two tests below pin one each.
func TestClusterStatus_ConditionsSurviveATypedRoundTrip(t *testing.T) {
	original := ClusterStatus{
		RegistrationStatus: RegistrationStatusRegistered,
		Conditions: []metav1.Condition{{
			Type:               "ControllerConnected",
			Status:             metav1.ConditionTrue,
			Reason:             "Connected",
			Message:            "reconciling against the active hub",
			LastTransitionTime: metav1.Now(),
		}},
	}

	encoded, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ClusterStatus
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	require.Len(t, decoded.Conditions, 1,
		"a status round-trip must not drop the worker's conditions")
	assert.Equal(t, "ControllerConnected", decoded.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, decoded.Conditions[0].Status)
	assert.Equal(t, "Connected", decoded.Conditions[0].Reason)
}

func TestClustersCRD_DescribesStatusConditions(t *testing.T) {
	raw, err := os.ReadFile(clustersCRDPath)
	require.NoError(t, err, "the generated Cluster CRD must be readable from the repo")

	var crd struct {
		Spec struct {
			Versions []struct {
				Name   string `json:"name"`
				Schema struct {
					OpenAPIV3Schema struct {
						Properties struct {
							Status struct {
								Properties map[string]interface{} `json:"properties"`
							} `json:"status"`
						} `json:"properties"`
					} `json:"openAPIV3Schema"`
				} `json:"schema"`
			} `json:"versions"`
		} `json:"spec"`
	}
	require.NoError(t, utilyaml.Unmarshal(raw, &crd))
	require.NotEmpty(t, crd.Spec.Versions)

	for _, v := range crd.Spec.Versions {
		props := v.Schema.OpenAPIV3Schema.Properties.Status.Properties
		assert.Contains(t, props, "conditions",
			"version %s prunes status.conditions; regenerate the CRD after changing ClusterStatus", v.Name)
		assert.Contains(t, props, "activeController",
			"version %s prunes status.activeController; regenerate the CRD after changing ClusterStatus", v.Name)
	}
}
