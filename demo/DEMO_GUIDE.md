# 🎯 K8s AI Healer — Hackathon Demo Guide

This guide shows you exactly how to demo the K8s AI Healer to impress judges and audience.

---

## 🎬 Demo Flow (15 minutes)

### Part 1: Setup & Introduction (2 min)

**What to say:**
> "We built K8s AI Healer — an intelligent system that detects and fixes infrastructure problems that Kubernetes doesn't see. Unlike traditional monitoring which only alerts humans, our system uses AI to predict failures 24-72 hours ahead and automatically heals them."

**What to show:**
1. Open the dashboard in your browser (make it FULL SCREEN)
2. Point out the live stats updating every 5 seconds
3. Highlight the 6 AI capabilities shown at the bottom

---

### Part 2: The Problem (3 min)

**What to say:**
> "Kubernetes health checks only look at process liveness. But there are many invisible problems:
> - Containers can be 'stuck' — alive but unresponsive
> - DNS can fail inside containers while the pod looks healthy
> - /tmp directories fill up causing silent failures
> - Memory leaks happen gradually — you only notice when it's too late
> 
> Traditional monitoring requires human intervention. We automate the entire detect → diagnose → fix cycle."

**What to show:**
Deploy the demo apps with intentional problems:
```bash
kubectl apply -f demo/demo-app.yaml
```

Wait 30 seconds, then show the pods:
```bash
kubectl get pods -n demo-app -w
```

Point out that Kubernetes shows them as "Running" but they have problems brewing.

---

### Part 3: Live Healing Demo (7 min)

**Watch the dashboard** — refresh it every 10-15 seconds. The AI will detect issues and heal them automatically.

#### What You'll See (in order):

**1. Crash Loop Detection (1-2 min)**
- The `crash-loop` pod restarts every 30 seconds
- AI detects: "CRASH_LOOP" pattern
- Shows exit code analysis
- Dashboard shows: **"INVESTIGATE_RESTARTS"** action

**What to say:**
> "See — the crash-loop pod is restarting. Our AI detected the pattern, analyzed the exit code, and is investigating. In production, it would trigger a rollback or alert the team with full diagnostics."

**2. Memory Leak Prediction (2-3 min)**
- The `memory-leak` pod gradually consumes RAM
- AI runs linear regression over the last 10 measurements
- Predicts: "OOM in 18.5 hours"
- Dashboard shows: **"RESTART_POD"** before OOM happens

**What to say:**
> "The memory-leak app is growing at 3% per hour. Our AI predicted it will OOM in ~18 hours and restarted it proactively. Traditional monitoring would only alert AFTER the crash."

**3. /tmp Cleanup (1-2 min)**
- The `disk-filler` pod fills /tmp with large files
- AI runs `df /tmp` checks inside the container
- Detects: "/tmp directory 95% full"
- Dashboard shows: **"CLEANUP_TMP"** — deletes old files automatically

**What to say:**
> "This container was filling /tmp with temp files. The AI detected it was 95% full and automatically cleaned up old files and large logs — all without restarting the pod."

**4. CPU Overload & Auto-Scaling (1-2 min)**
- The `cpu-hog` pod consumes 15%+ CPU
- AI detects sustained high CPU
- Dashboard shows: **"SCALE_UP"** — adds replicas to the deployment

**What to say:**
> "The CPU-hog was overloaded. Instead of waiting for it to slow down or crash, the AI scaled the deployment up by 1 replica — spreading the load."

---

### Part 4: The Technology (2 min)

**What to say:**
> "How does it work?
> 1. **Metrics Collection** — every 30 seconds, we pull CPU/memory from the Kubernetes Metrics API
> 2. **Container Diagnostics** — we exec into containers and run ps, df, nslookup, ping — things Kubernetes never checks
> 3. **AI Prediction** — linear regression over historical data predicts failures 24-72 hours ahead
> 4. **Auto-Healing** — based on the issue type, we clean disk, restart pods, scale deployments, or fix DNS
> 
> Everything runs locally — zero external dependencies, fully air-gapped ready."

**What to show:**
Open a terminal and show the logs:
```bash
kubectl logs -f deployment/k8s-healer -n healer-system --tail=50
```

Point out:
- Real-time diagnostics output
- AI predictions with confidence scores
- Healing actions being executed

---

### Part 5: Q&A Prep (1 min)

**Common Questions:**

**Q: What if the AI makes the wrong decision?**
> A: We have safety limits — max 3 actions per pod to prevent loops, dry-run mode for testing, and system pods (kube-*) are never touched. All actions are logged with full audit trail.

**Q: How is this different from tools like Prometheus?**
> A: Prometheus monitors and alerts humans. We automate the entire loop — detect, diagnose, predict, and heal. No human in the loop until after the problem is fixed.

**Q: Can it handle custom workloads?**
> A: Yes — it's workload-agnostic. We don't care what your app does. We monitor infrastructure signals: CPU, memory, disk, DNS, network connectivity. Works with any container.

**Q: Does it work in production?**
> A: Absolutely. We designed it for air-gapped environments — zero external dependencies. After the first `make vendor`, it builds and deploys forever with no internet.

---

## 📊 Dashboard Tips for Maximum Impact

### Before the demo:
1. **Open the dashboard in full-screen mode** (F11 in most browsers)
2. **Use a large monitor or projector** — the gradient background looks stunning on big screens
3. **Refresh it once** before starting to show the live updates working

### During the demo:
1. **Split screen** — dashboard on left, terminal on right
2. Point to specific cards as actions happen
3. Let the live updates speak for themselves — the real-time refresh is impressive

### If nothing happens immediately:
> "The AI runs checks every 30 seconds. In a production cluster with hundreds of pods, you'd see constant activity. Our demo cluster is small, so let me show you the previous actions..."

Then click through to `/actions` endpoint or show the terminal logs.

---

## 🚀 Commands Cheat Sheet

### Setup (before demo):
```bash
# Deploy the healer
make deploy

# Port-forward the dashboard
kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system

# Open dashboard
open http://localhost:8080
```

### During demo:
```bash
# Deploy demo apps with problems
kubectl apply -f demo/demo-app.yaml

# Watch pods
kubectl get pods -n demo-app -w

# Watch healer logs
kubectl logs -f deployment/k8s-healer -n healer-system --tail=50

# Check healing actions
curl http://localhost:8080/status | jq '.recent_actions'

# Force a specific issue (if needed)
kubectl exec -it deployment/disk-filler -n demo-app -- sh -c "dd if=/dev/zero of=/tmp/huge.tmp bs=1M count=50"
```

### Cleanup (after demo):
```bash
# Remove demo apps
kubectl delete namespace demo-app

# Optionally remove healer
make undeploy
```

---

## 🎤 Elevator Pitch (30 seconds)

> "K8s AI Healer is an intelligent infrastructure auto-healing system. While Kubernetes only checks if your containers are alive, we detect invisible problems like stuck processes, DNS failures, memory leaks, and disk issues. We use AI to predict failures 24-72 hours ahead and automatically fix them — no human intervention needed. Everything runs locally with zero external dependencies. We've demonstrated it healing 5 different failure types in real-time."

---

## 💡 Pro Tips

1. **Practice the flow** — run through it 2-3 times before the real demo
2. **Have a backup** — record a video of the dashboard in action in case Wi-Fi fails
3. **Highlight the gradient UI** — mention "modern, production-ready dashboard"
4. **Emphasize zero dependencies** — this is a huge win for enterprise/air-gapped deployments
5. **Show confidence** — if asked a tough question, acknowledge it honestly and pivot to what you've proven

---

Good luck! 🚀
