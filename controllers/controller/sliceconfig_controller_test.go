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

	var Cluster1 *v1alpha1.Cluster
	var Cluster2 *v1alpha1.Cluster
	var Project *v1alpha1.Project
	BeforeAll(func() {
		ctx := context.Background()

		Project = &v1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cisco",
				Namespace: controlPlaneNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, Project)).Should(Succeed())
		GinkgoWriter.Println("Project", Project)

		// Check is namespace is created
		ns := v1.Namespace{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name: "kubeslice-cisco",
			}, &ns)
			return err == nil
		}, timeout, interval).Should(BeTrue())

		Cluster1 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-1",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.12"},
			},
		}

		Cluster2 = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "worker-2",
				Namespace: "kubeslice-cisco",
			},
			Spec: v1alpha1.ClusterSpec{
				NodeIPs: []string{"11.11.11.13"},
			},
		}

		Expect(k8sClient.Create(ctx, Cluster1)).Should(Succeed())
		// update cluster status
		getKey := types.NamespacedName{
			Namespace: Cluster1.Namespace,
			Name:      Cluster1.Name,
		}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, getKey, Cluster1)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		Cluster1.Status.CniSubnet = []string{"192.168.0.0/24"}
		Cluster1.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
		Expect(k8sClient.Status().Update(ctx, Cluster1)).Should(Succeed())

		Expect(k8sClient.Create(ctx, Cluster2)).Should(Succeed())
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
		Expect(k8sClient.Status().Update(ctx, Cluster2)).Should(Succeed())

	})
	AfterAll(func() {
		// Eventually(func() bool {
		// 	ctx := context.Background()
		// 	err := k8sClient.Delete(ctx, Cluster1)
		// 	if err != nil {
		// 		return false
		// 	}
		// 	return true
		// }, timeout, interval).Should(BeTrue())
		// Eventually(func() bool {
		// 	ctx := context.Background()
		// 	err := k8sClient.Delete(ctx, Cluster2)
		// 	if err != nil {
		// 		return false
		// 	}
		// 	return true
		// }, timeout, interval).Should(BeTrue())
		// Eventually(func() bool {
		// 	ctx := context.Background()
		// 	err := k8sClient.Delete(ctx, Project)
		// 	if err != nil {
		// 		return false
		// 	}
		// 	return true
		// }, timeout, interval).Should(BeTrue())
		// Expect(k8sClient.Delete(ctx, Cluster1)).Should(Succeed())
		// Expect(k8sClient.Delete(ctx, Cluster2)).Should(Succeed())
		// Expect(k8sClient.Delete(ctx, Project)).Should(Succeed())
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
					SliceGatewayProvider: v1alpha1.WorkerSliceGatewayProvider{
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
				if err != nil {
					return false
				}
				return true
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
				return lSliceConfig.Spec.VPNConfig == nil
			}, timeout, interval).Should(BeTrue())
		})

		It("When Update on Slice without VPN Configuration with VPN Config It should create pass without errors and VPN Config shall nil", func() {
			By("Updating a existing Slice CR")
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
				return lSliceConfig.Spec.VPNConfig == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, getKey, &lSliceConfig)
				if err != nil {
					return false
				}
				lSliceConfig.Spec.VPNConfig = &v1alpha1.VPNConfiguration{Cipher: "AES-128-CBC"}
				err = k8sClient.Update(ctx, &lSliceConfig)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// Eventually(func() bool {
			// 	err := k8sClient.Get(ctx, getKey, &lSliceConfig)
			// 	if nil != err {
			// 		return false
			// 	}
			// 	return lSliceConfig.Spec.VPNConfig == nil
			// }, timeout, interval).Should(BeTrue())
		})
		/**
		PDescribe("When Update Slice CR without VPN Configuration with Cipher AES-128-CBC ", func() {
			It("It should create pass without errors and VPN Config shall nil", func() {
				By("Creating a new Slice CR")
				// Get the Created Slice Config
				lSliceConfig := v1alpha1.SliceConfig{}
				getKey := types.NamespacedName{
					Name:      sliceName,
					Namespace: sliceNamespace,
				}
				Expect(k8sClient.Get(ctx, getKey, &lSliceConfig)).Should(Succeed())
				// lSliceConfig.VPNConfig = &v1alpha1.VPNConfiguration{Cipher: "AES-128-CBC"}
				Expect(k8sClient.Update(ctx, &lSliceConfig)).Should(Succeed())
				Expect(lSliceConfig.Spec.VPNConfig).To(BeNil())
			})
		})
		**/
	})
})
var _ = PDescribe("Slice Config controller - VPN Config Tests without VPN Config", func() {

	When("When Creating Slice CR with VPN Configuration", func() {
		It("It should create pass without errors", func() {
			By("Creating a new Slice CR")
			ctx := context.Background()

			slice := &v1alpha1.SliceConfig{
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
					QosProfileDetails: &v1alpha1.QOSProfile{
						BandwidthCeilingKbps: 5120,
						DscpClass:            "AF11",
					},
					VPNConfig: &v1alpha1.VPNConfiguration{
						Cipher: "AES-128-CBC",
					},
				},
			}
			Expect(k8sClient.Create(ctx, slice)).Should(Succeed())
			// Get the Created Slice Config
			createdSliceConfig := v1alpha1.SliceConfig{}
			getKey := types.NamespacedName{
				Name:      sliceName,
				Namespace: sliceNamespace,
			}
			Expect(k8sClient.Get(ctx, getKey, &createdSliceConfig)).Should(Succeed())
			Expect(createdSliceConfig.Spec.VPNConfig.Cipher).To(Equal("AES-128-CBC"))
		})
	})
})
