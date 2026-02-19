# 🎪 AI Predictor & Auto-Healer — Hackathon Demo Kit

Everything you need for a winning 10-minute presentation.

---

## 🚀 Quick Start (Automated)

```bash
cd demo
./setup-demo.sh
```

This script:
1. ✅ Deploys the AI Predictor & Auto-Healer
2. ✅ Deploys a problematic demo app (memory leaks, disk issues, crashes)
3. ✅ Starts a traffic generator to accelerate issues

Then follow the on-screen instructions to open the dashboard.

---

## 📂 What's Included

```
demo/
├── setup-demo.sh                    ← One-command setup
├── DEMO-SCRIPT.md                   ← Complete 10-15 min presentation guide
├── problem-app.py                   ← Intentionally broken Python app
├── Dockerfile                       ← Container for the problem app
└── problem-app-deployment.yaml      ← Kubernetes deployment
```

---

## 🎯 Manual Setup (If You Prefer)

### 1. Deploy the Healer

```bash
cd ..   # Go back to project root
make vendor   # If not done already
make deploy
```

### 2. Build and Deploy Problem App

```bash
cd demo

# Build
docker build -t problem-app:latest .

# Load into cluster
minikube image load problem-app:latest
# or for kind:
# kind load docker-image problem-app:latest

# Deploy
kubectl apply -f problem-app-deployment.yaml
```

### 3. Generate Traffic

```bash
kubectl run load-gen --image=busybox --restart=Never -- \
  sh -c "while true; do wget -q -O- http://problem-app.default.svc.cluster.local; sleep 2; done"
```

### 4. Open the Dashboard

```bash
kubectl port-forward svc/ai-predictor-healer 8080:8080 -n ai-healer-system
# Visit: http://localhost:8080
```

---

## 🎤 Presentation Checklist

Before you present:

- [ ] Healer dashboard is open (http://localhost:8080)
- [ ] Problem app is running (`kubectl get pods -l app=problem-app`)
- [ ] Traffic generator is active (`kubectl get pods load-gen`)
- [ ] You've read `DEMO-SCRIPT.md` once
- [ ] You have logs streaming in a second terminal
- [ ] Cluster has internet (for Google Fonts in the UI)

---

## 🔥 What the Judges Will See

1. **Stunning UI** — Cyberpunk/neural network themed dashboard with live updates
2. **Real-time healing** — Actions appearing as the AI detects and fixes issues
3. **Predictive intelligence** — "Memory leak detected → OOM in 18 hours" forecasts
4. **Autonomous repair** — Pods restarting, /tmp cleaning, network fixes — all automatic

---

## 💡 Pro Tips

### Make Issues Appear Faster

```bash
# Hit the problem app rapidly
for i in {1..100}; do
  kubectl exec -it load-gen -- wget -q -O- http://problem-app.default.svc.cluster.local
done
```

### Show Technical Depth

```bash
# Show logs with predictions
kubectl logs deployment/ai-predictor-healer -n ai-healer-system --tail=100

# Show actual Go code (if asked)
cat ../internal/predictor/predictor.go | grep -A 20 "PredictIssues"
```

### If Demo Breaks

Have backup screenshots of:
- The dashboard with healing actions
- Logs showing predictions
- Pod restarts in action

---

## 🎯 Key Demo Moments

| Minute | What to Show | Impact |
|---|---|---|
| 0-1 | Problem: K8s can't see stuck containers, memory leaks | Set up the challenge |
| 1-3 | Solution: AI Healer dashboard (cyberpunk UI) | Visual wow factor |
| 3-5 | Deploy broken app, show it's "healthy" in K8s | Highlight the gap |
| 5-8 | Watch dashboard light up with healing actions | Live AI in action |
| 8-10 | Show pod restarts, cleaned /tmp, predictions | Prove it works |
| 10+ | Q&A: Tech stack, safety, scalability | Technical credibility |

---

## 🏆 Winning Points

Emphasize these unique aspects:

1. **Proactive not reactive** — predicts 24-72 hours ahead
2. **Deep visibility** — detects stuck containers via exec diagnostics
3. **Fully autonomous** — no alerts, no human intervention
4. **Production-ready** — safety limits, dry-run mode, self-contained
5. **Zero dependencies** — works air-gapped, no external APIs

---

## 📞 Troubleshooting

**Dashboard shows "No healing actions yet"**
→ Wait 1-2 minutes, the healer checks every 30 seconds. Refresh the problem app more to accelerate issues.

**Problem app not causing issues**
→ The traffic generator might not be running. Check: `kubectl get pods load-gen`

**Healer not detecting anything**
→ Check healer logs: `kubectl logs deployment/ai-predictor-healer -n ai-healer-system -f`

**Cluster too slow**
→ Reduce memory limits on problem-app to trigger OOM faster:
```yaml
limits:
  memory: "128Mi"  # Lower = faster OOM
```

---

Good luck! 🚀
