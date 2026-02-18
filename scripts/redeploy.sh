#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# K8s AI Healer — Fast Redeploy (for code-test iteration)
#
# This script:
#   1. Rebuilds the Docker image (picks up your code changes)
#   2. Loads it into the cluster
#   3. Forces a pod restart so Kubernetes uses the new image
#
# Usage (after making code changes):
#   ./scripts/redeploy.sh
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-k8s-healer}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
NAMESPACE="healer-system"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

# Guard: vendor/ must exist
if [ ! -d "vendor" ]; then
  echo "❌ Run 'make vendor' first"
  exit 1
fi

# Guard: cluster must be reachable
if ! kubectl cluster-info --request-timeout=10s >/dev/null 2>&1; then
  echo "❌ Cluster unreachable. Start your cluster first."
  exit 1
fi

CTX=$(kubectl config current-context 2>/dev/null || echo "none")

# Step 1: Rebuild
echo "🔨 Rebuilding image ${FULL_IMAGE} (offline, using vendor/)..."
docker build -t "${FULL_IMAGE}" .
echo "✅ Built"

# Step 2: Load into cluster
echo ""
if echo "${CTX}" | grep -q "minikube"; then
  echo "📤 Loading into minikube..."
  minikube image load "${FULL_IMAGE}"
elif echo "${CTX}" | grep -q "kind"; then
  CLUSTER_NAME=$(echo "${CTX}" | sed 's/kind-//')
  echo "📤 Loading into kind (${CLUSTER_NAME})..."
  kind load docker-image "${FULL_IMAGE}" --name "${CLUSTER_NAME}"
else
  echo "ℹ️  Generic cluster — ensure image is accessible"
fi
echo "✅ Loaded"

# Step 3: Force pod restart
echo ""
echo "♻️  Forcing rollout restart to pick up new image..."
kubectl rollout restart deployment/k8s-healer -n "${NAMESPACE}"

echo "⏳ Waiting for new pods to come up (120s)..."
kubectl rollout status deployment/k8s-healer -n "${NAMESPACE}" --timeout=120s

echo ""
echo "✅ Redeployed! New code is now running."
echo "   Logs: kubectl logs -f deployment/k8s-healer -n ${NAMESPACE}"
