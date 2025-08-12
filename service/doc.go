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

// Package service provides the business logic layer for the KubeSlice Controller.
//
// This package implements the core services that orchestrate the creation, management,
// and lifecycle of KubeSlice resources across worker clusters. The service layer acts
// as an intermediary between the Kubernetes controllers and the underlying resource
// management operations.
//
// # Architecture Overview
//
// The service package follows a modular architecture where each service is responsible
// for managing specific aspects of the KubeSlice ecosystem:
//
//   - Project management and multi-tenancy
//   - Cluster registration and lifecycle
//   - Slice configuration and networking
//   - Access control and RBAC
//   - Service import/export across slices
//   - VPN key rotation and security
//   - Quality of Service (QoS) management
//
// # Service Dependencies
//
// Services are designed with clear dependency relationships and are bootstrapped
// through the bootstrap.go file. The dependency injection pattern ensures loose
// coupling and facilitates testing.
//
// # Core Services
//
// Project Services:
//   - ProjectService: Manages project lifecycle, namespaces, and tenant isolation
//   - NamespaceService: Handles Kubernetes namespace creation and management
//   - AccessControlService: Manages RBAC roles, service accounts, and permissions
//
// Cluster Management Services:
//   - ClusterService: Handles worker cluster registration and lifecycle
//   - SliceConfigService: Manages slice configurations and networking policies
//
// Network and Gateway Services:
//   - WorkerSliceGatewayService: Manages gateway deployments and inter-cluster connectivity
//   - WorkerSliceGatewayRecyclerService: Handles cleanup of unused gateway resources
//   - ServiceExportConfigService: Manages service export configurations across slices
//   - WorkerServiceImportService: Handles service import from other clusters
//
// Security Services:
//   - VpnKeyRotationService: Manages VPN certificate rotation and security
//   - SecretService: Handles secret management and distribution
//
// Quality and Resource Management:
//   - SliceQoSConfigService: Manages Quality of Service configurations
//   - JobService: Handles Kubernetes job creation and management
//
// # Service Interfaces
//
// Each service implements a well-defined interface (e.g., IProjectService, IClusterService)
// that abstracts the implementation details and enables easy mocking for unit tests.
// This design promotes testability and allows for alternative implementations.
//
// # Reconcile Pattern
//
// Most services implement a Reconcile pattern that:
//  1. Fetches the current state of resources
//  2. Compares with desired state
//  3. Takes corrective actions to achieve desired state
//  4. Updates resource status and emits events
//
// # Error Handling
//
// Services follow consistent error handling patterns using Go's error interface.
// Errors are properly wrapped with context information and logged using the
// structured logging framework.
//
// # Metrics and Observability
//
// Services integrate with the metrics package to provide observability into
// KubeSlice operations. Key metrics include resource creation/deletion times,
// error rates, and operational status.
//
// # Constants and Configuration
//
// Network configuration constants like DefaultSubnetMask and DefaultVPNCipher
// are defined in kube_slice_resource_names.go to avoid magic strings and
// improve maintainability.
//
// # Usage Example
//
//	// Bootstrap services with dependencies
//	services := service.WithServices(
//		workerSliceConfigService,
//		projectService,
//		clusterService,
//		sliceConfigService,
//		// ... other services
//	)
//
//	// Use services in controllers
//	result, err := services.ProjectService.ReconcileProject(ctx, request)
//
// # Testing
//
// The service package includes comprehensive test suites with mocked dependencies.
// Each service can be tested in isolation using the generated mock interfaces
// found in the mocks/ subdirectory.
//
// For more details on individual services, refer to their respective documentation
// and the KubeSlice architecture documentation at https://kubeslice.io/documentation/
package service
