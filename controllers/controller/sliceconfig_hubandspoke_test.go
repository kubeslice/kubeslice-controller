package controller

import (
	"context"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// End-to-end (envtest) test for the Hub-and-Spoke topology (#304). It drives the
// real SliceConfig reconciler and asserts that, for a HubAndSpoke slice with three
// clusters and worker-1 as the hub, the controller builds a partial mesh: only the
// hub<->spoke gateway links are created (no spoke<->spoke), the hub side is the
// Server and the spoke side the Client, and only the spoke gateways are marked to
// route the entire slice subnet via the hub.
var _ = Describe("SliceConfig HubAndSpoke topology (partial mesh)", Ordered, func() {
	const (
		projectName = "hns"
		nsName      = "kubeslice-hns"
		sliceName   = "hns-slice"
	)
	ctx := context.Background()
	var project *v1alpha1.Project

	// register a Cluster and mark it Registered with a CNI subnet, the way the
	// worker operator would, so the SliceConfig validation and gateway creation
	// treat it as a live worker.
	registerCluster := func(name, cniSubnet, nodeIP string) {
		c := &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsName},
			Spec:       v1alpha1.ClusterSpec{NodeIPs: []string{nodeIP}},
		}
		Eventually(func() bool {
			return k8sClient.Create(ctx, c) == nil
		}, timeout, interval).Should(BeTrue())

		key := types.NamespacedName{Namespace: nsName, Name: name}
		Eventually(func() bool {
			return k8sClient.Get(ctx, key, c) == nil
		}, timeout, interval).Should(BeTrue())

		c.Status.CniSubnet = []string{cniSubnet}
		c.Status.NetworkPresent = true
		c.Status.RegistrationStatus = v1alpha1.RegistrationStatusRegistered
		Eventually(func() bool {
			return k8sClient.Status().Update(ctx, c) == nil
		}, timeout, interval).Should(BeTrue())
	}

	gatewayExists := func(name string) bool {
		gw := workerv1alpha1.WorkerSliceGateway{}
		return k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: nsName}, &gw) == nil
	}
	gatewayAbsent := func(name string) bool {
		gw := workerv1alpha1.WorkerSliceGateway{}
		return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: nsName}, &gw))
	}
	getGateway := func(name string) workerv1alpha1.WorkerSliceGateway {
		gw := workerv1alpha1.WorkerSliceGateway{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: nsName}, &gw)).Should(Succeed())
		return gw
	}

	BeforeAll(func() {
		project = &v1alpha1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: projectName, Namespace: controlPlaneNamespace},
		}
		Eventually(func() bool {
			return k8sClient.Create(ctx, project) == nil
		}, timeout, interval).Should(BeTrue())

		ns := v1.Namespace{}
		Eventually(func() bool {
			return k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, &ns) == nil
		}, timeout, interval).Should(BeTrue())

		registerCluster("worker-1", "192.168.0.0/24", "10.10.0.1") // hub
		registerCluster("worker-2", "192.168.1.0/24", "10.10.0.2") // spoke
		registerCluster("worker-3", "192.168.2.0/24", "10.10.0.3") // spoke
	})

	AfterAll(func() {
		// Best-effort cleanup. This test uses its own project/namespace, so any
		// leftovers don't affect other specs, and envtest tears everything down at
		// suite end. Deboarding a slice fully (finalizers) is slow in envtest, so we
		// don't block the suite on it.
		slice := v1alpha1.SliceConfig{}
		if k8sClient.Get(ctx, types.NamespacedName{Name: sliceName, Namespace: nsName}, &slice) == nil {
			slice.Spec.Clusters = []string{}
			_ = k8sClient.Update(ctx, &slice)
			_ = k8sClient.Delete(ctx, &slice)
		}
		for _, name := range []string{"worker-1", "worker-2", "worker-3"} {
			_ = k8sClient.Delete(ctx, &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsName}})
		}
		_ = k8sClient.Delete(ctx, project)
	})

	It("builds only hub<->spoke gateways and skips spoke<->spoke", func() {
		slice := &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{Name: sliceName, Namespace: nsName},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:    []string{"worker-1", "worker-2", "worker-3"},
				MaxClusters: 4,
				SliceSubnet: "10.7.0.0/16",
				SliceGatewayProvider: &v1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: "OpenVPN",
					SliceCaType:      "Local",
				},
				SliceIpamType: "Local",
				SliceType:     "Application",
				Topology: &v1alpha1.TopologySpec{
					Mode: v1alpha1.TopologyModeHubAndSpoke,
					Hubs: []string{"worker-1"},
				},
				QosProfileDetails: &v1alpha1.QOSProfile{
					BandwidthCeilingKbps: 5120,
					DscpClass:            "AF11",
				},
			},
		}
		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		// the four hub<->spoke gateway links must be created
		Eventually(func() bool { return gatewayExists(sliceName + "-worker-1-worker-2") }, timeout, interval).Should(BeTrue())
		Eventually(func() bool { return gatewayExists(sliceName + "-worker-2-worker-1") }, timeout, interval).Should(BeTrue())
		Eventually(func() bool { return gatewayExists(sliceName + "-worker-1-worker-3") }, timeout, interval).Should(BeTrue())
		Eventually(func() bool { return gatewayExists(sliceName + "-worker-3-worker-1") }, timeout, interval).Should(BeTrue())

		// the spoke<->spoke links must never be created (partial mesh)
		Consistently(func() bool {
			return gatewayAbsent(sliceName+"-worker-2-worker-3") && gatewayAbsent(sliceName+"-worker-3-worker-2")
		}, "3s", interval).Should(BeTrue())

		// hub side is Server and does not route the entire subnet
		hubGw := getGateway(sliceName + "-worker-1-worker-2")
		Expect(hubGw.Spec.GatewayHostType).To(Equal("Server"))
		Expect(hubGw.Spec.RouteEntireSliceSubnet).To(BeFalse())

		// spoke side is Client and routes the entire slice subnet via the hub
		spokeGw := getGateway(sliceName + "-worker-2-worker-1")
		Expect(spokeGw.Spec.GatewayHostType).To(Equal("Client"))
		Expect(spokeGw.Spec.RouteEntireSliceSubnet).To(BeTrue())
	})

	It("full mesh is unaffected: builds all gateway pairs with no entire-subnet routing", func() {
		// Same three clusters, but a FullMesh slice. This guards against a
		// regression where the hub-and-spoke change leaks into the default
		// topology: full mesh must still create every pair, and no gateway may be
		// marked to route the entire slice subnet.
		const fmName = "fm-slice"
		slice := &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{Name: fmName, Namespace: nsName},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:    []string{"worker-1", "worker-2", "worker-3"},
				MaxClusters: 4,
				SliceSubnet: "10.8.0.0/16",
				SliceGatewayProvider: &v1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: "OpenVPN",
					SliceCaType:      "Local",
				},
				SliceIpamType: "Local",
				SliceType:     "Application",
				Topology:      &v1alpha1.TopologySpec{Mode: v1alpha1.TopologyModeFullMesh},
				QosProfileDetails: &v1alpha1.QOSProfile{
					BandwidthCeilingKbps: 5120,
					DscpClass:            "AF11",
				},
			},
		}
		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		// all six gateway links (every pair, both directions) are created,
		// including the spoke<->spoke pair that hub-and-spoke omits.
		gwName := func(a, b string) string { return fmName + "-" + a + "-" + b }
		for _, pair := range [][2]string{
			{"worker-1", "worker-2"}, {"worker-2", "worker-1"},
			{"worker-1", "worker-3"}, {"worker-3", "worker-1"},
			{"worker-2", "worker-3"}, {"worker-3", "worker-2"},
		} {
			name := gwName(pair[0], pair[1])
			Eventually(func() bool { return gatewayExists(name) }, timeout, interval).Should(BeTrue(), name)
		}

		// no gateway in a full-mesh slice routes the entire slice subnet
		list := workerv1alpha1.WorkerSliceGatewayList{}
		Expect(k8sClient.List(ctx, &list, client.InNamespace(nsName))).Should(Succeed())
		for i := range list.Items {
			gw := &list.Items[i]
			if gw.Spec.SliceName == fmName {
				Expect(gw.Spec.RouteEntireSliceSubnet).To(BeFalse(), gw.Name)
			}
		}

		// cleanup this slice (best-effort)
		slice.Spec.Clusters = []string{}
		_ = k8sClient.Update(ctx, slice)
		_ = k8sClient.Delete(ctx, slice)
	})
})
