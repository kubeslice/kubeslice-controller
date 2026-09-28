# Hub-and-Spoke (Partial Mesh) test suite

Every test that covers the **Hub-and-Spoke partial-mesh topology** and its
**spoke-to-spoke routing** (issues #300–#304, #471), organized by what it
verifies and how to run it. It also documents the manual dataplane
end-to-end scenarios that were run on real Kind clusters, with the exact
commands and observed results.

**Transport scope:** this document covers the feature on **OpenVPN**, which is
fully tested end-to-end. WireGuard is **deferred** — see
[Section 7 Testing notes & coverage gaps](#7-testing-notes--coverage-gaps) for why the WireGuard dataplane
cannot come up yet (a pre-existing controller key-generation gap, outside this
feature).

The feature spans four repositories:

| Repo | Role |
|---|---|
| `kubeslice-controller` | Topology API + webhook validation, edge computation, marks each spoke's gateway with `RouteEntireSliceSubnet`, and aggregates gateway health into the SliceConfig `TopologyConverged` status |
| `worker-operator` | Programs the entire-slice route on **both** the spoke gateway pod and the slice router, and reports each gateway's tunnel connectivity back to the hub |
| `gateway-sidecar` | Installs the tunnel route as two more-specific halves so NSM cannot overwrite it |
| `apis` | Shared CRD types — the topology fields and the gateway connection-status fields |

---

## Layout

Coverage is layered; each layer catches a different class of bug and runs at a
different cost:

| Layer | Location | What it needs |
|---|---|---|
| Unit | `service/*_test.go` (controller), `controllers/slicegateway/*_test.go` (worker), `pkg/sidecar/sidecarpb/*_test.go` (sidecar) | nothing (pure Go / fake mocks) |
| Reconciler (envtest) | `controllers/controller/sliceconfig_hubandspoke_test.go` (controller) | envtest `etcd`/`kube-apiserver` binaries |
| Control-plane E2E (automated) | `test/e2e/*_test.go`, build tag `e2e` — run with `make test-e2e-hns` | `docker`, `kind`, `kubectl`, `make`; one disposable Kind cluster |
| Dataplane E2E (manual runbook) | [Section 5](#5-dataplane-end-to-end-runbook-openvpn) | `docker`, `kind`, `kubectl`, `helm`; real disposable Kind clusters |

The automated control-plane E2E suite (`test/e2e/`) builds this branch's
controller image, spins up **one** disposable Kind cluster prefixed `e2e-hns-`
(created and deleted within the run via `t.Cleanup`, never touching your own
clusters), runs the real controller (reconcilers + webhooks + CRDs) against
three fake pre-registered worker Cluster CRs, and asserts the control-plane
behaviour. It does **not** install real workers/NSM/gateways — that (the
dataplane) is the manual runbook in [Section 5](#5-dataplane-end-to-end-runbook-openvpn).

```bash
make test-e2e-hns   # go test -tags e2e -v -timeout 20m ./test/e2e/...
```

Scenarios (all pass; ~4 min total):

| Subtest | Asserts |
|---|---|
| `PartialMesh_HubAndSpokeSkipsSpokeToSpoke` | 4 gateways; hub=Server/`<none>`, spoke=Client/`true`; no spoke↔spoke |
| `FullMesh_Unaffected` | full mesh → 6 gateways, no gateway carries the flag |
| `TopologyChange_ReconcilesFlag` | FullMesh→HubAndSpoke re-sets the spoke flag to `true` |
| `HubChange_NoStaleServerFlag` | hub-change round-trip leaves no Server gateway with `route=true` |
| `Webhook_RejectsInvalidTopologies` | all 7 invalid topologies rejected at admission |
| `StatusFields_Persist` | #471 connection-status fields survive a status write |

Run the pure unit layer:

```bash
# controller topology + webhook + gateway-creation unit tests
cd kubeslice-controller && go test ./service/ -run 'TestResolveTopologyEdges|TestTopologyEdgeSetContains|test_validateTopology|HubAndSpoke'

# controller TopologyConverged status aggregation (#303 branch)
cd kubeslice-controller && go test ./service/ -run TestBuildTopologyConvergedCondition

# worker-operator route selection
cd worker-operator && go test ./controllers/slicegateway/ -run TestRemoteNsmSubnetForRoute

# worker-operator flag propagation (hub controller)
cd worker-operator && go test ./pkg/hub/controllers/ -run 'TestNewMeshGatewayConfig|TestStaticGatewayConfigChanged'

# worker-operator gateway connection status (feature/471-report-gateway-status)
cd worker-operator && go test ./pkg/hub/controllers/ -run 'TestDeriveGatewayConnectionState|TestReconcileGatewayConnectionStatus|TestReasonMessageForState'

# gateway-sidecar route split + MSS clamp
cd gateway-sidecar && go test ./pkg/sidecar/sidecarpb/ -run 'TestMoreSpecificHalves|TestTunnelMSSClampCommands|TestStaleTunnelRouteKeys'
```

> **Known gate — the controller `service` package.** On `master` today the
> `service` test binary does **not compile standalone**, for two reasons
> unrelated to this feature: `util.Client` is undefined and
> `ObjectMeta.ClusterName` was removed upstream. Both are fixed by **PR #404**
> (`fix/test-type-mismatch`). With #404 applied, the `service` commands above
> compile and pass (verified locally). The worker-operator and gateway-sidecar
> commands pass today as-is.

Run the reconciler (envtest) layer — needs envtest assets:

```bash
cd kubeslice-controller
export KUBEBUILDER_ASSETS=$(setup-envtest use 1.26.1 -p path)   # or your local assets path
go test ./controllers/controller/ -args -ginkgo.focus="HubAndSpoke topology"
```

The full `controllers/controller` suite also contains unrelated, occasionally
flaky specs (`vpnkey_rotation_controller_test.go`); the
`-ginkgo.focus="HubAndSpoke topology"` filter runs only this feature's specs.

---

## 1. Controller unit tests: topology edge computation

File: `service/topology_resolver_test.go`. Pure functions, no mocks.

### `TestResolveTopologyEdges` — 4 cases

`ResolveTopologyEdges(clusters, topology)` returns the desired gateway edges.

- **`nil topology is full mesh in cluster order`** — no topology → every cluster
  pair, in list order, with the entire-subnet flag OFF. The backward-compat
  guarantee: existing slices are unchanged.
- **`explicit FullMesh is full mesh`** — `mode: FullMesh` behaves identically to
  no topology.
- **`hub and spoke: hub is server, no spoke-to-spoke`** — hub=worker-1 →
  produces `worker-1↔worker-2` and `worker-1↔worker-3` (hub as server, flag ON
  on the spoke side) and **never** `worker-2↔worker-3`.
- **`hub is server even when not first in the cluster list`** — hub role is
  driven by `topology.hubs`, not by cluster ordering.

### `TestTopologyEdgeSetContains`

The desired-edge set is **direction-insensitive**: for hub=worker-1, both
`Contains("worker-1","worker-2")` and `Contains("worker-2","worker-1")` are
true (server-side and client-side gateway of a pair map to the same edge), and
the spoke↔spoke pair `worker-2↔worker-3` is **not** a member in either
direction. This is what lets cleanup delete a stale spoke↔spoke pair without
accidentally deleting one side of a desired pair.

---

## 2. Controller unit tests: topology webhook validation

File: `service/slice_config_webhook_validation_test.go` → `test_validateTopology`.
Wired into **both** `ValidateSliceConfigCreate` and `ValidateSliceConfigUpdate`,
so a topology change on a live slice is validated too.

11 table-driven cases — 3 accepted, 8 rejected (with the exact error text
asserted):

| # | Case | Result | Error contains |
|---|---|---|---|
| 1 | absent topology (full mesh default) | accept | — |
| 2 | explicit FullMesh without hubs | accept | — |
| 3 | valid HubAndSpoke with one hub | accept | — |
| 4 | HubAndSpoke without hubs | reject | `requires at least one hub` |
| 5 | hub not a member of clusters | reject | `is not a member of spec.clusters` |
| 6 | single-cluster HubAndSpoke | reject | `requires at least 2 clusters` |
| 7 | FullMesh with hubs | reject | `hubs must be empty when mode is FullMesh` |
| 8 | more than one hub (single-hub MVP) | reject | `only one hub is supported in this release` |
| 9 | unknown mode | reject | `unknown topology mode` |
| 10 | hubs without mode | reject | `mode must be set to HubAndSpoke when hubs is specified` |
| 11 | HubAndSpoke on a no-network slice | reject | `not supported for a no-network slice` |

Note: the single-hub restriction (case 8) is ordered **before** the duplicate
check, so `[worker-1, worker-1]` reports "only one hub is supported" (the more
specific error) rather than "duplicate".

---

## 3. Controller unit tests: gateway creation (partial mesh)

File: `service/worker_slice_gateway_service_test.go`. Uses the testify client
mock; asserts the exact set of gateway create / delete / update calls.

- **`TestCreateMinimumWorkerSliceGateways_HubAndSpokeSkipsSpokeToSpoke`** — for a
  HubAndSpoke slice with 3 clusters, exactly the two hub↔spoke pairs are
  processed (3 cluster fetches, 4 gateway existence checks); the spoke↔spoke
  pair is never created; each surviving spoke client gateway has
  `RouteEntireSliceSubnet` reconciled to `true`.
- **`TestCreateMinimumWorkerSliceGateways_HubAndSpokeCleansUpSpokeToSpoke`** — a
  pre-existing spoke↔spoke gateway pair (e.g. left from a FullMesh→HubAndSpoke
  switch), given the *correct* gateway number and with both clusters still
  members, is deleted **purely** because its edge is no longer in the desired
  topology.

**Regression (hub-change flag):** the reconcile now runs on **both** sides of an
existing pair — the client is set to the desired value, and the **server is
forced to `false`**. Without this, a hub change that turns a former client
gateway (flag `true`) into a server would leave a stale `true` on the hub side,
making the hub route the entire slice and misdirect all traffic. Fixed in
`reconcileRouteEntireSliceSubnet`; verified live in [Section 5 E9](#5-dataplane-end-to-end-runbook-openvpn).

---

## 3a. Controller unit tests: TopologyConverged status aggregation (#303)

File: `service/topology_status_test.go` → `TestBuildTopologyConvergedCondition`.
`buildTopologyConvergedCondition` folds every gateway's connection state into a
single `TopologyConverged` condition on the SliceConfig. 5 cases:

- **`no gateways is trivially converged`** — an empty gateway set converges.
- **`all connected is converged`** — every gateway `Connected` → converged.
- **`one not connected is not converged and is named`** — a single down gateway
  flips the condition and the message names it.
- **`empty connection state is reported as Pending`** — an unset state is treated
  as `Pending`, not silently "up".
- **`first not-ready is chosen deterministically by name`** — when several are
  not ready, the one reported is chosen by name so the message is stable.

---

## 4. Controller reconciler tests (envtest)

File: `controllers/controller/sliceconfig_hubandspoke_test.go` (Ginkgo, real API
server via envtest). 6 specs:

- **`builds only hub<->spoke gateways and skips spoke<->spoke`** — applying a
  HubAndSpoke SliceConfig creates the 4 WorkerSliceGateway objects (2 pairs) and
  no spoke↔spoke object.
- **`full mesh is unaffected: builds all gateway pairs with no entire-subnet
  routing`** — a FullMesh (or no-topology) slice builds all pairs and never sets
  `RouteEntireSliceSubnet`.
- **`reconciles RouteEntireSliceSubnet when the topology changes on an existing
  slice`** — switching an existing slice's topology updates the flag on the
  surviving gateways (the control-plane half of the E8/E9 dataplane scenarios).
- **`clears the flag on a gateway that becomes the hub side after a hub change`** —
  a hub change turns a former spoke client (flag `true`) into the new hub's server;
  the flag must be cleared to `false` so the hub doesn't route the whole slice, and
  set on the gateway that becomes the new spoke client (direct test of the
  both-sides reconcile).
- **`marks a no-network slice TopologyConverged=True with NoGatewaysRequired`** — a
  NONET slice still gets a `TopologyConverged` condition (guards the early-return
  path that previously skipped the status write).
- **`re-creates a missing client gateway of a partial pair (self-heal)`** — with the
  server gateway present but the client deleted, the reconcile re-creates the client
  instead of getting stuck on an AlreadyExists error re-touching the server.

---

## 4a. Worker-operator unit tests: route selection & flag propagation

**Route selection** — file: `controllers/slicegateway/slicegateway_route_test.go`
→ `TestRemoteNsmSubnetForRoute`. 3 cases:

- **full-mesh / normal gateway** → routes the **peer** gateway's subnet.
- **spoke→hub gateway** → routes the **entire slice** subnet (so the spoke sends
  all slice traffic, including other spokes', to the hub).
- **spoke→hub gateway, slice subnet unknown** → returns **not ready**, telling
  the caller to requeue instead of programming a wrong route.

**Flag propagation** — file: `pkg/hub/controllers/slicegateway_config_test.go`.
The hub controller copies the controller-set `RouteEntireSliceSubnet` from the
`WorkerSliceGateway` spec onto the local `SliceGateway.Status.Config` that the
dataplane reads:

- **`TestNewMeshGatewayConfig_PropagatesRouteEntireSliceSubnet`** — the flag is
  copied in both states (`true`/`false`), along with the other config fields.
- **`TestStaticGatewayConfigChanged_DetectsRouteFlag`** — a flag flip is detected
  as a change (so the worker re-syncs when the controller toggles it), and an
  already-matching config reports no change (no status churn).

## 4b. Gateway-sidecar unit tests: tunnel route split & MSS clamp

File: `pkg/sidecar/sidecarpb/route_split_test.go`.

**`TestMoreSpecificHalves`** — the route split, 5 cases:

- `10.11.0.0/16` → `10.11.0.0/17` + `10.11.128.0/17`
- `10.11.0.0/20` → `10.11.0.0/21` + `10.11.8.0/21`
- `10.11.32.3/32` → unchanged (a host route is not split)
- `10.11.0.0/31` → `10.11.0.0/32` + `10.11.0.1/32` (smallest splittable v4)
- `fd00::/48` → `fd00::/49` + `fd00:0:0:8000::/49` (IPv6 splits too)

**`TestStaleTunnelRouteKeys`** — the teardown diff. On a topology/subnet change the
sidecar withdraws the tunnel routes it previously installed that are no longer
desired (e.g. the entire-slice `/17`s when a slice flips HubAndSpoke→FullMesh),
so a stale relay route isn't left behind. Verifies the previous-vs-desired set
difference: full flip (both old routes stale), no-change (nothing stale), and
partial overlap (only the non-desired key stale).

**Why split at all:** NSM continuously re-asserts a route for the whole slice
subnet via `nsm0` using the *same* prefix the tunnel wants. A `RouteReplace` on
that exact prefix only wins until NSM re-asserts, so the route flaps and
spoke-to-spoke traffic intermittently black-holes. Installing two strictly
more-specific halves means longest-prefix match always selects the tunnel and
NSM can never overwrite it.

**`TestTunnelMSSClampCommands`** — the TCP MSS clamp rule. Verifies the
`iptables` command targets the mangle `FORWARD` chain on the given interface
with `--clamp-mss-to-pmtu`, that the check (`-C`) and add (`-A`) commands differ
only in that verb, and that the interface is parameterized (correct for any
tunnel, not hard-wired). The clamp keeps full-size TCP segments from
black-holing across the smaller-MTU tunnel — needed for spoke-to-spoke, which
crosses two tunnels.

## 4c. Worker-operator unit tests: gateway connection status (#471)

File: `pkg/hub/controllers/slicegateway_status_test.go` (worker-operator). The
worker derives each gateway's tunnel state from its pod statuses and reports it
onto the hub's `WorkerSliceGateway` status — the write side of the fields
checked live in [Section 5 E12](#5-dataplane-end-to-end-runbook-openvpn).

- **`TestDeriveGatewayConnectionState`** — 7 cases: no pod status → `Pending`;
  all pods up → `Connected`; at least one up → `Connected` (HA pair); all down →
  `NotConnected`; unknown/empty pod states are not counted as up; nil pod entries
  are ignored; all-nil pod entries (nothing reported) → `Pending`.
- **`TestReconcileGatewayConnectionStatus`** — 2 subtests: writes `Connected`
  when a tunnel is up; **no write when the state is unchanged** (idempotent —
  avoids status churn).
- **`TestReasonMessageForState`** — the state → (reason, message) mapping, e.g.
  `Connected` → (`TunnelEstablished`, `gateway tunnel is up`).

---

## 5. Dataplane end-to-end runbook (OpenVPN)

Real setup: 1 controller + 3 worker Kind clusters, NSM, real OpenVPN gateways,
running the feature images. Slice `10.11.0.0/16`, `HubAndSpoke`, hub = worker-1,
spokes = worker-2 (`10.11.16.x`) and worker-3 (`10.11.32.x`).

> Contexts below: `kind-kubeslice-controller`, `kind-kubeslice-worker1|2|3`.
> `CPOD` = the `iperf-sleep` client pod on worker-2; `SIP` = the `iperf-server`
> pod's slice IP on worker-3.

### E1 — Gateway roles + flag
```bash
kubectl get workerslicegateways -n kubeslice-avesha --context kind-kubeslice-controller \
  -o custom-columns=NAME:.metadata.name,HOST:.spec.gatewayHostType,ROUTE:.spec.routeEntireSliceSubnet
```
**Expect:** 4 gateways — hub (`worker-1-worker-*`) = `Server` / `<none>`, spoke
(`worker-2|3-worker-1`) = `Client` / `true`.

### E2 — No spoke↔spoke gateway
```bash
kubectl get workerslicegateway demo-hub-and-spoke-worker-2-worker-3 -n kubeslice-avesha --context kind-kubeslice-controller
```
**Expect:** `NotFound`.

### E3 — Route split is live (the core fix)
```bash
kubectl exec <spoke-1 gateway pod> -n kubeslice-system -c kubeslice-sidecar \
  --context kind-kubeslice-worker2 -- ip route
```
**Expect:** `10.11.0.0/17` and `10.11.128.0/17 via 10.11.255.1 dev tun0`,
out-specifying NSM's `10.11.0.0/16 via ... nsm0`.

### E4 — Spoke→spoke ping (relayed via hub)
```bash
kubectl exec $CPOD -n iperf -c sidecar --context kind-kubeslice-worker2 -- ping -c 5 $SIP
```
**Expect:** 0% packet loss; `ttl` reduced (a hop through the hub). Verified: `5 received, 0% packet loss`.

### E5 — Spoke→spoke iperf (throughput)
```bash
kubectl exec $CPOD -n iperf -c iperf --context kind-kubeslice-worker2 -- iperf -c $SIP -p 5201 -t 8 -i 2
```
**Expect:** a steady TCP transfer. Verified: ~4.4 Mbit/s (a local-Kind number,
not a benchmark).

### E6 — HA gateway failover
```bash
# with a continuous ping running, delete the ACTIVE spoke gateway pod:
kubectl delete po <spoke-1 active gateway> -n kubeslice-system --context kind-kubeslice-worker2
```
**Expect:** killing the **standby** = 0% loss; killing the **active** = ~15–20 s
disruption then self-recovers; the pod is replaced automatically. Verified:
~18 s window, recovered to 0% loss, pair back to `Running`.

### E7 — Tunnel down via firewall rule
```bash
# block on BOTH spoke-1 gateway pods:
kubectl exec <gw> -n kubeslice-system -c kubeslice-sidecar --context kind-kubeslice-worker2 -- iptables -I FORWARD -o tun0 -j DROP
# ... ping now fails ...
kubectl exec <gw> -n kubeslice-system -c kubeslice-sidecar --context kind-kubeslice-worker2 -- iptables -D FORWARD -o tun0 -j DROP
```
**Expect:** 100% loss while blocked → 0% after removing the rule. Verified.

### E8 — Topology change: FullMesh ↔ HubAndSpoke
```bash
# to full mesh:
kubectl patch sliceconfig demo-hub-and-spoke -n kubeslice-avesha --context kind-kubeslice-controller --type=json -p='[{"op":"remove","path":"/spec/topology"}]'
# back to hub-and-spoke:
kubectl patch sliceconfig demo-hub-and-spoke -n kubeslice-avesha --context kind-kubeslice-controller --type=merge -p='{"spec":{"topology":{"mode":"HubAndSpoke","hubs":["worker-1"]}}}'
```
**Expect:** FullMesh → 6 gateways, flag cleared on all; HubAndSpoke → 4
gateways, spoke↔spoke removed, spoke flags reconciled back to `true`. Verified.

### E9 — Hub change round-trip (the flag-fix scenario)
```bash
kubectl patch sliceconfig demo-hub-and-spoke -n kubeslice-avesha --context kind-kubeslice-controller --type=merge -p='{"spec":{"topology":{"mode":"HubAndSpoke","hubs":["worker-2"]}}}'
kubectl patch sliceconfig demo-hub-and-spoke -n kubeslice-avesha --context kind-kubeslice-controller --type=merge -p='{"spec":{"topology":{"mode":"HubAndSpoke","hubs":["worker-1"]}}}'
```
**Expect:** after the round-trip, **no `Server` gateway carries
`routeEntireSliceSubnet=true`** (the fix), and the dataplane recovers to 0% loss.
Verified. Note: a hub change is disruptive — gateway pods rebuild and the
dataplane takes several minutes to reconverge.

### E10 — Add / remove a spoke
```bash
# remove worker-3 (update clusters AND applicationNamespaces together):
kubectl patch sliceconfig demo-hub-and-spoke -n kubeslice-avesha --context kind-kubeslice-controller --type=merge \
  -p='{"spec":{"clusters":["worker-1","worker-2"],"namespaceIsolationProfile":{"applicationNamespaces":[{"namespace":"iperf","clusters":["worker-1","worker-2"]}],"isolationEnabled":false}}}'
# add it back: same patch with worker-3 restored in both lists.
```
**Expect:** remove → 2 gateways, worker-3 unreachable; add back → 4 gateways,
dataplane recovers to 0% loss (several-minute reconverge). Verified.

### E11 — Webhook rejects invalid topologies
Apply the 7 invalid SliceConfigs (two hubs, hub-not-member, no-hubs,
hubs-without-mode, FullMesh-with-hubs, unknown-mode, duplicate-hubs). **Expect:**
each rejected at admission with the matching error from [Section 2](#2-controller-unit-tests-topology-webhook-validation).

### E12 — Connection-status fields persist (#471)
```bash
kubectl patch workerslicegateway <gw> -n kubeslice-avesha --context kind-kubeslice-controller --subresource=status --type=merge \
  -p '{"status":{"connectionState":"Connected","reason":"TunnelEstablished","message":"up","lastTransitionTime":"2026-01-01T00:00:00Z"}}'
kubectl get workerslicegateway <gw> -n kubeslice-avesha --context kind-kubeslice-controller -o jsonpath='{.status.connectionState}|{.status.reason}|{.status.message}'
```
**Expect:** the values persist. Before #471 the CRD lacked these fields and the
API server pruned them.

---

## 6. Coverage map (issue → tests)

| Issue / PR | Scope | Covered by |
|---|---|---|
| #300 / #406 | ADR: partial-mesh MVP design | design doc |
| #301 / #408 | Topology API + webhook | Section 2 (`test_validateTopology`, 10 cases) |
| #302 / #410 | Edge computation | Section 1 (`TestResolveTopologyEdges`, `TestTopologyEdgeSetContains`) |
| #303 / #422 | Gateway health → `TopologyConverged` status | Section 3a (`TestBuildTopologyConvergedCondition`, 5 cases) |
| #304 / #424 | Spoke gateways route entire slice + e2e | Section 3, Section 4, Section 4a, Section 4b; Section 5 |
| #471 / apis#44 / worker#495 | Gateway connection-status fields | Section 4c (worker derive/reconcile/reason); Section 5 E12 |
| worker #496 | Program entire-slice route (pod + router) | Section 4a; Section 5 E3–E5 |
| gateway-sidecar #64 | Route split + MSS clamp | Section 4b; Section 5 E3 |

---

## 7. Testing notes & coverage gaps

What could not be tested, and behaviour observed while running the suites above.

- **WireGuard dataplane could not be tested (blocked, pre-existing).** The
  control-plane *was* tested on WireGuard and passes — a `sliceGatewayType:
  Wireguard` HubAndSpoke slice creates the correct gateways with the flag. But
  the WireGuard **dataplane** cannot be exercised: the gateway pod gets stuck
  `Init:0/1` with `references non-existent secret key: serverPublicKeyWgFile`,
  because the controller has **no WireGuard key-generation code** (`GenerateCerts`
  only produces the OpenVPN config, so the gateway secret contains
  `ovpnConfigFile` instead of WireGuard keys). This is a pre-existing gap in the
  controller's cert-generation subsystem, outside the spoke-to-spoke feature, so
  every dataplane scenario in Section 5 is verified on **OpenVPN only**. Testing
  WireGuard end-to-end first needs WireGuard key generation in the controller.
- **Observed: active-gateway failover is not instant (~15–20 s).** In scenario E6,
  killing the *standby* gateway caused no disruption (0% loss); killing the *active* one
  disrupted traffic for ~15–20 s while the slice router's ECMP withdrew the dead
  next-hop, then recovered. Faster failover would need health-check-driven
  next-hop withdrawal; the `/17` route split hardens the steady-state route but
  does not speed up this reconvergence.
- **Out of scope: changing the hub on a live slice.** E9 verifies the
  `RouteEntireSliceSubnet` flag reconciles correctly on a hub change (no stale
  `true` on a server), but gateway VPN roles (`gatewayHostType`) are fixed at
  creation and not re-assigned, so a live hub change leaves the old-hub↔new-hub
  pair with stale roles. The hub is meant to be set at slice creation; to change
  it, recreate the slice.
- **The feature ships as stacked PRs, so its tests live across several
  branches.** No single branch has all of them checked out at once yet:
  topology + edges + spoke routing + e2e (Sections 1–4b) are on the #304 branch;
  `TopologyConverged` aggregation (Section 3a) is on the #303 branch (#422); the
  worker connection-status tests (Section 4c) are on the worker
  `feature/471-report-gateway-status` branch (#471/#495). They come together on
  the `integration/hub-and-spoke-e2e` branch, which should be brought up to date
  so `make test` / `make test-e2e-hns` run the whole suite in one place. This
  document is the single reference for **all** of them regardless of branch.
- **The `service` unit tests need PR #404** to compile as a standalone test
  binary (see [Layout](#layout)) — the one command in this doc that does not run
  green today.
