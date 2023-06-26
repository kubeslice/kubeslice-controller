package controller

import (
	"context"
	"os"
	"time"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	var workerSliceConfig1 *workerv1alpha1.WorkerSliceConfig
	var workerSliceConfig2 *workerv1alpha1.WorkerSliceConfig
	var namespace v1.Namespace
	var octet1 = 1
	var octet = 0
	Context("With Minimal VPNKeyRotationConfig Created", func() {
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
				Clusters:    []string{"worker-1", "worker-2"},
				MaxClusters: 4,
				SliceSubnet: "10.1.0.0/16",
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
				LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
					ClusterName:   "worker-2",
					GatewayName:   sliceName + "-worker-2-worker-1",
					GatewaySubnet: "10.1.64.0/18",
					NodeIps:       []string{"34.86.68.168"},
					VpnIp:         "10.1.255.2",
				},
				RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
					ClusterName:   "worker-1",
					GatewayName:   sliceName + "-worker-1-worker-2",
					GatewaySubnet: "10.1.0.0/18",
					NodeIps:       []string{"20.97.26.147"},
					VpnIp:         "10.1.255.1",
				},
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
				LocalGatewayConfig: workerv1alpha1.SliceGatewayConfig{
					ClusterName:   "worker-1",
					GatewayName:   sliceName + "-worker-1-worker-2",
					GatewaySubnet: "10.1.0.0/18",
					NodeIps:       []string{"20.97.26.147"},
					VpnIp:         "10.1.255.1",
				},
				RemoteGatewayConfig: workerv1alpha1.SliceGatewayConfig{
					ClusterName:   "worker-2",
					GatewayName:   sliceName + "-worker-2-worker-1",
					GatewaySubnet: "10.1.64.0/18",
					NodeIps:       []string{"34.86.68.168"},
					VpnIp:         "10.1.255.2",
				},
			},
		}
		workerSliceConfig1 = &workerv1alpha1.WorkerSliceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "worker-1",
				Namespace: sliceNamespace,
				Labels: map[string]string{
					"worker-cluster":      "worker-1",
					"original-slice-name": sliceName,
				},
			},
			Spec: workerv1alpha1.WorkerSliceConfigSpec{
				Octet: &octet1,
			},
		}

		workerSliceConfig2 = &workerv1alpha1.WorkerSliceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName + "worker-2",
				Namespace: sliceNamespace,
				Labels: map[string]string{
					"worker-cluster":      "worker-2",
					"original-slice-name": sliceName,
				},
			},
			Spec: workerv1alpha1.WorkerSliceConfigSpec{
				Octet: &octet,
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

		It("Should Update VPNKeyRotationConfig with correct cluster gateway mapping", func() {

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
		It("Should update vpnkeyrotationconfig with certificateCreation and Expiry TS", func() {
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

		It("Should recreate/retrigger jobs for cert creation", func() {
			os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", controlPlaneNamespace)
			Expect(k8sClient.Create(ctx, workerSliceConfig1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, workerSliceConfig2)).Should(Succeed())

			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: vpn.Namespace,
				Name:      vpn.Name,
			}, createdVpnKeyConfig)

			Expect(err).To(BeNil())

			// expire it
			time := time.Now().Add(-1 * time.Hour)
			createdVpnKeyConfig.Spec.CertificateExpiryTime = &metav1.Time{Time: time}
			err = k8sClient.Update(ctx, createdVpnKeyConfig)
			Expect(err).To(BeNil())

			// expect new jobs to be created
			job := batchv1.JobList{}
			o := map[string]string{
				"SLICE_NAME": sliceName,
			}
			listOpts := []client.ListOption{
				client.MatchingLabels(
					o,
				),
			}
			Eventually(func() bool {
				err = k8sClient.List(ctx, &job, listOpts...)
				if err != nil {
					return false
				}
				return len(job.Items) > 0
			}, timeout, interval).Should(BeTrue())
		})
		It("Should Update VPNKey Rotation Config in case a new cluster is added", func() {
			// update sliceconfig
			slice.Spec.Clusters = append(slice.Spec.Clusters, "worker-3")
			Expect(k8sClient.Update(ctx, slice)).Should(Succeed())
			// NOTE:since slice reconciler is not present in ITs yet, manually update vpnkeyrotation CR
			// update vpnsliceconfig
			vpn.Spec.Clusters = append(vpn.Spec.Clusters, "worker-3")
			Expect(k8sClient.Update(ctx, vpn)).Should(Succeed())
			// should update cluster-mapping

		})
	})
})
