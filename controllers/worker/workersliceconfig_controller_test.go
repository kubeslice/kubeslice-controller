package worker

import (
	"context"
	"time"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

type fakeWorkerSliceService struct{}

func (f *fakeWorkerSliceService) ReconcileWorkerSliceConfig(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (f *fakeWorkerSliceService) ComputeClusterMap(_ []string, _ []workerv1alpha1.WorkerSliceConfig) map[string]int {
	return map[string]int{
		"cluster-1": 1,
		"cluster-2": 2,
	}
}

func (f *fakeWorkerSliceService) DeleteWorkerSliceConfigByLabel(_ context.Context, _ map[string]string, _ string) error {
	return nil
}

func (f *fakeWorkerSliceService) ListWorkerSliceConfigs(_ context.Context, _ map[string]string, _ string) ([]workerv1alpha1.WorkerSliceConfig, error) {
	return []workerv1alpha1.WorkerSliceConfig{}, nil
}

func (f *fakeWorkerSliceService) CreateMinimalWorkerSliceConfig(
	_ context.Context,
	clusters []string,
	_ string,
	_ map[string]string,
	_ string,
	_ string,
	_ string,
	_ map[string]*controllerv1alpha1.SliceGatewayServiceType,
) (map[string]int, error) {
	return map[string]int{"cluster-1": len(clusters)}, nil
}

func (f *fakeWorkerSliceService) CreateMinimalWorkerSliceConfigForNoNetworkSlice(
	_ context.Context,
	_ []string,
	_ string,
	_ map[string]string,
	_ string,
) error {
	return nil
}

var _ = Describe("WorkerSliceConfig Controller", func() {
	const (
		WorkerSliceConfigName = "test-worker-slice-config"
		WorkerSliceConfigNS   = "default"
		timeout               = time.Second * 10
		interval              = time.Millisecond * 250
	)

	Context("When creating a WorkerSliceConfig", func() {
		It("Should successfully reconcile the WorkerSliceConfig resource", func() {
			By("Creating the WorkerSliceConfig resource")
			ctx := context.Background()

			workerSliceConfig := &workerv1alpha1.WorkerSliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WorkerSliceConfigName,
					Namespace: WorkerSliceConfigNS,
				},
				Spec: workerv1alpha1.WorkerSliceConfigSpec{
					SliceName:   "test-slice",
					SliceSubnet: "10.1.0.0/16",
					SliceType:   "Application",
					SliceGatewayProvider: workerv1alpha1.WorkerSliceGatewayProvider{
						SliceGatewayType:        "OpenVPN",
						SliceCaType:             "Local",
						SliceGatewayServiceType: "NodePort",
						SliceGatewayProtocol:    "UDP",
					},
					SliceIpamType: "Local",
					QosProfileDetails: workerv1alpha1.QOSProfile{
						QueueType:               "HTB",
						Priority:                1,
						TcType:                  "tbf",
						BandwidthCeilingKbps:    10000,
						BandwidthGuaranteedKbps: 5000,
						DscpClass:               "AF21",
					},
					NamespaceIsolationProfile: workerv1alpha1.NamespaceIsolationProfile{
						IsolationEnabled:      false,
						ApplicationNamespaces: []string{"app-ns"},
						AllowedNamespaces:     []string{"allowed-ns"},
					},
					IpamClusterOctet:  10,
					ClusterSubnetCIDR: "192.168.0.0/16",
					ExternalGatewayConfig: workerv1alpha1.ExternalGatewayConfig{
						Ingress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: true,
						},
						Egress: workerv1alpha1.ExternalGatewayConfigOptions{
							Enabled: false,
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, workerSliceConfig)).Should(Succeed())

			By("Ensuring the WorkerSliceConfig resource can be retrieved")
			fetched := &workerv1alpha1.WorkerSliceConfig{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      WorkerSliceConfigName,
					Namespace: WorkerSliceConfigNS,
				}, fetched)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(fetched.Spec.SliceName).To(Equal("test-slice"))
			Expect(fetched.Spec.SliceSubnet).To(Equal("10.1.0.0/16"))
		})
	})

	Context("When reconciling an existing WorkerSliceConfig", func() {
		It("Should trigger the reconcile logic without errors", func() {
			ctx := context.Background()
			reconciler := &WorkerSliceConfigReconciler{
				Client:             k8sClient,
				Scheme:             scheme.Scheme,
				WorkerSliceService: &fakeWorkerSliceService{},
				Log:                nil,
				EventRecorder:      nil,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      WorkerSliceConfigName,
					Namespace: WorkerSliceConfigNS,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
