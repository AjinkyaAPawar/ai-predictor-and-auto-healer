#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# K8s AI Healer — Build & Deploy (100% local, zero external registry)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-k8s-healer}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
NAMESPACE="healer-system"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

# ── Guard: vendor/ must exist ─────────────────────────────────────────────────
if [ ! -d "vendor" ]; then
  echo ""
  echo "❌ vendor/ directory not found."
  echo ""
  echo "   Run this once (requires internet), then re-run this script:"
  echo "     make vendor"
  echo "     # or: go mod vendor"
  echo ""
  exit 1
fi

# ── Guard: cluster must be reachable ──────────────────────────────────────────
echo "🔍 Checking Kubernetes cluster connectivity..."
if ! kubectl cluster-info --request-timeout=10s >/dev/null 2>&1; then
  echo ""
  echo "❌ Cannot reach a Kubernetes cluster."
  echo ""
  echo "   Start your cluster first, then re-run this script:"
  echo "     minikube:  minikube start"
  echo "     kind:      kind create cluster"
  echo "     other:     export KUBECONFIG=/path/to/your/kubeconfig"
  echo ""
  echo "   Verify with:  kubectl cluster-info"
  echo ""
  exit 1
fi

CTX=$(kubectl config current-context 2>/dev/null || echo "none")
echo "✅ Cluster reachable: ${CTX}"

# ── Step 1: Build Docker image (offline using vendor/) ────────────────────────
echo ""
echo "🔨 Building Docker image ${FULL_IMAGE} (offline, using vendor/)..."
docker build -t "${FULL_IMAGE}" .
echo "✅ Image built: ${FULL_IMAGE}"

# ── Step 2: Load image into cluster (no registry push) ───────────────────────
echo ""
if echo "${CTX}" | grep -q "minikube"; then
  echo "📤 Detected minikube — loading image..."
  minikube image load "${FULL_IMAGE}"
  echo "✅ Image loaded into minikube"

elif echo "${CTX}" | grep -q "kind"; then
  CLUSTER_NAME=$(echo "${CTX}" | sed 's/kind-//')
  echo "📤 Detected kind (cluster: ${CLUSTER_NAME}) — loading image..."
  kind load docker-image "${FULL_IMAGE}" --name "${CLUSTER_NAME}"
  echo "✅ Image loaded into kind cluster: ${CLUSTER_NAME}"

else
  echo "ℹ️  Generic/remote cluster (${CTX})"
  echo "   Image built locally as: ${FULL_IMAGE}"
  echo "   Push to your internal registry if the cluster can't see this image:"
  echo "     docker tag ${FULL_IMAGE} registry.yourcompany.com/${FULL_IMAGE}"
  echo "     docker push registry.yourcompany.com/${FULL_IMAGE}"
  echo "   Then update deployments/deployment.yaml image: field."
  echo ""
fi

# ── Step 3: Apply Kubernetes manifests ───────────────────────────────────────
echo "🚀 Applying Kubernetes manifests..."
# --validate=false avoids hitting the openapi schema endpoint (works even when
# the API server returns a non-standard or unreachable openapi/v2 path)
kubectl apply --validate=false -f deployments/namespace.yaml
kubectl apply --validate=false -f deployments/rbac.yaml
kubectl apply --validate=false -f deployments/deployment.yaml

# ── Step 4: Wait for rollout ──────────────────────────────────────────────────
echo ""
echo "⏳ Waiting for deployment to become available (timeout: 120s)..."
if ! kubectl wait --for=condition=available --timeout=120s \
    deployment/k8s-healer -n "${NAMESPACE}" 2>/dev/null; then
  echo "⚠️  wait timed out — checking rollout status..."
  kubectl rollout status deployment/k8s-healer -n "${NAMESPACE}"
fi

echo ""
echo "✅ Deployment complete! Running entirely from local image."
echo ""
echo "   Pods:      kubectl get pods -n ${NAMESPACE}"
echo "   Logs:      kubectl logs -f deployment/k8s-healer -n ${NAMESPACE}"
echo "   Dashboard: kubectl port-forward svc/k8s-healer 8080:8080 -n ${NAMESPACE}"
echo "              → http://localhost:8080"
