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

// Package ha implements cross-cluster (Active/Standby) high availability for the
// kubeslice-controller. It coordinates leadership between two separate hub
// clusters so that only the Active hub writes to worker clusters. This is
// distinct from controller-runtime's in-cluster --leader-elect, which only
// coordinates multiple pods sharing a single API server. See ADR #293.
package ha

import (
	"fmt"
	"strings"
)

// HAMode identifies the high-availability role this controller instance plays.
type HAMode string

const (
	// ModeActive means this controller holds (or competes for) leadership by
	// renewing a Lease on its own cluster; while it is the leader its reconcilers
	// write to worker clusters.
	ModeActive HAMode = "active"
	// ModeStandby means this controller mirrors the Active hub and never writes.
	// In issue #294 a Standby watches the Active's Lease but does not promote;
	// promotion is issue #297.
	ModeStandby HAMode = "standby"
	// ModeStandalone is the default single-hub behaviour: always the leader, no
	// Lease and no remote watching — identical to the controller before HA.
	ModeStandalone HAMode = "standalone"
)

// ParseHAModeStrict converts a flag/env string into an HAMode, rejecting a
// non-empty value that names no known mode. Empty still means ModeStandalone,
// which is what every deployment that passes no --ha-mode at all gets.
//
// Coercing a typo to standalone looks like failing safe and is the opposite.
// Standalone is unconditionally the leader, so a hub started with
// --ha-mode=stanby does not become an inert Standby: it becomes a second
// unfenced writer alongside the real Active, reconciling the same worker
// clusters. That is the dual-writer state the whole design exists to prevent,
// reached silently, from one transposed letter in a chart value. A hub that
// refuses to start is recoverable in a way that one is not.
func ParseHAModeStrict(s string) (HAMode, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ModeStandalone, nil
	}
	mode := HAMode(strings.ToLower(trimmed))
	if !mode.IsValid() {
		return ModeStandalone, fmt.Errorf(
			"unknown --ha-mode %q: expected %q, %q or %q", s, ModeActive, ModeStandby, ModeStandalone)
	}
	return mode, nil
}

// ParseHAMode converts a flag/env string into an HAMode, mapping empty or
// unrecognised values to ModeStandalone. Callers acting on operator input
// should prefer ParseHAModeStrict, which reports the unrecognised value instead
// of guessing; see its comment for why guessing is unsafe here.
func ParseHAMode(s string) HAMode {
	switch HAMode(strings.ToLower(strings.TrimSpace(s))) {
	case ModeActive:
		return ModeActive
	case ModeStandby:
		return ModeStandby
	default:
		return ModeStandalone
	}
}

// IsValid reports whether m is one of the known modes.
func (m HAMode) IsValid() bool {
	switch m {
	case ModeActive, ModeStandby, ModeStandalone:
		return true
	default:
		return false
	}
}
