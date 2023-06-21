package controller

import (
	"context"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	sliceName      = "test-slice"
	sliceNamespace = "test-ns"
)

var _ = Describe("VpnKeyRoation Controller", Ordered, func() {
	var vpn *v1alpha1.VpnKeyRotation
	var slice *v1alpha1.SliceConfig
	var clientGw *workerv1alpha1.WorkerSliceGateway
	var serverGw *workerv1alpha1.WorkerSliceGateway
	var namespace v1.Namespace
	Context("With Minimal VPNKeyRotaionConfig Created", func() {
		ctx := context.Background()
		vpn = &v1alpha1.VpnKeyRotation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: sliceNamespace,
			},
			Spec: v1alpha1.VpnKeyRotationSpec{
				Clusters:         []string{"worker-1", "worker-2"},
				RotationInterval: 30,
				SliceName:        sliceName,
			},
		}
		slice = &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: sliceNamespace,
			},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters: []string{"worker-1", "worker-2"},
			},
		}
		clientGw = &workerv1alpha1.WorkerSliceGateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-worker-2-worker-1",
				Namespace: sliceNamespace,
				Labels: map[string]string{
					"worker-cluster":      "worker-2",
					"original-slice-name": sliceName,
				},
			},
			Spec: workerv1alpha1.WorkerSliceGatewaySpec{
				GatewayHostType: "Client",
			},
		}
		serverGw = &workerv1alpha1.WorkerSliceGateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "-worker-1-worker-2",
				Namespace: sliceNamespace,
				Labels: map[string]string{
					"worker-cluster":      "worker-1",
					"original-slice-name": sliceName,
				},
			},
			Spec: workerv1alpha1.WorkerSliceGatewaySpec{
				GatewayHostType: "Server",
			},
		}
		namespace = v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns",
			},
		}
		BeforeAll(func() {
			Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
		})

		It("Should Update VPNKeyRotaionConfig with correct cluster gateway mapping", func() {

			// it should create sliceconfig
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			// it should create workerslicegateways
			Expect(k8sClient.Create(ctx, serverGw)).Should(Succeed())
			Expect(k8sClient.Create(ctx, clientGw)).Should(Succeed())
			// it should create vpnkeyrotation config
			Expect(k8sClient.Create(ctx, vpn)).Should(Succeed())

			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey := types.NamespacedName{
				Namespace: vpn.Namespace,
				Name:      vpn.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				Expect(err).To(BeNil())
				return createdVpnKeyConfig.Spec.ClusterGatewayMapping != nil
			}, timeout, interval).Should(BeTrue())

			expectedMap := map[string][]string{
				"worker-1": {sliceName + "-worker-1-worker-2"},
				"worker-2": {sliceName + "-worker-2-worker-1"},
			}

			// check if map is contructed correctly
			Expect(createdVpnKeyConfig.Spec.ClusterGatewayMapping).To(Equal(expectedMap))
		})
		It("Should update vpnkeyrotaionconfig with certificateCreation and Expiry TS", func() {
			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey := types.NamespacedName{
				Namespace: vpn.Namespace,
				Name:      vpn.Name,
			}
			// check if creation TS is not zero
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				Expect(err).To(BeNil())
				return !createdVpnKeyConfig.Spec.CertificateCreationTime.IsZero()
			}, timeout, interval).Should(BeTrue())

			// check if expiry TS is not zero
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				Expect(err).To(BeNil())
				return !createdVpnKeyConfig.Spec.CertificateExpiryTime.IsZero()
			}, timeout, interval).Should(BeTrue())

		})

	})
})
