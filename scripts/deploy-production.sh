#!/bin/bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-k8s-healer}"
IMAGE_TAG="${IMAGE_TAG:-production}"
NAMESPACE="healer-system"

echo "🚀 Deploying K8s Healer in PRODUCTION mode (real healing actions enabled)..."
echo "⚠️  WARNING: This will perform REAL healing actions on your cluster!"
echo ""
read -p "Are you sure? Type 'yes' to continue: " confirm
if [[ "${confirm}" != "yes" ]]; then
  echo "Aborted."
  exit 1
fi

# Build the production image
echo "🔨 Building production image (${IMAGE_NAME}:${IMAGE_TAG})..."
docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" .

# Write a production-specific deployment manifest
PROD_MANIFEST="deployments/deployment-production.yaml"
cat > "${PROD_MANIFEST}" << YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8s-healer-production
  namespace: ${NAMESPACE}
  labels:
    app: k8s-healer-production
    app.kubernetes.io/name: k8s-ai-healer
    app.kubernetes.io/version: "4.0"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: k8s-healer-production
  template:
    metadata:
      labels:
        app: k8s-healer-production
    spec:
      serviceAccountName: k8s-healer
      containers:
      - name: healer
        image: ${IMAGE_NAME}:${IMAGE_TAG}
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        env:
        - name: HEALER_PORT
          value: "8080"
        - name: HEALER_DRY_RUN
          value: "false"
        - name: HEALER_LOG_LEVEL
          value: "info"
        - name: HEALER_CHECK_INTERVAL
          value: "30"
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 30
          failureThreshold: 3
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 15
          failureThreshold: 3
          timeoutSeconds: 5
YAML

echo "📄 Applying namespace and RBAC (if not already applied)..."
kubectl apply -f deployments/namespace.yaml
kubectl apply -f deployments/rbac.yaml

echo "📄 Applying production deployment..."
kubectl apply -f "${PROD_MANIFEST}"

echo "⏳ Waiting for production deployment to become available..."
kubectl wait --for=condition=available --timeout=90s \
  deployment/k8s-healer-production -n "${NAMESPACE}"

echo ""
echo "✅ Production deployment complete!"
echo "   View logs:       kubectl logs -f deployment/k8s-healer-production -n ${NAMESPACE}"
echo "   Open dashboard:  kubectl port-forward deployment/k8s-healer-production 8080:8080 -n ${NAMESPACE}"
