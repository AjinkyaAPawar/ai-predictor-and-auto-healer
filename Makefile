# K8s AI Healer — Makefile
# ─────────────────────────────────────────────────────────────────────────────
# FIRST TIME SETUP (requires internet, run once):
#   make vendor        # downloads all Go deps into vendor/
#
# AFTER THAT — everything is 100% offline:
#   make build         # compile binary
#   make docker-build  # build Docker image
#   make deploy        # build + load into cluster + apply manifests
# ─────────────────────────────────────────────────────────────────────────────

BINARY     := healer
IMAGE_NAME := k8s-healer
IMAGE_TAG  := latest
NAMESPACE  := healer-system
FULL_IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: all vendor build build-linux run run-dry docker-build load-image \
        deploy undeploy logs status fmt vet test clean help \
        check-cluster check-vendor check-docker

all: build

# ── Internal guard targets ───────────────────────────────────────────────────

# Verify a Kubernetes cluster is reachable before doing anything kubectl-related.
# Uses `kubectl cluster-info` which hits the API server directly (no openapi schema).
check-cluster:
	@echo "🔍 Checking Kubernetes cluster connectivity..."
	@if ! kubectl cluster-info --request-timeout=10s >/dev/null 2>&1; then \
		echo ""; \
		echo "❌ Cannot reach a Kubernetes cluster."; \
		echo ""; \
		echo "   Make sure one of the following is running and configured:"; \
		echo "     minikube:  minikube start"; \
		echo "     kind:      kind create cluster"; \
		echo "     other:     export KUBECONFIG=/path/to/your/kubeconfig"; \
		echo ""; \
		echo "   Then verify with:  kubectl cluster-info"; \
		echo ""; \
		exit 1; \
	fi
	@echo "✅ Cluster reachable: $$(kubectl config current-context 2>/dev/null || echo '(unknown context)')"

# Verify vendor/ exists before any Go or Docker build.
check-vendor:
	@if [ ! -d "vendor" ]; then \
		echo ""; \
		echo "❌ vendor/ directory not found."; \
		echo ""; \
		echo "   Run this once (requires internet):"; \
		echo "     make vendor"; \
		echo ""; \
		exit 1; \
	fi

# Verify Docker daemon is running.
check-docker:
	@if ! docker info >/dev/null 2>&1; then \
		echo ""; \
		echo "❌ Docker daemon is not running."; \
		echo "   Start Docker and retry."; \
		echo ""; \
		exit 1; \
	fi

# ── Dependency / vendor ──────────────────────────────────────────────────────

## vendor: Download all Go dependencies into vendor/ (ONE TIME, needs internet)
vendor:
	@echo "📦 Vendoring Go dependencies (needs internet — run once only)..."
	go mod download
	go mod vendor
	@echo "✅ vendor/ is ready. All future builds work fully offline."

# ── Build ────────────────────────────────────────────────────────────────────

## build: Compile the Go binary using vendor/ (offline)
build: check-vendor
	@echo "▶ Building $(BINARY) (offline, using vendor/)..."
	@mkdir -p bin
	go build -mod=vendor -o bin/$(BINARY) cmd/healer/main.go
	@echo "✅ Binary ready: bin/$(BINARY)"

## build-linux: Compile a static Linux binary for Docker (offline)
build-linux: check-vendor
	@echo "▶ Building static Linux binary (offline)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -mod=vendor \
		-ldflags="-s -w -extldflags=-static" \
		-o bin/$(BINARY) \
		cmd/healer/main.go
	@echo "✅ Linux binary ready: bin/$(BINARY)"

# ── Local run ────────────────────────────────────────────────────────────────

## run: Run locally using ~/.kube/config
run: build
	@echo "▶ Starting healer locally..."
	./bin/$(BINARY)

## run-dry: Run in dry-run mode (logs actions, makes zero changes)
run-dry: build
	@echo "▶ Starting healer in DRY-RUN mode (no real changes)..."
	HEALER_DRY_RUN=true ./bin/$(BINARY)

# ── Docker ───────────────────────────────────────────────────────────────────

## docker-build: Build the Docker image offline using vendor/
docker-build: check-vendor check-docker
	@echo "▶ Building Docker image $(FULL_IMAGE) (offline, using vendor/)..."
	docker build -t $(FULL_IMAGE) .
	@echo "✅ Image built: $(FULL_IMAGE)"

# ── Image loading ─────────────────────────────────────────────────────────────

## load-image: Load the built image into your local cluster (no registry push needed)
load-image: docker-build check-cluster
	@$(MAKE) --no-print-directory _load-image-inner

# Internal target: runs in shell so we can use if/elif at runtime (not parse-time)
_load-image-inner:
	@CTX=$$(kubectl config current-context 2>/dev/null || echo "none"); \
	echo "▶ Loading image $(FULL_IMAGE) into cluster context: $$CTX"; \
	if echo "$$CTX" | grep -q "minikube"; then \
		echo "   Detected: minikube"; \
		minikube image load $(FULL_IMAGE); \
		echo "✅ Image loaded into minikube"; \
	elif echo "$$CTX" | grep -q "kind"; then \
		CLUSTER=$$(echo "$$CTX" | sed 's/kind-//'); \
		echo "   Detected: kind (cluster: $$CLUSTER)"; \
		kind load docker-image $(FULL_IMAGE) --name "$$CLUSTER"; \
		echo "✅ Image loaded into kind cluster: $$CLUSTER"; \
	else \
		echo ""; \
		echo "ℹ️  Cluster type: generic/remote (context: $$CTX)"; \
		echo "   The image $(FULL_IMAGE) is built locally."; \
		echo "   To use it on a remote cluster, push to your internal registry:"; \
		echo "     docker tag $(FULL_IMAGE) registry.yourcompany.com/$(FULL_IMAGE)"; \
		echo "     docker push registry.yourcompany.com/$(FULL_IMAGE)"; \
		echo "   Then update deployments/deployment.yaml:"; \
		echo "     image: registry.yourcompany.com/$(FULL_IMAGE)"; \
		echo "     imagePullPolicy: IfNotPresent"; \
		echo ""; \
		echo "   Continuing with kubectl apply (image must be accessible to the cluster)..."; \
	fi

# ── Deploy ───────────────────────────────────────────────────────────────────

## deploy: Full pipeline — build image + load into cluster + apply manifests
deploy: load-image
	@echo ""
	@echo "▶ Applying Kubernetes manifests..."
	kubectl apply --validate=false -f deployments/namespace.yaml
	kubectl apply --validate=false -f deployments/rbac.yaml
	kubectl apply --validate=false -f deployments/deployment.yaml
	@echo ""
	@echo "⏳ Waiting for deployment to become available (timeout: 120s)..."
	kubectl wait --for=condition=available --timeout=120s \
		deployment/k8s-healer -n $(NAMESPACE) 2>/dev/null || \
	kubectl rollout status deployment/k8s-healer -n $(NAMESPACE)
	@echo ""
	@echo "✅ Deployed! Running from local image — zero external dependencies."
	@echo ""
	@echo "   Pods:      kubectl get pods -n $(NAMESPACE)"
	@echo "   Logs:      kubectl logs -f deployment/k8s-healer -n $(NAMESPACE)"
	@echo "   Dashboard: kubectl port-forward svc/k8s-healer 8080:8080 -n $(NAMESPACE)"
	@echo "              → http://localhost:8080"

## redeploy: Rebuild image + force restart (fast iteration: code change → running pod)
redeploy: load-image
	@echo ""
	@echo "♻️  Force-restarting deployment to pick up new image..."
	kubectl rollout restart deployment/k8s-healer -n $(NAMESPACE)
	@echo "⏳ Waiting for rollout to complete (timeout: 120s)..."
	kubectl rollout status deployment/k8s-healer -n $(NAMESPACE) --timeout=120s
	@echo ""
	@echo "✅ Redeployed! New image is now running."
	@echo "   Logs: kubectl logs -f deployment/k8s-healer -n $(NAMESPACE)"

## undeploy: Remove all Kubernetes resources
undeploy: check-cluster
	@echo "▶ Removing all Kubernetes resources..."
	kubectl delete -f deployments/deployment.yaml --ignore-not-found
	kubectl delete -f deployments/rbac.yaml --ignore-not-found
	kubectl delete -f deployments/namespace.yaml --ignore-not-found
	@echo "✅ All resources removed"

# ── Observability ─────────────────────────────────────────────────────────────

## logs: Tail live logs from the deployed pod
logs:
	kubectl logs -f deployment/k8s-healer -n $(NAMESPACE)

## status: Show deployment, pod, and service status
status:
	@echo "=== Context ==="
	@kubectl config current-context 2>/dev/null || echo "No context set"
	@echo ""
	@echo "=== Deployment ==="
	@kubectl get deployment k8s-healer -n $(NAMESPACE) 2>/dev/null || echo "Not deployed"
	@echo ""
	@echo "=== Pods ==="
	@kubectl get pods -n $(NAMESPACE) -o wide 2>/dev/null || echo "No pods found"
	@echo ""
	@echo "=== Service ==="
	@kubectl get svc k8s-healer -n $(NAMESPACE) 2>/dev/null || echo "No service found"

# ── Code quality ──────────────────────────────────────────────────────────────

## fmt: Format Go source
fmt:
	go fmt ./...

## vet: Run Go vet (uses vendor/)
vet: check-vendor
	go vet -mod=vendor ./...

## test: Run tests (uses vendor/)
test: check-vendor
	go test -mod=vendor ./... -v

# ── Cleanup ───────────────────────────────────────────────────────────────────

## clean: Remove compiled binary
clean:
	@rm -f bin/$(BINARY)
	@echo "✅ Cleaned"

# ── Help ──────────────────────────────────────────────────────────────────────

## help: Show this help message
help:
	@echo ""
	@echo "K8s AI Healer — Make targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
	@echo ""
	@echo "Quick start:"
	@echo "  make vendor      ← run once with internet"
	@echo "  make deploy      ← then this forever, fully offline"
	@echo ""
