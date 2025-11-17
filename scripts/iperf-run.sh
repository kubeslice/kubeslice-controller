#!/usr/bin/env bash
# Helper script to deploy iperf server/client and run tests across two clusters
# Usage:
#   scripts/iperf-run.sh --server-context <ctx> --sleep-context <ctx> --project-namespace <ns> --slice-file <slice.yaml> --slice-name <name>

set -euo pipefail

usage(){
  cat <<EOF
usage: $0 --server-context CTX --sleep-context CTX --controller-context CTX --slice-file PATH --slice-name NAME [--project-namespace NAMESPACE]

Options:
  --server-context    Kubernetes context for worker cluster where iperf-server will be deployed
  --sleep-context     Kubernetes context for worker cluster where iperf-sleep will be deployed
  --controller-context Kubernetes context where KubeSlice controller resides (used to read ServiceImport info)
  --slice-file        SliceConfig YAML to apply (must reference the slice name)
  --slice-name        Slice name used by ServiceExport
  --project-namespace Project namespace where the slice is applied (defaults to kubeslice)
  --help              Show this message
EOF
  exit 1
}

SERVER_CTX=""
SLEEP_CTX=""
CONTROLLER_CTX=""
SLICE_FILE=""
SLICE_NAME=""
PROJECT_NS="kubeslice"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-context) SERVER_CTX="$2"; shift 2;;
    --sleep-context) SLEEP_CTX="$2"; shift 2;;
    --controller-context) CONTROLLER_CTX="$2"; shift 2;;
    --slice-file) SLICE_FILE="$2"; shift 2;;
    --slice-name) SLICE_NAME="$2"; shift 2;;
    --project-namespace) PROJECT_NS="$2"; shift 2;;
    --help) usage ;;
    *) echo "Unknown arg: $1"; usage ;;
  esac
done

if [[ -z "$SERVER_CTX" || -z "$SLEEP_CTX" || -z "$CONTROLLER_CTX" || -z "$SLICE_FILE" || -z "$SLICE_NAME" ]]; then
  usage
fi

echo "Using server-context=$SERVER_CTX sleep-context=$SLEEP_CTX controller-context=$CONTROLLER_CTX slice-file=$SLICE_FILE slice-name=$SLICE_NAME project-namespace=$PROJECT_NS"

set -x

# 1) Apply slice to controller/project namespace
kubectl --context "$CONTROLLER_CTX" apply -f "$SLICE_FILE" -n "$PROJECT_NS"

# 2) Deploy iperf-server in server cluster
kubectl --context "$SERVER_CTX" create ns iperf --dry-run=client -o yaml | kubectl --context "$SERVER_CTX" apply -f -
sed "s/<slice-name>/$SLICE_NAME/g" docs/iperf/iperf-server.yaml | kubectl --context "$SERVER_CTX" apply -f - -n iperf || true

# 3) Deploy iperf-sleep in sleep cluster
kubectl --context "$SLEEP_CTX" create ns iperf --dry-run=client -o yaml | kubectl --context "$SLEEP_CTX" apply -f -
kubectl --context "$SLEEP_CTX" apply -f docs/iperf/iperf-sleep.yaml -n iperf

# 4) Wait for pods to become ready (server)
echo "Waiting for iperf-server pod..."
kubectl --context "$SERVER_CTX" -n iperf wait --for=condition=ready pod -l app=iperf-server --timeout=120s

echo "Waiting for iperf-sleep pod..."
kubectl --context "$SLEEP_CTX" -n iperf wait --for=condition=ready pod -l app=iperf-sleep --timeout=120s

# 5) Derive DNS name (short name should be available)
SHORT_DNS="iperf-server.iperf.svc.slice.local"

echo "Using short DNS: $SHORT_DNS"

# 6) Exec into sleep pod and run iperf
SLEEP_POD=$(kubectl --context "$SLEEP_CTX" -n iperf get pods -l app=iperf-sleep -o jsonpath='{.items[0].metadata.name}')
echo "Using sleep pod: $SLEEP_POD"

OUTFILE="iperf-${SLICE_NAME}-$(date +%Y%m%dT%H%M%S).log"
kubectl --context "$SLEEP_CTX" -n iperf exec -c iperf "$SLEEP_POD" -- iperf -c "$SHORT_DNS" -p 5201 -i 1 -t 10 > "$OUTFILE" 2>&1 || true

echo "iperf output saved to $OUTFILE"
echo
echo "--- output ---"
cat "$OUTFILE"

echo "Done"
