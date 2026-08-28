package controller

import (
	"context"

	"github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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

	It("reconciles RouteEntireSliceSubnet when the topology changes on an existing slice", func() {
		// A slice that starts as FullMesh and is later switched to HubAndSpoke.
		// The spoke<->hub edge exists in both topologies, so its gateway is NOT
		// recreated on the change. This guards the bug where RouteEntireSliceSubnet
		// was only written at creation and left stale on the surviving gateway,
		// which silently breaks spoke-to-spoke on an upgraded slice.
		const tName = "shift-slice"
		key := types.NamespacedName{Name: tName, Namespace: nsName}
		gwName := tName + "-worker-2-worker-1" // spoke-2 -> worker-1 (hub-to-be)

		slice := &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{Name: tName, Namespace: nsName},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:    []string{"worker-1", "worker-2", "worker-3"},
				MaxClusters: 4,
				SliceSubnet: "10.9.0.0/16",
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

		// full mesh: the gateway exists and does not route the entire subnet
		Eventually(func() bool { return gatewayExists(gwName) }, timeout, interval).Should(BeTrue())
		Expect(getGateway(gwName).Spec.RouteEntireSliceSubnet).To(BeFalse())
		uidBefore := getGateway(gwName).UID

		// switch to HubAndSpoke with worker-1 as the hub
		latest := &v1alpha1.SliceConfig{}
		Expect(k8sClient.Get(ctx, key, latest)).Should(Succeed())
		latest.Spec.Topology = &v1alpha1.TopologySpec{Mode: v1alpha1.TopologyModeHubAndSpoke, Hubs: []string{"worker-1"}}
		Expect(k8sClient.Update(ctx, latest)).Should(Succeed())

		// the surviving spoke->hub gateway must now route the entire slice subnet
		Eventually(func() bool {
			return getGateway(gwName).Spec.RouteEntireSliceSubnet
		}, timeout, interval).Should(BeTrue())
		// and it must be the same object, reconciled in place, not recreated
		Expect(getGateway(gwName).UID).To(Equal(uidBefore))

		// switch back to FullMesh: the flag must be cleared again
		Expect(k8sClient.Get(ctx, key, latest)).Should(Succeed())
		latest.Spec.Topology = &v1alpha1.TopologySpec{Mode: v1alpha1.TopologyModeFullMesh}
		Expect(k8sClient.Update(ctx, latest)).Should(Succeed())
		Eventually(func() bool {
			return getGateway(gwName).Spec.RouteEntireSliceSubnet
		}, timeout, interval).Should(BeFalse())

		// cleanup (best-effort)
		if k8sClient.Get(ctx, key, latest) == nil {
			latest.Spec.Clusters = []string{}
			_ = k8sClient.Update(ctx, latest)
			_ = k8sClient.Delete(ctx, latest)
		}
	})

	It("clears the flag on a gateway that becomes the hub side after a hub change", func() {
		// Directly exercises the both-sides RouteEntireSliceSubnet reconcile across a
		// hub change (the higher-risk transition the code comment calls out). With
		// hub=worker-1, the gateway <slice>-worker-2-worker-1 is the spoke's client
		// (flag true). Changing the hub to worker-2 makes that same gateway the new
		// hub's server side, so its flag MUST be cleared to false - otherwise the hub
		// would route the entire slice and misdirect traffic. Symmetrically,
		// <slice>-worker-1-worker-2 flips from hub server (false) to spoke client (true).
		const hcName = "hubchange-slice"
		key := types.NamespacedName{Name: hcName, Namespace: nsName}
		becomesServer := hcName + "-worker-2-worker-1" // client(true) now -> server(false) after change
		becomesClient := hcName + "-worker-1-worker-2" // server(false) now -> client(true) after change

		slice := &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: nsName},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:    []string{"worker-1", "worker-2", "worker-3"},
				MaxClusters: 4,
				SliceSubnet: "10.12.0.0/16",
				SliceGatewayProvider: &v1alpha1.WorkerSliceGatewayProvider{
					SliceGatewayType: "OpenVPN",
					SliceCaType:      "Local",
				},
				SliceIpamType: "Local",
				SliceType:     "Application",
				Topology:      &v1alpha1.TopologySpec{Mode: v1alpha1.TopologyModeHubAndSpoke, Hubs: []string{"worker-1"}},
				QosProfileDetails: &v1alpha1.QOSProfile{
					BandwidthCeilingKbps: 5120,
					DscpClass:            "AF11",
				},
			},
		}
		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		// initial state (hub=worker-1)
		Eventually(func() bool { return gatewayExists(becomesServer) && gatewayExists(becomesClient) }, timeout, interval).Should(BeTrue())
		Eventually(func() bool { return getGateway(becomesServer).Spec.RouteEntireSliceSubnet }, timeout, interval).Should(BeTrue())
		Expect(getGateway(becomesClient).Spec.RouteEntireSliceSubnet).To(BeFalse())

		// change the hub to worker-2
		latest := &v1alpha1.SliceConfig{}
		Expect(k8sClient.Get(ctx, key, latest)).Should(Succeed())
		latest.Spec.Topology = &v1alpha1.TopologySpec{Mode: v1alpha1.TopologyModeHubAndSpoke, Hubs: []string{"worker-2"}}
		Expect(k8sClient.Update(ctx, latest)).Should(Succeed())

		// the former client (now the hub server side) must be cleared to false
		Eventually(func() bool { return getGateway(becomesServer).Spec.RouteEntireSliceSubnet }, timeout, interval).Should(BeFalse())
		// and the former server (now the spoke client side) must be set to true
		Eventually(func() bool { return getGateway(becomesClient).Spec.RouteEntireSliceSubnet }, timeout, interval).Should(BeTrue())

		// cleanup (best-effort)
		if k8sClient.Get(ctx, key, latest) == nil {
			latest.Spec.Clusters = []string{}
			_ = k8sClient.Update(ctx, latest)
			_ = k8sClient.Delete(ctx, latest)
		}
	})

	It("marks a no-network slice TopologyConverged=True with NoGatewaysRequired", func() {
		// A no-network (NONET) slice has no gateway links, so it must still report a
		// TopologyConverged condition (True / NoGatewaysRequired). This guards the
		// early-return path that previously skipped the status write entirely.
		const nnName = "nonet-slice"
		slice := &v1alpha1.SliceConfig{
			ObjectMeta: metav1.ObjectMeta{Name: nnName, Namespace: nsName},
			Spec: v1alpha1.SliceConfigSpec{
				Clusters:                     []string{"worker-1", "worker-2"},
				MaxClusters:                  4,
				SliceSubnet:                  "10.13.0.0/16",
				OverlayNetworkDeploymentMode: v1alpha1.NONET,
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
		Expect(k8sClient.Create(ctx, slice)).Should(Succeed())

		Eventually(func() bool {
			s := v1alpha1.SliceConfig{}
			if k8sClient.Get(ctx, types.NamespacedName{Name: nnName, Namespace: nsName}, &s) != nil {
				return false
			}
			cond := apimeta.FindStatusCondition(s.Status.Conditions, v1alpha1.SliceConditionTypeTopologyConverged)
			return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == v1alpha1.SliceReasonNoGatewaysRequired
		}, timeout, interval).Should(BeTrue())

		// cleanup (best-effort)
		nn := v1alpha1.SliceConfig{}
		if k8sClient.Get(ctx, types.NamespacedName{Name: nnName, Namespace: nsName}, &nn) == nil {
			nn.Spec.Clusters = []string{}
			_ = k8sClient.Update(ctx, &nn)
			_ = k8sClient.Delete(ctx, &nn)
		}
	})
})
