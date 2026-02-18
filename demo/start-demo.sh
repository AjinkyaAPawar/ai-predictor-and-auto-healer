#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# K8s AI Healer — Hackathon Demo Starter
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

echo "🎬 Starting K8s AI Healer Demo Setup..."
echo ""

# Check if healer is deployed
if ! kubectl get deployment k8s-healer -n healer-system >/dev/null 2>&1; then
  echo "❌ K8s AI Healer is not deployed yet."
  echo ""
  echo "   Deploy it first:"
  echo "     make deploy"
  echo ""
  exit 1
fi

echo "✅ Healer is deployed"
echo ""

# Deploy demo apps
echo "📦 Deploying demo applications with intentional problems..."
kubectl apply -f demo/demo-app.yaml

echo ""
echo "⏳ Waiting for demo pods to start (30 seconds)..."
sleep 30

echo ""
echo "✅ Demo setup complete!"
echo ""
echo "════════════════════════════════════════════════════════════════════════════"
echo ""
echo "🎯 Next Steps for Your Presentation:"
echo ""
echo "1. Open the dashboard (in a separate terminal):"
echo "   kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system"
echo ""
echo "2. Open in your browser (FULL SCREEN for demo):"
echo "   http://localhost:8080"
echo ""
echo "3. Watch the live healing actions appear on the dashboard!"
echo ""
echo "4. In another terminal, watch the pods:"
echo "   kubectl get pods -n demo-app -w"
echo ""
echo "5. Watch healer logs (optional):"
echo "   kubectl logs -f deployment/k8s-healer -n healer-system --tail=50"
echo ""
echo "════════════════════════════════════════════════════════════════════════════"
echo ""
echo "📚 Full demo guide: demo/DEMO_GUIDE.md"
echo ""
echo "The demo apps are creating problems that the AI will detect and fix:"
echo "  • crash-loop    → Restarts every 30s (triggers restart analysis)"
echo "  • memory-leak   → Gradual memory growth (triggers prediction & restart)"
echo "  • disk-filler   → Fills /tmp (triggers automatic cleanup)"
echo "  • cpu-hog       → High CPU load (triggers auto-scaling)"
echo "  • network-test  → DNS checks (may trigger network fixes)"
echo ""
echo "Watch the dashboard for 3-5 minutes to see all healing actions! 🚀"
