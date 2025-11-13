package controller

import (
	"context"

	v1 "k8s.io/api/core/v1"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Slice Config controller Tests", Ordered, func() {
	const (
		sliceName      = "test-slice"
		sliceNamespace = "kubeslice-ibm"
	)
	var Cluster1 *v1alpha1.Cluster
	var Cluster2 *v1alpha1.Cluster
	var Project *v1alpha1.Project
	var projectName = "ibm"
	BeforeAll(func() {
		ctx := context.Background()

		Project = &v1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      projectName,
				Namespace: controlPlaneNamespace,
			},
		}

		Eventually(func() bool {
			err := k8sClient.Create(ctx, Project)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// Check is namespace is created
		ns := v1.Namespace{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name: "kubeslice-" + projectName,
			}, &ns)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		Cluster1 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-1",
				Namespace: "kubeslice-" + projectName,
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.12"},
			},
		}

		Cluster2 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-2",
				Namespace: "kubeslice-" + projectName,
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.13"},
			},
		}

		Eventually(func() bool {
			err := k8sClient.Create(ctx, Cluster1)
			GinkgoWriter.Println(err)

			return err == nil
		}, timeout, interval).Should(BeTrue())
		// update cluster status
		getKey := types.NamespacedName{
			Namespace: Cluster1.Namespace,
			Name:      Cluster1.Name,
		}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, Cluster1)
			GinkgoWriter.Println(err)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		Cluster1.Status.CniSubnet = []string{"192.168.0.0/24"}
		Cluster1.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
		Cluster1.Status.NetworkPresent = true

		Eventually(func() bool {
			err := k8sClient.Status().Update(ctx, Cluster1)
			GinkgoWriter.Println(err)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		//Debug
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, Cluster1)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		GinkgoWriter.Println("Cluster1 RegistrationStatus ", Cluster1.Status.RegistrationStatus)

		Eventually(func() bool {
			err := k8sClient.Create(ctx, Cluster2)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// update cluster status
		getKey = types.NamespacedName{
			Namespace: Cluster2.Namespace,
			Name:      Cluster2.Name,
		}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, Cluster2)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		Cluster2.Status.CniSubnet = []string{"192.168.1.0/24"}
		Cluster2.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
		Cluster2.Status.NetworkPresent = true

		Eventually(func() bool {
			err := k8sClient.Status().Update(ctx, Cluster2)
			GinkgoWriter.Println(err)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		//Debug
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, Cluster2)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		GinkgoWriter.Println(Cluster2.Status.RegistrationStatus)
		GinkgoWriter.Println("Cluster2 RegistrationStatus ", Cluster1.Status.RegistrationStatus)
	})
	AfterAll(func() {
		ctx := context.Background()

		Eventually(func() bool {
			err := k8sClient.Delete(ctx, Cluster1)
			GinkgoWriter.Println(err)
			return nil == err
		}, timeout, interval).Should(BeTrue())
		Eventually(func() bool {
			err := k8sClient.Delete(ctx, Cluster2)
			GinkgoWriter.Println(err)
			return nil == err
		}, timeout, interval).Should(BeTrue())
		Eventually(func() bool {
			err := k8sClient.Delete(ctx, Project)
			GinkgoWriter.Println(err)
			return nil == err
		}, timeout, interval).Should(BeTrue())
	})

	Describe("Slice Config controller - VPN Config Tests without VPN Config", func() {
		var slice *v1alpha1.SliceConfig
		BeforeEach(func() {
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
		})
		AfterEach(func() {
			// update sliceconfig tor remove clusters
			createdSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &createdSliceConfig)
				if err != nil {
					return false
				}
				createdSliceConfig.Spec.Clusters = []string{}
				err = k8sClient.Update(ctx, &createdSliceConfig)
				return nil == err
			}, timeout, interval).Should(BeTrue())

			// wait till workersliceconfigs are deleted
			workerSliceConfigList := workerv1alpha1.WorkerSliceConfigList{}
			ls := map[string]string{
				"original-slice-name": sliceName,
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

			Expect(k8sClient.Delete(ctx, &createdSliceConfig)).Should(Succeed())
			getKey = types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &createdSliceConfig)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("When Creating Slice CR without VPN Configuration It should create pass without errors and VPN Config shall nil", func() {
			By("Creating a new Slice CR")
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

			// Get the Created Slice Config
			lSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &lSliceConfig)
				if nil != err {
					return false
				}
				return lSliceConfig.Spec.VPNConfig.Cipher == "AES-256-CBC"
			}, timeout, interval).Should(BeTrue())
		})

		It("When Update on Slice without VPN Configuration with VPN Config It should fail to update with errors", func() {
			By("Updating a existing Slice CR")
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

			// Get the Created Slice Config
			lSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}

			Eventually(func() bool {
				var errString = `admission webhook "vsliceconfig.kb.io" denied the request: SliceConfig.controller.kubeslice.io "test-slice" is invalid: Spec.VPNConfig.Cipher: Invalid value: "AES-128-CBC": cannot be updated`

				err := k8sClient.Get(ctx, getKey, &lSliceConfig)
				if nil != err {
					return false
				}
				GinkgoWriter.Println("Get VPNConfig", lSliceConfig.Spec.VPNConfig)

				lSliceConfig.Spec.VPNConfig = &v1alpha1.VPNConfiguration{Cipher: "AES-128-CBC"}

				err = k8sClient.Update(ctx, &lSliceConfig)
				GinkgoWriter.Println("Update Error", err)
				return errString == err.Error()
			}, timeout, interval).Should(BeTrue())
		})
	})
	Describe("Slice Config controller - VPN Config Tests VPN Config", func() {
		var slice *v1alpha1.SliceConfig
		BeforeEach(func() {
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
					VPNConfig: &v1alpha1.VPNConfiguration{
						Cipher: "AES-128-CBC",
					},
				},
			}
		})

		AfterEach(func() {
			// update sliceconfig tor remove clusters
			createdSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &createdSliceConfig)
				if err != nil {
					return false
				}
				GinkgoWriter.Println("createdSliceConfig", createdSliceConfig.Spec.VPNConfig)

				createdSliceConfig.Spec.Clusters = []string{}
				err = k8sClient.Update(ctx, &createdSliceConfig)
				GinkgoWriter.Println("Detach Cluster Error", err)
				return nil == err
			}, timeout, interval).Should(BeTrue())

			// wait till workersliceconfigs are deleted
			workerSliceConfigList := workerv1alpha1.WorkerSliceConfigList{}
			ls := map[string]string{
				"original-slice-name": sliceName,
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
			GinkgoWriter.Println("Detached Clusters from Slice Successful")
			Expect(k8sClient.Delete(ctx, &createdSliceConfig)).Should(Succeed())
			getKey = types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &createdSliceConfig)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

		It("When Creating Slice CR with VPN Configuration It should create pass without errors and VPN Config shall nil", func() {
			By("Creating a new Slice CR")
			Eventually(func() bool {
				err := k8sClient.Create(ctx, slice)
				GinkgoWriter.Println(err)
				return nil == err
			}, timeout, interval).Should(BeTrue())

			// Get the Created Slice Config
			lSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &lSliceConfig)
				if nil != err {
					return false
				}
				GinkgoWriter.Println(lSliceConfig.Spec.VPNConfig)
				return lSliceConfig.Spec.VPNConfig.Cipher == "AES-128-CBC"
			}, timeout, interval).Should(BeTrue())
		})

		It("When Update on Slice with VPN Configuration with VPN Config It should fail to update with error", func() {
			By("Updating a existing Slice CR")
			Eventually(func() bool {
				err := k8sClient.Create(ctx, slice)
				GinkgoWriter.Println(err)
				return nil == err
			}, timeout, interval).Should(BeTrue())

			// Get the Created Slice Config
			lSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}

			Eventually(func() bool {
				var expErrStr = `admission webhook "vsliceconfig.kb.io" denied the request: SliceConfig.controller.kubeslice.io "test-slice" is invalid: Spec.VPNConfig.Cipher: Invalid value: "AES-256-CBC": cannot be updated`
				err := k8sClient.Get(ctx, getKey, &lSliceConfig)
				if nil != err {
					return false
				}
				GinkgoWriter.Println("Get VPNConfig", lSliceConfig.Spec.VPNConfig)

				lSliceConfig.Spec.VPNConfig.Cipher = "AES-256-CBC"

				err = k8sClient.Update(ctx, &lSliceConfig)
				return expErrStr == err.Error()
			}, timeout, interval).Should(BeTrue())
		})

		It("When Update on Slice with VPN Configuration with VPN Config It should succeed to update without errors", func() {
			By("Updating a existing Slice CR")
			Eventually(func() bool {
				err := k8sClient.Create(ctx, slice)
				GinkgoWriter.Println(err)
				return nil == err
			}, timeout, interval).Should(BeTrue())

			// Get the Created Slice Config
			lSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &lSliceConfig)
				if nil != err {
					return false
				}
				GinkgoWriter.Println("Get VPNConfig", lSliceConfig.Spec.VPNConfig)
				lSliceConfig.Spec.VPNConfig.Cipher = "AES-128-CBC"
				GinkgoWriter.Println("Slice VPNConfig", lSliceConfig)

				err = k8sClient.Update(ctx, &lSliceConfig)
				return nil == err
			}, timeout, interval).Should(BeTrue())
		})
	})
})

var _ = Describe("Slice Config controller - Topology Tests", Ordered, func() {
	var slice *v1alpha1.SliceConfig
	var topologyCluster1 *v1alpha1.Cluster
	var topologyCluster2 *v1alpha1.Cluster
	var topologyCluster3 *v1alpha1.Cluster
	const topologySliceName = "test-topology-slice"
	const topoProjectName = "topology-project"
	const topoSliceNamespace = "kubeslice-topology-project"

	BeforeAll(func() {
		ctx := context.Background()

		// Create project for topology tests
		topoProject := &v1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      topoProjectName,
				Namespace: controlPlaneNamespace,
			},
		}

		Eventually(func() bool {
			err := k8sClient.Create(ctx, topoProject)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// Check namespace is created
		ns := v1.Namespace{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name: topoSliceNamespace,
			}, &ns)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// Create topology test clusters
		topologyCluster1 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "topo-worker-1",
				Namespace: topoSliceNamespace,
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.20"},
			},
		}

		topologyCluster2 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "topo-worker-2",
				Namespace: topoSliceNamespace,
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.21"},
			},
		}

		topologyCluster3 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "topo-worker-3",
				Namespace: topoSliceNamespace,
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.22"},
			},
		}

		// Create and register first cluster
		Eventually(func() bool {
			err := k8sClient.Create(ctx, topologyCluster1)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		getKey := types.NamespacedName{
			Namespace: topologyCluster1.Namespace,
			Name:      topologyCluster1.Name,
		}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, topologyCluster1)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		topologyCluster1.Status.CniSubnet = []string{"192.168.2.0/24"}
		topologyCluster1.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
		topologyCluster1.Status.ClusterHealth = &v1alpha1.ClusterHealth{ClusterHealthStatus: v1alpha1.ClusterHealthStatusNormal}
		topologyCluster1.Status.NetworkPresent = true

		Eventually(func() bool {
			err := k8sClient.Status().Update(ctx, topologyCluster1)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// Create and register second cluster
		Eventually(func() bool {
			err := k8sClient.Create(ctx, topologyCluster2)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		getKey = types.NamespacedName{
			Namespace: topologyCluster2.Namespace,
			Name:      topologyCluster2.Name,
		}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, topologyCluster2)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		topologyCluster2.Status.CniSubnet = []string{"192.168.3.0/24"}
		topologyCluster2.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
		topologyCluster2.Status.ClusterHealth = &v1alpha1.ClusterHealth{ClusterHealthStatus: v1alpha1.ClusterHealthStatusNormal}
		topologyCluster2.Status.NetworkPresent = true

		Eventually(func() bool {
			err := k8sClient.Status().Update(ctx, topologyCluster2)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		// Create and register third cluster
		Eventually(func() bool {
			err := k8sClient.Create(ctx, topologyCluster3)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		getKey = types.NamespacedName{
			Namespace: topologyCluster3.Namespace,
			Name:      topologyCluster3.Name,
		}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, topologyCluster3)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		topologyCluster3.Status.CniSubnet = []string{"192.168.4.0/24"}
		topologyCluster3.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
		topologyCluster3.Status.ClusterHealth = &v1alpha1.ClusterHealth{ClusterHealthStatus: v1alpha1.ClusterHealthStatusNormal}
		topologyCluster3.Status.NetworkPresent = true

		Eventually(func() bool {
			err := k8sClient.Status().Update(ctx, topologyCluster3)
			return err == nil
		}, timeout, interval).Should(BeTrue())
	})

	BeforeEach(func() {
		slice = &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      topologySliceName,
				Namespace: topoSliceNamespace,
			},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:    []string{"topo-worker-1", "topo-worker-2"},
				MaxClusters: 10,
				SliceSubnet: "10.2.0.0/16",
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
	})

	AfterEach(func() {
		ls := map[string]string{
			"original-slice-name": topologySliceName,
		}
		listOpts := []client.ListOption{
			client.MatchingLabels(ls),
		}

		getKey := types.NamespacedName{
			Name:      topologySliceName,
			Namespace: topoSliceNamespace,
		}

		existingSlice := &v1alpha1.SliceConfig{}
		err := k8sClient.Get(ctx, getKey, existingSlice)
		if err != nil {
			Expect(errors.IsNotFound(err)).To(BeTrue())
			return
		}

		Expect(k8sClient.Delete(ctx, existingSlice)).Should(Succeed())

		Eventually(func() bool {
			workerSliceConfigList := workerv1alpha1.WorkerSliceConfigList{}
			err := k8sClient.List(ctx, &workerSliceConfigList, listOpts...)
			if err != nil {
				return false
			}
			if len(workerSliceConfigList.Items) == 0 {
				return true
			}
			for i := range workerSliceConfigList.Items {
				if delErr := k8sClient.Delete(ctx, &workerSliceConfigList.Items[i]); delErr != nil && !errors.IsNotFound(delErr) {
					GinkgoWriter.Printf("failed deleting WorkerSliceConfig %s/%s: %v\n", workerSliceConfigList.Items[i].Namespace, workerSliceConfigList.Items[i].Name, delErr)
				}
			}
			return false
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			workerSliceGatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err := k8sClient.List(ctx, &workerSliceGatewayList, listOpts...)
			if err != nil {
				return false
			}
			if len(workerSliceGatewayList.Items) == 0 {
				return true
			}
			for i := range workerSliceGatewayList.Items {
				if delErr := k8sClient.Delete(ctx, &workerSliceGatewayList.Items[i]); delErr != nil && !errors.IsNotFound(delErr) {
					GinkgoWriter.Printf("failed deleting WorkerSliceGateway %s/%s: %v\n", workerSliceGatewayList.Items[i].Namespace, workerSliceGatewayList.Items[i].Name, delErr)
				}
			}
			return false
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			fresh := &v1alpha1.SliceConfig{}
			err := k8sClient.Get(ctx, getKey, fresh)
			return errors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
	})

	It("Should create 1 gateway pair for full-mesh topology with 2 clusters", func() {
		By("Creating a SliceConfig with full-mesh topology and 2 clusters")
		slice.Spec.TopologyConfig = &v1alpha1.TopologyConfig{
			TopologyType: v1alpha1.TopologyFullMesh,
		}
		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		By("Verifying 2 gateway objects are created for 1 bidirectional pair (n*(n-1)/2 = 1 pair for n=2, but 2 gateway objects)")
		sliceKey := types.NamespacedName{
			Name:      topologySliceName,
			Namespace: topoSliceNamespace,
		}
		Eventually(func() bool {
			createdSlice := &v1alpha1.SliceConfig{}
			err := k8sClient.Get(ctx, sliceKey, createdSlice)
			if err != nil {
				GinkgoWriter.Println("Error getting slice:", err)
				return false
			}

			// Check that gateway pairs were created (2 gateways for 1 bidirectional pair)
			gatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err = k8sClient.List(ctx, &gatewayList,
				client.MatchingLabels{"original-slice-name": slice.Name})

			GinkgoWriter.Println("Gateway count:", len(gatewayList.Items))
			return err == nil && len(gatewayList.Items) == 2
		}, timeout, interval).Should(BeTrue())
	})

	It("Should create 3 gateway pairs for full-mesh topology with 3 clusters", func() {
		By("Creating SliceConfig with full-mesh topology and 3 clusters")
		slice.Spec.Clusters = []string{"topo-worker-1", "topo-worker-2", "topo-worker-3"}
		slice.Spec.TopologyConfig = &v1alpha1.TopologyConfig{
			TopologyType: v1alpha1.TopologyFullMesh,
		}
		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		By("Verifying 6 gateway objects are created for 3 pairs (n*(n-1)/2 = 3 pairs for n=3, but 6 gateway objects)")
		Eventually(func() bool {
			gatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err := k8sClient.List(ctx, &gatewayList,
				client.MatchingLabels{"original-slice-name": slice.Name})

			GinkgoWriter.Println("Gateway count for 3 clusters:", len(gatewayList.Items))
			return err == nil && len(gatewayList.Items) == 6
		}, timeout, interval).Should(BeTrue())
	})

	It("Should default to full-mesh when topology config is nil", func() {
		By("Creating SliceConfig without topology config")
		slice.Spec.TopologyConfig = nil // No topology specified
		slice.Spec.Clusters = []string{"topo-worker-1", "topo-worker-2"}

		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		By("Verifying it defaults to full-mesh (2 gateway objects for 1 bidirectional pair with 2 clusters)")
		Eventually(func() bool {
			gatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err := k8sClient.List(ctx, &gatewayList,
				client.MatchingLabels{"original-slice-name": slice.Name})

			GinkgoWriter.Println("Gateway count for nil topology:", len(gatewayList.Items))
			return err == nil && len(gatewayList.Items) == 2
		}, timeout, interval).Should(BeTrue())
	})

	It("Should exclude forbidden edges from restricted topology", func() {
		By("Creating SliceConfig with restricted topology and forbidden edges")
		slice.Spec.Clusters = []string{"topo-worker-1", "topo-worker-2", "topo-worker-3"}
		slice.Spec.TopologyConfig = &v1alpha1.TopologyConfig{
			TopologyType: v1alpha1.TopologyRestricted,
			ForbiddenEdges: []v1alpha1.ForbiddenEdge{
				{
					SourceCluster:  "topo-worker-1",
					TargetClusters: []string{"topo-worker-3"}, // block 1->3
				},
			},
		}

		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		By("Verifying 4 gateway objects are created (forbidding 1↔3 removes both directions due to bidirectional tunnels, leaving 2 pairs * 2 objects = 4 gateways)")
		Eventually(func() bool {
			gatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err := k8sClient.List(ctx, &gatewayList,
				client.MatchingLabels{"original-slice-name": slice.Name})

			GinkgoWriter.Println("Gateway count (restricted):", len(gatewayList.Items))
			return err == nil && len(gatewayList.Items) == 4
		}, timeout, interval).Should(BeTrue())
	})

	It("Should create gateway pairs from custom connectivity matrix", func() {
		By("Creating SliceConfig with custom topology matrix")
		slice.Spec.Clusters = []string{"topo-worker-1", "topo-worker-2", "topo-worker-3"}
		slice.Spec.TopologyConfig = &v1alpha1.TopologyConfig{
			TopologyType: v1alpha1.TopologyCustom,
			ConnectivityMatrix: []v1alpha1.ConnectivityEntry{
				{
					SourceCluster:  "topo-worker-1",
					TargetClusters: []string{"topo-worker-2"}, // only 1->2
				},
			},
		}

		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		By("Verifying 2 gateway objects are created from custom matrix (1 pair specified becomes bidirectional → 2 objects)")
		Eventually(func() bool {
			gatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err := k8sClient.List(ctx, &gatewayList,
				client.MatchingLabels{"original-slice-name": slice.Name})

			GinkgoWriter.Println("Gateway count (custom matrix):", len(gatewayList.Items))
			return err == nil && len(gatewayList.Items) == 2
		}, timeout, interval).Should(BeTrue())
	})

	It("Should update gateways when topology config changes", func() {
		By("Creating SliceConfig with 2 clusters initially (default full-mesh)")
		slice.Spec.TopologyConfig = nil
		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		By("Verifying 2 gateway objects for 2 clusters (full-mesh defaults to 1 pair → 2 objects)")
		Eventually(func() bool {
			gatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err := k8sClient.List(ctx, &gatewayList,
				client.MatchingLabels{"original-slice-name": slice.Name})

			GinkgoWriter.Println("Initial gateway count:", len(gatewayList.Items))
			return err == nil && len(gatewayList.Items) == 2
		}, timeout, interval).Should(BeTrue())

		By("Updating to 3 clusters with full-mesh")
		sliceKey := types.NamespacedName{
			Name:      topologySliceName,
			Namespace: topoSliceNamespace,
		}
		updatedSlice := &v1alpha1.SliceConfig{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, sliceKey, updatedSlice)
			if err != nil {
				return false
			}
			updatedSlice.Spec.Clusters = []string{"topo-worker-1", "topo-worker-2", "topo-worker-3"}
			updatedSlice.Spec.TopologyConfig = &v1alpha1.TopologyConfig{
				TopologyType: v1alpha1.TopologyFullMesh,
			}
			err = k8sClient.Update(ctx, updatedSlice)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		By("Verifying 6 gateway objects are now created for 3 bidirectional pairs")
		Eventually(func() bool {
			gatewayList := workerv1alpha1.WorkerSliceGatewayList{}
			err := k8sClient.List(ctx, &gatewayList,
				client.MatchingLabels{"original-slice-name": slice.Name})

			GinkgoWriter.Println("Updated gateway count:", len(gatewayList.Items))
			return err == nil && len(gatewayList.Items) == 6
		}, timeout, interval).Should(BeTrue())
	})
})
