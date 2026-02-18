# K8s AI Infrastructure Healer

Advanced AI-powered Kubernetes monitoring and auto-healing system — **100% self-contained, zero external dependencies**.

> Built and deployed entirely from local source. No external registry, no remote manifests, no internet required after initial vendor setup.

---

## How Self-Containment Works

| Step | What happens | Internet needed? |
|---|---|---|
| `make vendor` | Downloads Go deps into `vendor/` | ✅ Yes — once only |
| `make docker-build` | Builds image using `vendor/`, no `go mod download` | ❌ No |
| `make deploy` | Loads image into cluster, applies local manifests | ❌ No |
| Kubernetes runs it | `imagePullPolicy: Never` — uses local image only | ❌ No |

After the first `make vendor`, the project builds and deploys forever with zero network access.

---

## Project Structure

```
k8s-ai-healer/
├── cmd/healer/main.go              # Entry point — reads env vars, wires components
├── internal/
│   ├── actions/actions.go          # Pod restart, deployment scale-up
│   ├── api/api.go                  # HTTP REST API + web dashboard (port 8080)
│   ├── collector/collector.go      # Pod & node metrics via K8s API
│   ├── diagnostics/
│   │   ├── auto_healer.go          # Cleanup tmp/disk, fix DNS/network
│   │   ├── container_checks.go     # DNS, disk, /tmp, network checks per container
│   │   ├── diagnostics.go          # Stuck container detection via exec
│   │   └── restart_analyzer.go     # Restart pattern & OOM analysis
│   └── predictor/predictor.go      # Trend analysis, memory leak detection
├── deployments/
│   ├── namespace.yaml              # healer-system Namespace
│   ├── rbac.yaml                   # ServiceAccount + ClusterRole + Binding
│   ├── deployment.yaml             # Deployment + Service (imagePullPolicy: Never)
│   └── install.yaml                # All-in-one manifest (also local image)
├── scripts/
│   ├── setup-vendor.sh             # One-time: vendor all Go deps (needs internet)
│   ├── deploy.sh                   # Build + load + deploy (fully offline)
│   └── deploy-production.sh        # Production deploy with confirmation prompt
├── vendor/                         # Vendored Go dependencies (commit this!)
├── Dockerfile                      # Multi-stage: vendored build → scratch image
├── Makefile                        # All workflows in one place
├── go.mod
└── go.sum
```

---

## Quick Start

### Step 1 — Vendor dependencies (one time, needs internet)

```bash
make vendor
# or: ./scripts/setup-vendor.sh
```

This downloads all Go module dependencies into `vendor/`. After this runs once, **everything else is offline**.

### Step 2 — Build, load & deploy

```bash
make deploy
```

This automatically:
1. Builds the Docker image using `vendor/` (no `go mod download`)
2. Detects your cluster type (minikube / kind / other)
3. Loads the image directly into the cluster — no registry needed
4. Applies all Kubernetes manifests

### Step 3 — Open the dashboard

```bash
kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system
# Then open: http://localhost:8080
```

---

## Installation Options

### Option A — Full offline deploy (recommended)

```bash
# One time only (internet required):
make vendor

# All subsequent runs are 100% offline:
make deploy
```

### Option B — Manual step-by-step

```bash
# 1. Vendor dependencies (once, with internet)
go mod vendor

# 2. Build the Docker image
docker build -t k8s-healer:latest .

# 3. Load into your cluster
minikube image load k8s-healer:latest      # minikube
# kind load docker-image k8s-healer:latest  # kind

# 4. Apply manifests
kubectl apply -f deployments/namespace.yaml
kubectl apply -f deployments/rbac.yaml
kubectl apply -f deployments/deployment.yaml

# 5. Wait for it to come up
kubectl wait --for=condition=available --timeout=90s \
  deployment/k8s-healer -n healer-system
```

### Option C — Using the all-in-one manifest

```bash
# Build the image first (see above), then:
kubectl apply -f deployments/install.yaml
```

### Option D — Run locally (no Kubernetes needed)

```bash
make build     # produces bin/healer
make run       # uses ~/.kube/config
make run-dry   # dry-run mode: logs what would happen, no real actions
```

---

## Cluster Type Notes

### minikube

```bash
make deploy
# Automatically runs: minikube image load k8s-healer:latest
```

### kind

```bash
make deploy
# Automatically runs: kind load docker-image k8s-healer:latest --name <cluster>
```

### Internal registry (production clusters)

```bash
# Build locally
docker build -t k8s-healer:latest .

# Tag for your internal registry
docker tag k8s-healer:latest registry.yourcompany.com/k8s-healer:latest
docker push registry.yourcompany.com/k8s-healer:latest

# Update the image field in deployments/deployment.yaml:
#   image: registry.yourcompany.com/k8s-healer:latest
#   imagePullPolicy: IfNotPresent   (change from Never for registry use)

kubectl apply -f deployments/namespace.yaml
kubectl apply -f deployments/rbac.yaml
kubectl apply -f deployments/deployment.yaml
```

---

## Configuration

All settings via environment variables — no config files.

| Variable | Default | Description |
|---|---|---|
| `HEALER_PORT` | `8080` | HTTP API / dashboard port |
| `HEALER_DRY_RUN` | `false` | `true` = log actions but make no changes |
| `HEALER_LOG_LEVEL` | `info` | Verbosity: `info` or `debug` |
| `HEALER_CHECK_INTERVAL` | `30` | Seconds between health check cycles |
| `KUBECONFIG` | `~/.kube/config` | Kubeconfig path (local runs only) |

### Dry-Run Mode (safe testing)

```bash
# Local
HEALER_DRY_RUN=true ./bin/healer

# Or edit deployments/deployment.yaml:
# - name: HEALER_DRY_RUN
#   value: "true"
```

---

## What It Detects & Fixes

### Detection

| What | How |
|---|---|
| Stuck containers | `exec` into container, check `ps`/`uptime` — if exec fails or load is constant, container is stuck |
| DNS failures | `nslookup kubernetes.default.svc.cluster.local` inside container |
| Disk space | `df /` inside container — WARNING >80%, CRITICAL >90% |
| `/tmp` full | `df /tmp` — WARNING >85%, CRITICAL >95% |
| Network issues | `ping`/`wget` to cluster API and external from inside container |
| Restart loops | Counts restarts, calculates frequency, reads exit codes (OOMKilled, SIGKILL) |
| Memory leak | Linear regression over last 10 measurements — predicts time to OOM |
| CPU growth | Same trend analysis — predicts time to saturation |

### Auto-Healing Actions

| Problem | Action taken |
|---|---|
| `/tmp` > 95% | Delete files older than 1 day, files > 10MB, `*.tmp`, `core.*` |
| Disk > 90% | Truncate large log files, delete old temp files |
| DNS fails | Restart the pod (DNS recovers via pod re-schedule) |
| Network fails | Flush route cache; if still failing, restart pod |
| CPU > 15% | Scale parent Deployment up by 1 replica |
| Memory > 15% | Restart pod |
| Memory leak predicted | Restart pod (urgency based on hours-to-failure) |

### Safety Features

- **Dry-run mode** — `HEALER_DRY_RUN=true` logs everything, changes nothing
- **Max 3 actions per pod** — prevents infinite restart loops
- **System pod exclusion** — never touches `kube-*` or `healer-*` namespaces
- **Graceful restart** — uses Kubernetes Delete (Deployment controller recreates)

---

## API Reference

All endpoints on `http://localhost:8080` (after port-forwarding).

### `GET /health`

```bash
curl http://localhost:8080/health
```
```json
{
  "status": "UP",
  "timestamp": "2024-01-01T12:00:00Z",
  "service": "k8s-ai-healer",
  "version": "3.0"
}
```

### `GET /status`

```bash
curl http://localhost:8080/status
```
```json
{
  "status": "ACTIVE",
  "timestamp": "2024-01-01T12:00:00Z",
  "total_actions": 12,
  "system_health": "HEALTHY",
  "recent_actions": [...]
}
```

### `GET /actions`

```bash
curl http://localhost:8080/actions
```
```json
{
  "total_actions": 12,
  "actions": [
    {
      "ActionType": "CLEANUP_TMP",
      "PodName": "my-app-abc123",
      "Namespace": "default",
      "ContainerName": "app",
      "Status": "COMPLETED",
      "Timestamp": "2024-01-01T12:00:00Z",
      "Result": "Cleanup executed"
    }
  ]
}
```

### `GET /`

Web dashboard — open in browser after port-forwarding.

---

## Kubernetes RBAC

The healer runs as a dedicated ServiceAccount with the minimum required permissions:

| Resource | Verbs | Purpose |
|---|---|---|
| `pods` | get, list, watch, delete | Monitor and restart pods |
| `pods/exec` | create | Run diagnostic commands inside containers |
| `pods/log` | get | Fetch logs for diagnostics |
| `events` | get, list, watch | Restart pattern analysis |
| `nodes` | get, list, watch | Node metrics |
| `deployments` | get, list, watch, update, patch | Scale-up healing |
| `replicasets` | get, list, watch | Deployment controller support |
| `metrics.k8s.io/pods,nodes` | get, list | CPU/memory data |

All RBAC resources are in `deployments/rbac.yaml`.

---

## Makefile Reference

```
make vendor        Download all Go deps into vendor/ (once, needs internet)
make build         Compile binary locally using vendor/ (offline)
make build-linux   Compile static Linux binary (for Docker)
make run           Build and run locally with ~/.kube/config
make run-dry       Run in dry-run mode (no real changes)
make docker-build  Build Docker image using vendor/ (offline)
make load-image    Load image into minikube/kind cluster
make deploy        Full pipeline: build + load + apply manifests
make undeploy      Delete all Kubernetes resources
make logs          Tail live pod logs
make status        Show deployment, pod, and service status
make fmt           Run go fmt
make vet           Run go vet
make test          Run go test
make clean         Delete compiled binary
```

---

## Troubleshooting

### `vendor/` directory missing

```
❌ Run 'make vendor' first
```

Run `make vendor` (requires internet — one time only).

### Metrics API not available

```
Warning: Metrics API not available: the server could not find the requested resource
```

The Metrics Server add-on is not installed in your cluster. The healer continues to work — trend analysis and predictions are disabled until metrics are available.

Install Metrics Server for your cluster type:

**minikube:**
```bash
minikube addons enable metrics-server
```

**kind / other clusters:**
```bash
# Download the manifest locally, then apply from disk (no external URL at runtime):
curl -Lo deployments/metrics-server.yaml \
  https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# For kind (needs insecure TLS):
kubectl patch -f deployments/metrics-server.yaml \
  --local \
  --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]' \
  -o yaml > deployments/metrics-server-patched.yaml

kubectl apply -f deployments/metrics-server-patched.yaml
```

Verify:
```bash
kubectl top nodes
kubectl top pods -A
```

### Image not found — ErrImageNeverPull

```
Failed to pull image "k8s-healer:latest": rpc error: ... ErrImageNeverPull
```

The image isn't loaded into the cluster. Run:

```bash
# minikube
minikube image load k8s-healer:latest

# kind
kind load docker-image k8s-healer:latest --name <your-cluster-name>
```

Or run `make deploy` which does this automatically.

### RBAC / Permission denied

```bash
# Check if the service account has the right permissions
kubectl auth can-i get pods \
  --as=system:serviceaccount:healer-system:k8s-healer -A

kubectl auth can-i create pods/exec \
  --as=system:serviceaccount:healer-system:k8s-healer -A
```

Re-apply if needed:
```bash
kubectl apply -f deployments/rbac.yaml
```

### Container exec checks fail

Expected behaviour for minimal/distroless containers (no `/bin/sh`, `df`, `nslookup`, etc.). The healer logs these as warnings and continues — it will not incorrectly flag these as stuck.

---

## Production Deployment

```bash
# Uses deploy-production.sh which:
# 1. Asks for confirmation
# 2. Builds image with vendor/
# 3. Loads into cluster
# 4. Deploys as k8s-healer-production with real healing actions
./scripts/deploy-production.sh
```

Or with a custom image tag:

```bash
IMAGE_NAME=k8s-healer IMAGE_TAG=v4.0 ./scripts/deploy-production.sh
```

---

## Development Workflow

```bash
# One-time setup
make vendor

# Edit code, then fast rebuild + redeploy:
make docker-build
minikube image load k8s-healer:latest   # or kind load ...
kubectl rollout restart deployment/k8s-healer -n healer-system

# Watch logs
make logs

# Dry-run to test logic without side effects
make run-dry
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

Made with ❤️ for the Kubernetes community
