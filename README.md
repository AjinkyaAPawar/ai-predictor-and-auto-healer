# K8s AI Infrastructure Healer

Advanced AI-powered Kubernetes infrastructure monitoring and auto-healing system that detects and fixes issues Kubernetes doesn't see.

## Overview

K8s AI Infrastructure Healer is a comprehensive monitoring and auto-healing solution that goes beyond standard Kubernetes health checks. It uses AI algorithms to predict failures 24-72 hours in advance and automatically remediates infrastructure issues before they impact your applications.

### Key Features

- **Predictive Intelligence**: Forecasts resource exhaustion and failures up to 72 hours ahead
- **Auto-Healing**: Automatically fixes network issues, disk space problems, and stuck containers
- **Advanced Diagnostics**: Detects problems that Kubernetes health checks miss
- **Memory Leak Detection**: Identifies and predicts memory leaks with time-to-failure estimates
- **Web Dashboard**: Real-time monitoring with REST API at `http://localhost:8080`
- **Zero External Dependencies**: Works with the standard Kubernetes API only

### What Problems Does It Solve?

1. **Stuck Containers**: Detects containers that pass health checks but are unresponsive
2. **Network Connectivity Issues**: Identifies and fixes internal cluster network problems
3. **Disk Space Management**: Monitors and automatically cleans `/tmp` directories
4. **Memory Leaks**: Predicts memory exhaustion before it happens
5. **Performance Degradation**: Detects gradual performance decline over time
6. **Restart Loops**: Analyzes restart patterns to prevent crash loops

---

## Project Structure

```
k8s-ai-healer/
├── cmd/
│   └── healer/
│       └── main.go              # Entry point — reads env vars, wires all components
├── internal/
│   ├── actions/
│   │   └── actions.go           # Healing actions: pod restart, deployment scale
│   ├── api/
│   │   └── api.go               # HTTP REST API + web dashboard (port 8080)
│   ├── collector/
│   │   └── collector.go         # Pod and node metrics collection via K8s API
│   ├── diagnostics/
│   │   ├── auto_healer.go       # Auto-healing: cleanup, DNS fix, network fix
│   │   ├── container_checks.go  # DNS, disk, /tmp, network container checks
│   │   ├── diagnostics.go       # Stuck container detection via exec
│   │   └── restart_analyzer.go  # Restart pattern and OOM analysis
│   └── predictor/
│       └── predictor.go         # AI predictions: trend analysis, memory leak detection
├── deployments/
│   ├── namespace.yaml           # healer-system Namespace
│   ├── rbac.yaml                # ServiceAccount + ClusterRole + ClusterRoleBinding
│   ├── deployment.yaml          # Deployment + Service (for local image builds)
│   └── install.yaml             # All-in-one manifest (uses public Docker Hub image)
├── scripts/
│   ├── deploy.sh                # Build image + deploy to Kubernetes
│   └── deploy-production.sh     # Production deployment with confirmation prompt
├── Dockerfile                   # Multi-stage Docker build
├── Makefile                     # Common build/run/deploy targets
├── go.mod
└── go.sum
```

---

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.20+)
- `kubectl` configured and pointing to your cluster
- **Metrics Server** installed in the cluster ([install guide](#installing-metrics-server))
- Docker (for building the image)
- Go 1.21+ (for local builds only)

### Option 1: One-Command Install (using public image)

```bash
kubectl apply -f https://raw.githubusercontent.com/Pavel-P09/k8s-ai-healer/main/deployments/install.yaml
```

Then verify:

```bash
# Check pods are running
kubectl get pods -n healer-system

# View live logs
kubectl logs -f deployment/k8s-healer -n healer-system

# Open web dashboard
kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system
# Then visit http://localhost:8080
```

### Option 2: Build from Source + Deploy

```bash
# Clone the repository
git clone https://github.com/Pavel-P09/k8s-ai-healer.git
cd k8s-ai-healer

# Build image and deploy to Kubernetes
make deploy
```

### Option 3: Local Binary (for testing/development)

```bash
# Build the binary
make build         # produces bin/healer

# Run locally (uses ~/.kube/config)
make run

# Run in dry-run mode (no real changes)
make run-dry
```

### Option 4: Docker Run (local testing)

```bash
# Build the Docker image
docker build -t k8s-healer:latest .

# Run with your kubeconfig mounted
docker run -v ~/.kube/config:/root/.kube/config k8s-healer:latest
```

---

## Kubernetes RBAC

The healer requires the following permissions:

| Resource | Verbs | Reason |
|---|---|---|
| `pods` | get, list, watch, delete | Monitor and restart unhealthy pods |
| `pods/exec` | create | Run diagnostic commands inside containers |
| `pods/log` | get | Fetch logs for diagnostics |
| `events` | get, list, watch | Analyze restart patterns |
| `nodes` | get, list, watch | Node metrics collection |
| `deployments` | get, list, watch, update, patch | Scale deployments for healing |
| `metrics.k8s.io/pods,nodes` | get, list | CPU/memory metrics |

All manifests in `deployments/rbac.yaml` handle this automatically.

---

## Configuration

All configuration is via **environment variables** — no config files needed.

| Variable | Default | Description |
|---|---|---|
| `HEALER_PORT` | `8080` | HTTP API server port |
| `HEALER_DRY_RUN` | `false` | If `true`, log actions but don't execute them |
| `HEALER_LOG_LEVEL` | `info` | Log verbosity (`info`, `debug`) |
| `HEALER_CHECK_INTERVAL` | `30` | Seconds between health checks |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig (local runs only) |

### Example: Enable Dry-Run Mode

```bash
export HEALER_DRY_RUN=true
export HEALER_CHECK_INTERVAL=60
./bin/healer
```

Or in Kubernetes, edit `deployments/deployment.yaml` and set the `HEALER_DRY_RUN` env var to `"true"`.

---

## API Reference

The healer exposes a REST API on port 8080 (default).

### `GET /health` — Liveness/Readiness Check

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

### `GET /status` — System Status

```bash
curl http://localhost:8080/status
```

```json
{
  "status": "ACTIVE",
  "timestamp": "2024-01-01T12:00:00Z",
  "total_actions": 42,
  "system_health": "HEALTHY",
  "recent_actions": [...]
}
```

### `GET /actions` — Healing Actions History

```bash
curl http://localhost:8080/actions
```

```json
{
  "total_actions": 42,
  "actions": [
    {
      "ActionType": "RESTART_POD_NETWORK",
      "PodName": "web-app-abc123",
      "Namespace": "default",
      "ContainerName": "app",
      "Status": "COMPLETED",
      "Timestamp": "2024-01-01T12:00:00Z",
      "Result": "Pod restarted successfully"
    }
  ]
}
```

### `GET /` — Web Dashboard

Open `http://localhost:8080` in your browser for the interactive dashboard.

---

## How It Works

### Detection & Prediction Loop (every 30 seconds)

1. **Metrics Collection** — Fetches CPU/memory for all pods and nodes via the Metrics Server API
2. **Stuck Container Detection** — Runs `ps`, `uptime`, and `df` inside containers to detect unresponsive ones
3. **Container Health Checks** — Checks DNS resolution, disk space, `/tmp` fullness, network connectivity
4. **Restart Pattern Analysis** — Analyzes restart counts, frequencies, and exit codes (OOMKilled, SIGKILL, etc.)
5. **Trend Analysis** — Linear regression over the last 10 measurements to detect growing resource usage
6. **Failure Prediction** — Forecasts time-to-failure for memory leaks and CPU exhaustion (24-72h window)
7. **Auto-Healing** — Executes safe remediation actions based on detected issues

### Auto-Healing Actions

| Trigger | Action |
|---|---|
| `/tmp` > 95% full | Delete files older than 1 day and files > 10MB |
| Disk > 90% full | Truncate large log files, delete old temp files |
| DNS fails | Restart the pod (DNS fix via pod restart) |
| Network fails | Flush route cache; restart pod if still failing |
| CPU > 15% | Scale up the parent Deployment by 1 replica |
| Memory > 15% or memory leak | Restart the pod |
| Stuck container (exec failing) | Flag for restart |

### Safety Features

- **Dry-run mode** — set `HEALER_DRY_RUN=true` to see what *would* happen without making changes
- **Action limits** — max 3 actions per pod to prevent restart loops
- **System pod exclusion** — never touches `kube-*` or `healer-*` namespaces
- **Graceful restarts** — uses Kubernetes `Delete` (deployment controller recreates the pod)

---

## Installing Metrics Server

The healer gracefully degrades if Metrics Server is unavailable (metrics show as 0), but full predictive features require it.

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

For local clusters (minikube, kind) that use self-signed certs, add `--kubelet-insecure-tls`:

```bash
kubectl patch deployment metrics-server -n kube-system \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
```

Verify Metrics Server is working:

```bash
kubectl top nodes
kubectl top pods -A
```

---

## Makefile Targets

```
make build          Build the binary locally (bin/healer)
make build-linux    Build a static Linux binary (for Docker cross-compilation)
make run            Build and run locally using ~/.kube/config
make run-dry        Run in dry-run mode (no real changes)
make docker-build   Build the Docker image
make docker-push    Push the Docker image to a registry
make deploy         Build Docker image + apply all Kubernetes manifests
make undeploy       Delete all Kubernetes resources
make logs           Tail live logs from the deployed pod
make status         Show Kubernetes deployment and pod status
make fmt            Run go fmt
make vet            Run go vet
make test           Run go test
make clean          Delete the built binary
```

---

## Troubleshooting

### Metrics API not available

```
Warning: Metrics API not available: the server could not find the requested resource
```

Install the Metrics Server (see above). The healer continues to work without it — trend analysis and predictions will be disabled until metrics are available.

### Permission denied / RBAC errors

```bash
# Check if the service account has the required permissions
kubectl auth can-i get pods \
  --as=system:serviceaccount:healer-system:k8s-healer -A

kubectl auth can-i create pods/exec \
  --as=system:serviceaccount:healer-system:k8s-healer -A
```

If permissions are missing, re-apply the RBAC manifest:

```bash
kubectl apply -f deployments/rbac.yaml
```

### Pod exec failures (container checks returning errors)

This is expected for minimal containers (distroless, scratch-based) that don't have `/bin/sh` or standard tools like `df`, `nslookup`, `ps`. The healer logs these as warnings and continues — it will not incorrectly flag these containers as stuck.

### Dashboard not accessible

```bash
# Port-forward the service
kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system
```

Then open `http://localhost:8080`.

### Check healer logs

```bash
kubectl logs -f deployment/k8s-healer -n healer-system
```

---

## Production Deployment

For production, use the `deploy-production.sh` script which prompts for confirmation before applying:

```bash
./scripts/deploy-production.sh
```

Or use the Makefile with a custom image:

```bash
IMAGE_NAME=myregistry.io/myorg/k8s-ai-healer IMAGE_TAG=v4.0 make deploy
```

---

## Monitoring Integration

### Prometheus

The healer exposes JSON endpoints that can be scraped or integrated with a Prometheus exporter. Add to your `prometheus.yml`:

```yaml
- job_name: 'k8s-healer'
  static_configs:
  - targets: ['k8s-healer.healer-system:8080']
  metrics_path: '/status'
```

### Grafana

Visualize:
- Healing action frequency over time
- Pod restart patterns
- Resource usage trends
- System health score

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run `make fmt && make vet && make test`
5. Submit a pull request

### Development Setup

```bash
git clone https://github.com/Pavel-P09/k8s-ai-healer.git
cd k8s-ai-healer

# Download Go dependencies
go mod download

# Build
make build

# Run tests
make test

# Run locally (requires a working kubeconfig)
make run-dry
```

---

## Roadmap

- [ ] Support for custom healing actions via CRD
- [ ] Prometheus `/metrics` endpoint
- [ ] Integration with PagerDuty / Slack alerting
- [ ] Multi-cluster support
- [ ] Machine learning model improvements
- [ ] Grafana dashboard templates

---

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

---

Made with ❤️ for the Kubernetes community
