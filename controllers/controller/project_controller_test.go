package controller

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Project controller", func() {
	When("When Creating Project CR", func() {
		It("It should pass without errors", func() {
			By("By creating a new Project CR")
			ctx := context.Background()
			Project := &v1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: "avesha", Namespace: "kubeslice-controller"},
				Spec: v1alpha1.ProjectSpec{
					ServiceAccount: v1alpha1.ServiceAccount{
						ReadWrite: []string{"admin"},
					},
				},
			}
			Namespace := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "kubeslice-controller"},
			}
			Expect(k8sClient.Create(ctx, Namespace)).Should(Succeed())

			Expect(k8sClient.Create(ctx, Project)).Should(Succeed())
			By("By looking up the created Project CR")
			projectLookupKey := types.NamespacedName{Name: "avesha", Namespace: "kubeslice-controller"}
			createdProject := &v1alpha1.Project{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, projectLookupKey, createdProject)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			//By("By calling Project Reconcile")
			//projectReconcile := &ProjectReconciler{
			//	Client:         k8sClient,
			//	Scheme:         k8sClient.Scheme(),
			//	Log:            controllerLog.With("name", "Project"),
			//	ProjectService: svc.ProjectService,
			//	EventRecorder:  &eventRecorder,
			//}
			//By("By verifying no errors from Project Reconcile")
			////service.ProjectNamespacePrefix = "kubeslice"
			//res, err := projectReconcile.Reconcile(ctx, ctrl.Request{NamespacedName: projectLookupKey})
			//Expect(err).To(BeNil())
			//Expect(res).To(Equal(ctrl.Result{}))
		})
	})
})
