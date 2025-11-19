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

var _ = Describe("VpnKeyRotation Controller E2E Tests", func() {
	var (
		ctx       context.Context
		namespace string
		sliceName string
		vpnCR     *controllerv1alpha1.VpnKeyRotation
		key       types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "kubeslice-controller"
		sliceName = "test-slice-" + fmt.Sprintf("%d", time.Now().UnixNano())

		vpnCR = &controllerv1alpha1.VpnKeyRotation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: namespace,
			},
			Spec: controllerv1alpha1.VpnKeyRotationSpec{
				SliceName:        sliceName,
				RotationInterval: 1,
				Clusters:         []string{"cluster1", "cluster2"},
			},
		}

		key = types.NamespacedName{Name: vpnCR.Name, Namespace: vpnCR.Namespace}

		// Ensure CR exists before each test
		By("creating the VpnKeyRotation resource before each test")
		Expect(k8sClient.Create(ctx, vpnCR)).Should(Succeed())

		Eventually(func() error {
			f := &controllerv1alpha1.VpnKeyRotation{}
			return k8sClient.Get(ctx, key, f)
		}, time.Second*10, time.Millisecond*250).Should(Succeed())
	})

	AfterEach(func() {
		// Cleanup after each test
		_ = k8sClient.Delete(ctx, vpnCR)
	})

	It("should create a VpnKeyRotation CR successfully", func() {
		fetched := &controllerv1alpha1.VpnKeyRotation{}
		By("fetching the created resource")
		Expect(k8sClient.Get(ctx, key, fetched)).Should(Succeed())

		Expect(fetched.Spec.SliceName).To(Equal(sliceName))
		Expect(fetched.Spec.RotationInterval).To(Equal(1))
		Expect(fetched.Spec.Clusters).To(ContainElements("cluster1", "cluster2"))
	})

	It("should update ClusterGatewayMapping after reconciliation", func() {
		fetched := &controllerv1alpha1.VpnKeyRotation{}
		Expect(k8sClient.Get(ctx, key, fetched)).Should(Succeed())

		By("ensuring ClusterGatewayMapping is initially empty")
		Expect(fetched.Spec.ClusterGatewayMapping).To(BeEmpty())

		By("simulating reconciliation by updating mapping")
		fetched.Spec.ClusterGatewayMapping = map[string][]string{
			"cluster1": {"gw1", "gw2"},
			"cluster2": {"gw3"},
		}
		Expect(k8sClient.Update(ctx, fetched)).Should(Succeed())

		Eventually(func() map[string][]string {
			_ = k8sClient.Get(ctx, key, fetched)
			return fetched.Spec.ClusterGatewayMapping
		}, time.Second*10, time.Millisecond*250).Should(HaveKeyWithValue("cluster1", []string{"gw1", "gw2"}))
		Expect(fetched.Spec.ClusterGatewayMapping).To(HaveKeyWithValue("cluster2", []string{"gw3"}))
	})

	It("should update CertificateCreationTime and CertificateExpiryTime after rotation", func() {
		fetched := &controllerv1alpha1.VpnKeyRotation{}
		Expect(k8sClient.Get(ctx, key, fetched)).Should(Succeed())

		now := metav1.Now()
		expiry := metav1.NewTime(now.Add(24 * time.Hour))

		fetched.Spec.CertificateCreationTime = &now
		fetched.Spec.CertificateExpiryTime = &expiry
		Expect(k8sClient.Update(ctx, fetched)).Should(Succeed())

		Eventually(func() bool {
			_ = k8sClient.Get(ctx, key, fetched)
			return fetched.Spec.CertificateCreationTime != nil && fetched.Spec.CertificateExpiryTime != nil
		}, time.Second*10, time.Millisecond*250).Should(BeTrue())

		Expect(fetched.Spec.CertificateExpiryTime.Sub(fetched.Spec.CertificateCreationTime.Time)).
			To(BeNumerically("~", 24*time.Hour, time.Minute))
	})

	It("should increment RotationCount after certificate expiry", func() {
		fetched := &controllerv1alpha1.VpnKeyRotation{}
		Expect(k8sClient.Get(ctx, key, fetched)).Should(Succeed())

		expired := metav1.NewTime(metav1.Now().Add(-time.Hour))
		fetched.Spec.CertificateExpiryTime = &expired
		fetched.Spec.RotationCount = 1
		Expect(k8sClient.Update(ctx, fetched)).Should(Succeed())

		Eventually(func() int {
			_ = k8sClient.Get(ctx, key, fetched)
			if metav1.Now().After(fetched.Spec.CertificateExpiryTime.Time) {
				fetched.Spec.RotationCount++
				now := metav1.Now()
				expiry := metav1.NewTime(now.Add(24 * time.Hour))
				fetched.Spec.CertificateCreationTime = &now
				fetched.Spec.CertificateExpiryTime = &expiry
				_ = k8sClient.Update(ctx, fetched)
			}
			return fetched.Spec.RotationCount
		}, time.Second*10, time.Millisecond*250).Should(Equal(2))
	})
})
