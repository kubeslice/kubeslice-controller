# ADR: Hub-and-Spoke Partial Mesh Topology

| Field | Value |
|---|---|
| Status | Proposed |
| Author | Shreesha Shetty |
| Mentors | Gourish Biradar, Prabhu Navali, Rahul Kumar |
| Date | 2026-06-24 |
| Gate for | worker-operator #470, worker-operator #471 (controller-side work below is the primary path; see Decision 2) |
| Related | [#300](https://github.com/kubeslice/kubeslice-controller/issues/300) (ADR), [#301](https://github.com/kubeslice/kubeslice-controller/issues/301) (CRD), [#302](https://github.com/kubeslice/kubeslice-controller/issues/302) (Edge Compute), [#303](https://github.com/kubeslice/kubeslice-controller/issues/303) (Status), [#304](https://github.com/kubeslice/kubeslice-controller/issues/304) (E2E) |

---

## Context

KubeSlice currently uses a full-mesh topology, where every cluster in a Slice creates a VPN tunnel to every other cluster.

Hub-and-Spoke introduces a topology where one or more clusters are designated as hubs and the remaining clusters become spokes. Tunnels are created only between hub↔spoke and hub↔hub pairs; direct spoke↔spoke tunnels are not created.

MVP scope: HubAndSpoke mode only, with exactly **one hub** (enforced by webhook rule 10; the design below is written for general `|H|` but multi-hub is not reachable this term). Linear topology, auto hub-selection, and hub failover are out of scope for this mentorship term and must not be implemented.

---

## Decisions

### 1. CRD schema

We add an optional `spec.topology` section to SliceConfig:

```yaml
spec:
  # The list of ALL clusters participating in this slice
  clusters:
    - hub-cluster-1
    - spoke-a
    - spoke-b
    - spoke-c

  # [NEW] The topology configuration
  topology:
    mode: HubAndSpoke     # HubAndSpoke | FullMesh; absent = FullMesh
    hubs:
      - hub-cluster-1     # must be a member of spec.clusters
```

`mode` and `hubs` are both optional fields. Zero-value behavior: absent `topology` → full mesh, identical to today. `mode: FullMesh` is also valid as an explicit declaration.

**No `spokes` field — deliberately.** #301 sketches an optional `topology.spokes` list, but this schema omits it: spokes are always *derived* as `spec.clusters − hubs`. An explicit list would allow a subset of the non-hub clusters, leaving the remainder as slice members with no tunnels at all — an orphan state the webhook would have to forbid, forcing the field's only valid value to be its own default ("all non-hub members", as #301 itself defines). Deriving spokes eliminates that dead-weight field along with three validation rules (hub/spoke overlap, spoke membership, orphan members). Since `topology` is an optional block, an explicit `spokes` field can be added later as a non-breaking change if a real use case appears.

`spec.clusters` interacts with the existing `MaxClusters` cap (`+kubebuilder:validation:Maximum=32`, default 16). HubAndSpoke is arguably what makes raising that cap justifiable — since it removes the O(n²) growth that motivated the cap in the first place — but that's a follow-up decision.

The `topology` block carries its own validation markers: `hubs` is `+optional`, `mode` is an enum (`FullMesh`/`HubAndSpoke`). These are what the webhook (see Validation webhook section) enforces at admission time.

### 2. Resolver placement

`TopologyResolver` runs in **kubeslice-controller**, before gateway objects are generated. It takes the list of clusters and the topology configuration as input and returns the desired set of connections (`EdgeSet`).

The `EdgeSet` gates **both halves** of the existing gateway flow: it is fed into `createMinimumGatewaysIfNotExists` so that spoke↔spoke pairs are never created in the first place, and into `cleanupObsoleteGateways` so that pairs whose edge is no longer desired get deleted. The cleanup half matters because today's cleanup logic is membership-based only (it deletes a gateway when one of its clusters leaves `spec.clusters`) — on a FullMesh→HubAndSpoke switch both spoke clusters still exist, so without edge-aware cleanup the pre-existing spoke↔spoke gateways would survive the mode change. Together these are the sole mechanism that controls tunnel topology; no corresponding field is required on `WorkerSliceConfig`.

The reconciler (the controller's existing gateway service) does not contain any topology-specific logic; it simply consumes the `EdgeSet` produced by the resolver. Keeping the resolver separate allows new topology modes to be added in the future without changing the reconciler. Because the gating happens controller-side, the worker-operator needs no change to enforce the topology itself (which pairs get tunnels) — the bulk of that work is in kubeslice-controller. Two worker-side changes remain, and they are scoped deliberately: (a) status reporting — pushing tunnel connectivity up to the controller (Decision 6), and (b) routing behavior *within* an established tunnel, which spokes need for spoke-to-spoke reachability (Decision 7). Neither affects which tunnels exist; that stays controller-owned.

### 3. Edge computation

Given `H = hubs` and `S = all clusters not in H (spokes)`, the algorithm creates exactly two kinds of edges:

- **Hub↔Spoke:** every hub connects to every spoke.
- **Hub↔Hub:** every hub connects to every other hub (only when `|H| > 1`).
- **Spoke↔Spoke:** never created.

```text
for each hub h in H:
    for each spoke s in S:
        add edge (h, s)

for each unique pair of hubs (h1, h2):
    add edge (h1, h2)
```

#### Example 1: 1 hub, 3 spokes

Hubs: `[hub-1]`
Spokes: `[a, b, c]`

| Edge | Type |
|------|------|
| hub-1 ↔ a | hub↔spoke |
| hub-1 ↔ b | hub↔spoke |
| hub-1 ↔ c | hub↔spoke |

**Total:** 3 edges (full mesh would be 6).

#### Example 2: 2 hubs, 3 spokes

Hubs: `[hub-1, hub-2]`
Spokes: `[a, b, c]`

| Edge | Type |
|------|------|
| hub-1 ↔ a | hub↔spoke |
| hub-1 ↔ b | hub↔spoke |
| hub-1 ↔ c | hub↔spoke |
| hub-2 ↔ a | hub↔spoke |
| hub-2 ↔ b | hub↔spoke |
| hub-2 ↔ c | hub↔spoke |
| hub-1 ↔ hub-2 | hub↔hub |

#### General Formula

```text
Total edges = |H| × |S| + (|H| × (|H| - 1)) / 2
```

**MVP scope note:** the algorithm above is written for a general `|H|`, but the MVP caps `hubs` at one entry via webhook rule 10. Hub↔hub edges, Example 2, and the multi-hub terms of the formula document the design's generality and are not reachable this term; enabling multi-hub later means removing that webhook rule, with no schema or algorithm change.

**Units, precisely:** the counts above are *logical link* counts. Each link is realized as one `WorkerSliceGateway` Server/Client pair, which the worker-operator turns into gateway Deployments (with HA pod redundancy) on each side. So the actual object count and pod count are each a constant multiple of the link count above — the reduction claim in this ADR is measured in links/`WorkerSliceGateway` pairs, not pods or Deployments.

**Client/Server role assignment.** The controller currently assigns Server/Client deterministically by cluster ordering in the pair-construction loop (`sourceCluster` = Server, `destinationCluster` = Client). For HubAndSpoke this must be pinned explicitly: **the hub is always the Server** (stable, externally reachable endpoint, typically via NodePort/LoadBalancer) and **spokes are always Clients** that dial in. Without this rule, ordering could assign a spoke as the reachable endpoint, which breaks edge sites that commonly sit behind NAT and can't accept inbound connections. The edge-computation step above must set role by hub/spoke membership, not by cluster-list order.

### 4. Peer list delivery

**No new field on `WorkerSliceConfig` or elsewhere.** The existing `WorkerSliceGateway` object is already the unit of delivery — each one created by `createMinimumGatewaysIfNotExists` (gated by the `EdgeSet` from Decision 2/3) names exactly one desired peer link, with `Server`/`Client` role already carrying direction (Decision 3). The worker-operator already watches `WorkerSliceGateway` and reconciles a tunnel per object, so a spoke's "desired peers" is simply the set of `WorkerSliceGateway` objects where it participates — nothing new to compute or push.

Removal works the same way: `cleanupObsoleteGateways` deletes the `WorkerSliceGateway` object for an edge that's no longer in the `EdgeSet`, and the worker-operator's existing finalizer/delete handling tears down the corresponding tunnel. There is no separate "desired peers" list to reconcile against — the live set of `WorkerSliceGateway` objects for a cluster *is* that list, so it can never drift out of sync with what the controller created.

### 5. Backward compatibility

Existing SliceConfigs with no `topology` field continue to operate with full-mesh behavior after upgrade. When `topology` is `nil`, the `TopologyResolver` returns the complete edge set (all pairs), and the controller keeps creating `WorkerSliceGateway` pairs for every cluster pair exactly as it does today. This resolver-level default is what preserves current behavior.

Full mesh is a controller-side behavior — generated by the existing pair-construction loop — and is not something an individual worker computes or falls back to on its own. Backward compatibility is achieved entirely at the resolver, by matching today's controller behavior when `topology` is absent.

### 6. Status aggregation

Per-link connectivity is reported on `WorkerSliceGateway.status`, since `WorkerSliceGateway` represents one side of an individual gateway link between two clusters.

**Where the signal comes from.** The tunnel-liveness signal already exists in the worker cluster: the worker-operator's gateway reconciler polls each gateway pod's sidecar over gRPC (`GetStatus`) and records `TunnelState` (`UP`/`DOWN`/`UNKNOWN`) per pod on its *local* `SliceGateway.Status.GatewayPodStatus`. Today that signal never leaves the worker cluster. This decision adds the missing upward sync: the worker-operator's hub-facing `SliceGwReconciler` (which already holds a hub client and owns the `WorkerSliceGateway` relationship) reads the local pod statuses, derives a per-gateway connection state, and writes it to `WorkerSliceGateway.status` on the controller cluster.

**New status fields on `WorkerSliceGatewayStatus`** (currently the struct only has `GatewayNumber` and `ClusterInsertionIndex` — the fields below are new and require CRD regeneration):

```yaml
status:
  connectionState: Connected      # Connected | NotConnected | Pending
  lastTransitionTime: "2026-06-24T10:05:00Z"
  reason: DialTimeout
  message: "gateway dial failed after 30s"
```

**Per-gateway state rule (HA-aware).** Each gateway side runs redundant pods; the gateway is `Connected` when **at least one** pod's tunnel is UP (redundancy means traffic flows as long as one tunnel is up), `NotConnected` when all pods report DOWN, and `Pending` when no pod status is available yet (e.g. just created). The aggregator treats an absent/empty `connectionState` — a freshly created object the worker has not yet written to — the same as `Pending`. The status is written only when the derived state changes, so `lastTransitionTime` moves only on real transitions and flapping tunnels don't spam updates. Each worker writes only its own side's `WorkerSliceGateway` object, so there is no cross-cluster write contention.

**Controller aggregation.** The controller aggregates all `WorkerSliceGateway` objects belonging to the Slice (via the existing ownership label) and reports overall topology health on `SliceConfig.status`. The unit of the summary counts is the *edge* (logical link, matching Decision 3): an edge is **ready** only when both of its `WorkerSliceGateway` objects — Server and Client side — report `Connected`. Grouping the two objects of an edge requires no extra bookkeeping: gateway names deterministically encode the cluster pair (`<slice>-<source>-<destination>` / `<slice>-<destination>-<source>`), so the aggregator pairs them by name. Alongside the condition, the status carries the topology summary requested by #303:

```yaml
status:
  topology:
    hubs: ["hub-1"]
    spokesCount: 3
    desiredEdges: 3
    readyEdges: 2
    failingEdges: 1
  conditions:
  - type: TopologyConverged
    status: "False"
    reason: EdgesNotReady
    message: "2/3 links connected; hub-1↔spoke-b NotConnected (DialTimeout)"
    lastTransitionTime: "2026-06-24T10:05:00Z"
```

`TopologyConverged` is:

* `True` when `failingEdges == 0` and `readyEdges == desiredEdges` (reason `AllEdgesReady`). A slice with no gateways required (single cluster, or no-network mode) is `True` with reason `NoGatewaysRequired` — explicitly converged, not merely unset.
* `False` when one or more desired gateway links are not `Connected` (including `Pending` ones). The condition message identifies the first failing gateway link.

This proposal adds a standard `Conditions []metav1.Condition` field (plus the `topology` summary block) to `SliceConfigStatus`, which currently only contains `KubesliceEvents []KubesliceEvent`.

**Dependencies this decision creates:**

1. **CRD/type changes** in kubeslice-controller: the new `WorkerSliceGatewayStatus` fields and the `SliceConfigStatus` additions are Go type changes requiring `make generate && make manifests`; they should land with the #301 CRD PR, before #303 is implemented against them.
2. **Two new watches**, without which the pipeline never fires: the worker-operator's hub `SliceGwReconciler` must additionally watch the *local* `SliceGateway` CR (mapping back to its `WorkerSliceGateway`) so a tunnel-state flip triggers the upward sync; and the controller's SliceConfig reconciler must additionally watch `WorkerSliceGateway` (mapping via the ownership label to the owning `SliceConfig`) so a status write triggers re-aggregation. Today the SliceConfig controller watches only `SliceConfig` itself.
3. **Worker-operator change** (the status-reporting change acknowledged in Decision 2): the upward sync in the hub `SliceGwReconciler`, tracked as worker-operator #471.

When gateway links are removed (for example when the topology changes), their corresponding `WorkerSliceGateway` objects are deleted through the existing cleanup flow, so their status disappears with them and no stale entries remain; newly created objects start as `Pending` and hold `TopologyConverged` at `False` until their tunnels come up.

### 7. Spoke-to-spoke reachability

Routes on a cluster's `vl3-slice-router` are only added for clusters it is directly tunneled to (`sliceRouterInjectRoute` in `router-sidecar`, driven by `SendConnectionContextToSliceRouter` in worker-operator — one route per `WorkerSliceGateway` link). Under HubAndSpoke, a spoke only has a tunnel to the hub, so its router never learns that other spokes exist, and traffic addressed to another spoke never even leaves the originating cluster.

Proposed approach: instead of giving a spoke a route to just the hub's own subnet, give it a route for **"anything that isn't mine → send to the hub"** — a default-style route scoped to the Slice's own CIDR range (never a literal `0.0.0.0/0`, so it cannot leak traffic outside the slice). The hub already holds a direct route to every spoke it's connected to, so once traffic reaches the hub, it forwards correctly on to the target spoke.

This design is self-maintaining: when a new spoke joins later, existing spokes don't need any route update — they already forward anything unfamiliar to the hub by default.

**Multi-hub readiness (not implemented in this MVP):** the next hop for this route is modeled as a list of hub IPs, not a single IP, even though MVP (single hub) only ever populates it with one entry. When a second hub is introduced later, the same route simply gains a second next hop and load-balances via ECMP — no redesign needed.

**Accepted trade-offs:** the hub becomes a more significant single point of failure than in full mesh — a hub outage now breaks both hub↔spoke and spoke↔spoke communication. Spoke-to-spoke traffic also takes two hops instead of one. Both are accepted for this MVP, consistent with the existing "Hub health-based failover" non-goal.

---

## Validation webhook

Rejects on CREATE and UPDATE:

| # | Validation Rule | Error Message |
|---|---|---|
| 1 | `mode: HubAndSpoke` AND `hubs` is empty or absent | `HubAndSpoke topology requires at least one hub` |
| 2 | `mode: HubAndSpoke` AND a name in `hubs` is not in `spec.clusters` | `hub is not a member of spec.clusters` |
| 3 | `mode: HubAndSpoke` AND all clusters are listed as hubs (no spokes) | `HubAndSpoke topology requires at least one spoke cluster` |
| 4 | `mode: FullMesh` AND `hubs` is non-empty | `hubs must be empty when mode is FullMesh` |
| 5 | `hubs` contains duplicate cluster names | `duplicate hub entry: X` |
| 6 | `mode` is set to an unrecognized value | `unknown topology mode; valid values: FullMesh, HubAndSpoke` |
| 7 | `mode: HubAndSpoke` AND `len(spec.clusters) < 2` | `HubAndSpoke topology requires at least 2 clusters` |
| 8 | `len(spec.clusters) > MaxClusters` (delegates to the existing `MaxClusters` validation; HubAndSpoke does not bypass it) | `participating clusters cannot be greater than MaxClusterCount :X` |
| 9 | `hubs` is non-empty AND `topology.mode` is absent | `mode must be set to HubAndSpoke when hubs is specified` |
| 10 | `mode: HubAndSpoke` AND `len(hubs) > 1` | `only one hub is supported in this release` |

---

## Sequences

**Normal convergence:**
```
Operator   → PATCH SliceConfig (mode: HubAndSpoke, hubs: [hub-1])
Controller → TopologyResolver.Resolve() → EdgeSet
Controller → EdgeSet gates createMinimumGatewaysIfNotExists:
             creates WorkerSliceGateway pairs only for edges in EdgeSet
             (hub-1↔spoke-a, hub-1↔spoke-b, hub-1↔spoke-c)
Workers    → SliceGwReconciler brings up gateway tunnels for the created pairs
Workers    → update WorkerSliceGateway.status (connectionState)
Controller → aggregates WorkerSliceGateway statuses → TopologyConverged condition on SliceConfig.status.conditions
```

---

## Non-goals

- Linear topology
- Auto hub selection
- Multi-hub load balancing
- Hub health-based failover (accepted single-hub SPOF risk for this term)
- Any topology mode other than `HubAndSpoke` and `FullMesh`

---

## Open issues

- **Rewire safety.** How the controller should transition `WorkerSliceGateway` pairs when `hubs` changes (convergence strategy, concurrent-edit guard, disruption trade-offs, and zero-downtime rewiring as a future extension) is undecided and out of scope for this term. Whoever picks it up should design it against the resolver/`EdgeSet` model in Decisions 2 and 3.

---
