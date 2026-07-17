# HA cross-cluster RBAC (issue #295)

`active-cluster-clusterrole.yaml` is a least-privilege grant for the
identity behind a Standby's `--ha-active-kubeconfig` flag: read-only
(`get`/`list`/`watch`) access to `Namespace` plus every resource type in
`pkg/ha.CRDMirrorSet` (`RemoteSyncer` never writes to the Active cluster —
only reads).

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

## Credential mirroring (a later PR)

If/when `pkg/ha.CredentialMirrorSet` (Secrets, ServiceAccounts, Roles,
RoleBindings — for #297's post-promotion use) is wired in, this
`ClusterRole` will need `secrets`/`serviceaccounts`/`roles`/`rolebindings`
appended. Worth knowing ahead of time: RBAC cannot scope `Secret` access by
`.type`, so that addition grants read access to **every** Secret in the
project namespaces on the Active cluster, not just the credential ones
`RemoteSyncer` actually mirrors — a real credential-exposure tradeoff to
weigh when that lands, not just an implementation detail.
