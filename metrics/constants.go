/*
 * Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * ou may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package metrics

var (
	PROMETHEUS_SERVICE_ENDPOINT = "http://kubeslice-controller-prometheus-service:9090"

	// IPAM Pool Metrics
	IPAMPoolTotalSubnets     = "kubeslice_ipam_pool_total_subnets"
	IPAMPoolAllocatedSubnets = "kubeslice_ipam_pool_allocated_subnets"
	IPAMPoolAvailableSubnets = "kubeslice_ipam_pool_available_subnets"
)
