# Active/Standby HA runbook

Operator procedures for the cross-cluster HA controller (issues #293–#297,
observability per #298).

Everything below is written against the shipped manifests and the current code,
not against a generic kubebuilder layout. Two names differ from what you may
expect and both matter:

- The controller's namespace on a hub is **`kubeslice-controller`**, not
  `kubeslice-system`. `kubeslice-system` is a *worker* namespace (ADR #293
  Decision 1) and does not exist on a hub cluster at all.
- The Deployment is **`kubeslice-controller-manager`**.

Substitute your own contexts for `<active-ctx>` and `<standby-ctx>` throughout.

---

## 0. Vocabulary: there are two Leases, and they are unrelated

A hub cluster carries two `coordination.k8s.io/v1` Leases and confusing them
leads to the wrong conclusion every time:

| Lease | What it is |
|---|---|
| `kubeslice-controller-ha` | **The HA lease.** Cross-cluster: which *hub* is Active. What a Standby watches and what promotion acquires. |
| `<hash>.kubeslice.io` (e.g. `6a2ced6b.kubeslice.io`) | controller-runtime's own `--leader-elect` lease — which *pod* leads inside one cluster. Nothing to do with HA. |

```
kubectl --context <active-ctx> -n kubeslice-controller \
  get lease kubeslice-controller-ha \
  -o custom-columns=HOLDER:.spec.holderIdentity,RENEWED:.spec.renewTime
```

A stale `renewTime` on that Lease is the single fact the whole failover mechanism
turns on.

---

## 1. Verify Active/Standby status

### Reading the metrics endpoint

The manager binds its metrics to `127.0.0.1:8080` and the `kube-rbac-proxy`
sidecar re-exposes them with TLS and authorization on `8443`. Two ways in.

**Port-forward (simplest, and what to use while debugging).** `kubectl
port-forward` attaches to the pod's own network namespace, so the
loopback-bound port is reachable even though nothing outside the pod can dial
it directly:

```
kubectl --context <active-ctx> -n kubeslice-controller \
  port-forward deploy/kubeslice-controller-manager 8080:8080
```

```
curl -s localhost:8080/metrics | grep kubeslice_controller_ha_
```

**Through the proxied Service (what a scraper does).** Requires a token bound
to the `kubeslice-controller-metrics-reader` ClusterRole; the Service is
`kubeslice-controller-controller-manager-metrics-service:8443`.

> If `/metrics` returns nothing through the sidecar, check that the manager is
> started with `--metrics-secure=false`. The sidecar is configured with
> `--upstream=http://127.0.0.1:8080/`, but that flag defaults to *true*, which
> makes controller-runtime serve TLS on 8080 — plain HTTP against a TLS listener,
> and the endpoint goes dark. `config/default/manager_auth_proxy_patch.yaml`
> sets it correctly; a Helm-deployed controller needs the same.

### Which hub is Active

```
curl -s localhost:8080/metrics | grep kubeslice_controller_ha_leader_status
```

`1` = this instance holds leadership and its reconcilers are writing. `0` = it is
not writing. Run it against both hubs: **exactly one should report 1.**

Two `1`s at once is split brain — go to §6.

Two `0`s means no hub is reconciling. Either a promotion is in flight (check
`ha_promotion_duration_seconds`), or an Active has lost its Lease without a
Standby taking over (§5).

### Is the Standby actually protecting anything

A Standby that cannot read the Active's Lease will never promote, and looks
perfectly healthy until the day it is needed. Two gauges answer this:

```
curl -s localhost:8080/metrics | grep -E 'ha_armed|ha_remote_lease_age_seconds'
```

- `ha_armed 0` on a Standby → **it has never once read the Active's Lease and
  cannot fail over.** Almost always credentials or RBAC; go to §5.
- `ha_remote_lease_age_seconds` → seconds since the Active last renewed. Should
  hover below `--ha-lease-duration`. This is the leading indicator: it climbs
  *before* a failover, so it is the one to alert on.

### Recent transitions

```
kubectl --context <active-ctx> -n kubeslice-controller get events \
  --field-selector reason=PromotedToActive
```

The full set of HA reasons, all recorded against the `kubeslice-controller-ha`
Lease so a single `get events -n kubeslice-controller` shows the lot:

| Reason | Type | Meaning |
|---|---|---|
| `BecameActive` | Normal | Started in Active mode |
| `BecameStandby` | Normal | Started in Standby mode |
| `PromotedToActive` | Normal | Completed a promotion |
| `LeadershipLost` | Warning | Failed to renew past `--ha-renew-deadline`; stopped reconciling |
| `PromotionAborted` | Warning | Considered promoting and refused — see `ha_promotions_aborted_total` for which guard |
| `HAMirrorSyncFailed` | Warning | Mirror could not apply an object; retrying |

> `HAMirrorSyncFailed` is the event issue #298's table calls `SyncError`. The
> name shipped earlier (#295) and is left alone rather than renamed under
> anyone's existing alerts.

### How long has this hub been Active

```
curl -s localhost:8080/metrics \
  | grep kubeslice_controller_ha_last_promotion_timestamp_seconds
```

Subtract from `time()` for the age. Absent means *this process* has not promoted
— it does not mean the hub never did. Both this and `ha_failover_total` reset on
restart; the durable record is the `PromotedToActive` Event and the Lease's
`holderIdentity`.

---

## 2. Simulate a failover

A deliberate test. Expect a window of `--ha-lease-duration` +
`--ha-padding-seconds` in which no hub reconciles.

**Before you start**, confirm the Standby is armed (§1) — otherwise you are
testing nothing and the "failure" will be permanent.

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  logs deploy/kubeslice-controller-manager -c manager \
  --tail=20 | grep -i 'armed\|active hub lease'
```

Stop the Active:

```
kubectl --context <active-ctx> -n kubeslice-controller \
  scale deploy/kubeslice-controller-manager --replicas=0
```

Watch the Standby decide:

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  logs -f deploy/kubeslice-controller-manager -c manager \
  | grep -iE 'stale|promot|leadership'
```

The sequence to expect, in this order:

1. `active hub lease is STALE; evaluating promotion`
2. `state mirror stopped and confirmed exited`
3. `acquired lease on this hub`
4. `published activeController for the new Active`
5. `re-enqueued all reconciled types after promotion`
6. `PROMOTED to active`

Then confirm on the promoted hub:

```
curl -s localhost:8080/metrics | grep -E 'ha_leader_status|ha_failover_total'
```

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  get lease kubeslice-controller-ha \
  -o jsonpath='{.spec.holderIdentity}'
```

And that workers were told:

Cluster CRs live in the project namespace (`kubeslice-<project>`, e.g.
`kubeslice-avesha`), not in `kubeslice-controller`:

```
kubectl --context <standby-ctx> -n <project-namespace> \
  get cluster -o custom-columns=\
NAME:.metadata.name,ACTIVE:.status.activeController.activeIdentity,\
ENDPOINT:.status.activeController.endpoint
```

> Do **not** look for `status.conditions` on a Cluster CR. `ClusterStatus` has no
> `Conditions` field — only `clusterHealth.componentStatuses`, which is rebuilt
> from scratch on every pass. Worker-side connection health is tracked
> separately in worker-operator #469.

### Restoring afterwards

Bring the old Active back as a **Standby**, or you will have two Actives. It
needs `--ha-mode=standby` and a kubeconfig pointing at the newly promoted hub —
see §3, which is the same procedure.

```
kubectl --context <active-ctx> -n kubeslice-controller \
  scale deploy/kubeslice-controller-manager --replicas=1
```

> **Objects the demoted hub created itself do not go away, and cannot be deleted
> while it is a Standby.** They carry reconciler finalizers but no
> `ha.kubeslice.io/synced-from` label, so two rules combine against them: the
> mirror only manages objects it created, and the write fence stops this hub's
> reconcilers from clearing a finalizer. A `kubectl delete` against one hangs
> indefinitely with `deletionTimestamp` set and nothing to remove the finalizer.
>
> Both behaviours are correct — a Standby must not write, and the mirror must not
> delete objects it does not own — but the leftovers are real. To clear one:
>
> ```
> kubectl -n <project-namespace> patch <kind> <name> \
>   --type=merge -p '{"metadata":{"finalizers":null}}'
> ```
>
> Check for them after any role swap:
>
> ```
> kubectl --context <standby-ctx> -n <project-namespace> get <kind> \
>   -o json | jq -r '.items[]
>   | select(.metadata.labels["ha.kubeslice.io/synced-from"] == null)
>   | .metadata.name'
> ```

---

## 3. Rotate the Active kubeconfig credential

The Standby reaches the Active with a kubeconfig in a Secret, mounted at the
path given by `--ha-active-kubeconfig`:

| | |
|---|---|
| Secret | `ha-active-kubeconfig` in `kubeslice-controller` on the **Standby** |
| Key | `active.kubeconfig` |
| Mounted at | `/var/run/ha/active.kubeconfig` (per the deployment's flag) |

Rotate when the credential expires, when the Active's API server certificate
changes, or **whenever the Active's address changes**.

> **The trap, and the most common cause of a dead Standby.** This kubeconfig
> embeds both the Active's `server:` URL *and* its CA bundle. On Docker-based
> clusters (kind), node IPs are assigned in container start order and **are not
> stable across restarts** — a host reboot can permute them between clusters.
> The Standby then dials an address that now belongs to a *different* cluster,
> presenting a different CA, and fails with:
>
> ```
> tls: failed to verify certificate: x509: certificate signed by unknown authority
> ```
>
> That reads like a broken credential and is not one — it is a stale address.
> **Re-derive the IPs before regenerating anything:**
>
> ```
> docker inspect -f \
>   '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \
>   <cluster>-control-plane
> ```

Check what the Standby currently believes:

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  get secret ha-active-kubeconfig \
  -o jsonpath='{.data.active\.kubeconfig}' \
  | base64 -d | grep -E 'server:|certificate-authority'
```

Replace it:

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  create secret generic ha-active-kubeconfig \
  --from-file=active.kubeconfig=<path-to-new-kubeconfig> \
  --dry-run=client -o yaml | kubectl --context <standby-ctx> apply -f -
```

The kubeconfig is read once at start-up, so the Standby must restart:

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  rollout restart deploy/kubeslice-controller-manager
```

Nothing is lost by restarting. The mirror is rebuilt from the Active on every
start, and the reverse-diff prune pass reconciles anything missed while the
Standby was down. Confirm it came back armed:

```
curl -s localhost:8080/metrics | grep -E 'ha_armed|ha_remote_lease_reads_total'
```

`ha_remote_lease_reads_total{result="ok"}` must be climbing. If only
`result="error"` climbs, the new credential is not working either.

The identity this kubeconfig authenticates as also needs read access **on the
Active hub** — see `config/ha/README.md`, which is applied there, not here.

---

## 4. Troubleshoot: sync lag is high

Symptom: the Standby's copy of the world is behind the Active's.

**Distinguish "slow" from "stuck" first** — these have different causes and the
two metrics disagree deliberately:

```
curl -s localhost:8080/metrics \
  | grep -E 'ha_sync_lag_seconds|ha_sync_queue_depth|ha_sync_errors_total'
```

- **Lag high, depth low** → each object takes a long time; look at latency to the
  Active hub.
- **Lag normal, depth climbing** → the syncer is keeping up with only a fraction
  of the work. Lag is only observed for items that *completed*, so a healthy lag
  figure here is survivorship bias. Raise `--ha-sync-workers`.
- **`ha_sync_errors_total` climbing** → real failures, retrying with backoff.
  Break down by `kind` and `operation`; the matching `HAMirrorSyncFailed` Events
  name the specific objects.

Then check connectivity to the Active, which underlies all three:

```
curl -s localhost:8080/metrics \
  | grep -E 'ha_remote_lease_reads_total|ha_remote_lease_age_seconds'
```

Errors here mean the problem is the link or the credential (§3), not the syncer.

### The drift backstop

```
curl -s localhost:8080/metrics | grep ha_prune_
```

- `ha_prune_last_run_timestamp_seconds` far older than `--ha-sync-interval` → the
  prune pass is not running. It waits for the remote cache to sync before its
  first pass, so a cache that never synced leaves it silent. This is a
  Standby-only series; its absence on an Active is correct, not a stall.
- `ha_prune_resurrected_total` climbing steadily → **zero is the healthy value.**
  Prune re-enqueues objects the event path missed, so a backstop that fires
  constantly means the informer path is dropping work. Worth investigating on its
  own, not just tolerating.

---

## 5. Troubleshoot: promotion is not firing

The Active is gone and no Standby took over. Work down in order — the checks are
cheapest-first and each rules out the ones below it.

**1. Is this hub even a Standby?**

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  get deploy kubeslice-controller-manager \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="manager")].args}'
```

`--ha-mode=standby` must be present. Note that an *unrecognised* value is
rejected at start-up rather than silently treated as standalone, so a hub with a
typo will be crash-looping with `invalid HA configuration`, not running quietly.

**2. Is it armed?** This is the single most common answer.

```
curl -s localhost:8080/metrics | grep ha_armed
```

`0` means it has never read the Active's Lease, and the arming rule then forbids
promotion by design — a hub that never saw the Active alive must never conclude
it died. Otherwise a mistyped namespace or a missing RBAC grant would become a
guaranteed split brain on first boot. Diagnose:

```
kubectl --context <standby-ctx> -n kubeslice-controller \
  logs deploy/kubeslice-controller-manager -c manager \
  | grep -i "never been read successfully"
```

That log line carries a hint listing the three things to check:
`--ha-active-kubeconfig`, RBAC for `coordination.k8s.io/leases` on the Active,
and the Lease namespace. Also see §3 for the stale-address trap.

**3. Did it consider promoting and refuse?**

```
curl -s localhost:8080/metrics | grep ha_promotions_aborted_total
```

The `reason` label is the diagnosis:

| Reason | What happened | What to do |
|---|---|---|
| `self_unhealthy` | This hub could not reach **its own** API server, so "the Active is gone" was equally consistent with *this* hub being broken | Fix this cluster; the refusal was correct |
| `lease_live` | The Active renewed between polls — the staleness verdict was a polling race | Nothing. The Active is alive |
| `mirror_not_stopped` | The mirror did not confirm it stopped within `--ha-promotion-grace-period` | See the warning below |
| `lease_acquire_failed` | Could not write the Lease on its own cluster | Fix this cluster's API server; it retries next tick |
| `already_promoting` | A concurrent tick held the latch | Nothing; benign |
| `no_remote_client` | Asked to promote with no client to the Active | Configuration; should be unreachable from the watch loop |

Each of these also emits a `PromotionAborted` Warning Event.

> **`mirror_not_stopped` needs a follow-up.** An abort at that point leaves a
> Standby that has stopped mirroring and cannot restart it — promotion holds only
> a one-way stop handle. Later attempts still work, but if the Active recovers
> first, the guards will (correctly) refuse to promote and this hub stays a
> Standby whose mirror is dead, drifting further from the Active. The log says so
> at error level: *"promotion aborted after the state mirror was stopped"*.
> **Restart the Standby** to resume mirroring.

**4. Is detection just slower than you expected?**

```
curl -s localhost:8080/metrics | grep ha_failover_detection_seconds
```

Detection cannot beat `--ha-lease-duration` + `--ha-padding-seconds`, plus up to
one `--ha-retry-period` of polling granularity. If that budget is too slow for
you, those are the flags to shorten — at the cost of promoting on shorter
evidence.

**5. Did promotion start and stall?**

```
curl -s localhost:8080/metrics | grep ha_promotion_step_duration_seconds
```

The `step` label localises it: `stop_mirror`, `acquire_lease`,
`publish_active_controller`, `kick_reconcilers`, `emit_event`. A step sitting at
the `--ha-promotion-grace-period` ceiling is the one waiting on something.

---

## 6. Split brain: both hubs report leadership

`ha_leader_status 1` on both hubs. Objects overwrite each other, prune's reverse
diff resurrects deletions, and workers receive contradictory instructions.

**Split brain is an explicit non-goal of ADR #293 Decision 8** — there is no
fencing token or quorum here. A sustained network partition between hubs *can*
produce it, and recovery is manual.

Pick the hub to keep — normally the one with the newer
`ha_last_promotion_timestamp_seconds`, or whichever workers are actually
reporting to. Then:

1. Scale the loser to 0.
2. Confirm the survivor holds `kubeslice-controller-ha` and reports
   `ha_leader_status 1`.
3. Check `status.activeController.activeIdentity` on every Cluster CR names the
   survivor.
4. Reconcile divergence by hand — objects written on the loser during the
   partition are not merged by anything.
5. Bring the loser back as a Standby (§3).

---

## Alerting starting points

| Condition | Meaning |
|---|---|
| `sum(kubeslice_controller_ha_leader_status) != 1` | No Active, or two |
| `kubeslice_controller_ha_armed == 0` on a Standby | HA is not actually protecting anything |
| `kubeslice_controller_ha_remote_lease_age_seconds` > ½ the failover budget | Leading indicator; fires before a failover |
| `time() - kubeslice_controller_ha_lease_last_renew_time_seconds` > `--ha-renew-deadline` | Active is about to drop leadership |
| `rate(kubeslice_controller_ha_remote_lease_reads_total{result="error"}[5m]) > 0` | Standby is losing sight of the Active |
| `increase(kubeslice_controller_ha_promotions_aborted_total[1h]) > 0` | A takeover was considered and refused |
| `increase(kubeslice_controller_ha_prune_resurrected_total[1h]) > 0` | The event path is dropping work |
| `increase(kubeslice_controller_ha_active_publish_errors_total[15m]) > 0` | Failover may work without any worker noticing |

Do not alert on the value of a timestamp gauge — alert on its age.

**Role-scoped metrics carry a `mode` label and exist only on the role they
describe**, which is what makes the expressions above safe to write without
filtering by hub:

| Metric | Published by |
|---|---|
| `ha_lease_last_renew_time_seconds{mode="active"}` | an Active only |
| `ha_armed{mode="standby"}` | a Standby only |
| `ha_remote_lease_age_seconds{mode="standby"}` | a Standby only (dropped on promotion) |
| `ha_prune_last_run_timestamp_seconds{mode="standby"}` | a Standby only |
| `ha_last_promotion_timestamp_seconds{mode="active"}` | only after this process has promoted |

So `ha_armed == 0` matches Standbys and nothing else, and an *absent*
`ha_prune_last_run_timestamp_seconds` on an Active is correct rather than a
stalled backstop. `ha_leader_status` is the deliberate exception — both roles
publish it, because `sum(...) != 1` has to be expressible across the pair.

The reason absence is engineered rather than assumed: a plain registered gauge
always reports `0`, so simply not setting one is not the same as not having it. A
zeroed timestamp reads as 1970, and `time() - metric` then returns decades and
fires forever. Each of these is a labelled vector so that the series genuinely
does not exist on the wrong role.
