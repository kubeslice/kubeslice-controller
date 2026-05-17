/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 */

package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFullMeshClusterPairIndices guards the full-mesh edge walk used by
// createMinimumGatewaysIfNotExists (see #355): correct count, no duplicates,
// stable i<j ordering so hub-and-spoke work can compare against this baseline.
func TestFullMeshClusterPairIndices(t *testing.T) {
	for n := 0; n <= 8; n++ {
		n := n
		t.Run(fmt.Sprintf("n_%d", n), func(t *testing.T) {
			t.Parallel()
			pairs := fullMeshClusterPairIndices(n)
			wantLen := 0
			if n >= 2 {
				wantLen = n * (n - 1) / 2
			}
			if wantLen == 0 {
				require.Nil(t, pairs)
				return
			}
			require.Len(t, pairs, wantLen)

			seen := make(map[[2]int]struct{}, wantLen)
			for _, ij := range pairs {
				require.Less(t, ij[0], ij[1], "every pair must use i<j ordering")
				require.GreaterOrEqual(t, ij[0], 0)
				require.Less(t, ij[1], n)
				seen[ij] = struct{}{}
			}
			require.Len(t, seen, wantLen, "full mesh must not emit duplicate index pairs")
		})
	}
}
