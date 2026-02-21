# HOW WE HANDLE EACH ISSUE - CODE MAPPING

## Quick Reference: Issue → Detection → Code Location → Action

---

## ✅ 1. MEMORY LEAKS

### How We Detect:
- **Linear regression** on memory usage over 10-minute window
- If memory grows >1% per hour consistently → Memory leak detected

### Code Location:
```
File: internal/predictor/predictor.go
Function: analyzePodAdvanced()
Lines: ~400-475

Key logic:
memSlope := p.calculateSlope(history, current, "memory")
if memSlope > 1.0 {  // 1% per hour growth
    trend.IsMemoryLeak = true
}
```

### Action Taken:
**RESTART_POD** - Delete and recreate the pod
```
File: internal/actions/actions.go
Function: RestartPod()

What it does:
1. Delete pod via Kubernetes API
2. Deployment controller recreates it automatically
3. New pod starts with fresh memory (0%)
4. Memory leak gone

Command executed:
clientset.CoreV1().Pods(namespace).Delete(ctx, podName, deleteOptions)
```

### Why This Works:
Memory leaks = gradual memory buildup in code
Restart = Fresh start with clean memory
Time to fix: 30 seconds

---

## ✅ 2. CPU SPIKES

### How We Detect:
- Track CPU percentage over time
- If CPU grows >2% per hour OR >80% sustained → CPU spike detected

### Code Location:
```
File: internal/predictor/predictor.go
Function: analyzePodAdvanced()
Lines: ~400-475

Key logic:
cpuSlope := p.calculateSlope(history, current, "cpu")
if cpuSlope > 2.0 {  // 2% per hour growth
    trend.IsCPUGrowing = true
}
```

### Action Taken:
**SCALE_UP** - Add more pod replicas to distribute load
```
File: internal/actions/actions.go
Function: ScaleDeployment()

What it does:
1. Get current replica count (e.g., 3 pods)
2. Increase by 1 (e.g., 3 → 4 pods)
3. Patch Deployment via Kubernetes API
4. Load distributed across more pods

Command executed:
clientset.AppsV1().Deployments(namespace).Patch(ctx, deploymentName, 
    types.JSONPatchType, patchBytes, metav1.PatchOptions{})

Patch: {"spec":{"replicas": currentReplicas + 1}}
```

### Why This Works:
CPU spike = Too much work for one pod
Scale up = Share work across multiple pods
Each pod does less work = Lower CPU per pod

---

## ✅ 3. DISK SPACE ISSUES

### How We Detect:
- Execute `df -h /` and `df -h /tmp` inside container
- Parse output for percentage usage
- If >90% on root OR >95% on /tmp → Disk full detected

### Code Location:
```
File: internal/diagnostics/container_checks.go
Function: checkDiskSpace()
Lines: ~130-180

Key logic:
stdout := execCommand(ctx, namespace, podName, containerName, 
    []string{"sh", "-c", "df -h /"})

// Parse output: "Filesystem  Size  Used  Avail  Use%"
// Extract: "95%" → 95
if percentage > 90 {
    return ContainerCheck{Status: "CRITICAL"}
}
```

### Action Taken:
**CLEANUP_DISK** - Delete temporary files and truncate logs
```
File: internal/diagnostics/auto_healer.go
Function: cleanupDiskSpace()

What it does:
1. Find files in /tmp older than 1 day, larger than 10MB
2. Delete them: find /tmp -type f -mtime +1 -size +10M -delete
3. Find logs in /var/log larger than 50MB
4. Truncate to 10MB: truncate -s 10M /var/log/*.log

Commands executed via exec:
execCommand(ctx, namespace, podName, containerName, 
    []string{"sh", "-c", "find /tmp -type f -mtime +1 -size +10M -delete"})

execCommand(ctx, namespace, podName, containerName,
    []string{"sh", "-c", "find /var/log -name '*.log' -size +50M -exec truncate -s 10M {} \\;"})
```

### Why This Works:
Disk full = Old temp files and huge logs piling up
Cleanup = Delete unnecessary files
Result = Free space, pod keeps running

---

## ✅ 4. NETWORK FAILURES

### How We Detect:
- Execute `nslookup kubernetes.default.svc.cluster.local` inside container
- Execute `ping -c 1 <cluster-dns-ip>` 
- Parse output for errors: "server can't find", "connection timed out"

### Code Location:
```
File: internal/diagnostics/container_checks.go
Function: checkDNS() and checkNetworkConnectivity()
Lines: ~100-130, ~180-220

Key logic for DNS:
stdout := execCommand(ctx, namespace, podName, containerName,
    []string{"sh", "-c", "nslookup kubernetes.default.svc.cluster.local"})

if strings.Contains(stdout, "server can't find") ||
   strings.Contains(stdout, "connection timed out") {
    return ContainerCheck{Status: "CRITICAL"}
}

Key logic for Network:
stdout := execCommand(ctx, namespace, podName, containerName,
    []string{"sh", "-c", "ping -c 1 -W 2 10.96.0.10"})  // Cluster DNS IP

if exitCode != 0 {
    return ContainerCheck{Status: "WARNING"}
}
```

### Action Taken:
**RESTART_POD** - Network namespace corruption requires pod restart
```
File: internal/actions/actions.go
Function: RestartPod()

What it does:
1. Delete pod
2. Kubernetes recreates it
3. New pod gets fresh network namespace
4. DNS and routing tables reset

Why restart works:
- Network namespace corruption = Pod's network stack is broken
- Can't fix without destroying network namespace
- Restart = New network namespace = Clean slate
```

**Alternative (if shell available):**
FLUSH_ROUTE_CACHE - Clear routing tables before restart
```
File: internal/diagnostics/auto_healer.go

Command:
execCommand(ctx, namespace, podName, containerName,
    []string{"sh", "-c", "ip route flush cache"})

Then verify, if still broken → Restart pod
```

### Why This Works:
Network failures = Corrupted DNS cache or routing tables
Restart = Fresh network namespace
Result = Network connectivity restored

---

## ✅ 5. CRASH LOOPS

### How We Detect:
- Track restart count via Kubernetes API
- Track time between restarts
- If >3 restarts in <10 minutes → Crash loop detected

### Code Location:
```
File: internal/diagnostics/restart_analyzer.go
Function: AnalyzeRestartPatterns()
Lines: ~50-150

Key logic:
restartCount := pod.Status.ContainerStatuses[0].RestartCount
lastTerminated := pod.Status.ContainerStatuses[0].LastTerminationState.Terminated

if restartCount > 3 {
    timeSinceLastRestart := time.Since(lastTerminated.FinishedAt.Time)
    if timeSinceLastRestart.Minutes() < 10 {
        // Crash loop detected
        return RestartPattern{
            Pattern: "CRASH_LOOP",
            ExitCode: lastTerminated.ExitCode,  // e.g., 137 = OOMKilled
        }
    }
}
```

### Actions Taken (Multiple):

#### Action 1: RESTART_POD (for OOMKilled - Exit Code 137)
```
File: internal/diagnostics/auto_healer.go
Function: handleStuckContainer()

Logic:
if exitCode == 137 {  // OOMKilled
    // Memory limit too low, restart with recommendation to increase limits
    action = "RESTART_POD"
    recommendation = "Increase memory limits"
}
```

#### Action 2: ROLLBACK (for recent deployment causing crashes)
```
File: internal/actions/actions.go
Function: RollbackDeployment()

What it does:
1. Get Deployment's revision history
2. Find previous stable revision
3. Roll back via kubectl rollout undo

Command:
clientset.AppsV1().Deployments(namespace).Get(...)
// Check ReplicaSet history
// Find last stable ReplicaSet
// Update Deployment to use that template
```

#### Action 3: INCREASE_LIMITS (for resource exhaustion)
```
File: internal/actions/actions.go
Function: patchPodResources()

What it does:
1. Detect exit code 137 (OOMKilled)
2. Increase memory limits by 50%
3. Patch Deployment with new limits

Patch:
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "resources": {
            "limits": {
              "memory": "512Mi"  // was 256Mi
            }
          }
        }]
      }
    }
  }
}
```

### Why This Works:
Crash loops have different root causes:
- OOMKilled → Need more memory → Increase limits
- Bad deployment → Need old code → Rollback
- Corrupted state → Need fresh start → Restart

We detect the exit code to choose the right action.

---

## 📊 DECISION TREE SUMMARY

```
Issue Detected → Check Exit Code / Pattern → Choose Action

Memory Leak (memSlope > 1%/hr)
    → RESTART_POD
    → Reason: Fresh memory state

CPU Spike (cpuSlope > 2%/hr OR >80%)
    → SCALE_UP (+1 replica)
    → Reason: Distribute load

Disk Full (>90% on /, >95% on /tmp)
    → CLEANUP_DISK
    → Commands: delete old files, truncate logs
    → Reason: Free up space

DNS Failure (nslookup fails)
    → RESTART_POD
    → Reason: Fresh network namespace

Network Failure (ping fails)
    → FLUSH_ROUTE_CACHE (try first)
    → RESTART_POD (if flush fails)
    → Reason: Reset network stack

Crash Loop (>3 restarts in 10 min)
    → Check Exit Code:
        → 137 (OOMKilled) → INCREASE_LIMITS + RESTART
        → 143 (SIGTERM) → ROLLBACK (bad deployment)
        → 1 (General error) → RESTART_POD
        → 139 (SIGSEGV) → ROLLBACK (code bug)
```

---

## 🔍 HOW TO FIND THIS IN THE CODE

### Main Detection Logic:
```bash
# Prediction and trend analysis
cat internal/predictor/predictor.go | grep -A 20 "analyzePodAdvanced"

# Container health checks (disk, DNS, network)
cat internal/diagnostics/container_checks.go | grep -A 15 "checkDiskSpace\|checkDNS\|checkNetworkConnectivity"

# Restart pattern analysis
cat internal/diagnostics/restart_analyzer.go | grep -A 20 "AnalyzeRestartPatterns"
```

### Main Action Logic:
```bash
# Pod restart
cat internal/actions/actions.go | grep -A 10 "RestartPod"

# Scaling
cat internal/actions/actions.go | grep -A 10 "ScaleDeployment"

# Disk cleanup
cat internal/diagnostics/auto_healer.go | grep -A 15 "cleanupDiskSpace"

# Rollback
cat internal/actions/actions.go | grep -A 10 "RollbackDeployment"
```

---

## 🎯 QUICK ANSWER FOR JUDGES

**Judge: "How do you handle memory leaks?"**

**Answer:**
```
"We use linear regression to detect memory growth trends.
If memory is growing more than 1% per hour consistently,
we classify it as a memory leak.

The code is in internal/predictor/predictor.go - we calculate
the slope of memory usage over a 10-minute window.

Action: We restart the pod via the Kubernetes API, which gives
it a fresh memory state. The restart takes about 30 seconds.

Code location: internal/actions/actions.go - RestartPod() function.
We call clientset.CoreV1().Pods().Delete() and the Deployment
controller automatically recreates it."
```

**Judge: "What about CPU spikes?"**

**Answer:**
```
"We track CPU percentage over time. If CPU is growing more than
2% per hour or sustained above 80%, we detect it as a spike.

Same file: internal/predictor/predictor.go - calculateSlope() function.

Action: We scale up by adding one more pod replica. This distributes
the load across more instances, reducing CPU per pod.

Code: internal/actions/actions.go - ScaleDeployment() function.
We patch the Deployment with an incremented replica count."
```

**Judge: "How do you handle network failures?"**

**Answer:**
```
"We execute diagnostic commands inside the container using the
Kubernetes exec API. We run nslookup for DNS and ping for connectivity.

Code: internal/diagnostics/container_checks.go - checkDNS() and
checkNetworkConnectivity() functions.

Action: Network namespace corruption requires a pod restart.
We delete the pod and let Kubernetes recreate it with a fresh
network stack.

Optionally, we first try 'ip route flush cache' to clear routing
tables before restarting. Code in internal/diagnostics/auto_healer.go."
```

---

## 📝 ONE-LINER ANSWERS

| Issue | Detection Code | Action Code | Action Taken |
|-------|---------------|-------------|--------------|
| **Memory Leak** | `predictor.go:calculateSlope()` | `actions.go:RestartPod()` | Delete pod → Fresh memory |
| **CPU Spike** | `predictor.go:calculateSlope()` | `actions.go:ScaleDeployment()` | Add replica → Share load |
| **Disk Full** | `container_checks.go:checkDiskSpace()` | `auto_healer.go:cleanupDiskSpace()` | Delete old files, truncate logs |
| **DNS Failure** | `container_checks.go:checkDNS()` | `actions.go:RestartPod()` | Delete pod → Fresh network |
| **Crash Loop** | `restart_analyzer.go:AnalyzeRestartPatterns()` | `actions.go:RestartPod() OR RollbackDeployment()` | Restart OR Rollback based on exit code |

---

**This is the cheat sheet you need to answer any "how does it work" questions!** 🎯
