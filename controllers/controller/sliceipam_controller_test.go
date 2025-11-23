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

package controller

import (
	"context"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("SliceIpam Controller Tests", Ordered, func() {
	const (
		sliceIpamName      = "test-slice-ipam"
		sliceIpamNamespace = "kubeslice-test"
		sliceSubnet        = "10.1.0.0/16"
		subnetSize         = 24
	)

	var ctx context.Context

	BeforeAll(func() {
		ctx = context.Background()
	})

	Describe("SliceIpam Controller - Basic CRUD Operations", func() {
		var sliceIpam *v1alpha1.SliceIpam

		BeforeEach(func() {
			sliceIpam = &v1alpha1.SliceIpam{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceIpamName,
					Namespace: sliceIpamNamespace,
				},
				Spec: v1alpha1.SliceIpamSpec{
					SliceName:   sliceIpamName,
					SliceSubnet: sliceSubnet,
					SubnetSize:  subnetSize,
				},
			}
		})

		AfterEach(func() {
			// Clean up the SliceIpam resource
			createdSliceIpam := &v1alpha1.SliceIpam{}
			getKey := types.NamespacedName{
				Name:      sliceIpamName,
				Namespace: sliceIpamNamespace,
			}

			// Get the resource if it exists and delete it
			if err := k8sClient.Get(ctx, getKey, createdSliceIpam); err == nil {
				Expect(k8sClient.Delete(ctx, createdSliceIpam)).Should(Succeed())

				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, getKey, createdSliceIpam)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}
		})

		It("Should create SliceIpam successfully", func() {
			By("Creating a new SliceIpam resource")
			Expect(k8sClient.Create(ctx, sliceIpam)).Should(Succeed())

			// Verify the SliceIpam was created
			createdSliceIpam := &v1alpha1.SliceIpam{}
			getKey := types.NamespacedName{
				Name:      sliceIpamName,
				Namespace: sliceIpamNamespace,
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdSliceIpam)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Verify spec fields
			Expect(createdSliceIpam.Spec.SliceName).To(Equal(sliceIpamName))
			Expect(createdSliceIpam.Spec.SliceSubnet).To(Equal(sliceSubnet))
			Expect(createdSliceIpam.Spec.SubnetSize).To(Equal(subnetSize))
		})

		It("Should initialize status fields correctly", func() {
			By("Creating a new SliceIpam resource")
			Expect(k8sClient.Create(ctx, sliceIpam)).Should(Succeed())

			// Get the created SliceIpam and check if finalizer is added
			createdSliceIpam := &v1alpha1.SliceIpam{}
			getKey := types.NamespacedName{
				Name:      sliceIpamName,
				Namespace: sliceIpamNamespace,
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdSliceIpam)
				if err != nil {
					return false
				}
				// Check if finalizer is added by controller
				return len(createdSliceIpam.Finalizers) > 0
			}, timeout, interval).Should(BeTrue())
		})

		It("Should update SliceIpam spec successfully", func() {
			By("Creating a SliceIpam resource")
			Expect(k8sClient.Create(ctx, sliceIpam)).Should(Succeed())

			// Get the created resource
			createdSliceIpam := &v1alpha1.SliceIpam{}
			getKey := types.NamespacedName{
				Name:      sliceIpamName,
				Namespace: sliceIpamNamespace,
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdSliceIpam)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Updating the SubnetSize")
			createdSliceIpam.Spec.SubnetSize = 26
			Expect(k8sClient.Update(ctx, createdSliceIpam)).Should(Succeed())

			// Verify the update
			updatedSliceIpam := &v1alpha1.SliceIpam{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, updatedSliceIpam)
				if err != nil {
					return false
				}
				return updatedSliceIpam.Spec.SubnetSize == 26
			}, timeout, interval).Should(BeTrue())
		})
	})

	Describe("SliceIpam Controller - Status Management", func() {
		var sliceIpam *v1alpha1.SliceIpam

		BeforeEach(func() {
			sliceIpam = &v1alpha1.SliceIpam{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceIpamName + "-status",
					Namespace: sliceIpamNamespace,
				},
				Spec: v1alpha1.SliceIpamSpec{
					SliceName:   sliceIpamName + "-status",
					SliceSubnet: "10.2.0.0/16",
					SubnetSize:  24,
				},
			}
		})

		AfterEach(func() {
			// Clean up the SliceIpam resource
			createdSliceIpam := &v1alpha1.SliceIpam{}
			getKey := types.NamespacedName{
				Name:      sliceIpamName + "-status",
				Namespace: sliceIpamNamespace,
			}

			if err := k8sClient.Get(ctx, getKey, createdSliceIpam); err == nil {
				Expect(k8sClient.Delete(ctx, createdSliceIpam)).Should(Succeed())

				Eventually(func() bool {
					err := k8sClient.Get(ctx, getKey, createdSliceIpam)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}
		})

		It("Should calculate available subnets correctly", func() {
			By("Creating a SliceIpam resource")
			Expect(k8sClient.Create(ctx, sliceIpam)).Should(Succeed())

			// Get the created resource and verify status calculation
			createdSliceIpam := &v1alpha1.SliceIpam{}
			getKey := types.NamespacedName{
				Name:      sliceIpamName + "-status",
				Namespace: sliceIpamNamespace,
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdSliceIpam)
				if err != nil {
					return false
				}
				// For a /16 with /24 subnets, we should have 256 available subnets
				return createdSliceIpam.Status.AvailableSubnets > 0
			}, timeout, interval).Should(BeTrue())
		})
	})

	Describe("SliceIpam Controller - Deletion with Finalizer", func() {
		var sliceIpam *v1alpha1.SliceIpam

		BeforeEach(func() {
			sliceIpam = &v1alpha1.SliceIpam{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceIpamName + "-finalizer",
					Namespace: sliceIpamNamespace,
				},
				Spec: v1alpha1.SliceIpamSpec{
					SliceName:   sliceIpamName + "-finalizer",
					SliceSubnet: "10.3.0.0/16",
					SubnetSize:  24,
				},
			}
		})

		It("Should handle deletion with finalizer properly", func() {
			By("Creating a SliceIpam resource")
			Expect(k8sClient.Create(ctx, sliceIpam)).Should(Succeed())

			// Wait for controller to add finalizer
			createdSliceIpam := &v1alpha1.SliceIpam{}
			getKey := types.NamespacedName{
				Name:      sliceIpamName + "-finalizer",
				Namespace: sliceIpamNamespace,
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdSliceIpam)
				if err != nil {
					return false
				}
				return len(createdSliceIpam.Finalizers) > 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the SliceIpam resource")
			Expect(k8sClient.Delete(ctx, createdSliceIpam)).Should(Succeed())

			// Verify resource is eventually deleted (finalizer should be removed by controller)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdSliceIpam)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
