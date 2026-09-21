# Hub-and-Spoke Partial Mesh for KubeSlice — Final Technical Report

*LFX Mentorship 2026 · Final Technical Report*

A topology mode that scales inter-cluster tunnels from O(n²) to O(n), with spoke-to-spoke traffic relayed through the hub.

| | |
|---|---|
| **Mentee** | Shreesha (github.com/Shreesha001) |
| **Mentors** | Gourish Biradar (gourishkb), Prabhu Navali (pnavali), Rahul Kumar (Rahul-D78) |
| **Project** | `kubeslice-controller` #300 — Hub-and-Spoke Partial Mesh topology |
| **Repositories** | `kubeslice-controller`, `worker-operator`, `gateway-sidecar`, `apis` |
| **Term** | June – August 2026 |
| **Organisation** | KubeSlice (Avesha) · CNCF |

> **Status at submission.** The feature is complete and validated. All sub-pull-requests were merged into each repository's `hub-spoke-integration-branch`, and four final integration-to-`master` pull requests are open, awaiting the mentors' merge: `kubeslice-controller` #433, `worker-operator` #505, `gateway-sidecar` #65, and `apis` #49. The implementation was validated on a three-cluster Kind topology and on three real managed clusters across two clouds (Oracle OKE and two Linode LKE), where two spokes with no direct link reached each other through the hub at **0% packet loss**, and survived live cluster add/remove, topology switches, and gateway failover.

---

## 1. Executive summary

KubeSlice joins several Kubernetes clusters into one flat overlay network — a **slice** — by building a VPN tunnel between the gateways of each pair of clusters. Until this project the only topology was a **full mesh**: every cluster built a tunnel to every other cluster. That is O(n²) tunnels, which is wasteful when many clusters need only a central cluster and rarely each other.

This project adds a **Hub-and-Spoke partial mesh** topology alongside the existing full mesh. One cluster is the **hub**; every other cluster (a **spoke**) builds a tunnel only to the hub — O(n) tunnels. Spokes still reach each other, with their traffic **relayed through the hub**. The controller computes the desired set of gateway links, gates their creation and teardown, marks each spoke's gateway to route the entire slice to the hub, and aggregates every link's tunnel health into a single slice-level convergence status. Deployments that do not set a topology behave byte-for-byte as before.

**Delivered:** a topology API and admission-webhook validation on `SliceConfig`; a `TopologyResolver` that computes hub↔spoke edges and gates gateway creation and cleanup across live topology changes; a `RouteEntireSliceSubnet` flag propagated to the worker and programmed into the spoke dataplane; a more-specific route split in the gateway sidecar so the tunnel route survives NSM; a TCP-MSS clamp so full-size packets do not black-hole; a `TopologyConverged` slice condition and per-gateway connection status; four layers of testing; and real cross-cloud validation. The work spans four repositories and is delivered as four integration pull requests.

**Table 1 — Measured characteristics on real cross-cloud clusters (every figure observed live).**

| Measurement | Result | Environment |
|---|---|---|
| Spoke-to-spoke, via hub | 0% packet loss, ~54 ms RTT | Oracle OKE → Linode LKE hub → Linode LKE |
| Tunnel count vs full mesh (n clusters) | O(n) vs O(n²) | e.g. 100 clusters: 99 vs 4,950 |
| Live add / remove a spoke | self-reconverged | 3-cluster, traffic running |
| Topology switch (mesh↔hub-spoke) | few-second pause, then healed | 3-cluster, live |
| Gateway pod failure | active/standby failover | 3-cluster, live |
| 50 MB transfer integrity | sha256 match, 0 loss | cross-cloud spoke-to-spoke |

---

## 2. Problem, scope and requirements

A KubeSlice **controller** owns the user-facing custom resources — chiefly `SliceConfig` — and decomposes them into per-cluster `WorkerSliceGateway` objects. A **worker operator** on each cluster watches those objects and programs the local dataplane: it deploys the **gateway pods** that terminate the VPN tunnels. In full mesh the controller creates a gateway pair for *every* pair of clusters.

### 2.1 The scaling problem

With `n` clusters, full mesh needs `n(n−1)/2` tunnels — quadratic growth. For a topology where many clusters need only a central cluster (edge sites reporting to a datacentre, branches to a head office), most of those tunnels carry no traffic yet still consume gateway pods, keys, connection state, and operational attention, and adding one cluster means connecting it to every existing one. Hub-and-spoke reduces this to one tunnel per spoke.

### 2.2 Requirements

- **R1 Topology API** — a declarative way to select hub-and-spoke and name the hub, on `SliceConfig`.
- **R2 Correct edges** — the controller builds only hub↔spoke gateway pairs and no spoke↔spoke pair.
- **R3 Spoke-to-spoke** — spokes reach each other through the hub, without a direct tunnel.
- **R4 Live topology change** — switching mode, or adding/removing a cluster, reconciles edges and per-gateway flags with no stale state.
- **R5 Health visibility** — operators can tell whether the whole slice is connected, and which link is down.
- **R6 Backward compatibility** — a slice with no topology (or explicit full mesh) behaves exactly as before.
- **R7 Validation** — malformed topologies are rejected at admission with a clear error.

> **R6 shaped the design.** The topology field is optional; an absent `topology` (or `mode: FullMesh`) keeps the existing full-mesh behaviour, and every new CRD field is additive and optional, so no existing slice changes behaviour.

### 2.3 Non-goals

- **Linear / tree / multi-hub topologies.** The design keeps a single hub per slice; a more general routed-subnet model is identified as future work (Section 8).
- **WireGuard dataplane.** The control plane is transport-agnostic and works; the WireGuard dataplane is blocked on a pre-existing controller key-generation gap, outside this feature.

---

## 3. Architecture

A slice shares one address range (for example `10.11.0.0/16`), and each cluster is assigned a non-overlapping block of it. In hub-and-spoke the controller builds only the hub↔spoke pairs; the hub is the OpenVPN **Server** side of each pair and each spoke is a **Client**. The three-cluster shape used throughout validation:

```
                    HUB  (worker-2, 10.11.16.x)
                    [ gateway: Server to each spoke ]
                       /                    \
             tun0 (VPN)                      tun0 (VPN)
                     /                        \
   SPOKE-1 (worker-1, 10.11.0.x)      SPOKE-2 (worker-3, 10.11.32.x)
   [ gateway: Client, routeEntire    [ gateway: Client, routeEntire
     SliceSubnet = true ]              SliceSubnet = true ]

           X   no direct spoke-1 <-> spoke-2 tunnel   X
   spoke-1 -> hub -> spoke-2   (the hub relays)
```

The moving parts are: a `TopologyResolver` in the controller that computes the desired edge set from `spec.topology`; edge-gating in the gateway-creation path that creates only the resolved edges and tears down edges that are no longer desired; a `RouteEntireSliceSubnet` flag set on each spoke's client gateway; a status aggregator that rolls per-gateway connectivity into a `TopologyConverged` slice condition; in the worker operator, propagation of the flag and reporting of each gateway's tunnel state back to the hub; and in the gateway sidecar, the dataplane route programming that actually sends the whole slice to the hub.

**Table 2 — The issue breakdown and where each part landed.**

| Issue | Deliverable | Repository |
|---|---|---|
| #301 | Topology fields on `SliceConfig` CRD + webhook validation | controller |
| #302 | `TopologyResolver`: edge computation + gating + `RouteEntireSliceSubnet` flag | controller, apis |
| #303 | Gateway connectivity aggregated into `TopologyConverged` status | controller |
| #304 | End-to-end hub-and-spoke control-plane test suite | controller |
| #471 | Per-gateway `ConnectionState` reported by the worker | worker, apis |
| — | Spoke dataplane: entire-slice route, /17 split, MSS clamp, teardown | worker, gateway-sidecar |

### 3.1 Design decisions

- **The controller computes edges, not the worker.** A single `TopologyResolver` decides the desired links from one place, so topology stays consistent even across live mode changes. The single source of truth is which `WorkerSliceGateway` objects exist.
- **Simple convergence over raw per-link state.** Operators get one `TopologyConverged` condition ("all N gateway links connected", or which is down), rather than having to inspect each gateway.
- **Hub is the Server side.** Spokes are OpenVPN clients to the hub, which keeps the flag and relay logic on well-defined sides of each pair, and makes the "route the whole slice" behaviour a spoke-only concern.

---

## 4. Implementation

### 4.1 #301 — Topology API and webhook validation

A `topology` block was added to `SliceConfig.spec`, carrying a `mode` (`FullMesh` or `HubAndSpoke`) and a `hubs` list. The admission webhook rejects every malformed topology with a clear message: more than one hub, a hub that is not a slice member, a `HubAndSpoke` with no hubs, a hub without a mode, a `FullMesh` that names hubs, an unknown mode, duplicate hubs, and a `HubAndSpoke` on a no-network slice.

### 4.2 #302 — TopologyResolver and edge gating

`service/topology_resolver.go` computes the desired set of directed gateway edges from the topology. Gateway creation is **gated** on this set, so only hub↔spoke pairs are created, and edges that are no longer desired — after a mode switch or a cluster removal — are torn down. On a `FullMesh`→`HubAndSpoke` switch the surviving gateway pairs have their `RouteEntireSliceSubnet` flag reconciled on both sides, so no stale flag lingers (a real defect, D2, caught this).

### 4.3 #303 — TopologyConverged status

`service/topology_status.go` aggregates the connectivity of every gateway in the slice into one `TopologyConverged` condition on `SliceConfig.status`, with a message such as `"all 4 gateway links connected"`. When a tunnel drops, the condition flips and names the affected link, so operators see partial failure without checking each gateway. The watch is filtered to connection-state changes so the aggregation does not churn.

### 4.4 #471 — Per-gateway connection status (worker)

The worker derives each gateway's `ConnectionState` (`Connected` / `NotConnected` / `Pending`) from the gateway pods' reported tunnel state (at least one HA pod `UP` is `Connected`; all-nil is `Pending`), and writes it — with `Reason`, `Message` and `LastTransitionTime` — back to the hub `WorkerSliceGateway` status, where the controller aggregates it. The periodic status refresh is jittered (`wait.Jitter`) so gateways do not re-reconcile in lockstep across a fleet, and a single-writer fast-path skips the write when nothing changed.

### 4.5 Four repositories, not one

| Repository | Why it had to change |
|---|---|
| `kubeslice-controller` | Topology API + webhook, `TopologyResolver`, edge gating, `RouteEntireSliceSubnet` marking, `TopologyConverged` aggregation, e2e suite, docs. |
| `worker-operator` | Propagates the entire-slice route to the spoke gateway pod and slice router; reports gateway tunnel connectivity to the hub. |
| `gateway-sidecar` | Installs the tunnel route as two more-specific halves so NSM cannot overwrite it; clamps TCP MSS; withdraws stale routes on topology change. |
| `apis` | Shared `WorkerSliceGateway` types: the `RouteEntireSliceSubnet` flag and the gateway connection-status fields (and the regenerated CRD). |

---

## 5. The dataplane: routing spoke-to-spoke through the hub

The control plane decides *which* tunnels exist; the dataplane decides *where a packet goes*. This was the most subtle part of the project and the source of several defects, so it is documented in full.

### 5.1 The requirement

A spoke has one tunnel — to the hub. To reach any other spoke, it must send the **entire slice** (not just the hub's block) into that tunnel, and let the hub relay it. This is what the controller's `RouteEntireSliceSubnet` flag turns on. On the spoke gateway pod, the desired route is "the whole slice subnet `10.11.0.0/16` → `tun0`".

### 5.2 The collision with NSM (the /17 split)

The slice's local network is built by NSM (Network Service Mesh), which installs and continuously re-asserts a route for the same range to its own interface: `10.11.0.0/16 → nsm0` (keep local). Adding a competing `10.11.0.0/16 → tun0` collides on the *same* prefix — the kernel keeps one route per exact prefix, and NSM re-writes its own, so the tunnel route kept vanishing and spoke-to-spoke broke (defect D4, diagnosed by watching `ip route`).

**The fix** uses longest-prefix match: the router always prefers the more specific route. The `/16` is split into its two `/17` halves, both to the tunnel — more specific than NSM's `/16`, which NSM never touches:

```
10.11.0.0/16     -> nsm0     NSM's route (whole slice, local)  <- can't touch the /17s
10.11.0.0/17     -> tun0     first half of slice  -> hub        (more specific: wins)
10.11.128.0/17   -> tun0     second half of slice -> hub        (more specific: wins)
10.11.0.0/20     -> nsm0     spoke's OWN block (even more specific) -> stays local
```

The two `/17`s together cover the whole slice and out-rank NSM's `/16`, so remote-slice traffic reliably enters the tunnel; the spoke's own `/20` is more specific still, so local pods correctly stay on `nsm0`. Full mesh never hit this because each cluster had a direct tunnel and wrote small per-cluster `/20` routes, already more specific than NSM's `/16`; hub-and-spoke has to route the whole slice, the same size as NSM's route, hence the split.

### 5.3 MSS clamp and stale-route teardown

Two further dataplane fixes were needed. First, full-size TCP packets black-holed over the tunnel because the tunnel adds encapsulation overhead (defect D5): connections established but large transfers froze. An iptables MSS clamp on `FORWARD -o tun0` (clamp-mss-to-pmtu) caps segments to the tunnel path MTU, and a negative test proves transfers black-hole without it. Second, on a topology change a spoke could keep routing to a hub that is no longer its peer (defect D6); `staleTunnelRouteKeys` computes and withdraws the routes that are no longer desired, so the dataplane reconverges cleanly.

> **The dataplane was the honest hard part.** A four-cluster `iperf` run proved the hub *relay* worked while the spoke gateway's entire-slice route was still missing — the control plane looked correct but traffic looped at `nsm0`. Fixing it required understanding NSM's route behaviour, longest-prefix match, tunnel MTU, and route teardown, none of which show up in a control-plane test.

---

## 6. Verification

Four layers were used, each catching a class the layer below structurally cannot: unit tests with fake clients; reconciler tests under envtest (a real API server, no cluster); an automated control-plane end-to-end suite on a disposable Kind cluster; and a manual dataplane runbook, run on Kind and on three real clusters across two clouds.

**Table 3 — Test inventory for the feature.**

| Suite | Location | What it covers |
|---|---|---|
| Resolver unit | `service/topology_resolver_test.go` | edge computation for mesh and hub-and-spoke |
| Status unit | `service/topology_status_test.go` | `TopologyConverged` aggregation |
| Reconciler (envtest) | `controllers/.../sliceconfig_hubandspoke_test.go` | 6 specs: partial-mesh, full-mesh compat, topology switch, hub-change, partial-pair self-heal, no-network |
| Control-plane e2e | `test/e2e/*` (`make test-e2e-hns`) | partial-mesh build, full-mesh compat, flag reconcile on switch, webhook rejection |
| Worker unit | `pkg/hub/controllers/slicegateway_status_test.go` | connection-state derivation and single-writer status |
| Sidecar unit | `pkg/sidecar/sidecarpb/route_split_test.go` | /17 split, stale-route teardown, MSS clamp rule |
| Dataplane runbook | `docs/hub-and-spoke-testing.md` | real-cluster commands and observed results |

### 6.1 Real cross-cloud validation

Three managed clusters were used: **Oracle OKE** (spoke-1, also the controller), a **Linode LKE** hub, and a second **Linode LKE** spoke-2. On slice `10.11.0.0/16` (blocks `10.11.0.x`, `10.11.16.x`, `10.11.32.x`), two spokes with no direct link reached each other through the hub at **0% packet loss** and about **54 ms** RTT across two clouds — roughly the two spoke↔hub hops summed, confirming the relay. Also verified live: `iperf` throughput, a 50 MB transfer with matching sha256, the MSS-clamp negative test (black-holes without the clamp), gateway HA failover, break/heal of the `TopologyConverged` status, stale-route teardown on a topology flip in both directions, and adding/removing a spoke while traffic flowed. Recurring cluster churn (preemptible OKE nodes reclaimed overnight, an NSM admission-webhook certificate issue after a cert-manager restart) was handled with a documented recovery runbook and did not represent regressions in the feature.

The feature was also brought up on a **4-cluster Kind topology** (1 controller + 3 workers) with the real dataplane, where spoke-to-spoke reached 0% packet loss and `TopologyConverged` reported `True` with all four links connected.

---

## 7. Defects found and fixed

Most of the engineering effort went into defects, not features. The catalogue shows which assumptions were wrong and which layer caught each one. All were fixed before merge.

**Table 4 — Selected defects found and fixed during the project.**

| ID | Defect | Consequence | Found by |
|---|---|---|---|
| D1 | Missing-cluster lookup returned `(false, nil)`; the guard `if !found \|\| err != nil { return err }` returned success | Gateways silently not created for a missing member cluster, reported as success | Line-by-line review |
| D2 | `RouteEntireSliceSubnet` not reconciled on an existing gateway pair after a mode switch | FullMesh→HubAndSpoke silently broke spoke-to-spoke (stale flag) | Live cluster |
| D3 | Spoke gateway pod's entire-slice `tun0` route missing | Control plane correct but traffic looped at `nsm0`; relay unusable | 4-cluster iperf |
| D4 | Tunnel `/16` route overwritten by NSM's `/16` | Route kept vanishing; spoke-to-spoke flapped | `ip route` inspection |
| D5 | Full-size packets black-holed over the tunnel (MTU) | Connections up, but large transfers froze | Negative test |
| D6 | Stale tunnel routes left after a topology flip | Spoke kept routing to a hub that was no longer its peer | Topology-change test |
| D7 | `staleTunnelRouteKeys` returned map order (nondeterministic) | Potential flaky assertion; unstable teardown logging | Copilot review |
| D8 | `apis` CRD YAML not regenerated after adding fields | `connectionState` and `routeEntireSliceSubnet` absent from the CRD — the API server would prune them silently | Review pass |
| D9 | Deriving connection state from packet-loss to speed failover | **No improvement.** Measured flip ~139 s = baseline; the sidecar already handles sustained loss and the CRD field was stale — reverted | Measured on real hub failure |
| D10 | `ConnectionState` a free-form string | Invalid values could persist in etcd | Copilot review |
| D11 | Test named "no write when unchanged" actually wrote reason/message | Misleading coverage of the single-writer fast path | Copilot review |
| D12 | Fixed `time.Sleep(6s)` in the hub-change e2e | Timing-sensitive / flaky under load | Copilot review |
| D13 | Sample YAML said the controller "does not yet consume `spec.topology`" | Misleading docs after the feature landed | Copilot review |

**What the catalogue shows.** *(i)* The dataplane defects (D3–D6) were invisible to control-plane tests by construction — they needed real traffic on real interfaces. *(ii)* Two fixes each contained a narrower version of the bug they fixed (D2 across mode switches, D6 across topology flips), consistently enough to state as a rule: when reconciling a flag, reconcile it on *every* side of an *existing* object, not only on creation. *(iii)* D9 is the most instructive: an "obvious" improvement that measurement showed gave nothing, so it was removed — simple code that works beats clever code that does not earn its place. *(iv)* D8 would have made every connection-status field silently disappear on a real install; regenerating the CRD from the types is not optional.

---

## 8. Known limitations and future work

**Table 5 — Open items at submission. None blocks the merged feature.**

| Item | Impact | Status |
|---|---|---|
| Single hub per slice | No linear / tree / multi-hub topologies yet | Future: `RoutedSubnets []string` refactor (below) |
| WireGuard dataplane | Hub-and-spoke dataplane validated on OpenVPN only | Blocked on controller WG key-gen; control plane is transport-agnostic |
| Hub is a central dependency | Spoke-to-spoke depends on the hub being up | Mitigated by active/standby HA gateway pods; documented |
| Spoke-to-spoke latency | One extra hop through the hub | Inherent to hub-and-spoke; full mesh remains for latency-critical, any-to-any slices |
| `Reason`/`Message` reason codes | Connection-status reason codes are partial | Deferred by design (#471 partial) |

**Natural next step.** The `RouteEntireSliceSubnet` boolean expresses exactly one intent: "send the whole slice to the hub." A more general design replaces it with a controller-computed `RoutedSubnets []string` — the exact set of subnets each gateway should route — which would support linear chains, trees, and multiple hubs while remaining backward-compatible if added additively. This is identified as the first step of any multi-hub work.

---

## 9. Delivery summary

The feature was built as a stack of focused pull requests per repository, merged into each repository's `hub-spoke-integration-branch`, then delivered as one integration-to-`master` pull request per repository.

**Table 6 — The four final integration-to-`master` pull requests, open at submission.**

| Repository | Final PR | Bundles | State |
|---|---|---|---|
| `kubeslice-controller` | #433 | #404, #408, #410, #422, #424 | Open, mergeable |
| `worker-operator` | #505 | #495, #496 | Open, mergeable |
| `gateway-sidecar` | #65 | route split, MSS clamp, teardown | Open, mergeable |
| `apis` | #49 | gateway connection-status types + regenerated CRD | Open, mergeable |

Recommended merge order: `apis` first (the others' `go.mod` pins its commit), then `gateway-sidecar` and `worker-operator`, then `kubeslice-controller`. All Copilot review comments across the four PRs were triaged — some fixed, some declined with a technical rationale (for example a predicate that Copilot claimed defaulted to false was verified against the vendored controller-runtime source to default to true).

---

## 10. Conclusion

KubeSlice can now run a slice in hub-and-spoke partial mesh: the controller builds only hub↔spoke tunnels, scaling inter-cluster tunnels from O(n²) to O(n), while spokes still reach each other through the hub. The controller resolves the desired edges and gates their creation and teardown across live topology changes, marks each spoke to route the whole slice to the hub, and rolls per-gateway tunnel health into a single slice convergence status. Deployments that do not opt in are unchanged. The work spans four repositories and is delivered as four integration pull requests, backed by four layers of testing and real cross-cloud validation at zero packet loss.

The most useful output may not be the feature but the defect catalogue: thirteen real bugs, four of them invisible to control-plane tests because they lived in the dataplane, two introduced by an earlier fix, and one — the packet-loss experiment — where measurement showed the "improvement" was worthless and the right move was to delete it. And the evidence that mattered most was not modelled: on real clusters in two different clouds, two spokes with no direct link talked to each other through the hub at zero loss, and kept talking while a cluster was removed, the topology was switched, and a gateway was killed.

---

## Appendix A — Running the tests and deploying a slice

```sh
# --- Controller ---------------------------------------------------
go test ./service/... ./controllers/...          # unit + envtest specs
make test-e2e-hns                                 # control-plane e2e on a disposable Kind cluster
# --- Worker operator ----------------------------------------------
go test ./pkg/hub/controllers/...
# --- Gateway sidecar ----------------------------------------------
go test ./pkg/sidecar/sidecarpb/...               # /17 split, teardown, MSS clamp
```

```yaml
# Deploy a hub-and-spoke slice (worker-2 is the hub)
apiVersion: controller.kubeslice.io/v1alpha1
kind: SliceConfig
metadata:
  name: demo-hns
spec:
  sliceSubnet: 10.11.0.0/16
  clusters: [worker-1, worker-2, worker-3]
  topology:
    mode: HubAndSpoke
    hubs: [worker-2]
```

```sh
# Verify on a spoke gateway pod: the /17 split routes to the hub
kubectl -n kubeslice-system exec <spoke-gw-pod> -c kubeslice-sidecar -- ip route
#   10.11.0.0/17   via 10.11.255.1 dev tun0
#   10.11.128.0/17 via 10.11.255.1 dev tun0

# Verify slice health
kubectl get sliceconfig demo-hns -o jsonpath='{.status.conditions}'
#   type: TopologyConverged, status: "True", message: "all 4 gateway links connected"
```

> **Backward compatibility.** Omit `spec.topology` (or set `mode: FullMesh` with no hubs) and the controller builds the existing full mesh unchanged. Every new CRD field is additive and optional.
