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

import "testing"

func TestParseHAMode(t *testing.T) {
	cases := map[string]HAMode{
		"active":     ModeActive,
		"ACTIVE":     ModeActive,
		" standby ":  ModeStandby,
		"standalone": ModeStandalone,
		"":           ModeStandalone,
		"garbage":    ModeStandalone,
	}
	for in, want := range cases {
		if got := ParseHAMode(in); got != want {
			t.Errorf("ParseHAMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHAModeIsValid(t *testing.T) {
	for _, m := range []HAMode{ModeActive, ModeStandby, ModeStandalone} {
		if !m.IsValid() {
			t.Errorf("%q should be valid", m)
		}
	}
	if HAMode("nope").IsValid() {
		t.Error("unknown mode should be invalid")
	}
}

// TestParseHAModeStrict_RejectsTypos covers the case that makes lenient parsing
// dangerous: standalone is unconditionally the leader, so a hub whose --ha-mode
// was mistyped does not fail closed into an inert Standby, it fails OPEN into a
// second unfenced writer against the same worker clusters as the real Active.
func TestParseHAModeStrict_RejectsTypos(t *testing.T) {
	for _, bad := range []string{"stanby", "activ", "primary", "true", "STANDBYY"} {
		mode, err := ParseHAModeStrict(bad)
		if err == nil {
			t.Errorf("ParseHAModeStrict(%q) must reject an unknown mode, got %q", bad, mode)
		}
	}
}

// TestParseHAModeStrict_AcceptsKnownModesAndEmpty pins the other half: every
// deployment that passes no --ha-mode at all must keep getting standalone, and
// the documented spellings must survive surrounding whitespace and case.
func TestParseHAModeStrict_AcceptsKnownModesAndEmpty(t *testing.T) {
	for in, want := range map[string]HAMode{
		"":            ModeStandalone,
		"   ":         ModeStandalone,
		"standalone":  ModeStandalone,
		"active":      ModeActive,
		"standby":     ModeStandby,
		"  Standby  ": ModeStandby,
		"ACTIVE":      ModeActive,
	} {
		got, err := ParseHAModeStrict(in)
		if err != nil {
			t.Errorf("ParseHAModeStrict(%q) returned an error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseHAModeStrict(%q) = %q, want %q", in, got, want)
		}
	}
}
