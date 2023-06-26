package controller

import (
	"context"
	"os"
	"time"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
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

var _ = Describe("VpnKeyRotation Controller", Ordered, func() {
	var slice *v1alpha1.SliceConfig
	var cluster1 *v1alpha1.Cluster
	var cluster2 *v1alpha1.Cluster
	var cluster3 *v1alpha1.Cluster
	var namespace v1.Namespace
	Context("With Minimal SliceConfig Created", func() {
		ctx := context.Background()
		slice = &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: sliceNamespace,
			},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:    []string{"worker-1", "worker-2"},
				MaxClusters: 4,
				SliceSubnet: "10.1.0.0/16",
				SliceGatewayProvider: v1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: "OpenVPN",
					SliceCaType:      "Local",
				},
				SliceIpamType: "Local",
				SliceType:     "Application",
			},
		}
		namespace = v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns",
				Labels: map[string]string{
					"kubeslice-controller-resource-name": "Project-test-ns",
				},
			},
		}
		cluster1 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-1",
				Namespace: "test-ns",
			},
		}
		cluster2 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-2",
				Namespace: "test-ns",
			},
		}
		cluster3 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-3",
				Namespace: "test-ns",
			},
		}
		BeforeAll(func() {
			Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, cluster3)).Should(Succeed())
			// it should create sliceconfig
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
		})
		It("Should Fail Creating SliceConfig in case rotation interval validation fails", func() {
			s := slice.DeepCopy()
			// RotationInterval > 90
			// Expected Error:
			//{
			// 	Type: "FieldValueInvalid",
			// 	Message: "Invalid value: 90: spec.rotationInterval in body should be less than or equal to 90",
			// 	Field: "spec.rotationInterval",
			// },
			s.Spec.RotationInterval = 100
			Expect(k8sClient.Create(ctx, s)).Should(Not(Succeed()))

			// RotationInterval < 30
			s.Spec.RotationInterval = 20
			// Expected Error:
			// {
			// 	Type: "FieldValueInvalid",
			// 	Message: "Invalid value: 30: spec.rotationInterval in body should be greater than or equal to 30",
			// 	Field: "spec.rotationInterval",
			// },
			Expect(k8sClient.Create(ctx, s)).Should(Not(Succeed()))
		})
		It("Should create slice with default rotation interval=30days if not specified", func() {
			createdSliceConfig := &v1alpha1.SliceConfig{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}, createdSliceConfig)

			Expect(err).To(BeNil())
			Expect(createdSliceConfig.Spec.RotationInterval).Should(Equal(30))
		})
		It("Should Create VPNKeyRotationConfig Once Slice Is Created", func() {
			// it should create vpnkeyrotationconfig
			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey := types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
		It("Should Update VpnKeyRotationConfig with correct clusters(2) gateway mapping", func() {
			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey := types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
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
				Namespace: slice.Namespace,
				Name:      slice.Name,
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

		It("Should recreate/retrigger jobs for cert creation once it expires", func() {
			os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", controlPlaneNamespace)

			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
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
		// cluster onboarding tests
		It("Should Update VPNKeyRotation Config in case a new cluster is added", func() {
			// update sliceconfig
			createdSliceConfig := &v1alpha1.SliceConfig{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}, createdSliceConfig)

			Expect(err).To(BeNil())
			createdSliceConfig.Spec.Clusters = append(createdSliceConfig.Spec.Clusters, "worker-3")
			Expect(k8sClient.Update(ctx, createdSliceConfig)).Should(Succeed())

			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}

			Eventually(func() []string {
				err = k8sClient.Get(ctx, types.NamespacedName{
					Namespace: slice.Namespace,
					Name:      slice.Name,
				}, createdVpnKeyConfig)

				if err != nil {
					return []string{""}
				}
				return createdVpnKeyConfig.Spec.Clusters
			}, timeout, interval).Should(Equal([]string{"worker-1", "worker-2", "worker-3"}))
		})
		It("Should Update Cluster(3) Gateway Mapping", func() {
			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey := types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				Expect(err).To(BeNil())
				return createdVpnKeyConfig.Spec.ClusterGatewayMapping != nil
			}, timeout, interval).Should(BeTrue())

			// the length of map should be 3
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				Expect(err).To(BeNil())
				return len(createdVpnKeyConfig.Spec.ClusterGatewayMapping) == 3
			}, timeout, interval).Should(BeTrue())
		})

		It("Should Update VPNKey Rotation Config in case a cluster is de-boarded", func() {
			// update sliceconfig
			createdSliceConfig := &v1alpha1.SliceConfig{}

			err := k8sClient.Get(ctx, types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}, createdSliceConfig)

			Expect(err).To(BeNil())
			createdSliceConfig.Spec.Clusters = []string{"worker-1", "worker-2"}
			Expect(k8sClient.Update(ctx, createdSliceConfig)).Should(Succeed())

			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}

			Eventually(func() []string {
				err = k8sClient.Get(ctx, types.NamespacedName{
					Namespace: slice.Namespace,
					Name:      slice.Name,
				}, createdVpnKeyConfig)

				if err != nil {
					return []string{""}
				}
				return createdVpnKeyConfig.Spec.Clusters
			}, timeout, interval).Should(Equal([]string{"worker-1", "worker-2"}))
		})
		It("Should Update Cluster(2) Gateway Mapping", func() {
			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey := types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				Expect(err).To(BeNil())
				return createdVpnKeyConfig.Spec.ClusterGatewayMapping != nil
			}, timeout, interval).Should(BeTrue())

			// the length of map should be 2
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				Expect(err).To(BeNil())
				return len(createdVpnKeyConfig.Spec.ClusterGatewayMapping) == 2
			}, timeout, interval).Should(BeTrue())
		})
	})
})
