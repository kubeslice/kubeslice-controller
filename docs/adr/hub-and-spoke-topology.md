# ADR: Partial Mesh MVP — Hub-and-Spoke Topology

- **Status:** Proposed
- **Issue:** [#300](https://github.com/kubeslice/kubeslice-controller/issues/300)
- **Date:** 2026-05-05

---

## Context

KubeSlice currently establishes a **full mesh** of VPN gateways between all member clusters of a Slice. For N clusters this creates N×(N−1)/2 bidirectional gateway pairs. While full mesh delivers the lowest latency between any two clusters, it becomes operationally expensive at scale:

- 5 clusters → 10 gateway pairs, 10 cert-rotation cycles, 10 VPN tunnels
- 10 clusters → 45 gateway pairs

Many real workloads follow a **hub-and-spoke** pattern (e.g., centralised services cluster feeding edge/regional clusters). In such cases spoke-to-spoke links are unnecessary; eliminating them reduces resource usage, operational overhead, and failure blast-radius.

---

## Decision

Introduce a **`HubAndSpoke`** topology mode as the first and only topology type in the Partial Mesh MVP.

### Scope

**In scope:**
- API/CRD changes to `SliceConfig` (`topologyMode`, `hubs`, `spokes`)
- Controller computes desired gateway edges and reconciles `WorkerSliceGateway` objects
- `SliceConfig.Status.Topology` aggregates readiness summary
- Webhook validation for the new fields
- Kubernetes observability events for topology state

**Out of scope (non-goals for MVP):**
- Arbitrary adjacency-list / matrix graphs
- Auto hub-selection or optimization
- Multiple topology modes simultaneously
- Large-scale performance tuning

---

## Spec Examples (YAML)

### Example 1: One hub, spokes default to all other member clusters

```yaml
apiVersion: controller.kubeslice.io/v1alpha1
kind: SliceConfig
metadata:
  name: my-slice
  namespace: kubeslice-demo
spec:
  sliceSubnet: 192.168.0.0/16
  topologyMode: HubAndSpoke
  hubs:
    - hub-cluster
  clusters:
    - hub-cluster
    - spoke-a
    - spoke-b
    - spoke-c
```

Resulting edges: `hub-cluster ↔ spoke-a`, `hub-cluster ↔ spoke-b`, `hub-cluster ↔ spoke-c`  
*(No spoke-a ↔ spoke-b or spoke-a ↔ spoke-c links)*

### Example 2: Two hubs, explicit spoke list

```yaml
spec:
  topologyMode: HubAndSpoke
  hubs:
    - hub-east
    - hub-west
  spokes:
    - edge-1
    - edge-2
  clusters:
    - hub-east
    - hub-west
    - edge-1
    - edge-2
    - edge-3   # edge-3 is excluded because spokes list is explicit
```

Resulting edges: `hub-east ↔ edge-1`, `hub-east ↔ edge-2`, `hub-west ↔ edge-1`, `hub-west ↔ edge-2`

### Example 3: Full mesh (no topologyMode set — backward compatible)

```yaml
spec:
  clusters:
    - cluster-a
    - cluster-b
    - cluster-c
  # topologyMode not set → full mesh (default existing behaviour)
```

---

## Edge Computation Rules

The controller computes the set of desired gateway pairs using `computeDesiredPairs()`:

| Condition | Result |
|-----------|--------|
| `topologyMode` is empty | Full mesh: all N×(N−1)/2 pairs |
| `topologyMode: HubAndSpoke` | Only Hub↔Spoke pairs (see below) |
| `spokes` is empty | All clusters not in `hubs` become spokes automatically |
| `spokes` is non-empty | Only the listed clusters are spokes |
| Hub equals a spoke | Rejected by webhook validation |

**HubAndSpoke pair generation:**
```
For each hub H in hubs:
  For each spoke S in effective_spokes:
    if H ≠ S:
      add pair (H, S)
```

Pairs are **bidirectional**: a pair `(H, S)` generates one server-gateway on H and one client-gateway on S.

**Obsolete gateway cleanup:** Any existing `WorkerSliceGateway` whose pair is not in the desired set is deleted during reconciliation.

---

## Validation Rules and Error Handling

All rules are enforced by the admission webhook (`validateTopology`):

| Rule | Error type | Message |
|------|-----------|---------|
| `topologyMode` set to unknown value | `Invalid` | `"unsupported topology mode"` |
| `hubs` or `spokes` non-empty but `topologyMode` is empty | `Invalid` | `"TopologyMode is required when hubs or spokes are set"` |
| `hubs` list is empty when `topologyMode: HubAndSpoke` | `Required` | `"at least one hub is required for HubAndSpoke topology"` |
| `hubs` list has more than 2 entries | `Invalid` | `"HubAndSpoke topology supports a maximum of 2 hub clusters"` |
| Duplicate cluster names in `hubs` | `Duplicate` | field path `Spec.Hubs` |
| Duplicate cluster names in `spokes` | `Duplicate` | field path `Spec.Spokes` |
| Hub cluster not in `spec.clusters` | `Invalid` | `"hub must be part of clusters"` |
| Spoke cluster not in `spec.clusters` | `Invalid` | `"spoke must be part of clusters"` |
| A cluster appears in both `hubs` and `spokes` | `Invalid` | `"spoke cannot also be a hub"` |

---

## Topology Status (Observability)

After each reconciliation, `SliceConfig.Status.Topology` is updated:

```yaml
status:
  topology:
    mode: HubAndSpoke
    expectedConnections: 4   # number of desired hub↔spoke pairs
    createdConnections: 4    # number of WorkerSliceGateways matching desired pairs
    ready: true              # true when createdConnections == expectedConnections
```

### Kubernetes Events

| Event | Type | Trigger |
|-------|------|---------|
| `SliceTopologyHubAndSpokeEnabled` | Normal | Each reconcile when `topologyMode: HubAndSpoke` is set |
| `SliceTopologyReady` | Normal | When `createdConnections == expectedConnections` |
| `SliceTopologyDegraded` | Warning | When `createdConnections < expectedConnections` |

---

## Upgrade / Backward Compatibility Notes

1. **Existing slices without `topologyMode`** continue to behave as full mesh. No migration is required.
2. **Adding `topologyMode: HubAndSpoke` to an existing slice** triggers immediate cleanup of non-hub-spoke gateway pairs during the next reconciliation. This is a **disruptive operation** for spoke-to-spoke traffic — operators should plan a maintenance window.
3. **Changing hubs** (e.g., swapping hub-east for hub-west) causes the old hub's gateway pairs to be deleted and new pairs to be created. The topology status will transition through `Degraded → Ready` while new certs are generated.
4. **Removing `topologyMode`** (reverting to full mesh) is not yet supported via webhook (`preventUpdate` will reject changes to immutable fields in a future release). Currently, delete and recreate the SliceConfig.
5. The `TopologyStatus` sub-resource is additive — older controllers that do not understand it will ignore the field.

---

## Invariants

- A hub **must** be a member of `spec.clusters`.
- A spoke **must** be a member of `spec.clusters`.
- A cluster **cannot** appear in both `hubs` and `spokes`.
- `hubs` must contain **1 or 2** clusters (MVP constraint).
- When `spokes` is empty, all clusters in `spec.clusters` not in `hubs` automatically become spokes.
- If all clusters are hubs (i.e., effective spokes list is empty after defaulting), no gateway pairs are created and `TopologyStatus.Ready` is `true` with `expectedConnections: 0`.

---

## Consequences

### Positive
- Reduces gateway/cert count significantly for hub-spoke workload patterns
- Simpler failure domain: a spoke outage does not affect other spokes
- Aligns with common multi-cluster deployment topologies (e.g., central management + edge clusters)

### Negative / Trade-offs
- Spoke-to-spoke communication must route through a hub (increased latency for cross-spoke traffic)
- Hub is a single point of failure for spokes that rely on cross-cluster services
- With 2 hubs, each spoke maintains 2 tunnels instead of 1 (mitigated by the overall reduction vs full mesh)
