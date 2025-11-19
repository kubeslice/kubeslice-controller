# iPerf inter-cluster test for KubeSlice

This folder contains manifests and a small helper script to run iPerf tests between two worker clusters in a KubeSlice-enabled environment.

What is included
- `iperf-sleep.yaml` - client deployment (sleep + netshoot sidecar)
- `iperf-server.yaml` - server deployment and `ServiceExport` for the iperf server
- `slice-templates/` - three SliceConfig templates: `fullmesh`, `restricted`, `custom`
- `../../scripts/iperf-run.sh` - helper script to deploy and run tests (see scripts path)

Scenarios to test
- Full-mesh (default) — verify baseline connectivity and bandwidth
- Restricted — remove a forbidden edge and verify iperf is blocked
- Custom — only specific source→target connectivity enabled

Quick notes
- You need two or more registered worker clusters (contexts configured with `kubectx` or `kubectl --context`).
- Create the `iperf` namespace in each participating cluster before applying deployments:

```bash
kubectl --context <worker-context> create ns iperf
```

- The script does not modify controller code — it deploys SliceConfig/sample resources and the iperf workloads and runs `iperf` from the client pod.

See `../../scripts/iperf-run.sh --help` for usage.
