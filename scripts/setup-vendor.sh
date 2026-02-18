#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# K8s AI Healer — Vendor Setup (run ONCE with internet access)
#
# This downloads all Go dependencies into the vendor/ directory.
# After this runs successfully, the entire build + deploy chain works
# with ZERO internet access — forever.
#
# Usage:
#   ./scripts/setup-vendor.sh
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

echo "📦 Setting up Go vendor directory (requires internet — run once only)..."
echo ""

# Verify Go is installed
if ! command -v go &>/dev/null; then
  echo "❌ Go is not installed. Install Go 1.21+ from https://go.dev/dl/"
  echo "   After installing, re-run this script."
  exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo "✅ Go detected: ${GO_VERSION}"

# Download and vendor all dependencies
echo "⬇️  Downloading dependencies..."
go mod download

echo "📁 Vendoring dependencies into vendor/..."
go mod vendor

echo ""
echo "✅ Vendor setup complete!"
echo "   The vendor/ directory now contains all dependencies."
echo "   You can now build and deploy with zero internet access:"
echo ""
echo "     make deploy        # full build + kubernetes deploy"
echo "     make docker-build  # just build the Docker image"
echo "     make build         # build local binary"
echo ""
echo "   The vendor/ directory should be committed to git for fully"
echo "   air-gapped / offline environments."
