#!/usr/bin/env bash
set -euo pipefail
set -E

# E2E test for kumo Helm chart with ACK SQS controller.
#
# Flow:
#   1. Create a fresh kind cluster
#   2. Install Kyverno
#   3. Install kumo (injection.enabled=true)
#   4. Install ACK SQS controller in a labeled namespace
#   5. Verify AWS_ENDPOINT_URL was injected into the controller Pod
#   6. Create an ACK Queue custom resource
#   7. Verify the Queue is synced and usable via kumo
#   8. Delete the Queue and verify cleanup in kumo

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-kumo-e2e}"
KUMO_NS="${KUMO_NS:-kumo-system}"
ACK_NS="${ACK_NS:-ack-system}"
QUEUE_NS="${QUEUE_NS:-ack-test}"
KUMO_RELEASE_NAME="${KUMO_RELEASE_NAME:-kumo}"
QUEUE_CR_NAME="${QUEUE_CR_NAME:-helm-e2e-queue}"
QUEUE_NAME="${QUEUE_NAME:-helm-e2e-queue-$(date +%s)}"
ACK_SQS_RELEASE="${ACK_SQS_RELEASE:-ack-sqs-controller}"
ACK_SQS_CHART_VERSION="${ACK_SQS_CHART_VERSION:-1.4.1}"
ACK_CONTROLLER_LABEL="${ACK_CONTROLLER_LABEL:-app.kubernetes.io/instance=ack-sqs-controller}"
KUMO_INJECT_LABEL_KEY="${KUMO_INJECT_LABEL_KEY:-sivchari.github.io/kumo-inject}"
KUMO_INJECT_LABEL_VALUE="${KUMO_INJECT_LABEL_VALUE:-enabled}"
KUMO_IMAGE="${KUMO_IMAGE:-ghcr.io/sivchari/kumo:e2e-local}"
KUMO_E2E_SKIP_CLEANUP="${KUMO_E2E_SKIP_CLEANUP:-}"

KUBECTL=(kubectl --context "kind-${CLUSTER_NAME}")
KUMO_ENDPOINT="http://${KUMO_RELEASE_NAME}.${KUMO_NS}.svc.cluster.local:4566"
FAILED=0

log() {
  echo "[$(date +%H:%M:%S)] $*"
}

collect_diagnostics() {
  log "=== kind cluster info ==="
  "${KUBECTL[@]}" cluster-info 2>/dev/null || true

  log "=== namespaces ==="
  "${KUBECTL[@]}" get namespaces --show-labels 2>/dev/null || true

  log "=== all pods ==="
  "${KUBECTL[@]}" get pods -A -o wide 2>/dev/null || true

  log "=== events ==="
  "${KUBECTL[@]}" get events -A --sort-by='.lastTimestamp' 2>/dev/null | tail -40 || true

  log "=== Kyverno ClusterPolicy ==="
  "${KUBECTL[@]}" get clusterpolicy -o wide 2>/dev/null || true

  log "=== kumo svc/endpoints ==="
  "${KUBECTL[@]}" get svc,endpoints -n "$KUMO_NS" 2>/dev/null || true

  log "=== ACK controller pods ==="
  "${KUBECTL[@]}" get pods -n "$ACK_NS" -l "$ACK_CONTROLLER_LABEL" -o wide 2>/dev/null || true

  log "=== ACK controller logs ==="
  "${KUBECTL[@]}" logs -n "$ACK_NS" -l "$ACK_CONTROLLER_LABEL" --tail=200 2>/dev/null || true

  log "=== ACK Queue resources ==="
  "${KUBECTL[@]}" get queues.sqs.services.k8s.aws -A -o yaml 2>/dev/null || true

  log "=== kumo logs ==="
  "${KUBECTL[@]}" logs -n "$KUMO_NS" -l app.kubernetes.io/name=kumo --tail=200 2>/dev/null || true
}

fail() {
  FAILED=1
  log "FAIL: $*"
  collect_diagnostics
  exit 1
}

on_error() {
  if [[ "$FAILED" -eq 1 ]]; then
    exit 1
  fi

  FAILED=1
  log "Unexpected failure at line $1: $2"
  collect_diagnostics
  exit 1
}

require_cmds() {
  local cmd
  for cmd in "$@"; do
    command -v "$cmd" >/dev/null 2>&1 || fail "Required command not found: $cmd"
  done
}

cleanup() {
  if [[ -n "$KUMO_E2E_SKIP_CLEANUP" ]]; then
    log "Skipping cleanup because KUMO_E2E_SKIP_CLEANUP is set"
    return 0
  fi

  log "Cleaning up..."
  kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
}

wait_for_pods() {
  local ns="$1" label="$2" timeout="${3:-180}"
  log "Waiting for pods ($label) in $ns..."
  if ! "${KUBECTL[@]}" wait --for=condition=ready pod -l "$label" -n "$ns" --timeout="${timeout}s"; then
    "${KUBECTL[@]}" get pods -n "$ns" -l "$label" -o wide 2>/dev/null || true
    "${KUBECTL[@]}" describe pods -n "$ns" -l "$label" 2>/dev/null | tail -30 || true
    fail "Pods ($label) in $ns not ready within ${timeout}s"
  fi
}

wait_for_crd() {
  local crd="$1" timeout="${2:-120}"
  log "Waiting for CRD $crd..."

  local i conditions
  for i in $(seq 1 "$timeout"); do
    if "${KUBECTL[@]}" get "crd/$crd" >/dev/null 2>&1; then
      conditions="$("${KUBECTL[@]}" get "crd/$crd" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' 2>/dev/null || true)"
      if grep -q '^Established=True$' <<<"$conditions"; then
        break
      fi
    fi
    sleep 1
  done

  conditions="$("${KUBECTL[@]}" get "crd/$crd" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' 2>/dev/null || true)"
  if ! grep -q '^Established=True$' <<<"$conditions"; then
    "${KUBECTL[@]}" get "crd/$crd" -o yaml >&2 || true
    fail "CRD $crd not established within ${timeout}s"
  fi

  for i in $(seq 1 30); do
    if "${KUBECTL[@]}" api-resources --api-group="${crd#*.}" 2>/dev/null | grep -q "^$(echo "$crd" | cut -d. -f1)"; then
      return 0
    fi
    sleep 2
  done

  fail "CRD $crd not discoverable within timeout"
}

wait_for_namespace() {
  local ns="$1" timeout="${2:-60}"
  log "Waiting for namespace $ns..."
  "${KUBECTL[@]}" wait --for=jsonpath='{.status.phase}'=Active "namespace/$ns" --timeout="${timeout}s"
}

wait_for_queue_synced() {
  local ns="$1" name="$2" timeout="${3:-180}"
  log "Waiting for Queue/$name to sync..." >&2

  local attempts=$((timeout / 2))
  if (( attempts < 1 )); then
    attempts=1
  fi

  local i synced terminal terminal_message queue_url
  for i in $(seq 1 "$attempts"); do
    synced="$("${KUBECTL[@]}" get queue -n "$ns" "$name" -o jsonpath='{.status.conditions[?(@.type=="ACK.ResourceSynced")].status}' 2>/dev/null || true)"
    terminal="$("${KUBECTL[@]}" get queue -n "$ns" "$name" -o jsonpath='{.status.conditions[?(@.type=="ACK.Terminal")].status}' 2>/dev/null || true)"
    terminal_message="$("${KUBECTL[@]}" get queue -n "$ns" "$name" -o jsonpath='{.status.conditions[?(@.type=="ACK.Terminal")].message}' 2>/dev/null || true)"
    queue_url="$("${KUBECTL[@]}" get queue -n "$ns" "$name" -o jsonpath='{.status.queueURL}' 2>/dev/null || true)"

    if [[ "$synced" == "True" && -n "$queue_url" ]]; then
      printf '%s\n' "$queue_url"
      return 0
    fi

    if [[ "$terminal" == "True" ]]; then
      "${KUBECTL[@]}" get queue -n "$ns" "$name" -o yaml >&2 || true
      fail "Queue/$name entered ACK.Terminal: ${terminal_message:-unknown error}"
    fi

    if (( i % 10 == 0 )); then
      "${KUBECTL[@]}" get queue -n "$ns" "$name" -o yaml 2>/dev/null || true
      "${KUBECTL[@]}" logs -n "$ACK_NS" -l "$ACK_CONTROLLER_LABEL" --tail=20 2>/dev/null || true
    fi

    sleep 2
  done

  "${KUBECTL[@]}" get queue -n "$ns" "$name" -o yaml >&2 || true
  fail "Queue/$name was not synced within ${timeout}s"
}

assert_ack_controller_injected() {
  log "Verifying ACK controller env injection..."

  local pod endpoint
  pod="$("${KUBECTL[@]}" get pods -n "$ACK_NS" -l "$ACK_CONTROLLER_LABEL" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -z "$pod" ]]; then
    fail "ACK controller pod not found"
  fi

  endpoint="$("${KUBECTL[@]}" get pod -n "$ACK_NS" "$pod" -o jsonpath='{range .spec.containers[?(@.name=="controller")].env[?(@.name=="AWS_ENDPOINT_URL")]}{.value}{end}' 2>/dev/null || true)"
  if [[ "$endpoint" != "$KUMO_ENDPOINT" ]]; then
    "${KUBECTL[@]}" get pod -n "$ACK_NS" "$pod" -o yaml >&2 || true
    fail "ACK controller AWS_ENDPOINT_URL was not injected correctly"
  fi
}

# Run aws CLI command inside the cluster against kumo.
# Creates a pod, waits for completion, fetches logs, then cleans up.
kumo_aws() {
  local pod_name="aws-cli-$(date +%s%N)"
  local manifest
  manifest="$(mktemp)"

  {
    cat <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
spec:
  restartPolicy: Never
  containers:
  - name: aws
    image: amazon/aws-cli:latest
    env:
    - name: AWS_ACCESS_KEY_ID
      value: test
    - name: AWS_SECRET_ACCESS_KEY
      value: test
    - name: AWS_DEFAULT_REGION
      value: us-east-1
    - name: AWS_ENDPOINT_URL
      value: ${KUMO_ENDPOINT}
    args:
EOF
    local arg
    for arg in "$@"; do
      printf '    - %s\n' "$(jq -Rn --arg value "$arg" '$value')"
    done
  } >"$manifest"

  "${KUBECTL[@]}" apply -n "$KUMO_NS" -f "$manifest" >/dev/null
  rm -f "$manifest"

  local phase="" i
  for i in $(seq 1 120); do
    phase="$("${KUBECTL[@]}" get pod -n "$KUMO_NS" "$pod_name" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    case "$phase" in
      Succeeded)
        "${KUBECTL[@]}" logs -n "$KUMO_NS" "$pod_name"
        "${KUBECTL[@]}" delete pod -n "$KUMO_NS" "$pod_name" --grace-period=0 --force >/dev/null 2>&1 || true
        return 0
        ;;
      Failed)
        "${KUBECTL[@]}" describe pod -n "$KUMO_NS" "$pod_name" >&2 || true
        "${KUBECTL[@]}" logs -n "$KUMO_NS" "$pod_name" >&2 || true
        "${KUBECTL[@]}" delete pod -n "$KUMO_NS" "$pod_name" --grace-period=0 --force >/dev/null 2>&1 || true
        fail "AWS CLI pod failed: $pod_name"
        ;;
    esac
    sleep 1
  done

  "${KUBECTL[@]}" describe pod -n "$KUMO_NS" "$pod_name" >&2 || true
  "${KUBECTL[@]}" logs -n "$KUMO_NS" "$pod_name" >&2 || true
  "${KUBECTL[@]}" delete pod -n "$KUMO_NS" "$pod_name" --grace-period=0 --force >/dev/null 2>&1 || true
  fail "AWS CLI pod timed out: $pod_name"
}

main() {
  trap cleanup EXIT
  trap 'on_error "$LINENO" "$BASH_COMMAND"' ERR

  require_cmds kind helm kubectl jq docker

  log "Building kumo image locally..."
  docker build -f "$REPO_ROOT/docker/Dockerfile" -t "$KUMO_IMAGE" "$REPO_ROOT"

  log "Removing any existing kind cluster named $CLUSTER_NAME..."
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true

  log "Creating kind cluster: $CLUSTER_NAME"
  kind create cluster --name "$CLUSTER_NAME" --wait 60s

  log "Loading kumo image into kind cluster..."
  kind load docker-image "$KUMO_IMAGE" --name "$CLUSTER_NAME"

  log "Installing Kyverno..."
  helm repo add kyverno https://kyverno.github.io/kyverno/ --force-update >/dev/null
  helm install kyverno kyverno/kyverno \
    -n kyverno --create-namespace \
    --set admissionController.replicas=2 \
    --set backgroundController.enabled=false \
    --set cleanupController.enabled=false \
    --set reportsController.enabled=false \
    --set features.policyExceptions.enabled=true \
    --wait --timeout 3m
  wait_for_crd clusterpolicies.kyverno.io

  log "Installing kumo chart..."
  helm uninstall "$KUMO_RELEASE_NAME" -n "$KUMO_NS" >/dev/null 2>&1 || true
  helm install "$KUMO_RELEASE_NAME" "$REPO_ROOT/charts/kumo" \
    -n "$KUMO_NS" --create-namespace \
    --set injection.enabled=true \
    --set "injection.namespaceLabelKey=$KUMO_INJECT_LABEL_KEY" \
    --set "injection.namespaceLabelValue=$KUMO_INJECT_LABEL_VALUE" \
    --set kumo.image.tag=e2e-local \
    --wait --timeout 2m

  log "Verifying kumo health..."
  "${KUBECTL[@]}" rollout status statefulset/"$KUMO_RELEASE_NAME" -n "$KUMO_NS" --timeout=120s
  wait_for_pods "$KUMO_NS" "app.kubernetes.io/name=kumo" 60

  log "Creating and labeling $ACK_NS for kumo injection..."
  "${KUBECTL[@]}" create namespace "$ACK_NS" >/dev/null 2>&1 || true
  wait_for_namespace "$ACK_NS" 60
  "${KUBECTL[@]}" label namespace "$ACK_NS" "$KUMO_INJECT_LABEL_KEY=$KUMO_INJECT_LABEL_VALUE" --overwrite >/dev/null

  log "Installing ACK SQS controller..."
  helm uninstall "$ACK_SQS_RELEASE" -n "$ACK_NS" >/dev/null 2>&1 || true
  helm install "$ACK_SQS_RELEASE" oci://public.ecr.aws/aws-controllers-k8s/sqs-chart \
    -n "$ACK_NS" \
    --version "$ACK_SQS_CHART_VERSION" \
    --set aws.region=us-east-1 \
    --set aws.allow_unsafe_aws_endpoint_urls=true \
    --wait --timeout 3m

  wait_for_crd queues.sqs.services.k8s.aws
  wait_for_pods "$ACK_NS" "$ACK_CONTROLLER_LABEL" 120
  assert_ack_controller_injected
  log "ACK SQS controller is ready"

  log "Creating ACK Queue custom resource..."
  "${KUBECTL[@]}" create namespace "$QUEUE_NS" >/dev/null 2>&1 || true
  wait_for_namespace "$QUEUE_NS" 60

  local queue_manifest
  queue_manifest="$(mktemp)"
  cat >"$queue_manifest" <<EOF
apiVersion: sqs.services.k8s.aws/v1alpha1
kind: Queue
metadata:
  name: ${QUEUE_CR_NAME}
  namespace: ${QUEUE_NS}
spec:
  queueName: ${QUEUE_NAME}
  delaySeconds: "0"
  tags:
    purpose: helm-e2e
EOF
  "${KUBECTL[@]}" apply -f "$queue_manifest"
  rm -f "$queue_manifest"

  local queue_url resolved_queue_url queue_tag_value send_output message_id receive_output message_body queue_count list_queues_output

  queue_url="$(wait_for_queue_synced "$QUEUE_NS" "$QUEUE_CR_NAME" 180)"
  log "Queue synced with URL: $queue_url"

  log "Verifying Queue in kumo..."
  resolved_queue_url="$(kumo_aws sqs get-queue-url --queue-name "$QUEUE_NAME" --output json | jq -r '.QueueUrl // empty')"
  if [[ "$resolved_queue_url" != "$queue_url" ]]; then
    fail "Queue URL in kumo did not match ACK status"
  fi

  queue_tag_value="$(kumo_aws sqs list-queue-tags --queue-url "$queue_url" --output json | jq -r '.Tags.purpose // empty')"
  if [[ "$queue_tag_value" != "helm-e2e" ]]; then
    fail "Queue tag was not synced to kumo"
  fi

  send_output="$(kumo_aws sqs send-message --queue-url "$queue_url" --message-body "hello from ACK" --output json)"
  message_id="$(jq -r '.MessageId // empty' <<<"$send_output")"
  if [[ -z "$message_id" ]]; then
    fail "Failed to send message to ACK-managed queue"
  fi

  receive_output="$(kumo_aws sqs receive-message --queue-url "$queue_url" --max-number-of-messages 1 --wait-time-seconds 1 --output json)"
  message_body="$(jq -r '.Messages[0].Body // empty' <<<"$receive_output")"
  if [[ "$message_body" != "hello from ACK" ]]; then
    fail "Unexpected SQS message body: ${message_body:-<empty>}"
  fi

  log "Deleting ACK Queue custom resource..."
  "${KUBECTL[@]}" delete queue "$QUEUE_CR_NAME" -n "$QUEUE_NS" --wait=false >/dev/null
  if ! "${KUBECTL[@]}" wait --for=delete "queue/$QUEUE_CR_NAME" -n "$QUEUE_NS" --timeout=180s; then
    fail "Queue resource was not deleted within timeout"
  fi

  list_queues_output="$(kumo_aws sqs list-queues --queue-name-prefix "$QUEUE_NAME" --output json)"
  if [[ -z "$list_queues_output" ]]; then
    queue_count="0"
  else
    queue_count="$(jq '.QueueUrls // [] | length' <<<"$list_queues_output")"
  fi
  if [[ "$queue_count" != "0" ]]; then
    fail "Queue still exists in kumo after ACK resource deletion"
  fi

  echo ""
  echo "========================================"
  log "SUCCESS: ACK SQS controller reconciled Queue through kumo"
  echo "========================================"
}

main "$@"
