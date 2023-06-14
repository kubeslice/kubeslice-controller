package controller

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = XDescribe("Cluster controller", func() {
	When("When Creating Cluster CR", func() {
		It("It should pass without errors", func() {
			By("By creating a new Cluster CR")
			ctx := context.Background()
			cluster := &v1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				Spec:       v1alpha1.ClusterSpec{},
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())
			By("By looking up the created Cluster CR")
			clusterLookupKey := types.NamespacedName{Name: "test-cluster", Namespace: "default"}
			createdCluster := &v1alpha1.Cluster{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterLookupKey, createdCluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By calling Cluster Reconciler")
			clusterReconciler := &ClusterReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				Log:            controllerLog.With("name", "Cluster"),
				ClusterService: svc.ClusterService,
				EventRecorder:  &eventRecorder,
			}
			By("By verifying no errors from Cluster Reconciler")
			res, err := clusterReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: clusterLookupKey})
			Expect(err).To(BeNil())
			Expect(res).To(Equal(ctrl.Result{}))
		})
	})
})
