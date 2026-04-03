#!/usr/bin/env bash
set -euo pipefail

# E2E test for kumo Helm chart with AWS Load Balancer Controller.
#
# Flow:
#   1. Create kind cluster
#   2. Install Kyverno
#   3. Install kumo (injection.enabled=true)
#   4. Pre-create mock VPC/Subnets in kumo (via in-cluster AWS CLI)
#   5. Install cert-manager (required by AWS LB Controller webhooks)
#   6. Install AWS Load Balancer Controller
#   7. Create test Ingress
#   8. Verify Ingress gets a LoadBalancer hostname from kumo

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-kumo-e2e}"
KUMO_NS="kumo-system"
LBC_NS="aws-system"
KUMO_INJECT_LABEL_KEY="${KUMO_INJECT_LABEL_KEY:-sivchari.github.io/kumo-inject}"
KUMO_INJECT_LABEL_VALUE="${KUMO_INJECT_LABEL_VALUE:-enabled}"
KUMO_E2E_SKIP_CLEANUP="${KUMO_E2E_SKIP_CLEANUP:-}"

log() { echo "[$(date +%H:%M:%S)] $*"; }
fail() { log "FAIL: $*"; exit 1; }

# Prerequisite checks
for cmd in kind helm kubectl jq docker; do
  command -v "$cmd" >/dev/null 2>&1 || fail "Required command not found: $cmd"
done

KUMO_IMAGE="ghcr.io/sivchari/kumo:e2e-local"

cleanup() {
  if [[ -n "$KUMO_E2E_SKIP_CLEANUP" ]]; then
    log "Skipping cleanup because KUMO_E2E_SKIP_CLEANUP is set"
    return 0
  fi
  log "Cleaning up..."
  kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
}
trap cleanup EXIT

wait_for_pods() {
  local ns="$1" label="$2" timeout="${3:-180}"
  log "Waiting for pods ($label) in $ns..."
  if ! kubectl wait --for=condition=ready pod -l "$label" -n "$ns" --timeout="${timeout}s"; then
    log "Pod status in $ns:"
    kubectl get pods -n "$ns" -l "$label" -o wide
    kubectl describe pods -n "$ns" -l "$label" | tail -30
    fail "Pods ($label) in $ns not ready within ${timeout}s"
  fi
}

wait_for_crd() {
  local crd="$1" timeout="${2:-120}"
  log "Waiting for CRD $crd..."
  local i conditions
  for i in $(seq 1 "$timeout"); do
    if kubectl get "crd/$crd" >/dev/null 2>&1; then
      conditions="$(kubectl get "crd/$crd" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' 2>/dev/null || true)"
      if grep -q '^Established=True$' <<<"$conditions"; then
        break
      fi
    fi
    sleep 1
  done

  conditions="$(kubectl get "crd/$crd" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' 2>/dev/null || true)"
  if ! grep -q '^Established=True$' <<<"$conditions"; then
    kubectl get "crd/$crd" -o yaml >&2 || true
    fail "CRD $crd not established within ${timeout}s"
  fi

  for i in $(seq 1 30); do
    if kubectl api-resources --api-group="${crd#*.}" 2>/dev/null | grep -q "^$(echo "$crd" | cut -d. -f1)"; then
      return 0
    fi
    sleep 2
  done

  fail "CRD $crd not discoverable within timeout"
}

wait_for_namespace() {
  local ns="$1" timeout="${2:-60}"
  log "Waiting for namespace $ns..."
  kubectl wait --for=jsonpath='{.status.phase}'=Active "namespace/$ns" --timeout="${timeout}s"
}

# Run aws CLI command inside the cluster against kumo.
# Creates a pod, waits for completion, fetches logs, then cleans up.
kumo_aws() {
  local pod_name="aws-cli-$(date +%s%N)"
  local manifest
  local kubectl_cmd=(kubectl --context "kind-${CLUSTER_NAME}")
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
      value: http://kumo.${KUMO_NS}.svc.cluster.local:4566
    args:
EOF
    printf '    - %s\n' "$@"
  } >"$manifest"

  "${kubectl_cmd[@]}" apply -n "$KUMO_NS" -f "$manifest" >/dev/null
  rm -f "$manifest"

  local phase="" i
  for i in $(seq 1 120); do
    phase=$("${kubectl_cmd[@]}" get pod -n "$KUMO_NS" "$pod_name" -o jsonpath='{.status.phase}' 2>/dev/null || true)
    case "$phase" in
      Succeeded)
        "${kubectl_cmd[@]}" logs -n "$KUMO_NS" "$pod_name"
        "${kubectl_cmd[@]}" delete pod -n "$KUMO_NS" "$pod_name" --grace-period=0 --force >/dev/null 2>&1 || true
        return 0
        ;;
      Failed)
        "${kubectl_cmd[@]}" describe pod -n "$KUMO_NS" "$pod_name" >&2 || true
        "${kubectl_cmd[@]}" logs -n "$KUMO_NS" "$pod_name" >&2 || true
        "${kubectl_cmd[@]}" delete pod -n "$KUMO_NS" "$pod_name" --grace-period=0 --force >/dev/null 2>&1 || true
        fail "AWS CLI pod failed: $pod_name"
        ;;
    esac
    sleep 1
  done

  "${kubectl_cmd[@]}" describe pod -n "$KUMO_NS" "$pod_name" >&2 || true
  "${kubectl_cmd[@]}" logs -n "$KUMO_NS" "$pod_name" >&2 || true
  "${kubectl_cmd[@]}" delete pod -n "$KUMO_NS" "$pod_name" --grace-period=0 --force >/dev/null 2>&1 || true
  fail "AWS CLI pod timed out: $pod_name"
}

# -----------------------------------------------------------
# 0. Build kumo image locally
# -----------------------------------------------------------
log "Building kumo image locally..."
docker build -f "$REPO_ROOT/docker/Dockerfile" -t "$KUMO_IMAGE" "$REPO_ROOT"

# -----------------------------------------------------------
# 1. Create kind cluster
# -----------------------------------------------------------
log "Creating kind cluster: $CLUSTER_NAME"
kind create cluster --name "$CLUSTER_NAME" --wait 60s

# -----------------------------------------------------------
# 1.5. Load kumo image into kind
# -----------------------------------------------------------
log "Loading kumo image into kind cluster..."
kind load docker-image "$KUMO_IMAGE" --name "$CLUSTER_NAME"

# -----------------------------------------------------------
# 2. Install Kyverno
# -----------------------------------------------------------
log "Installing Kyverno..."
helm repo add kyverno https://kyverno.github.io/kyverno/ --force-update
helm install kyverno kyverno/kyverno \
  -n kyverno --create-namespace \
  --set admissionController.replicas=2 \
  --set backgroundController.enabled=false \
  --set cleanupController.enabled=false \
  --set reportsController.enabled=false \
  --set features.policyExceptions.enabled=true \
  --wait --timeout 3m
wait_for_crd clusterpolicies.kyverno.io

# -----------------------------------------------------------
# 3. Install kumo
# -----------------------------------------------------------
log "Installing kumo chart..."
helm uninstall kumo -n "$KUMO_NS" >/dev/null 2>&1 || true
helm install kumo "$REPO_ROOT/charts/kumo" \
  -n "$KUMO_NS" --create-namespace \
  --set injection.enabled=true \
  --set "injection.namespaceLabelKey=$KUMO_INJECT_LABEL_KEY" \
  --set "injection.namespaceLabelValue=$KUMO_INJECT_LABEL_VALUE" \
  --set kumo.image.tag=e2e-local \
  --wait --timeout 2m

log "Verifying kumo health..."
kubectl rollout status statefulset/kumo -n "$KUMO_NS" --timeout=120s
wait_for_pods "$KUMO_NS" "app.kubernetes.io/name=kumo" 60

# -----------------------------------------------------------
# 4. Pre-create mock VPC/Subnets in kumo
# -----------------------------------------------------------
log "Setting up mock AWS resources in kumo..."

# Create VPC
VPC_JSON="$(kumo_aws ec2 create-vpc \
  --cidr-block 10.0.0.0/16 \
  --output json)"
VPC_ID="$(jq -r '.Vpc.VpcId' <<<"$VPC_JSON")"
log "Created VPC: $VPC_ID"

if [ -z "$VPC_ID" ] || [ "$VPC_ID" = "null" ]; then
  fail "Failed to create VPC"
fi

# Create 2 subnets in different AZs (required for ALB)
SUBNET_JSON_1="$(kumo_aws ec2 create-subnet \
  --vpc-id "$VPC_ID" \
  --cidr-block 10.0.1.0/24 \
  --availability-zone us-east-1a \
  --output json)"
SUBNET_ID_1="$(jq -r '.Subnet.SubnetId' <<<"$SUBNET_JSON_1")"
log "Created Subnet 1: $SUBNET_ID_1"

SUBNET_JSON_2="$(kumo_aws ec2 create-subnet \
  --vpc-id "$VPC_ID" \
  --cidr-block 10.0.2.0/24 \
  --availability-zone us-east-1b \
  --output json)"
SUBNET_ID_2="$(jq -r '.Subnet.SubnetId' <<<"$SUBNET_JSON_2")"
log "Created Subnet 2: $SUBNET_ID_2"

# Note: kumo does not implement CreateTags. Subnet IDs are passed directly
# to the Ingress annotation (alb.ingress.kubernetes.io/subnets) so auto-discovery
# via tags is not needed for this test.

# -----------------------------------------------------------
# 5. Install cert-manager
# -----------------------------------------------------------
log "Installing cert-manager..."
helm repo add jetstack https://charts.jetstack.io --force-update
kubectl create namespace cert-manager 2>/dev/null || true
wait_for_namespace cert-manager 60
helm install cert-manager jetstack/cert-manager \
  -n cert-manager --create-namespace \
  --set crds.enabled=true \
  --wait --timeout 3m

# -----------------------------------------------------------
# 6. Install AWS Load Balancer Controller
# -----------------------------------------------------------
log "Creating and labeling $LBC_NS for kumo injection..."
kubectl create namespace "$LBC_NS" 2>/dev/null || true
wait_for_namespace "$LBC_NS" 60
kubectl label namespace "$LBC_NS" "$KUMO_INJECT_LABEL_KEY=$KUMO_INJECT_LABEL_VALUE" --overwrite

log "Installing AWS Load Balancer Controller..."
helm repo add eks https://aws.github.io/eks-charts --force-update
if ! helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n "$LBC_NS" \
  --set clusterName="$CLUSTER_NAME" \
  --set vpcId="$VPC_ID" \
  --set region=us-east-1 \
  --set enableWafv2=false \
  --set enableWaf=false \
  --set enableShield=false \
  --set serviceAccount.create=true \
  --set serviceAccount.name=aws-load-balancer-controller \
  --timeout 3m \
  --wait; then
  log "WARNING: helm install returned non-zero, checking pod status..."
  kubectl get pods -n "$LBC_NS" -l app.kubernetes.io/name=aws-load-balancer-controller -o wide
  POD_COUNT=$(kubectl get pods -n "$LBC_NS" -l app.kubernetes.io/name=aws-load-balancer-controller --no-headers 2>/dev/null | wc -l)
  if [[ "$POD_COUNT" -eq 0 ]]; then
    fail "AWS LB Controller install failed and no pods were created"
  fi
  log "Pods exist, continuing..."
fi

wait_for_pods "$LBC_NS" "app.kubernetes.io/name=aws-load-balancer-controller" 120
log "AWS LB Controller is ready"

# -----------------------------------------------------------
# 7. Create test Ingress
# -----------------------------------------------------------
log "Creating test application and Ingress..."

kubectl create namespace test-app 2>/dev/null || true
wait_for_namespace test-app 60
kubectl label namespace test-app "$KUMO_INJECT_LABEL_KEY=$KUMO_INJECT_LABEL_VALUE" --overwrite

INGRESS_MANIFEST="$(mktemp)"
cat >"$INGRESS_MANIFEST" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.27-alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: test-app
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  namespace: test-app
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/subnets: "${SUBNET_ID_1},${SUBNET_ID_2}"
spec:
  ingressClassName: alb
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nginx
            port:
              number: 80
EOF
kubectl apply -f "$INGRESS_MANIFEST"
rm -f "$INGRESS_MANIFEST"

# -----------------------------------------------------------
# 8. Verify Ingress gets a LoadBalancer hostname
# -----------------------------------------------------------
log "Waiting for Ingress to get LoadBalancer address..."
ADDRESS=""
for i in $(seq 1 60); do
  ADDRESS=$(kubectl get ingress test-ingress -n test-app \
    -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || true)
  if [ -n "$ADDRESS" ]; then
    break
  fi
  if (( i % 10 == 0 )); then
    log "Still waiting... ($i/60)"
    kubectl get ingress test-ingress -n test-app -o wide 2>/dev/null || true
    kubectl logs -n "$LBC_NS" -l app.kubernetes.io/name=aws-load-balancer-controller --tail=5 2>/dev/null || true
  fi
  sleep 5
done

echo ""
echo "========================================"
if [ -n "$ADDRESS" ]; then
  log "SUCCESS: Ingress has LoadBalancer address: $ADDRESS"

  # Verify LoadBalancer exists in kumo
  log "Verifying LoadBalancer in kumo..."
  LB_COUNT=$(kumo_aws elbv2 describe-load-balancers \
    --output json | jq '.LoadBalancers | length' 2>/dev/null || echo "0")
  log "LoadBalancers in kumo: $LB_COUNT"

  TG_COUNT=$(kumo_aws elbv2 describe-target-groups \
    --output json | jq '.TargetGroups | length' 2>/dev/null || echo "0")
  log "TargetGroups in kumo: $TG_COUNT"

  echo "========================================"
  exit 0
else
  log "FAIL: Ingress did not get a LoadBalancer address within 5 minutes"
  echo ""
  log "--- Ingress status ---"
  kubectl describe ingress test-ingress -n test-app 2>/dev/null || true
  echo ""
  log "--- AWS LB Controller logs ---"
  kubectl logs -n "$LBC_NS" -l app.kubernetes.io/name=aws-load-balancer-controller --tail=50 2>/dev/null || true
  echo ""
  log "--- AWS LB Controller env (verify kumo injection) ---"
  kubectl get pods -n "$LBC_NS" -l app.kubernetes.io/name=aws-load-balancer-controller \
    -o jsonpath='{range .items[*].spec.containers[*]}{.env}{"\n"}{end}' 2>/dev/null || true
  echo "========================================"
  exit 1
fi
