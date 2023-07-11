package controller

import (
	"context"
	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
)

var _ = Describe("Cluster Controller", Ordered, func() {
	Context("With Cluster Created", func() {
		var project *v1alpha1.Project
		os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", controlPlaneNamespace)
		ctx := context.Background()
		project = &v1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "avesha1",
				Namespace: controlPlaneNamespace,
			},
		}

		BeforeAll(func() {
			Expect(k8sClient.Create(ctx, project)).Should(Succeed())
			// check is namespace is created
			ns := v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: "kubeslice-avesha1",
				}, &ns)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
		AfterAll(func() {
			Expect(k8sClient.Delete(ctx, project)).Should(Succeed())
		})

		It("Should pass creating cluster", func() {
			cluster1 := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-1",
					Namespace: "kubeslice-avesha1",
				},
				Spec: v1alpha1.ClusterSpec{
					NodeIPs: []string{"11.11.11.12"},
				},
			}
			Expect(k8sClient.Create(ctx, cluster1)).Should(Succeed())
			getKey := types.NamespacedName{
				Namespace: cluster1.Namespace,
				Name:      cluster1.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster1)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// update cluster status
			cluster1.Status.CniSubnet = []string{"192.168.0.0/24"}
			cluster1.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster1)).Should(Succeed())

			Expect(k8sClient.Delete(ctx, cluster1)).Should(Succeed())
		})

		It("Should fail creating cluster if applied in non-project namespace", func() {
			cluster2 := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-2",
					Namespace: "default",
				},
				Spec: v1alpha1.ClusterSpec{
					NodeIPs: []string{"11.11.11.12"},
				},
			}
			Expect(k8sClient.Create(ctx, cluster2)).ShouldNot(Succeed())
			//get the cluster
			getKey := types.NamespacedName{
				Namespace: cluster2.Namespace,
				Name:      cluster2.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster2)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("Should fail creating cluster if geolocations are invalid", func() {
			cluster3 := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-3",
					Namespace: "kubeslice-avesha1",
				},
				Spec: v1alpha1.ClusterSpec{
					NodeIPs: []string{"11.11.11.12"},
					ClusterProperty: v1alpha1.ClusterProperty{
						GeoLocation: v1alpha1.GeoLocation{
							Latitude:  "120.12345",
							Longitude: "100.12345",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster3)).ShouldNot(Succeed())
			//get the cluster
			getKey := types.NamespacedName{
				Namespace: cluster3.Namespace,
				Name:      cluster3.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster3)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("Should fail creating cluster if nodeIPs are invalid", func() {
			cluster4 := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-4",
					Namespace: "kubeslice-avesha1",
				},
				Spec: v1alpha1.ClusterSpec{
					NodeIPs: []string{"267.0.0.1"},
				},
			}
			Expect(k8sClient.Create(ctx, cluster4)).ShouldNot(Succeed())
			//get the cluster
			getKey := types.NamespacedName{
				Namespace: cluster4.Namespace,
				Name:      cluster4.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster4)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("Should pass deleting a cluster", func() {
			cluster5 := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-5",
					Namespace: "kubeslice-avesha1",
				},
				Spec: v1alpha1.ClusterSpec{
					NodeIPs: []string{"11.11.11.12"},
				},
			}
			Expect(k8sClient.Create(ctx, cluster5)).Should(Succeed())
			getKey := types.NamespacedName{
				Namespace: cluster5.Namespace,
				Name:      cluster5.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster5)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			// update cluster status
			cluster5.Status.CniSubnet = []string{"192.168.0.0/24"}
			cluster5.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster5)).Should(Succeed())

			Expect(k8sClient.Delete(ctx, cluster5)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster5)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
