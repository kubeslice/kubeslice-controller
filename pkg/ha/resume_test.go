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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resumeOpts is the shape main.go passes: a hub configured as a Standby, with
// the identity it was promoted under.
func resumeOpts() Options {
	return Options{
		Mode:           ModeStandby,
		Identity:       "hub-a",
		LeaseName:      DefaultLeaseName,
		LeaseNamespace: "kubeslice-controller",
		PaddingSeconds: 5 * time.Second,
		Log:            testLog(),
	}
}

func TestResumeAsActive_ResumesWhenThisHubHoldsALiveLease(t *testing.T) {
	opts := resumeOpts()
	lease := newLease(opts.LeaseName, opts.LeaseNamespace, opts.Identity, time.Now())
	c := fakeClient(t, lease)

	resume, reason, err := ResumeAsActive(context.Background(), c, opts)
	require.NoError(t, err)
	assert.True(t, resume, "a promoted hub restarting inside its lease duration must come back as the Active, not as a Standby with nobody to mirror from")
	assert.Contains(t, reason, "live HA lease")
}

func TestResumeAsActive_DefersWhenItsOwnLeaseIsStale(t *testing.T) {
	opts := resumeOpts()
	// Down long enough that the other hub's padding could have expired and it
	// could already have promoted. Becoming Active here would be a second
	// writer, which is the one outcome worth being conservative about.
	lease := newLease(opts.LeaseName, opts.LeaseNamespace, opts.Identity, time.Now().Add(-10*time.Minute))
	c := fakeClient(t, lease)

	resume, reason, err := ResumeAsActive(context.Background(), c, opts)
	require.NoError(t, err)
	assert.False(t, resume)
	assert.Contains(t, reason, "stale")
}

func TestResumeAsActive_DefersWhenAnotherHubHoldsTheLease(t *testing.T) {
	opts := resumeOpts()
	lease := newLease(opts.LeaseName, opts.LeaseNamespace, "hub-b", time.Now())
	c := fakeClient(t, lease)

	resume, reason, err := ResumeAsActive(context.Background(), c, opts)
	require.NoError(t, err)
	assert.False(t, resume)
	assert.Contains(t, reason, "hub-b")
}

func TestResumeAsActive_NoLeaseMeansNeverPromoted(t *testing.T) {
	// The ordinary case: a Standby that has always been a Standby. NotFound must
	// read as "stay a Standby", not as an error.
	resume, reason, err := ResumeAsActive(context.Background(), fakeClient(t), resumeOpts())
	require.NoError(t, err)
	assert.False(t, resume)
	assert.Contains(t, reason, "never promoted")
}

func TestResumeAsActive_UnreadableLeaseIsAnErrorNotADemotion(t *testing.T) {
	// A transient API failure is not evidence that this hub is not the Active.
	// Returning false here would silently demote a promoted hub on a blip, so the
	// caller is given the error and keeps the configured mode instead.
	resume, _, err := ResumeAsActive(context.Background(), failingReadClient(t), resumeOpts())
	require.Error(t, err)
	assert.False(t, resume)
}

func TestResumeAsActive_OnlyAppliesToStandbyMode(t *testing.T) {
	for _, mode := range []HAMode{ModeActive, ModeStandalone} {
		opts := resumeOpts()
		opts.Mode = mode
		// A live lease under this identity is present, so the only reason to
		// return false is the mode gate itself.
		c := fakeClient(t, newLease(opts.LeaseName, opts.LeaseNamespace, opts.Identity, time.Now()))

		resume, reason, err := ResumeAsActive(context.Background(), c, opts)
		require.NoError(t, err)
		assert.False(t, resume, "mode %s must not be overridden", mode)
		assert.Contains(t, reason, "not standby")
	}
}

func TestResumeAsActive_ResolvesTheSameLeaseTargetAsTheElector(t *testing.T) {
	// The check reads the Lease before an elector exists, so it must resolve the
	// same name, namespace and identity the elector would from the same Options.
	// Left to drift, this would read the wrong Lease and always return false —
	// failing open in a way no test of the true path would catch.
	opts := Options{Mode: ModeStandby, Identity: "hub-a", Log: testLog()}
	elector := NewClusterLeaderElector(fakeClient(t), fakeClient(t), opts)

	resolved := applyDefaults(opts)
	assert.Equal(t, elector.leaseName, resolved.LeaseName)
	assert.Equal(t, elector.leaseNS, resolved.LeaseNamespace)
	assert.Equal(t, elector.identity, resolved.Identity)
	assert.Equal(t, elector.padding, resolved.PaddingSeconds)

	// And end to end through the exported function, using those resolved values.
	c := fakeClient(t, newLease(resolved.LeaseName, resolved.LeaseNamespace, resolved.Identity, time.Now()))
	resume, _, err := ResumeAsActive(context.Background(), c, opts)
	require.NoError(t, err)
	assert.True(t, resume)
}
