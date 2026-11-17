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

package util

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation"
)

func serviceExportOwnerName(service, namespace, slice string) string {
	return fmt.Sprintf("ServiceExportConfig-%s-%s-%s", service, namespace, slice)
}

func requireValidOwnerLabel(t *testing.T, name string, label map[string]string) {
	t.Helper()
	require.Equal(t, LabelValueResourceOwner, label[LabelResourceOwner], name)
	for key, value := range label {
		require.Empty(t, validation.IsValidLabelValue(value), "%s: %s=%q", name, key, value)
	}
}

func TestGetOwnerLabel_ShortNameIsUnchanged(t *testing.T) {
	label := GetOwnerLabel("SliceConfig-red")
	require.Equal(t, map[string]string{
		LabelResourceOwner: LabelValueResourceOwner,
		LabelManagedBy:     "SliceConfig-red",
	}, label)
}

func TestGetOwnerLabel_ExactlyOneLabelIsUnchanged(t *testing.T) {
	name := strings.Repeat("a", 63)
	require.Equal(t, map[string]string{
		LabelResourceOwner: LabelValueResourceOwner,
		LabelManagedBy:     name,
	}, GetOwnerLabel(name))
}

func TestGetOwnerLabel_LongNameWithValidPiecesIsUnchanged(t *testing.T) {
	// the cut falls inside "tenant": both pieces are valid label values
	name := serviceExportOwnerName("e2n-gp1--kgyo-predictor", "e2n-gp1--kgyo", "tenant-c9eaa03d")
	require.Equal(t, map[string]string{
		LabelResourceOwner:    LabelValueResourceOwner,
		LabelManagedBy:        "ServiceExportConfig-e2n-gp1--kgyo-predictor-e2n-gp1--kgyo-tenan",
		LabelManagedBy + "-1": "t-c9eaa03d",
	}, GetOwnerLabel(name))
}

func TestGetOwnerLabel_ThreePiecesAreUnchanged(t *testing.T) {
	name := strings.Repeat("a", 63) + strings.Repeat("b", 63) + "c"
	require.Equal(t, map[string]string{
		LabelResourceOwner:    LabelValueResourceOwner,
		LabelManagedBy:        strings.Repeat("a", 63),
		LabelManagedBy + "-1": strings.Repeat("b", 63),
		LabelManagedBy + "-2": "c",
	}, GetOwnerLabel(name))
}

func TestGetOwnerLabel_PieceStartingWithDashIsHashed(t *testing.T) {
	// the second 63-character piece would be "--7173-tenant-c9eaa03d"
	name := serviceExportOwnerName("ui-ai-gateway--7173-predictor", "ui-ai-gateway--7173", "tenant-c9eaa03d")
	require.True(t, strings.HasPrefix(name[63:], "-"))
	label := GetOwnerLabel(name)
	requireValidOwnerLabel(t, name, label)
	require.Len(t, label, 2)
	require.Equal(t, "ServiceExportConfig-ui-ai-gate_8bfa5d12b49f35abdc02b1f1ed132d34", label[LabelManagedBy])
}

func TestGetOwnerLabel_PieceEndingWithDashIsHashed(t *testing.T) {
	name := strings.Repeat("a", 62) + "-" + "b"
	label := GetOwnerLabel(name)
	requireValidOwnerLabel(t, name, label)
	require.Len(t, label, 2)
	require.Contains(t, label[LabelManagedBy], "_")
}

func TestGetOwnerLabel_HashedValueIsStableAndDistinct(t *testing.T) {
	first := serviceExportOwnerName("ui-ai-gateway--7173-predictor", "ui-ai-gateway--7173", "tenant-c9eaa03d")
	second := serviceExportOwnerName("ui-ai-gateway--7174-predictor", "ui-ai-gateway--7174", "tenant-c9eaa03d")
	require.Equal(t, GetOwnerLabel(first), GetOwnerLabel(first))
	// same readable prefix, different names
	require.NotEqual(t, GetOwnerLabel(first)[LabelManagedBy], GetOwnerLabel(second)[LabelManagedBy])
}

func TestGetOwnerLabel_DoesNotChangeTheSharedLabels(t *testing.T) {
	label := GetOwnerLabel("SliceConfig-red")
	label["worker-cluster"] = "cluster-1"
	require.Equal(t, map[string]string{LabelResourceOwner: LabelValueResourceOwner}, LabelsKubeSliceController)
}

func TestHashedOwnerLabelValue(t *testing.T) {
	// a leading character that cannot start a label value is dropped
	value := hashedOwnerLabelValue("-abc")
	require.True(t, strings.HasPrefix(value, "abc_"))
	require.Empty(t, validation.IsValidLabelValue(value))
	// no usable character: the digest alone
	value = hashedOwnerLabelValue("---")
	require.Len(t, value, ownerLabelDigestLength)
	require.Empty(t, validation.IsValidLabelValue(value))
	// the longest value is exactly the label limit
	value = hashedOwnerLabelValue(strings.Repeat("a", 200))
	require.Len(t, value, validation.LabelValueMaxLength)
}

// Every name gets valid labels; a name whose plain split is valid keeps exactly that split;
// two names never share a label set.
func TestGetOwnerLabel_RandomNames(t *testing.T) {
	const characters = "abcdefghijklmnopqrstuvwxyz0123456789----.."
	random := rand.New(rand.NewSource(1))
	seen := map[string]string{}
	for n := 0; n < 50000; n++ {
		raw := make([]byte, 1+random.Intn(260))
		for i := range raw {
			raw[i] = characters[random.Intn(len(characters))]
		}
		name := "Kind-" + string(raw)
		label := GetOwnerLabel(name)
		requireValidOwnerLabel(t, name, label)

		split := splitOwnerLabel(name)
		splitIsValid := true
		for _, value := range split {
			if len(validation.IsValidLabelValue(value)) != 0 {
				splitIsValid = false
			}
		}
		if splitIsValid {
			split[LabelResourceOwner] = LabelValueResourceOwner
			require.Equal(t, split, label, name)
		} else {
			require.Len(t, label, 2, name)
		}

		key := fmt.Sprint(label)
		if other, found := seen[key]; found {
			require.Equal(t, other, name, "two names share the labels %s", key)
		}
		seen[key] = name
	}
}
