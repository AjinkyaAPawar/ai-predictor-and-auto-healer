# ─────────────────────────────────────────────────────────────────────────────
# AI Predictor & Auto-Healer — Fully Offline Docker Build
#
# HOW TO BUILD (no internet required after first vendor setup):
#   1. Run once (with internet): go mod vendor
#   2. Then build offline forever: docker build -t ai-predictor-healer:latest .
#
# The final image is scratch-based (zero OS, zero shell, zero attack surface).
# ─────────────────────────────────────────────────────────────────────────────

# ── Stage 1: Build ──────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go module files and vendor directory (must exist - run `go mod vendor` first)
COPY go.mod go.sum ./
COPY vendor/ vendor/

# Copy all source code
COPY cmd/     cmd/
COPY internal/ internal/

# Build fully static binary using vendor (no network calls during build)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -mod=vendor \
      -ldflags="-s -w -extldflags=-static" \
      -o /ai-healer \
      cmd/ai-healer/main.go

# ── Stage 2: Minimal runtime image ──────────────────────────────────────────
# Using scratch = zero OS packages, zero shell, zero CVEs from base image
FROM scratch

# Copy TLS certificates so the binary can reach the K8s API over HTTPS
# These come from the builder image — no external download needed
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compiled binary
COPY --from=builder /ai-healer /ai-healer

# Expose the API/dashboard port
EXPOSE 8080

ENTRYPOINT ["/ai-healer"]
