#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# K8s AI Healer — Complete Demo Setup Script
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

echo "🏆 K8s AI Healer — Hackathon Demo Setup"
echo "════════════════════════════════════════"
echo ""

# Check cluster
if ! kubectl cluster-info --request-timeout=5s >/dev/null 2>&1; then
  echo "❌ No Kubernetes cluster detected."
  echo "   Start minikube or kind first."
  exit 1
fi

CTX=$(kubectl config current-context 2>/dev/null || echo "unknown")
echo "✅ Using cluster: ${CTX}"
echo ""

# Detect cluster type
if echo "${CTX}" | grep -q "minikube"; then
  CLUSTER_TYPE="minikube"
elif echo "${CTX}" | grep -q "kind"; then
  CLUSTER_TYPE="kind"
  CLUSTER_NAME=$(echo "${CTX}" | sed 's/kind-//')
else
  CLUSTER_TYPE="other"
fi

echo "🔍 Detected cluster type: ${CLUSTER_TYPE}"
echo ""

# ── Step 1: Deploy the Healer ────────────────────────────────────────────────
echo "📦 Step 1/3: Building and deploying K8s AI Healer..."
cd ..

if [ ! -d "vendor" ]; then
  echo "   Running: make vendor (one-time setup)..."
  make vendor >/dev/null 2>&1
fi

echo "   Building healer image..."
docker build -t k8s-healer:latest . >/dev/null 2>&1

echo "   Loading into cluster..."
if [ "${CLUSTER_TYPE}" = "minikube" ]; then
  minikube image load k8s-healer:latest >/dev/null 2>&1
elif [ "${CLUSTER_TYPE}" = "kind" ]; then
  kind load docker-image k8s-healer:latest --name "${CLUSTER_NAME}" >/dev/null 2>&1
fi

echo "   Applying manifests..."
kubectl apply -f deployments/namespace.yaml >/dev/null 2>&1
kubectl apply -f deployments/rbac.yaml >/dev/null 2>&1
kubectl apply -f deployments/deployment.yaml >/dev/null 2>&1

echo "   Waiting for healer to start..."
kubectl wait --for=condition=available --timeout=90s \
  deployment/k8s-healer -n healer-system >/dev/null 2>&1

echo "✅ Healer deployed!"
echo ""

# ── Step 2: Deploy the Problematic App ───────────────────────────────────────
cd demo
echo "🚨 Step 2/3: Building and deploying the problematic demo app..."

echo "   Building problem-app image..."
docker build -t problem-app:latest . >/dev/null 2>&1

echo "   Loading into cluster..."
if [ "${CLUSTER_TYPE}" = "minikube" ]; then
  minikube image load problem-app:latest >/dev/null 2>&1
elif [ "${CLUSTER_TYPE}" = "kind" ]; then
  kind load docker-image problem-app:latest --name "${CLUSTER_NAME}" >/dev/null 2>&1
fi

echo "   Deploying problem-app..."
kubectl apply -f problem-app-deployment.yaml >/dev/null 2>&1

echo "   Waiting for problem-app to start..."
kubectl wait --for=condition=available --timeout=60s \
  deployment/problem-app -n default >/dev/null 2>&1 || true

echo "✅ Problem app deployed!"
echo ""

# ── Step 3: Generate Traffic ─────────────────────────────────────────────────
echo "🔄 Step 3/3: Starting traffic generator to accelerate issues..."

kubectl run load-generator --image=busybox --restart=Never -- \
  sh -c "while true; do wget -q -O- http://problem-app.default.svc.cluster.local && echo .; sleep 2; done" \
  >/dev/null 2>&1 || true

echo "✅ Traffic generator started!"
echo ""

# ── Done ──────────────────────────────────────────────────────────────────────
echo "════════════════════════════════════════"
echo "🎉 Demo setup complete!"
echo "════════════════════════════════════════"
echo ""
echo "📊 Next steps for your presentation:"
echo ""
echo "1. Open the Healer dashboard:"
echo "   kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system"
echo "   → http://localhost:8080"
echo ""
echo "2. (Optional) Open the problem app:"
echo "   kubectl port-forward svc/problem-app 8081:80 -n default"
echo "   → http://localhost:8081"
echo ""
echo "3. Watch healer logs:"
echo "   kubectl logs -f deployment/k8s-healer -n healer-system"
echo ""
echo "4. Watch problem app get restarted:"
echo "   kubectl get pods -n default -l app=problem-app -w"
echo ""
echo "5. See the full demo script:"
echo "   cat DEMO-SCRIPT.md"
echo ""
echo "💡 The healer will detect and fix issues within 1-2 minutes."
echo "   Refresh the dashboard to see healing actions in real-time!"
