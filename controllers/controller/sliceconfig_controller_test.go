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
