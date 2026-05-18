# KubeSlice Controller — Active/Standby High Availability

> POC for *CNCF — KubeSlice: Controller HA (Active/Standby) Support (2026 Term 2)*

## Problem

The KubeSlice controller currently runs as a single replica. If that pod
crashes, is evicted, or its node fails, the entire multi-cluster control plane
stalls: no SliceConfig reconciliation, no gateway provisioning, no VPN key
rotation. Recovery time is bounded by pod rescheduling plus a cold cache warm-up.

This POC implements the foundational layer for an **Active/Standby** topology:
two (or more) controller replicas run concurrently; exactly one is **Active**
and reconciles, while the others stay **Standby** — warm, observing state, and
ready to take over within one lease period.

## Design

```
                ┌───────────────────────── HAManager ─────────────────────────┐
                │  orchestrates components, owns ControllerState, exposes      │
                │  IsActive() / WaitForActivePromotion() / metrics             │
                └───┬───────────────────┬───────────────────────┬─────────────┘
                    │                   │                       │
          ┌─────────▼────────┐ ┌────────▼─────────┐  ┌───────────▼──────────┐
          │ LeaderElection   │ │ StateSynchronizer│  │   HealthChecker      │
          │ coordination/v1  │ │ versioned        │  │ observes Active      │
          │ Lease, client-go │ │ snapshots + diff │  │ lease freshness      │
          └─────────┬────────┘ └────────┬─────────┘  └───────────┬──────────┘
                    │                   │                        │
                    └───────────────────┴────────────────────────┘
                                        │
                              Kubernetes API server
                              (Lease + KubeSlice CRDs)

   reconcilers ── wrapped by ──▶ ActiveGuard ──▶ runs only while role == Active
```

### Components

| Component | Responsibility |
|-----------|----------------|
| **LeaderElection** | Lease-based election via `client-go/tools/leaderelection`. Releases the lease on context cancellation so failover is fast. Transitions are surfaced through callbacks. |
| **StateSynchronizer** | Periodically collects KubeSlice CRDs into a revisioned `Snapshot` and computes the `SnapshotDiff` (added/removed/updated) vs. the prior revision. Keeps a Standby warm and gives an Active a current view immediately on promotion. |
| **HealthChecker** | Watches the Active controller's `Lease` renewal time. Flags the Active unhealthy only after `Threshold` consecutive failures — debouncing transient API errors. The stale-lease detection is panic-safe for leases missing optional fields. |
| **HAManager** | Wires the three components, owns `ControllerState`, drives Active↔Standby transitions, triggers an immediate resync on promotion, and exports Prometheus metrics. |
| **ActiveGuard** | A `reconcile.Reconciler` wrapper. On a Standby it drops requests without requeue and without mutating cluster state; controller-runtime's resync re-enqueues work once the instance is promoted. |

### Abstractions for testability

`ResourceCollector` and `LeaseReader` are narrow interfaces. Production uses
`NewKubeSliceCollector` (lists SliceConfig, Cluster, VpnKeyRotation,
ServiceExportConfig) and `NewLeaseReader` (controller-runtime client). Tests
inject in-memory implementations, so the suite exercises real synchronizer and
health logic — not stubs — without a live API server.

## Failover sequence

1. Active controller dies; its lease stops being renewed.
2. Standby `HealthChecker` observes the stale lease and flags it unhealthy.
3. `LeaderElection` on a Standby acquires the expired lease.
4. `HAManager` transitions to `RoleActive`, increments the transition counter,
   runs an immediate `Sync`, and signals `WaitForActivePromotion` waiters.
5. `ActiveGuard` now lets reconcile requests through; the next resync replays
   outstanding work against current state.

## Metrics

Registered with the controller-runtime registry:

| Metric | Type | Meaning |
|--------|------|---------|
| `kubeslice_ha_active` | gauge | 1 if this instance is Active |
| `kubeslice_ha_leadership_transitions_total` | counter | Active/Standby transitions |
| `kubeslice_ha_synced_resources` | gauge | resources in latest snapshot |
| `kubeslice_ha_last_sync_timestamp_seconds` | gauge | last successful sync time |
| `kubeslice_ha_active_healthy` | gauge | 1 if Active lease looks healthy |
| `kubeslice_ha_reconcile_skipped_total` | counter | reconciles skipped while Standby |

## Integration sketch

```go
hm, err := ha.NewHAManager(ha.HAManagerConfig{
    Identity:   os.Getenv("POD_NAME"),
    Namespace:  "kubeslice-controller",
    LeaseName:  "kubeslice-controller-ha",
    Logger:     logger,
    KubeClient: kubeClientset, // leader election
    Client:     mgr.GetClient(), // state collection + lease reads
})
if err != nil { return err }
go hm.Start(ctx)

// Gate every reconciler so only the Active instance writes.
guarded := ha.NewActiveGuard(hm, sliceConfigReconciler, logger)
```

## Testing

34 unit tests, run with `-race`:

```
go test -race ./pkg/ha/...
ok  	github.com/kubeslice/kubeslice-controller/pkg/ha
```

Coverage highlights: snapshot diffing (add/remove/update), collector-failure
resilience, health threshold + recovery, zero-lease-duration panic safety,
Active/Standby transitions, promotion-triggered resync, and `ActiveGuard`
gating.

## Scope and next steps

This POC delivers the HA control loop and the reconcile gate. Out of scope,
proposed for follow-up phases:

- Wiring `ActiveGuard` into every controller in `main.go`.
- Promoting the snapshot into a true warm informer cache.
- Worker-cluster endpoint updates on failover and in-flight reconcile handling.
- `envtest`-based integration tests simulating real lease expiry.
- Status conditions / events reflecting HA role on the controller's own CR.
