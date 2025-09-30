package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Project E2E", func() {
	const (
		ProjectName      = "cisco"
		ProjectNamespace = "default"
		timeout          = time.Second * 30
		interval         = time.Millisecond * 250
	)

	ctx := context.Background()

	AfterEach(func() {
		// cleanup project after each test
		project := &controllerv1alpha1.Project{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: ProjectName, Namespace: ProjectNamespace}, project)
		if err == nil {
			Expect(k8sClient.Delete(ctx, project)).To(Succeed())
		}
	})

	It("should create a Project successfully", func() {
		By("creating a Project resource")
		project := &controllerv1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ProjectName,
				Namespace: ProjectNamespace,
			},
			Spec: controllerv1alpha1.ProjectSpec{
				ServiceAccount: controllerv1alpha1.ServiceAccount{
					ReadWrite: []string{"john"},
				},
				DefaultSliceCreation: false,
			},
		}

		Expect(k8sClient.Create(ctx, project)).Should(Succeed())

		By("verifying the Project was created")
		created := &controllerv1alpha1.Project{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ProjectName, Namespace: ProjectNamespace}, created)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		Expect(created.Spec.ServiceAccount.ReadWrite).To(ContainElement("john"))
	})

	It("should update the Project with new service accounts", func() {
		By("creating a Project resource")
		project := &controllerv1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ProjectName,
				Namespace: ProjectNamespace,
			},
			Spec: controllerv1alpha1.ProjectSpec{
				ServiceAccount: controllerv1alpha1.ServiceAccount{
					ReadWrite: []string{"john"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, project)).Should(Succeed())

		By("updating the Project with ReadOnly user")
		Eventually(func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ProjectName, Namespace: ProjectNamespace}, project)
			if err != nil {
				return err
			}
			project.Spec.ServiceAccount.ReadOnly = []string{"alice"}
			return k8sClient.Update(ctx, project)
		}, timeout, interval).Should(Succeed())

		By("verifying the Project was updated")
		updated := &controllerv1alpha1.Project{}
		Eventually(func() []string {
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: ProjectName, Namespace: ProjectNamespace}, updated)
			return updated.Spec.ServiceAccount.ReadOnly
		}, timeout, interval).Should(ContainElement("alice"))
	})

	It("should delete a Project successfully", func() {
		By("creating a Project resource")
		project := &controllerv1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ProjectName,
				Namespace: ProjectNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, project)).Should(Succeed())

		By("deleting the Project")
		Expect(k8sClient.Delete(ctx, project)).Should(Succeed())

		By("verifying the Project is deleted")
		deleted := &controllerv1alpha1.Project{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ProjectName, Namespace: ProjectNamespace}, deleted)
			return err != nil
		}, timeout, interval).Should(BeTrue())
	})
})
