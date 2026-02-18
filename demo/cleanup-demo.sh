#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# K8s AI Healer — Demo Cleanup
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

echo "🧹 Cleaning up demo resources..."
echo ""

kubectl delete namespace demo-app --ignore-not-found

echo ""
echo "✅ Demo apps removed"
echo ""
echo "The K8s AI Healer is still running. To remove it:"
echo "  make undeploy"
