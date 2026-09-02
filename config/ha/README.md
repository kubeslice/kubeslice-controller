# HA cross-cluster RBAC (issue #295)

`active-cluster-clusterrole.yaml` is a least-privilege grant for the
identity behind a Standby's `--ha-active-kubeconfig` flag: read-only
(`get`/`list`/`watch`) access to `Namespace` plus every resource type in
`pkg/ha.CRDMirrorSet` and `pkg/ha.CredentialMirrorSet` (`RemoteSyncer`
never writes to the Active cluster — only reads), plus the Active's own
`coordination.k8s.io/v1` `Lease` — the same kubeconfig is also used by
#294's `WatchRemoteLease` to read the Active's Lease directly, not just by
`RemoteSyncer` to mirror resources.

## This is not applied by this repo's own deploy flow

Nothing here is referenced by `config/rbac/kustomization.yaml` or
`config/default/kustomization.yaml`. Those govern the RBAC a controller
grants *itself* on the cluster it's running in. This manifest is different:
it must be applied **on the Active hub cluster**, granting access to
whatever identity the *Standby's* kubeconfig authenticates as — a cluster
this repo's own kustomize overlays have no way to reach, since it's a
separate cluster entirely.

Apply it manually (or via whatever provisioning tooling manages the Active
hub) against the Active cluster:

```
kubectl --context <active-hub-context> apply -f active-cluster-clusterrole.yaml
```

Fill in the `ClusterRoleBinding`'s `subjects` first — the correct subject
depends on how the Standby authenticates to the Active (a `ServiceAccount`
if dialing in-cluster, a client-cert `User` if using a flattened
kubeconfig Secret, as the current dev demo does).

## A known, deliberate gap this manifest does not close

Nothing in this repo automates applying this to a real Active cluster —
that's cross-cluster provisioning, out of scope for a single controller
repo. Logged as a follow-up, not built.

## Credential mirroring and the Secret-read tradeoff

`pkg/ha.CredentialMirrorSet` (Secrets, ServiceAccounts, Roles,
RoleBindings — for #297's post-promotion use) is wired in, and this
`ClusterRole` grants the reads it needs. Weigh the
Secret rule before applying it: RBAC cannot scope `Secret` access by
`.type` or by namespace *label*, and a `ClusterRole` +
`ClusterRoleBinding` is cluster-wide — so the Standby's identity can read
**every** Secret on the Active hub, not just the gateway-certificate
Secrets `RemoteSyncer` actually mirrors. The syncer itself only *copies*
credential objects whose namespace it also mirrors (the label-scoped
project-namespace boundary — notably excluding the controller's own
namespace, whose name can match the project-namespace prefix), and it
carries SA-token Secrets as empty shells whose token bytes are stripped
before the write, but none of that
narrows what the identity *could* read if the kubeconfig leaked —
protect it like the credential it is. The narrower alternative — per-namespace `RoleBinding`s in each
project namespace instead of the cluster-wide binding — works with the
same `ClusterRole`, at the cost of maintaining those bindings as projects
come and go (cross-cluster provisioning tooling this repo deliberately
does not ship).
