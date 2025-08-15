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
	serviceExportConfigName1 = "sec-test-1"
	serviceExportConfigName2 = "sec-test-2"
)

var _ = Describe("ServiceExportConfig controller", func() {
	When("Creating ServiceExportConfig CR", func() {
		It("should create ServiceExportConfig CR without errors", func() {
			By("Creating a new ServiceExportConfig CR")
			ctx := context.Background()

			sec := &controllerv1alpha1.ServiceExportConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceExportConfigName1,
					Namespace: controlPlaneNamespace,
				},
				Spec: controllerv1alpha1.ServiceExportConfigSpec{
					ServiceName:      "test-service",
					ServiceNamespace: "default",
					SourceCluster:    "test-cluster",
					SliceName:        "test-slice",
					ServiceDiscoveryPorts: []controllerv1alpha1.ServiceDiscoveryPort{
						{
							Name:            "http",
							Protocol:        "TCP",
							Port:            80,
							ServicePort:     8080,
							ServiceProtocol: "TCP",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sec)).Should(Succeed())

			By("Looking up the created ServiceExportConfig CR")
			secLookupKey := types.NamespacedName{
				Name:      serviceExportConfigName1,
				Namespace: controlPlaneNamespace,
			}
			createdSec := &controllerv1alpha1.ServiceExportConfig{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secLookupKey, createdSec)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Deleting the created ServiceExportConfig CR")
			Expect(k8sClient.Delete(ctx, createdSec)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secLookupKey, createdSec)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should handle deletion of a ServiceExportConfig CR gracefully", func() {
			By("Creating a new ServiceExportConfig CR")
			ctx := context.Background()

			sec := &controllerv1alpha1.ServiceExportConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceExportConfigName2,
					Namespace: controlPlaneNamespace,
				},
				Spec: controllerv1alpha1.ServiceExportConfigSpec{
					ServiceName:      "another-service",
					ServiceNamespace: "default",
					SourceCluster:    "test-cluster-2",
					SliceName:        "another-slice",
					ServiceDiscoveryPorts: []controllerv1alpha1.ServiceDiscoveryPort{
						{
							Name:            "https",
							Protocol:        "TCP",
							Port:            443,
							ServicePort:     8443,
							ServiceProtocol: "TCP",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sec)).Should(Succeed())

			secLookupKey := types.NamespacedName{
				Name:      serviceExportConfigName2,
				Namespace: controlPlaneNamespace,
			}

			createdSec := &controllerv1alpha1.ServiceExportConfig{}

			By("Deleting the created ServiceExportConfig CR")
			Expect(k8sClient.Delete(ctx, sec)).Should(Succeed())

			By("Looking up the deleted ServiceExportConfig CR")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secLookupKey, createdSec)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
