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
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestHASyncMetrics_RecordAndLabel(t *testing.T) {
	haSyncLagSeconds.Reset()
	haSyncErrorsTotal.Reset()

	haSyncLagSeconds.WithLabelValues("SliceConfig", "create").Observe(0.5)
	haSyncErrorsTotal.WithLabelValues("SliceConfig", "update").Inc()
	haSyncErrorsTotal.WithLabelValues("SliceConfig", "update").Inc()

	assert.Equal(t, 1, testutil.CollectAndCount(haSyncLagSeconds))
	assert.Equal(t, float64(2), testutil.ToFloat64(haSyncErrorsTotal.WithLabelValues("SliceConfig", "update")))
}
