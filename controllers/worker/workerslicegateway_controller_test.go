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
	wsGatewayName1      = "test-wsgateway"
	wsGatewayNamespace1 = "default"
	wsGatewayName2      = "test-wsgateway-delete"
	wsGatewayNamespace2 = "default"
)

var _ = Describe("WorkerSliceGateway Controller", func() {
	When("Creating WorkerSliceGateway CR", func() {
		It("should create the CR and reconcile without errors", func() {
			By("Creating a new WorkerSliceGateway CR")
			ctx := context.Background()

			workerSliceGateway := &workerv1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      wsGatewayName1,
					Namespace: wsGatewayNamespace1,
				},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:               "test-slice",
					GatewayType:             "OpenVPN",
					GatewayHostType:         "Server",
					GatewayConnectivityType: "NodePort",
					GatewayProtocol:         "UDP",
					GatewayCredentials: workerv1alpha1.GatewayCredentials{
						SecretName: "test-secret",
					},
					LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{"10.0.0.1"},
						NodePort:      30001,
						GatewayName:   "local-gateway",
						ClusterName:   "local-cluster",
						VpnIp:         "192.168.1.1",
						GatewaySubnet: "192.168.1.0/24",
					},
					RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
						NodeIps:       []string{"10.0.0.2"},
						NodePort:      30002,
						GatewayName:   "remote-gateway",
						ClusterName:   "remote-cluster",
						VpnIp:         "192.168.2.1",
						GatewaySubnet: "192.168.2.0/24",
					},
					GatewayNumber: 1,
				},
			}

			Expect(k8sClient.Create(ctx, workerSliceGateway)).Should(Succeed())

			By("Looking up the created WorkerSliceGateway CR")
			wsLookupKey := types.NamespacedName{
				Name:      wsGatewayName1,
				Namespace: wsGatewayNamespace1,
			}
			createdWSG := &workerv1alpha1.WorkerSliceGateway{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, wsLookupKey, createdWSG)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying the spec fields match what was set")
			Expect(createdWSG.Spec.SliceName).To(Equal("test-slice"))
			Expect(createdWSG.Spec.LocalGatewayConfig.GatewayName).To(Equal("local-gateway"))
			Expect(createdWSG.Spec.RemoteGatewayConfig.GatewayName).To(Equal("remote-gateway"))
			Expect(createdWSG.Spec.GatewayNumber).To(Equal(1))

			By("Deleting the created WorkerSliceGateway CR")
			Expect(k8sClient.Delete(ctx, createdWSG)).Should(Succeed())

			By("Verifying the CR is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, wsLookupKey, createdWSG)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should delete WorkerSliceGateway CR without errors", func() {
			By("Creating a new WorkerSliceGateway CR for deletion test")
			ctx := context.Background()

			workerSliceGateway := &workerv1alpha1.WorkerSliceGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      wsGatewayName2,
					Namespace: wsGatewayNamespace2,
				},
				Spec: workerv1alpha1.WorkerSliceGatewaySpec{
					SliceName:               "delete-slice",
					GatewayType:             "OpenVPN",
					GatewayHostType:         "Client",
					GatewayConnectivityType: "NodePort",
					GatewayProtocol:         "TCP",
					GatewayNumber:           2,
				},
			}

			Expect(k8sClient.Create(ctx, workerSliceGateway)).Should(Succeed())

			wsLookupKey := types.NamespacedName{
				Name:      wsGatewayName2,
				Namespace: wsGatewayNamespace2,
			}
			createdWSG := &workerv1alpha1.WorkerSliceGateway{}

			By("Deleting the created WorkerSliceGateway CR")
			Expect(k8sClient.Delete(ctx, workerSliceGateway)).Should(Succeed())

			By("Verifying the WorkerSliceGateway CR is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, wsLookupKey, createdWSG)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
