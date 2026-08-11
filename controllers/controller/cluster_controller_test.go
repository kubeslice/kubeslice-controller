package controller

import (
	"context"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	clusterName1      = "cluster-avesha"
	clusterNamespace1 = "kubeslice-" + clusterName1
	clusterName2      = "cluster-demo"
	clusterNamespace2 = "kubeslice-" + clusterName2
)

var _ = Describe("Cluster controller", func() {
	When("Creating Cluster CR", func() {
		It("should create Cluster CR and related resources without errors", func() {
			By("Creating a new Cluster CR")
			ctx := context.Background()

			cluster := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName1,
					Namespace: controlPlaneNamespace,
				},
				Spec: v1alpha1.ClusterSpec{
					ClusterProperty: v1alpha1.ClusterProperty{
						Telemetry: v1alpha1.Telemetry{
							TelemetryProvider: "test-property",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			By("Looking up the created Cluster CR")
			clusterLookupKey := types.NamespacedName{
				Name:      clusterName1,
				Namespace: controlPlaneNamespace,
			}
			createdCluster := &v1alpha1.Cluster{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterLookupKey, createdCluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Cluster Namespace")
			nsLookupKey := types.NamespacedName{
				Name: clusterNamespace1,
			}
			createdNS := &v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, nsLookupKey, createdNS)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Cluster Role")
			roleLookupKey := types.NamespacedName{
				Name:      "kubeslice-cluster-role",
				Namespace: clusterNamespace1,
			}
			createdRole := &rbacv1.Role{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleLookupKey, createdRole)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Cluster Role Binding")
			rbLookupKey := types.NamespacedName{
				Name:      "kubeslice-cluster-rolebinding",
				Namespace: clusterNamespace1,
			}
			createdRB := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rbLookupKey, createdRB)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Cluster Service Account")
			saLookupKey := types.NamespacedName{
				Name:      "kubeslice-cluster-sa",
				Namespace: clusterNamespace1,
			}
			createdSA := &v1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saLookupKey, createdSA)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Deleting the created Cluster CR")
			Expect(k8sClient.Delete(ctx, createdCluster)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterLookupKey, createdCluster)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("should handle deletion of a Cluster CR gracefully", func() {
			By("Creating a new Cluster CR")
			ctx := context.Background()

			cluster := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName2,
					Namespace: controlPlaneNamespace,
				},
				Spec: v1alpha1.ClusterSpec{
					ClusterProperty: v1alpha1.ClusterProperty{
						Telemetry: v1alpha1.Telemetry{
							TelemetryProvider: "test-property-2",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			clusterLookupKey := types.NamespacedName{
				Name:      clusterName2,
				Namespace: controlPlaneNamespace,
			}

			createdCluster := &v1alpha1.Cluster{}

			By("Deleting the created Cluster CR")
			Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())

			By("Looking up the deleted Cluster CR")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterLookupKey, createdCluster)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
