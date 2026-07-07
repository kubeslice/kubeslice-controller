/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 */

package util

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is an alias for controller-runtime's client interface, used by tests
// and helpers that historically referred to util.Client.
type Client = client.Client
