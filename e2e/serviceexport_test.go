package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ServiceExportConfig Controller", func() {
	Context("When creating a ServiceExportConfig", func() {
		It("Should reconcile and set labels correctly", func() {
			ctx := context.Background()

			sliceName := "red-" + fmt.Sprintf("%d", time.Now().UnixNano())

			slice := &controllerv1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sliceName,
					Namespace: controlPlaneNamespace,
				},
				Spec: controllerv1alpha1.SliceConfigSpec{
					Clusters:    []string{"cluster-1"},
					MaxClusters: 2,
				},
			}
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

			// ServiceExportConfig
			sec := &controllerv1alpha1.ServiceExportConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysql-alpha-" + sliceName + "-cluster-1",
					Namespace: controlPlaneNamespace,
				},
				Spec: controllerv1alpha1.ServiceExportConfigSpec{
					ServiceName:      "mysql",
					ServiceNamespace: "alpha",
					SourceCluster:    "cluster-1",
					SliceName:        sliceName,
					ServiceDiscoveryPorts: []controllerv1alpha1.ServiceDiscoveryPort{
						{
							Name:     "tcp",
							Protocol: "tcp",
							Port:     3306,
						},
					},
					ServiceDiscoveryEndpoints: []controllerv1alpha1.ServiceDiscoveryEndpoint{
						{
							PodName: "mysql-pod-abc",
							NsmIp:   "10.1.1.1",
							DnsName: "mysql." + sliceName + ".slice.local",
							Cluster: "cluster-1",
							Port:    3306,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sec)).Should(Succeed())

			// Verify reconciliation applied labels
			created := &controllerv1alpha1.ServiceExportConfig{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      sec.Name,
					Namespace: controlPlaneNamespace,
				}, created)
				if err != nil {
					return false
				}
				return created.Labels["original-slice-name"] == sliceName &&
					created.Labels["worker-cluster"] == "cluster-1" &&
					created.Labels["service-name"] == "mysql" &&
					created.Labels["service-namespace"] == "alpha"

			}, timeout, interval).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, sec)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
		})
	})
})
