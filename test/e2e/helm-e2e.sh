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

log() { echo "[$(date +%H:%M:%S)] $*"; }
fail() { log "FAIL: $*"; exit 1; }

# Prerequisite checks
for cmd in kind helm kubectl jq docker; do
  command -v "$cmd" >/dev/null 2>&1 || fail "Required command not found: $cmd"
done

KUMO_IMAGE="ghcr.io/sivchari/kumo:e2e-local"

cleanup() {
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

# Run aws CLI command inside the cluster against kumo.
# Creates a pod, waits for completion, fetches logs, then cleans up.
kumo_aws() {
  local pod_name="aws-cli-$(date +%s%N)"
  kubectl run -n "$KUMO_NS" "$pod_name" --restart=Never --image=amazon/aws-cli:latest \
    --env="AWS_ACCESS_KEY_ID=test" --env="AWS_SECRET_ACCESS_KEY=test" \
    --env="AWS_DEFAULT_REGION=us-east-1" --env="AWS_ENDPOINT_URL=http://kumo.${KUMO_NS}.svc.cluster.local:4566" \
    -- "$@" >/dev/null 2>&1
  kubectl wait -n "$KUMO_NS" --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod_name" --timeout=60s >/dev/null 2>&1
  kubectl logs -n "$KUMO_NS" "$pod_name"
  kubectl delete pod -n "$KUMO_NS" "$pod_name" --grace-period=0 >/dev/null 2>&1 || true
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
  --set admissionController.replicas=1 \
  --set backgroundController.enabled=false \
  --set cleanupController.enabled=false \
  --set reportsController.enabled=false \
  --wait --timeout 3m

# -----------------------------------------------------------
# 3. Install kumo
# -----------------------------------------------------------
log "Installing kumo chart..."
helm install kumo "$REPO_ROOT/charts/kumo" \
  -n "$KUMO_NS" --create-namespace \
  --set injection.enabled=true \
  --set kumo.image.tag=e2e-local \
  --wait --timeout 2m

log "Verifying kumo health..."
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=kumo -n "$KUMO_NS" --timeout=60s

# -----------------------------------------------------------
# 4. Pre-create mock VPC/Subnets in kumo
# -----------------------------------------------------------
log "Setting up mock AWS resources in kumo..."

# Create VPC
VPC_ID=$(kumo_aws ec2 create-vpc \
  --cidr-block 10.0.0.0/16 \
  --output json | jq -r '.Vpc.VpcId')
log "Created VPC: $VPC_ID"

if [ -z "$VPC_ID" ] || [ "$VPC_ID" = "null" ]; then
  fail "Failed to create VPC"
fi

# Create 2 subnets in different AZs (required for ALB)
SUBNET_ID_1=$(kumo_aws ec2 create-subnet \
  --vpc-id "$VPC_ID" \
  --cidr-block 10.0.1.0/24 \
  --availability-zone us-east-1a \
  --output json | jq -r '.Subnet.SubnetId')
log "Created Subnet 1: $SUBNET_ID_1"

SUBNET_ID_2=$(kumo_aws ec2 create-subnet \
  --vpc-id "$VPC_ID" \
  --cidr-block 10.0.2.0/24 \
  --availability-zone us-east-1b \
  --output json | jq -r '.Subnet.SubnetId')
log "Created Subnet 2: $SUBNET_ID_2"

# Note: kumo does not implement CreateTags. Subnet IDs are passed directly
# to the Ingress annotation (alb.ingress.kubernetes.io/subnets) so auto-discovery
# via tags is not needed for this test.

# -----------------------------------------------------------
# 5. Install cert-manager
# -----------------------------------------------------------
log "Installing cert-manager..."
helm repo add jetstack https://charts.jetstack.io --force-update
helm install cert-manager jetstack/cert-manager \
  -n cert-manager --create-namespace \
  --set crds.enabled=true \
  --wait --timeout 3m

# -----------------------------------------------------------
# 6. Install AWS Load Balancer Controller
# -----------------------------------------------------------
log "Creating and labeling $LBC_NS for kumo injection..."
kubectl create namespace "$LBC_NS" 2>/dev/null || true
kubectl label namespace "$LBC_NS" kumo.appthrust.io/inject=enabled --overwrite

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

kubectl create namespace test-app
kubectl label namespace test-app kumo.appthrust.io/inject=enabled

kubectl apply -f - <<EOF
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
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/subnets: "${SUBNET_ID_1},${SUBNET_ID_2}"
spec:
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
