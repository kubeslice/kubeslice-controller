package controller

import (
	"context"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	sliceQoSConfigName1 = "sliceqos-test-1"
	sliceQoSConfigName2 = "sliceqos-test-2"
)

var _ = Describe("SliceQoSConfig controller", func() {
	When("Creating SliceQoSConfig CR", func() {
		It("should create and delete SliceQoSConfig CR without errors", func() {
			By("Creating a new SliceQoSConfig CR")
			ctx := context.Background()

			expectedSpec := controllerv1alpha1.SliceQoSConfigSpec{
				QueueType:               "HTB",
				Priority:                1,
				TcType:                  "BANDWIDTH_CONTROL",
				BandwidthCeilingKbps:    100000,
				BandwidthGuaranteedKbps: 50000,
				DscpClass:               "AF11",
			}

			sliceQoS := &controllerv1alpha1.SliceQoSConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceQoSConfigName1,
					Namespace: controlPlaneNamespace,
				},
				Spec: expectedSpec,
			}
			Expect(k8sClient.Create(ctx, sliceQoS)).Should(Succeed())

			By("Looking up the created SliceQoSConfig CR")
			sliceQoSLookupKey := types.NamespacedName{
				Name:      sliceQoSConfigName1,
				Namespace: controlPlaneNamespace,
			}
			createdSliceQoS := &controllerv1alpha1.SliceQoSConfig{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceQoSLookupKey, createdSliceQoS)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the created CR has the expected spec values")
			Expect(createdSliceQoS.Spec).To(Equal(expectedSpec))

			By("Deleting the created SliceQoSConfig CR")
			Expect(k8sClient.Delete(ctx, createdSliceQoS)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceQoSLookupKey, createdSliceQoS)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should handle deletion of a SliceQoSConfig CR gracefully", func() {
			By("Creating a new SliceQoSConfig CR")
			ctx := context.Background()

			expectedSpec := controllerv1alpha1.SliceQoSConfigSpec{
				QueueType:               "HTB",
				Priority:                2,
				TcType:                  "BANDWIDTH_CONTROL",
				BandwidthCeilingKbps:    200000,
				BandwidthGuaranteedKbps: 100000,
				DscpClass:               "AF21",
			}

			sliceQoS := &controllerv1alpha1.SliceQoSConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceQoSConfigName2,
					Namespace: controlPlaneNamespace,
				},
				Spec: expectedSpec,
			}
			Expect(k8sClient.Create(ctx, sliceQoS)).Should(Succeed())

			sliceQoSLookupKey := types.NamespacedName{
				Name:      sliceQoSConfigName2,
				Namespace: controlPlaneNamespace,
			}

			createdSliceQoS := &controllerv1alpha1.SliceQoSConfig{}

			By("Looking up the created CR and verifying spec values")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceQoSLookupKey, createdSliceQoS)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdSliceQoS.Spec).To(Equal(expectedSpec))

			By("Deleting the created SliceQoSConfig CR")
			Expect(k8sClient.Delete(ctx, sliceQoS)).Should(Succeed())

			By("Ensuring the CR is actually deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, sliceQoSLookupKey, createdSliceQoS)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
