package e2e

import (
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SliceQoSConfig E2E Tests", func() {
	const (
		testNamespace = "default"
		qosName       = "profile1"
		timeout       = time.Second * 10
		interval      = time.Millisecond * 250
	)

	AfterEach(func() {
		By("Cleaning up SliceQoSConfig after each test")
		qos := &controllerv1alpha1.SliceQoSConfig{}
		err := k8sClient.Get(ctx, ObjectKey(testNamespace, qosName), qos)
		if err == nil {
			Expect(k8sClient.Delete(ctx, qos)).To(Succeed())
		}
	})

	It("should create a SliceQoSConfig successfully", func() {
		By("Creating a new SliceQoSConfig")
		qos := &controllerv1alpha1.SliceQoSConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      qosName,
				Namespace: testNamespace,
			},
			Spec: controllerv1alpha1.SliceQoSConfigSpec{
				QueueType:               "HTB",
				Priority:                1,
				TcType:                  "BANDWIDTH_CONTROL",
				BandwidthCeilingKbps:    5120,
				BandwidthGuaranteedKbps: 2562,
				DscpClass:               "AF11",
			},
		}
		Expect(k8sClient.Create(ctx, qos)).To(Succeed())

		By("Verifying the SliceQoSConfig exists")
		Eventually(func() bool {
			fetched := &controllerv1alpha1.SliceQoSConfig{}
			err := k8sClient.Get(ctx, ObjectKey(testNamespace, qosName), fetched)
			return err == nil
		}, timeout, interval).Should(BeTrue())
	})

	It("should update an existing SliceQoSConfig", func() {
		By("Ensuring any previous SliceQoSConfig is fully deleted before re-creating")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, ObjectKey(testNamespace, qosName), &controllerv1alpha1.SliceQoSConfig{})
			return err != nil
		}, timeout*3, interval).Should(BeTrue())

		By("Creating the SliceQoSConfig first")
		qos := &controllerv1alpha1.SliceQoSConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      qosName,
				Namespace: testNamespace,
			},
			Spec: controllerv1alpha1.SliceQoSConfigSpec{
				QueueType:               "HTB",
				Priority:                1,
				TcType:                  "BANDWIDTH_CONTROL",
				BandwidthCeilingKbps:    5120,
				BandwidthGuaranteedKbps: 2562,
				DscpClass:               "AF11",
			},
		}
		Expect(k8sClient.Create(ctx, qos)).To(Succeed())

		By("Updating the Priority field")
		Eventually(func() error {
			fetched := &controllerv1alpha1.SliceQoSConfig{}
			if err := k8sClient.Get(ctx, ObjectKey(testNamespace, qosName), fetched); err != nil {
				return err
			}
			fetched.Spec.Priority = 5
			return k8sClient.Update(ctx, fetched)
		}, timeout, interval).Should(Succeed())

		By("Verifying the updated Priority field")
		Eventually(func() int {
			fetched := &controllerv1alpha1.SliceQoSConfig{}
			_ = k8sClient.Get(ctx, ObjectKey(testNamespace, qosName), fetched)
			return fetched.Spec.Priority
		}, timeout, interval).Should(Equal(5))
	})

	It("should delete an existing SliceQoSConfig", func() {
		By("Ensuring any previous SliceQoSConfig is fully deleted before re-creating")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, ObjectKey(testNamespace, qosName), &controllerv1alpha1.SliceQoSConfig{})
			return err != nil
		}, timeout*3, interval).Should(BeTrue())

		By("Creating the SliceQoSConfig first")
		qos := &controllerv1alpha1.SliceQoSConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      qosName,
				Namespace: testNamespace,
			},
			Spec: controllerv1alpha1.SliceQoSConfigSpec{
				QueueType:               "HTB",
				Priority:                1,
				TcType:                  "BANDWIDTH_CONTROL",
				BandwidthCeilingKbps:    5120,
				BandwidthGuaranteedKbps: 2562,
				DscpClass:               "AF11",
			},
		}
		Expect(k8sClient.Create(ctx, qos)).To(Succeed())

		By("Deleting the SliceQoSConfig")
		Expect(k8sClient.Delete(ctx, qos)).To(Succeed())

		By("Verifying the SliceQoSConfig is deleted")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, ObjectKey(testNamespace, qosName), qos)
			return err != nil
		}, timeout*3, interval).Should(BeTrue())
	})

})
