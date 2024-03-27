package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/events"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("VpnKeyRotation Controller", Ordered, func() {
	const (
		sliceName      = "test-slice"
		sliceNamespace = "kubeslice-cisco"
	)
	Context("With Minimal SliceConfig Created", func() {
		var project *v1alpha1.Project
		var slice *v1alpha1.SliceConfig
		var cluster1 *v1alpha1.Cluster
		var cluster2 *v1alpha1.Cluster
		var cluster3 *v1alpha1.Cluster
		os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", controlPlaneNamespace)
		ctx := context.Background()
		project = &v1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cisco",
				Namespace: controlPlaneNamespace,
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
				SliceGatewayProvider: &v1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: "OpenVPN",
					SliceCaType:      "Local",
				},
				SliceIpamType: "Local",
				SliceType:     "Application",
				QosProfileDetails: &v1alpha1.QOSProfile{
					BandwidthCeilingKbps: 5120,
					DscpClass:            "AF11",
				},
			},
		}
		cluster1 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-1",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.12"},
			},
		}
		cluster2 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-2",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.13"},
			},
		}
		cluster3 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-3",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.14"},
			},
		}
		BeforeAll(func() {
			Expect(k8sClient.Create(ctx, project)).Should(Succeed())
			// check is namespace is created
			ns := v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: "kubeslice-cisco",
				}, &ns)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Create(ctx, cluster1)).Should(Succeed())
			// update cluster status
			getKey := types.NamespacedName{
				Namespace: cluster1.Namespace,
				Name:      cluster1.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster1)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster1.Status.CniSubnet = []string{"192.168.0.0/24"}
			cluster1.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster1)).Should(Succeed())

			Expect(k8sClient.Create(ctx, cluster2)).Should(Succeed())
			// update cluster status
			getKey = types.NamespacedName{
				Namespace: cluster2.Namespace,
				Name:      cluster2.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster2)
				if err != nil {
					return false
				}
				cluster2.Status.CniSubnet = []string{"192.168.1.0/24"}
				cluster2.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
				err = k8sClient.Status().Update(ctx, cluster2)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Create(ctx, cluster3)).Should(Succeed())
			// update cluster status
			getKey = types.NamespacedName{
				Namespace: cluster3.Namespace,
				Name:      cluster3.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster3)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster3.Status.CniSubnet = []string{"192.168.2.0/24"}
			cluster3.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			cluster3.Status.ClusterHealth = &v1alpha1.ClusterHealth{
				ClusterHealthStatus: v1alpha1.ClusterHealthStatusNormal,
				LastUpdated:         metav1.Now(),
			}
			Expect(k8sClient.Status().Update(ctx, cluster3)).Should(Succeed())

			// it should create sliceconfig
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
		})
		AfterAll(func() {
			// update sliceconfig tor remove clusters
			createdSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}
			Expect(k8sClient.Get(ctx, getKey, &createdSliceConfig)).Should(Succeed())
			createdSliceConfig.Spec.Clusters = []string{}
			Expect(k8sClient.Update(ctx, &createdSliceConfig)).Should(Succeed())
			// wait till workersliceconfigs are deleted
			workerSliceConfigList := workerv1alpha1.WorkerSliceConfigList{}
			ls := map[string]string{
				"original-slice-name": slice.Name,
			}
			listOpts := []client.ListOption{
				client.MatchingLabels(ls),
			}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &workerSliceConfigList, listOpts...)
				if err != nil {
					return false
				}
				return len(workerSliceConfigList.Items) == 0
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Delete(ctx, cluster1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cluster2)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cluster3)).Should(Succeed())
			// it should create sliceconfig
			Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			getKey = types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, slice)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
		It("Should Fail Creating SliceConfig in case rotation interval validation fails", func() {
			s := &v1alpha1.SliceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice-1",
					Namespace: sliceNamespace,
				},
				Spec: v1alpha1.SliceConfigSpec{
					Clusters:    []string{"worker-1", "worker-2"},
					MaxClusters: 4,
					SliceSubnet: "10.1.0.0/16",
					SliceGatewayProvider: &v1alpha1.WorkerSliceGatewayProvider{
						SliceGatewayType: "OpenVPN",
						SliceCaType:      "Local",
					},
					SliceIpamType: "Local",
					SliceType:     "Application",
				},
			}
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
				if err != nil {
					return false
				}
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
				if err != nil {
					return false
				}
				return !createdVpnKeyConfig.Spec.CertificateCreationTime.IsZero()
			}, timeout, interval).Should(BeTrue())

			// check if expiry TS is not zero
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				if err != nil {
					return false
				}
				return !createdVpnKeyConfig.Spec.CertificateExpiryTime.IsZero()
			}, timeout, interval).Should(BeTrue())
		})

		It("Should recreate/retrigger jobs for cert creation once it expires", func() {
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
				if err != nil {
					return false
				}
				return createdVpnKeyConfig.Spec.ClusterGatewayMapping != nil
			}, timeout, interval).Should(BeTrue())

			// the length of map should be 3
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				if err != nil {
					return false
				}
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
				if err != nil {
					return false
				}
				return createdVpnKeyConfig.Spec.ClusterGatewayMapping != nil
			}, timeout, interval).Should(BeTrue())

			// the length of map should be 2
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				if err != nil {
					return false
				}
				return len(createdVpnKeyConfig.Spec.ClusterGatewayMapping) == 2
			}, timeout, interval).Should(BeTrue())
		})
	})
	Context("Webhook Tests", func() {
		var slice *v1alpha1.SliceConfig
		var cluster1 *v1alpha1.Cluster
		var cluster2 *v1alpha1.Cluster
		var cluster3 *v1alpha1.Cluster
		os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", controlPlaneNamespace)
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
				SliceGatewayProvider: &v1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: "OpenVPN",
					SliceCaType:      "Local",
				},
				SliceIpamType: "Local",
				SliceType:     "Application",
				QosProfileDetails: &v1alpha1.QOSProfile{
					BandwidthCeilingKbps: 5120,
					DscpClass:            "AF11",
				},
			},
		}
		cluster1 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-1",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.12"},
			},
		}
		cluster2 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-2",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.13"},
			},
		}
		cluster3 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-3",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.14"},
			},
		}
		BeforeAll(func() {
			ns := v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: "kubeslice-cisco",
				}, &ns)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Create(ctx, cluster1)).Should(Succeed())
			// update cluster status
			getKey := types.NamespacedName{
				Namespace: cluster1.Namespace,
				Name:      cluster1.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster1)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster1.Status.CniSubnet = []string{"192.168.0.0/24"}
			cluster1.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster1)).Should(Succeed())

			Expect(k8sClient.Create(ctx, cluster2)).Should(Succeed())
			// update cluster status
			getKey = types.NamespacedName{
				Namespace: cluster2.Namespace,
				Name:      cluster2.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster2)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster2.Status.CniSubnet = []string{"192.168.1.0/24"}
			cluster2.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster2)).Should(Succeed())

			Expect(k8sClient.Create(ctx, cluster3)).Should(Succeed())
			// update cluster status
			getKey = types.NamespacedName{
				Namespace: cluster3.Namespace,
				Name:      cluster3.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster3)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster3.Status.CniSubnet = []string{"10.1.1.1/16"}
			cluster3.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster3)).Should(Succeed())

			// it should create sliceconfig
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		})
		AfterAll(func() {
			// update sliceconfig tor remove clusters
			createdSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Namespace: sliceNamespace,
				Name:      sliceName,
			}
			Expect(k8sClient.Get(ctx, getKey, &createdSliceConfig)).Should(Succeed())
			createdSliceConfig.Spec.Clusters = []string{}
			Expect(k8sClient.Update(ctx, &createdSliceConfig)).Should(Succeed())
			// wait till workersliceconfigs are deleted
			workerSliceConfigList := workerv1alpha1.WorkerSliceConfigList{}
			ls := map[string]string{
				"original-slice-name": slice.Name,
			}
			listOpts := []client.ListOption{
				client.MatchingLabels(ls),
			}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &workerSliceConfigList, listOpts...)
				if err != nil {
					return false
				}
				return len(workerSliceConfigList.Items) == 0
			}, timeout, interval).Should(BeTrue())

			// it should delete sliceconfig
			Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cluster1)).Should(Succeed())
			clusterGetKey := types.NamespacedName{
				Namespace: cluster1.Namespace,
				Name:      cluster1.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterGetKey, cluster1)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, cluster2)).Should(Succeed())
			clusterGetKey = types.NamespacedName{
				Namespace: cluster2.Namespace,
				Name:      cluster2.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterGetKey, cluster2)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Delete(ctx, cluster3)).Should(Succeed())
			clusterGetKey = types.NamespacedName{
				Namespace: cluster3.Namespace,
				Name:      cluster3.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterGetKey, cluster3)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

		})
		It("Should not allow creating vpn keyrotation config if sliceconfig is not present", func() {
			vpnkeyRotation := v1alpha1.VpnKeyRotation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vpn",
					Namespace: slice.Namespace,
				},
				Spec: v1alpha1.VpnKeyRotationSpec{
					SliceName: "demo-vpn",
				},
			}
			Expect(k8sClient.Create(ctx, &vpnkeyRotation)).ShouldNot(Succeed())
		})
		It("Should not allow deleting vpnkeyrotation config, if slice is present and raise an event", func() {
			// get vpnkey rotation config
			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey := types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// should fail
			Expect(k8sClient.Delete(ctx, createdVpnKeyConfig)).ShouldNot(Succeed())
			// check for event
			eventList := &v1.EventList{}
			Eventually(func() bool {
				err := k8sClient.List(ctx, eventList, client.InNamespace("kubeslice-cisco"))
				return err == nil && len(eventList.Items) > 0 && eventFound(eventList, string(events.EventIllegalVPNKeyRotationConfigDelete))
			}, timeout, interval).Should(BeTrue())

		})
	})
	Context("SliceConfig RenewBefore Test Case", func() {
		var project *v1alpha1.Project
		var slice *v1alpha1.SliceConfig
		var cluster1 *v1alpha1.Cluster
		var cluster2 *v1alpha1.Cluster
		var cluster3 *v1alpha1.Cluster
		os.Setenv("KUBESLICE_CONTROLLER_MANAGER_NAMESPACE", controlPlaneNamespace)
		ctx := context.Background()
		project = &v1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cisco",
				Namespace: controlPlaneNamespace,
			},
		}
		slice = &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-slice-1",
				Namespace: sliceNamespace,
			},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:    []string{"worker-1", "worker-2"},
				MaxClusters: 4,
				SliceSubnet: "10.1.0.0/16",
				SliceGatewayProvider: &v1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: "OpenVPN",
					SliceCaType:      "Local",
				},
				SliceIpamType: "Local",
				SliceType:     "Application",
				QosProfileDetails: &v1alpha1.QOSProfile{
					BandwidthCeilingKbps: 5120,
					DscpClass:            "AF11",
				},
			},
		}
		cluster1 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-1",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.12"},
			},
		}
		cluster2 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-2",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.13"},
			},
		}
		cluster3 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-3",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.14"},
			},
		}
		BeforeAll(func() {
			ns := v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: "kubeslice-cisco",
				}, &ns)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Create(ctx, cluster1)).Should(Succeed())
			// update cluster status
			getKey := types.NamespacedName{
				Namespace: cluster1.Namespace,
				Name:      cluster1.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster1)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster1.Status.CniSubnet = []string{"192.168.0.0/24"}
			cluster1.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster1)).Should(Succeed())

			Expect(k8sClient.Create(ctx, cluster2)).Should(Succeed())
			// update cluster status
			getKey = types.NamespacedName{
				Namespace: cluster2.Namespace,
				Name:      cluster2.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster2)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster2.Status.CniSubnet = []string{"192.168.1.0/24"}
			cluster2.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster2)).Should(Succeed())

			Expect(k8sClient.Create(ctx, cluster3)).Should(Succeed())
			// update cluster status
			getKey = types.NamespacedName{
				Namespace: cluster3.Namespace,
				Name:      cluster3.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, cluster3)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			cluster3.Status.CniSubnet = []string{"10.1.1.1/16"}
			cluster3.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
			Expect(k8sClient.Status().Update(ctx, cluster3)).Should(Succeed())

			// it should create sliceconfig
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		})
		AfterAll(func() {
			// update sliceconfig tor remove clusters
			createdSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Namespace: sliceNamespace,
				Name:      slice.Name,
			}
			Expect(k8sClient.Get(ctx, getKey, &createdSliceConfig)).Should(Succeed())
			createdSliceConfig.Spec.Clusters = []string{}
			Expect(k8sClient.Update(ctx, &createdSliceConfig)).Should(Succeed())
			// wait till workersliceconfigs are deleted
			workerSliceConfigList := workerv1alpha1.WorkerSliceConfigList{}
			ls := map[string]string{
				"original-slice-name": slice.Name,
			}
			listOpts := []client.ListOption{
				client.MatchingLabels(ls),
			}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &workerSliceConfigList, listOpts...)
				if err != nil {
					return false
				}
				return len(workerSliceConfigList.Items) == 0
			}, timeout, interval).Should(BeTrue())

			// it should delete sliceconfig
			Expect(k8sClient.Delete(ctx, slice)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cluster1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cluster2)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cluster3)).Should(Succeed())

			Expect(k8sClient.Delete(ctx, project)).Should(Succeed())
		})
		// NOTE:since there would be no job conrtoller present - the secrets(certs) will not be created
		It("should create new jobs for cert creation once a valid renewBefore is set", func() {
			By("fetching workerslice gatways")
			workerSliceGwList := workerv1alpha1.WorkerSliceGatewayList{}
			ls := map[string]string{
				"original-slice-name": slice.Name,
			}
			listOpts := []client.ListOption{
				client.MatchingLabels(ls),
			}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &workerSliceGwList, listOpts...)
				if err != nil {
					return false
				}
				return len(workerSliceGwList.Items) == 2
			}, timeout, interval).Should(BeTrue())

			By("checking if jobs are created")
			job := batchv1.JobList{}
			o := map[string]string{
				"SLICE_NAME": slice.Name,
			}
			listOpts = []client.ListOption{
				client.MatchingLabels(
					o,
				),
			}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &job, listOpts...)
				if err != nil {
					return false
				}
				fmt.Println("len of jobs", len(job.Items))
				return len(job.Items) == 1
			}, timeout, interval).Should(BeTrue())

			createdSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Namespace: sliceNamespace,
				Name:      slice.Name,
			}
			Expect(k8sClient.Get(ctx, getKey, &createdSliceConfig)).Should(Succeed())
			now := metav1.Now()
			createdSliceConfig.Spec.RenewBefore = &now
			// update the sliceconfig
			Expect(k8sClient.Update(ctx, &createdSliceConfig)).Should(Succeed())

			// should update vpnkeyrotation config CertExpiryTS to now
			// get vpnkey rotation config
			createdVpnKeyConfig := &v1alpha1.VpnKeyRotation{}
			getKey = types.NamespacedName{
				Namespace: slice.Namespace,
				Name:      slice.Name,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, createdVpnKeyConfig)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			exyr, exmon, exday := now.Date()
			gotyr, gotmonth, gotday := createdVpnKeyConfig.Spec.CertificateExpiryTime.Date()
			Expect(exyr).To(Equal(gotyr))
			Expect(exmon).To(Equal(gotmonth))
			Expect(exday).To(Equal(gotday))

			// should create new job to create new certs
			By("checking if jobs are created")
			job = batchv1.JobList{}
			o = map[string]string{
				"SLICE_NAME": slice.Name,
			}
			listOpts = []client.ListOption{
				client.MatchingLabels(
					o,
				),
			}
			Eventually(func() bool {
				err := k8sClient.List(ctx, &job, listOpts...)
				if err != nil {
					return false
				}
				fmt.Println("len of jobs", len(job.Items))
				return len(job.Items) == 2
			}, timeout, interval).Should(BeTrue())
		})
	})
})

func eventFound(events *v1.EventList, eventTitle string) bool {
	for _, event := range events.Items {
		if event.Labels["eventTitle"] == eventTitle {
			return true
		}
	}
	return false
}
