# K8s AI Healer - Makefile
# -----------------------------------------------
# Usage:
#   make build          - Build the binary locally
#   make run            - Run locally (requires ~/.kube/config)
#   make docker-build   - Build Docker image
#   make deploy         - Full deploy to Kubernetes (build + apply manifests)
#   make undeploy       - Remove all Kubernetes resources
#   make logs           - Tail healer logs from Kubernetes
#   make status         - Show pod/deployment status
#   make clean          - Remove built binary

BINARY     := healer
IMAGE_NAME := k8s-healer
IMAGE_TAG  := latest
NAMESPACE  := healer-system

# Go build settings
GOOS       ?= linux
GOARCH     ?= amd64
CGO_ENABLED:= 0

.PHONY: all build run docker-build deploy undeploy logs status clean fmt vet test help

all: build

## build: Compile the Go binary for the current OS/arch
build:
	@echo "▶ Building $(BINARY)..."
	go build -o bin/$(BINARY) cmd/healer/main.go
	@echo "✅ Binary ready: bin/$(BINARY)"

## build-linux: Compile a static Linux binary (for Docker)
build-linux:
	@echo "▶ Building static Linux binary..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -o bin/$(BINARY) cmd/healer/main.go
	@echo "✅ Linux binary ready: bin/$(BINARY)"

## run: Run the healer locally using ~/.kube/config
run: build
	@echo "▶ Starting healer locally..."
	./bin/$(BINARY)

## run-dry: Run in dry-run mode (no real changes)
run-dry: build
	@echo "▶ Starting healer in DRY-RUN mode..."
	HEALER_DRY_RUN=true ./bin/$(BINARY)

## docker-build: Build the Docker image
docker-build:
	@echo "▶ Building Docker image $(IMAGE_NAME):$(IMAGE_TAG)..."
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "✅ Docker image built: $(IMAGE_NAME):$(IMAGE_TAG)"

## docker-push: Push Docker image to registry (set IMAGE_NAME to your registry path)
docker-push:
	@echo "▶ Pushing $(IMAGE_NAME):$(IMAGE_TAG)..."
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

## deploy: Build Docker image and deploy all Kubernetes resources
deploy: docker-build
	@echo "▶ Deploying to Kubernetes namespace $(NAMESPACE)..."
	kubectl apply -f deployments/namespace.yaml
	kubectl apply -f deployments/rbac.yaml
	kubectl apply -f deployments/deployment.yaml
	@echo "⏳ Waiting for deployment to become available..."
	kubectl wait --for=condition=available --timeout=90s deployment/k8s-healer -n $(NAMESPACE)
	@echo "✅ Deployment complete!"
	@echo ""
	@echo "  View pods:   kubectl get pods -n $(NAMESPACE)"
	@echo "  View logs:   kubectl logs -f deployment/k8s-healer -n $(NAMESPACE)"
	@echo "  Dashboard:   kubectl port-forward svc/k8s-healer 8080:8080 -n $(NAMESPACE)"

## undeploy: Remove all Kubernetes resources
undeploy:
	@echo "▶ Removing Kubernetes resources..."
	kubectl delete -f deployments/deployment.yaml --ignore-not-found
	kubectl delete -f deployments/rbac.yaml --ignore-not-found
	kubectl delete -f deployments/namespace.yaml --ignore-not-found
	@echo "✅ All resources removed"

## logs: Tail healer logs from Kubernetes
logs:
	kubectl logs -f deployment/k8s-healer -n $(NAMESPACE)

## status: Show pod and deployment status
status:
	@echo "=== Deployment ==="
	kubectl get deployment k8s-healer -n $(NAMESPACE) 2>/dev/null || echo "Not deployed"
	@echo ""
	@echo "=== Pods ==="
	kubectl get pods -n $(NAMESPACE) 2>/dev/null || echo "No pods found"

## fmt: Format Go source code
fmt:
	go fmt ./...

## vet: Run Go vet
vet:
	go vet ./...

## test: Run tests
test:
	go test ./... -v

## clean: Remove built binaries
clean:
	@rm -f bin/$(BINARY)
	@echo "✅ Cleaned"

## help: Show this help message
help:
	@echo "K8s AI Healer - Available make targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
