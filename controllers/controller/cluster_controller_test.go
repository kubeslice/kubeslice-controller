package controller

import (
	"context"
	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var (
	clusterName = "cluster1"
)

var _ = Describe("Cluster controller", func() {
	When("When Creating Cluster CR", func() {
		It("It should pass without errors", func() {
			By("Creating a new Cluster CR")
			ctx := context.Background()

			cluster := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: projectNamespace,
				},
				Spec: v1alpha1.ClusterSpec{
					NodeIPs: []string{
						"11.11.14.114",
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			By("Waiting for the cluster to be created")
			time.Sleep(10 * time.Second)

			By("Looking up the created cluster CR")
			clusterLookupKey := types.NamespacedName{
				Name:      clusterName,
				Namespace: controlPlaneNamespace,
			}
			createdCluster := &v1alpha1.Cluster{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterLookupKey, createdCluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
