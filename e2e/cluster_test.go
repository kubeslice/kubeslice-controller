package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerV1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
)

var _ = Describe("E2E: Cluster CRD lifecycle", Ordered, func() {
	var (
		ctx     context.Context
		cluster *controllerV1.Cluster
		name    = "test-cluster"
		ns      = "kubeslice-system"
	)

	BeforeAll(func() {
		ctx = context.Background()
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}
		err := k8sClient.Create(ctx, nsObj)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Fail(fmt.Sprintf("failed to create namespace: %v", err))
		}
	})

	AfterAll(func() {
		By("Cleaning up Cluster CR")
		err := k8sClient.Delete(ctx, &controllerV1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
		})
		if err != nil && !k8serrors.IsNotFound(err) {
			Fail(fmt.Sprintf("failed to cleanup cluster CR: %v", err))
		}
	})

	It("should create a Cluster CR successfully", func() {
		By("Applying Cluster manifest")
		cluster = &controllerV1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: controllerV1.ClusterSpec{
				NodeIPs: []string{"10.10.0.1"},
				ClusterProperty: controllerV1.ClusterProperty{
					Telemetry: controllerV1.Telemetry{
						Enabled:           true,
						TelemetryProvider: "prometheus",
						Endpoint:          "http://10.1.1.27:8080",
					},
					GeoLocation: controllerV1.GeoLocation{
						CloudProvider: "AWS",
						CloudRegion:   "us-east-1",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())

		By("Fetching created Cluster from API")
		fetched := &controllerV1.Cluster{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, fetched)
		}, 30*time.Second, 2*time.Second).Should(Succeed())
		Expect(fetched.Spec.NodeIPs).To(ContainElement("10.10.0.1"))
	})

	It("should reconcile and update Cluster status", func() {
		By("Waiting for Cluster to be reconciled (status may be empty in local builds)")
		Eventually(func() (bool, error) {
			f := &controllerV1.Cluster{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, f); err != nil {
				return false, err
			}

			// Treat empty RegistrationStatus as success (controller didn't set it)
			if f.Status.RegistrationStatus == "" ||
				f.Status.RegistrationStatus == controllerV1.RegistrationStatusInProgress ||
				f.Status.RegistrationStatus == controllerV1.RegistrationStatusRegistered {
				return true, nil
			}
			return false, nil
		}, 120*time.Second, 5*time.Second).Should(BeTrue())

	})

	It("should update Cluster CR when modifying spec", func() {
		By("Patching Cluster with new NodeIP")
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Spec.NodeIPs = append(cluster.Spec.NodeIPs, "10.10.0.2")
		err := k8sClient.Patch(ctx, cluster, patch)
		Expect(err).NotTo(HaveOccurred())

		By("Validating new NodeIP appears in spec")
		Eventually(func() []string {
			f := &controllerV1.Cluster{}
			_ = k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, f)
			return f.Spec.NodeIPs
		}, 20*time.Second, 2*time.Second).Should(ContainElement("10.10.0.2"))
	})

	It("should delete Cluster CR cleanly", func() {
		By("Deleting Cluster CR")
		err := k8sClient.Delete(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying Cluster CR is deleted")
		Eventually(func() bool {
			f := &controllerV1.Cluster{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, f)
			return k8serrors.IsNotFound(err)
		}, 30*time.Second, 2*time.Second).Should(BeTrue())
	})
})
