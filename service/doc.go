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

// Package service provides the core business logic services for managing KubeSlice resources.
//
// This package contains service implementations that handle the lifecycle management
// of KubeSlice custom resources including Projects, Clusters, SliceConfigs, and
// related components. Each service encapsulates the business logic for reconciling
// and managing specific resource types within the KubeSlice controller.
//
// Key Services:
//
//   - ProjectService: Manages KubeSlice projects and their associated namespaces,
//     access controls, and resource cleanup.
//
//   - ClusterService: Handles cluster registration, deregistration, and lifecycle
//     management within KubeSlice projects.
//
//   - SliceConfigService: Manages slice configurations, including network policies,
//     QoS profiles, and inter-cluster connectivity setup.
//
//   - AccessControlService: Provides RBAC management for service accounts, roles,
//     and role bindings across project namespaces.
//
//   - NamespaceService: Handles namespace creation, labeling, and cleanup for
//     KubeSlice projects.
//
//   - SecretService: Manages secrets for cluster authentication and inter-cluster
//     communication.
//
// Service Architecture:
//
// Services follow a dependency injection pattern where higher-level services
// depend on lower-level services through interfaces. This promotes testability
// and loose coupling between components.
//
// Most services implement a Reconcile pattern that:
//  1. Fetches the current state of resources
//  2. Compares with desired state
//  3. Takes corrective actions to achieve desired state
//  4. Updates resource status and emits events
//
// Error handling and metrics collection are integrated throughout the service
// layer to provide observability and debugging capabilities.
package service
