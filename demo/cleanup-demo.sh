#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# AI Predictor & Auto-Healer — Demo Cleanup
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

echo "🧹 Cleaning up demo resources..."
echo ""

kubectl delete namespace demo-app --ignore-not-found

echo ""
echo "✅ Demo apps removed"
echo ""
echo "The AI Predictor & Auto-Healer is still running. To remove it:"
echo "  make undeploy"
