#!/bin/bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-k8s-healer}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
NAMESPACE="healer-system"

echo "🔨 Building K8s Healer Docker image (${IMAGE_NAME}:${IMAGE_TAG})..."
docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" .

echo "🚀 Deploying to Kubernetes..."
kubectl apply -f deployments/namespace.yaml
kubectl apply -f deployments/rbac.yaml
kubectl apply -f deployments/deployment.yaml

echo "⏳ Waiting for deployment to become available (timeout: 90s)..."
kubectl wait --for=condition=available --timeout=90s \
  deployment/k8s-healer -n "${NAMESPACE}"

echo ""
echo "✅ Deployment complete!"
echo "   Check status:    kubectl get pods -n ${NAMESPACE}"
echo "   View logs:       kubectl logs -f deployment/k8s-healer -n ${NAMESPACE}"
echo "   Open dashboard:  kubectl port-forward svc/k8s-healer 8080:8080 -n ${NAMESPACE}"
echo "                    then visit http://localhost:8080"
