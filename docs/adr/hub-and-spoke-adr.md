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

MVP scope: HubAndSpoke mode only. Linear topology, auto hub-selection, and hub failover are out of scope for this mentorship term and must not be implemented.

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

`spec.clusters` interacts with the existing `MaxClusters` cap (`+kubebuilder:validation:Maximum=32`, default 16). HubAndSpoke is arguably what makes raising that cap justifiable — since it removes the O(n²) growth that motivated the cap in the first place — but that's a follow-up decision.

The `topology` block carries its own validation markers: `hubs` is `+optional`, `mode` is an enum (`FullMesh`/`HubAndSpoke`). These are what the webhook (see Validation webhook section) enforces at admission time.

### 2. Resolver placement

`TopologyResolver` runs in **kubeslice-controller**, before gateway objects are generated. It takes the list of clusters and the topology configuration as input and returns the desired set of connections (`EdgeSet`).

The `EdgeSet` gates `WorkerSliceGateway` creation directly: it is fed into `createMinimumGatewaysIfNotExists` so that spoke↔spoke pairs are never created in the first place, rather than being created and then left unused. This is the sole mechanism that controls tunnel topology; no corresponding field is required on `WorkerSliceConfig`.

The reconciler (the controller's existing gateway service) does not contain any topology-specific logic; it simply consumes the `EdgeSet` produced by the resolver. Keeping the resolver separate allows new topology modes to be added in the future without changing the reconciler. Because the gating happens controller-side, the worker-operator needs little or no change for the core feature to work — the bulk of this work is in kubeslice-controller, not worker-operator.

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

### 4. Rewire safety

**MVP decision: simple convergence.** When `hubs` changes, the controller does not orchestrate a phased transition. Every reconcile recomputes the desired edge set from `spec` alone and converges to it using the existing flow and its existing order:

1. `cleanupObsoleteGateways` deletes `WorkerSliceGateway` pairs whose edge is no longer in the desired set.
2. `createMinimumGatewaysIfNotExists` creates pairs for desired edges that do not exist yet.

This is idempotent and crash-safe by construction: the controller holds no in-memory or persisted transition state, so a restart at any point simply re-derives the desired set on the next reconcile and repairs whatever partial state exists. All gateway side effects (certificate generation via `GenerateCerts`, subnet/VPN address allocation via `BuildNetworkAddresses`, Secret cleanup and address reclamation) go through the existing provisioning and cleanup paths unchanged.

**Accepted trade-off:** between the deletion of an old hub link and the new hub link becoming ready, affected spokes lose slice connectivity for roughly the tunnel bring-up time (typically well under a minute). Hub changes are rare, administrator-initiated maintenance events, so a brief, bounded disruption is acceptable for the MVP. Zero-downtime rewiring (make-before-break) is documented future work and can be layered additively on the same resolver; nothing in the MVP design precludes it.

**Concurrent edit guard:** to keep transitions one-at-a-time and observable, the validation webhook rejects updates that change `hubs` while the slice's `TopologyConverged` condition is `False` (a previous topology change has not yet converged), with a clear "retry after convergence" error. Because the MVP controller never waits, convergence is normally reached within one tunnel bring-up cycle, so the lock window is short and cannot wedge: if edges fail to connect, the failure is reflected in the condition message and the administrator can still revert `hubs` to its previous value (reverting to the last-applied topology is always permitted).

### 5. Backward compatibility

Existing SliceConfigs with no `topology` field continue to operate with full-mesh behavior after upgrade. When `topology` is `nil`, the `TopologyResolver` returns the complete edge set (all pairs), and the controller keeps creating `WorkerSliceGateway` pairs for every cluster pair exactly as it does today. This resolver-level default is what preserves current behavior.

Full mesh is a controller-side behavior — generated by the existing pair-construction loop — and is not something an individual worker computes or falls back to on its own. Backward compatibility is achieved entirely at the resolver, by matching today's controller behavior when `topology` is absent.

### 6. Status aggregation

Per-link connectivity is reported on `WorkerSliceGateway.status`, since `WorkerSliceGateway` represents an individual gateway link between two clusters.

```yaml
status:
  connectionState: Connected      # Connected | NotConnected | Pending
  lastTransitionTime: "2026-06-24T10:05:00Z"
  reason: DialTimeout
  message: "gateway dial failed after 30s"
```

`lastTransitionTime` is updated only when the connection state changes, not on every reconcile.

The controller aggregates the status of all `WorkerSliceGateway` objects belonging to the Slice and reports overall topology health on `SliceConfig.status.conditions`.

This proposal adds a standard `Conditions []metav1.Condition` field to `SliceConfigStatus`, which currently only contains `KubesliceEvents []KubesliceEvent`.

```yaml
status:
  conditions:
  - type: TopologyConverged
    status: "True"
    reason: AllEdgesReady
    message: All desired topology links are established.
    lastTransitionTime: "2026-06-24T10:05:00Z"
```

`TopologyConverged` is:

* `True` when `failingEdges == 0` and `readyEdges == desiredEdges`.
* `False` when one or more desired gateway links are not connected. The condition message identifies the first failing gateway link.

When gateway links are removed (for example during hub rewiring), their corresponding `WorkerSliceGateway` objects are deleted through the existing cleanup flow and no stale status entries remain.

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
| 2 | `mode: HubAndSpoke` AND a name in `hubs` is not in `spec.clusters` | `Hub "X" is not a member of spec.clusters` |
| 3 | `mode: HubAndSpoke` AND all clusters are listed as hubs (no spokes) | `HubAndSpoke topology requires at least one spoke cluster` |
| 4 | `mode: FullMesh` AND `hubs` is non-empty | `hubs must be empty when mode is FullMesh` |
| 5 | `hubs` contains duplicate cluster names | `Duplicate hub entry: "X"` |
| 6 | `mode` is set to an unrecognized value | `Unknown topology mode "X"; valid values: FullMesh, HubAndSpoke` |
| 7 | `mode: HubAndSpoke` AND `len(spec.clusters) < 2` | `HubAndSpoke topology requires at least 2 clusters` |
| 8 | `len(spec.clusters) > MaxClusters` (delegates to the existing `MaxClusters` validation; HubAndSpoke does not bypass it) | `maximum number of clusters allowed is X` |
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

**Hub change (simple convergence):**
```
Operator   → PATCH SliceConfig (hubs: [hub-1] → [hub-2])
Webhook    → allowed (previous topology had converged; otherwise rejected
             with "retry after convergence")
Controller → TopologyResolver.Resolve() → new EdgeSet {hub-2↔spokes}
Controller → cleanupObsoleteGateways: delete hub-1 WorkerSliceGateway pair(s)
             no longer in the EdgeSet (address reclamation,
             credential/certificate cleanup via existing flow)
Controller → createMinimumGatewaysIfNotExists: create hub-2 pair(s)
             (GenerateCerts + BuildNetworkAddresses)
Workers    → SliceGwReconciler tears down old tunnel(s), brings up new ones
             (brief connectivity gap for affected spokes — accepted for MVP)
Workers    → report WorkerSliceGateway.status connectionState=Connected
Controller → aggregates statuses → TopologyConverged=True; hubs edits unlocked
```

---

## Non-goals

- Linear topology
- Auto hub selection
- Multi-hub load balancing
- Hub health-based failover (accepted single-hub SPOF risk for this term)
- Any topology mode other than `HubAndSpoke` and `FullMesh`

--- 