package worker

import (
	"context"

	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	workerServiceImportName1 = "workerserviceimport-test-1"
	workerServiceImportName2 = "workerserviceimport-test-2"
)

var _ = Describe("WorkerServiceImport controller", func() {
	When("Creating WorkerServiceImport CR", func() {
		It("should create and delete WorkerServiceImport CR without errors", func() {
			By("Creating a new WorkerServiceImport CR")
			ctx := context.Background()

			expectedSpec := workerv1alpha1.WorkerServiceImportSpec{
				ServiceName:      "test-service",
				ServiceNamespace: "default",
				SourceClusters:   []string{"cluster-a", "cluster-b"},
				SliceName:        "test-slice",
				ServiceDiscoveryEndpoints: []workerv1alpha1.ServiceDiscoveryEndpoint{
					{
						PodName: "pod-1",
						Cluster: "cluster-a",
						NsmIp:   "10.0.0.1",
						DnsName: "service.slice.local",
						Port:    8080,
					},
				},
				ServiceDiscoveryPorts: []workerv1alpha1.ServiceDiscoveryPort{
					{
						Name:            "http",
						Port:            8080,
						Protocol:        "TCP",
						ServicePort:     80,
						ServiceProtocol: "TCP",
					},
				},
				Aliases: []string{"alias1", "alias2"},
			}

			workerServiceImport := &workerv1alpha1.WorkerServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workerServiceImportName1,
					Namespace: controlPlaneNamespace,
				},
				Spec: expectedSpec,
			}
			Expect(k8sClient.Create(ctx, workerServiceImport)).Should(Succeed())

			By("Looking up the created WorkerServiceImport CR")
			lookupKey := types.NamespacedName{
				Name:      workerServiceImportName1,
				Namespace: controlPlaneNamespace,
			}
			createdObj := &workerv1alpha1.WorkerServiceImport{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdObj)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the created CR has the expected spec values")
			Expect(createdObj.Spec).To(Equal(expectedSpec))

			By("Deleting the created WorkerServiceImport CR")
			Expect(k8sClient.Delete(ctx, createdObj)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdObj)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should handle deletion of a WorkerServiceImport CR gracefully", func() {
			By("Creating a new WorkerServiceImport CR")
			ctx := context.Background()

			expectedSpec := workerv1alpha1.WorkerServiceImportSpec{
				ServiceName:      "test-service-2",
				ServiceNamespace: "default",
				SourceClusters:   []string{"cluster-x"},
				SliceName:        "slice-2",
				ServiceDiscoveryEndpoints: []workerv1alpha1.ServiceDiscoveryEndpoint{
					{
						PodName: "pod-2",
						Cluster: "cluster-x",
						NsmIp:   "10.0.0.2",
						DnsName: "svc2.slice.local",
						Port:    9090,
					},
				},
				ServiceDiscoveryPorts: []workerv1alpha1.ServiceDiscoveryPort{
					{
						Name:            "grpc",
						Port:            9090,
						Protocol:        "TCP",
						ServicePort:     90,
						ServiceProtocol: "TCP",
					},
				},
				Aliases: []string{"alias-x"},
			}

			workerServiceImport := &workerv1alpha1.WorkerServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workerServiceImportName2,
					Namespace: controlPlaneNamespace,
				},
				Spec: expectedSpec,
			}
			Expect(k8sClient.Create(ctx, workerServiceImport)).Should(Succeed())

			lookupKey := types.NamespacedName{
				Name:      workerServiceImportName2,
				Namespace: controlPlaneNamespace,
			}
			createdObj := &workerv1alpha1.WorkerServiceImport{}

			By("Looking up the created CR and verifying spec values")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdObj)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdObj.Spec).To(Equal(expectedSpec))

			By("Deleting the created WorkerServiceImport CR")
			Expect(k8sClient.Delete(ctx, workerServiceImport)).Should(Succeed())

			By("Ensuring the CR is actually deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdObj)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
