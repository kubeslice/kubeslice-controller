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
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// acquireOrRenewLease ensures a Lease named name/namespace exists, is held by
// identity, and has its renewTime stamped to now. It creates the Lease if it is
// missing and increments leaseTransitions whenever leadership changes hands.
// It is used by the Active's renewal loop (StartLeaseRenewal → renewOnce).
func acquireOrRenewLease(ctx context.Context, c client.Client, name, namespace, identity string, leaseDuration time.Duration) (*coordinationv1.Lease, error) {
	now := metav1.NewMicroTime(time.Now())
	// Round up to whole seconds and enforce a minimum of 1s. LeaseDurationSeconds
	// is an int32 count of seconds, so a sub-second duration must not truncate to
	// 0 (an invalid lease duration that also skews staleness checks).
	durationSeconds := int32((leaseDuration + time.Second - 1) / time.Second)
	if durationSeconds < 1 {
		durationSeconds = 1
	}

	lease := &coordinationv1.Lease{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, lease)
	if apierrors.IsNotFound(err) {
		transitions := int32(0)
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &identity,
				LeaseDurationSeconds: &durationSeconds,
				AcquireTime:          &now,
				RenewTime:            &now,
				LeaseTransitions:     &transitions,
			},
		}
		if err := c.Create(ctx, lease); err != nil {
			return nil, err
		}
		return lease, nil
	}
	if err != nil {
		return nil, err
	}

	// The Lease exists. If it was held by someone else (or no one), record a
	// transition and a fresh acquireTime; otherwise simply renew it.
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != identity {
		transitions := int32(1)
		if lease.Spec.LeaseTransitions != nil {
			transitions = *lease.Spec.LeaseTransitions + 1
		}
		lease.Spec.HolderIdentity = &identity
		lease.Spec.AcquireTime = &now
		lease.Spec.LeaseTransitions = &transitions
	}
	lease.Spec.LeaseDurationSeconds = &durationSeconds
	lease.Spec.RenewTime = &now
	if err := c.Update(ctx, lease); err != nil {
		return nil, err
	}
	return lease, nil
}

// getLease fetches a Lease. The Standby uses this to read the Active's Lease over
// the remote client (WatchRemoteLease → checkRemoteLeaseOnce).
func getLease(ctx context.Context, c client.Client, name, namespace string) (*coordinationv1.Lease, error) {
	lease := &coordinationv1.Lease{}
	if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, lease); err != nil {
		return nil, err
	}
	return lease, nil
}

// isLeaseStale reports whether the lease's renewTime is older than
// leaseDuration + padding relative to now. A nil lease or a missing renewTime is
// treated as stale: the holder has never checked in.
func isLeaseStale(lease *coordinationv1.Lease, padding time.Duration, now time.Time) bool {
	if lease == nil || lease.Spec.RenewTime == nil {
		return true
	}
	leaseDuration := time.Duration(0)
	if lease.Spec.LeaseDurationSeconds != nil {
		leaseDuration = time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second
	}
	deadline := lease.Spec.RenewTime.Time.Add(leaseDuration + padding)
	return now.After(deadline)
}
