package controller

import (
	"context"
	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	projectName      = "avesha"
	projectNamespace = "kubeslice-" + projectName
)

var _ = Describe("Project controller", func() {
	When("When Creating Project CR", func() {
		It("It should pass without errors", func() {
			By("Creating a new Project CR")
			ctx := context.Background()

			project := &v1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: controlPlaneNamespace,
				},
				Spec: v1alpha1.ProjectSpec{
					ServiceAccount: v1alpha1.ServiceAccount{
						ReadWrite: []string{"admin"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, project)).Should(Succeed())

			By("Looking up the created Project CR")
			projectLookupKey := types.NamespacedName{
				Name:      projectName,
				Namespace: controlPlaneNamespace,
			}
			createdProject := &v1alpha1.Project{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, projectLookupKey, createdProject)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Project Namespace")
			nsLookupKey := types.NamespacedName{
				Name: projectNamespace,
			}
			createdNS := &v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, nsLookupKey, createdNS)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Role")
			roleLookupKey := types.NamespacedName{
				Name:      "kubeslice-read-only",
				Namespace: projectNamespace,
			}
			createdRole := &rbacv1.Role{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, roleLookupKey, createdRole)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Role Binding")
			rbLookupKey := types.NamespacedName{
				Name:      "kubeslice-rbac-rw-admin",
				Namespace: projectNamespace,
			}
			createdRB := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, rbLookupKey, createdRB)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Service Account Secret")
			secretLookupKey := types.NamespacedName{
				Name:      "kubeslice-rbac-rw-admin",
				Namespace: projectNamespace,
			}
			createdSecret := &v1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, createdSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Looking up the created Project Service Account")
			saLookupKey := types.NamespacedName{
				Name:      "kubeslice-rbac-rw-admin",
				Namespace: projectNamespace,
			}
			createdSA := &v1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, saLookupKey, createdSA)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Deleting the created Project CR")
			Expect(k8sClient.Delete(ctx, createdProject)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, projectLookupKey, createdProject)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})

		It("It should pass the deletion without errors", func() {
			By("Creating a new Project CR")
			ctx := context.Background()

			project := &v1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: controlPlaneNamespace,
				},
				Spec: v1alpha1.ProjectSpec{
					ServiceAccount: v1alpha1.ServiceAccount{
						ReadWrite: []string{"admin"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, project)).Should(Succeed())

			projectLookupKey := types.NamespacedName{
				Name:      projectName,
				Namespace: projectNamespace,
			}

			createdProject := &v1alpha1.Project{}

			By("Deleting the created Project CR")
			Expect(k8sClient.Delete(ctx, project)).Should(Succeed())

			By("Looking up the Project CR")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, projectLookupKey, createdProject)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
