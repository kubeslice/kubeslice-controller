package e2e

import (
	"context"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SliceConfig E2E tests", func() {
	const (
		sliceConfigName = "e2e-sliceconfig"
		namespace       = "kubeslice-controller"
		timeout         = time.Second * 30
		interval        = time.Millisecond * 500
	)

	var ctx context.Context

	BeforeEach(func() {
		ctx = context.TODO()
	})

	It("should create a SliceConfig successfully", func() {
		By("creating a SliceConfig custom resource")
		sc := &controllerv1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceConfigName,
				Namespace: namespace,
			},
			Spec: controllerv1alpha1.SliceConfigSpec{
				SliceType:                    "Application",
				MaxClusters:                  2,
				OverlayNetworkDeploymentMode: "single-network",
				Clusters:                     []string{"worker-1", "worker-2"},
				NamespaceIsolationProfile: controllerv1alpha1.NamespaceIsolationProfile{
					IsolationEnabled: true,
					ApplicationNamespaces: []controllerv1alpha1.SliceNamespaceSelection{
						{
							Namespace: "test-01",
							Clusters:  []string{"worker-1"},
						},
						{
							Namespace: "test-02",
							Clusters:  []string{"worker-2"},
						},
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, sc)).Should(Succeed())

		By("verifying the SliceConfig exists")
		createdSC := &controllerv1alpha1.SliceConfig{}
		Eventually(func() error {
			return k8sClient.Get(ctx, ObjectKey(namespace, sliceConfigName), createdSC)
		}, timeout, interval).Should(Succeed())

		Expect(createdSC.Spec.SliceType).To(Equal("Application"))
		Expect(createdSC.Spec.MaxClusters).To(Equal(2))
		Expect(string(createdSC.Spec.OverlayNetworkDeploymentMode)).To(Equal("single-network"))
	})

	It("should update SliceConfig successfully", func() {
		By("updating the MaxClusters field")
		updatedSC := &controllerv1alpha1.SliceConfig{}
		Eventually(func() error {
			return k8sClient.Get(ctx, ObjectKey(namespace, sliceConfigName), updatedSC)
		}, timeout, interval).Should(Succeed())

		updatedSC.Spec.MaxClusters = 4
		Expect(k8sClient.Update(ctx, updatedSC)).Should(Succeed())

		By("verifying the updated MaxClusters field")
		Eventually(func() int {
			_ = k8sClient.Get(ctx, ObjectKey(namespace, sliceConfigName), updatedSC)
			return updatedSC.Spec.MaxClusters
		}, timeout, interval).Should(Equal(4))
	})

	It("should delete SliceConfig successfully", func() {
		By("deleting the SliceConfig custom resource")
		deleteSC := &controllerv1alpha1.SliceConfig{}
		Expect(k8sClient.Get(ctx, ObjectKey(namespace, sliceConfigName), deleteSC)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, deleteSC)).Should(Succeed())

		By("verifying the SliceConfig is deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, ObjectKey(namespace, sliceConfigName), deleteSC)
			return err != nil
		}, timeout, interval).Should(BeTrue())
	})
})
