package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-predictor-healer/internal/actions"
	"ai-predictor-healer/internal/api"
	"ai-predictor-healer/internal/collector"
	"ai-predictor-healer/internal/diagnostics"
	"ai-predictor-healer/internal/predictor"
	"ai-predictor-healer/internal/store"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// healerConfig holds runtime configuration read from environment variables.
type healerConfig struct {
	port          string
	dryRun        bool
	checkInterval time.Duration
	logLevel      string
}

func loadConfig() healerConfig {
	port := getEnv("HEALER_PORT", "8080")

	dryRun := false
	if v := os.Getenv("HEALER_DRY_RUN"); v == "true" || v == "1" {
		dryRun = true
	}

	checkIntervalSec := 30
	if v := os.Getenv("HEALER_CHECK_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			checkIntervalSec = n
		}
	}

	logLevel := getEnv("HEALER_LOG_LEVEL", "info")

	return healerConfig{
		port:          port,
		dryRun:        dryRun,
		checkInterval: time.Duration(checkIntervalSec) * time.Second,
		logLevel:      logLevel,
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func main() {
	log.SetOutput(io.Discard)

	cfg := loadConfig()

	fmt.Println("🤖 AI Predictor & Auto-Healer v4.0 - COMPLETE SYSTEM WITH API")
	fmt.Printf("📋 Config: port=%s, dryRun=%v, checkInterval=%s, logLevel=%s\n",
		cfg.port, cfg.dryRun, cfg.checkInterval, cfg.logLevel)

	clientset, metricsClient, restConfig, err := createClients()
	if err != nil {
		fmt.Printf("Failed to connect to Kubernetes: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Connected to cluster")

	col := collector.New(clientset, metricsClient)
	pred := predictor.New()
	actionEngine := actions.New(clientset, cfg.dryRun)
	diagEngine := diagnostics.New(clientset, restConfig)
	autoHealer := diagnostics.NewAutoHealer(diagEngine, cfg.dryRun)
	opsStore := store.New(2000)

	// Start HTTP API Server
	apiServer := api.NewAPIServer(opsStore, cfg.port)
	apiServer.Start()

	fmt.Println("🚀 AI Monitoring started - COMPLETE SYSTEM ACTIVE")
	fmt.Println("🛠️  Auto-fixing: DNS, disk, network, stuck containers")
	fmt.Printf("🌐 Web Dashboard:  http://localhost:%s\n", cfg.port)
	fmt.Printf("📊 Status API:     http://localhost:%s/status\n", cfg.port)
	fmt.Printf("🩺 Health Check:   http://localhost:%s/health\n", cfg.port)

	if cfg.dryRun {
		fmt.Println("⚠️  DRY-RUN mode enabled — no real healing actions will be taken")
	}

	for i := 1; ; i++ {
		ctx := context.TODO()
		cycleTs := time.Now()

		// Standard metrics collection
		metrics, err := col.GetAllPodMetrics(ctx)
		if err != nil {
			fmt.Printf("Error getting metrics: %v\n", err)
			time.Sleep(cfg.checkInterval)
			continue
		}

		// Emit observation events for obvious runtime states so the timeline always shows activity
		// (e.g. CrashLoopBackOff, Error, restart spikes) even if AI predictions are empty.
		correlationID := fmt.Sprintf("cycle-%d", i)
		for _, m := range metrics {
			if m.Status != "" && !strings.HasPrefix(m.Status, "Running") {
				opsStore.AppendTimeline(store.TimelineEvent{
					ID:            fmt.Sprintf("%s-obs-status-%s-%s", correlationID, m.Namespace, m.Name),
					Type:          "observation",
					Severity:      "warning",
					Title:         fmt.Sprintf("Pod status: %s", m.Status),
					Body:          fmt.Sprintf("Observed non-running status for %s/%s", m.Namespace, m.Name),
					Namespace:     m.Namespace,
					PodName:       m.Name,
					Timestamp:     cycleTs,
					CorrelationID: correlationID,
				})
			}
			if m.Restarts >= 3 {
				opsStore.AppendTimeline(store.TimelineEvent{
					ID:            fmt.Sprintf("%s-obs-restarts-%s-%s", correlationID, m.Namespace, m.Name),
					Type:          "observation",
					Severity:      "warning",
					Title:         fmt.Sprintf("Restart spike: %d restarts", m.Restarts),
					Body:          fmt.Sprintf("Observed restarts=%d for %s/%s", m.Restarts, m.Namespace, m.Name),
					Namespace:     m.Namespace,
					PodName:       m.Name,
					Timestamp:     cycleTs,
					CorrelationID: correlationID,
				})
			}
		}

		// Advanced diagnostics
		stuckContainers, err := diagEngine.DiagnoseStuckContainers(ctx, "")
		if err != nil {
			fmt.Printf("Stuck container diagnostics error: %v\n", err)
		}

		containerChecks, err := diagEngine.RunContainerChecks(ctx, "")
		if err != nil {
			fmt.Printf("Container checks error: %v\n", err)
		}

		restartPatterns, err := diagEngine.AnalyzeRestartPatterns(ctx, "")
		if err != nil {
			fmt.Printf("Restart analysis error: %v\n", err)
		}

		// Execute auto-healing actions
		var healingActions []diagnostics.HealingAction
		if len(containerChecks) > 0 {
			healingActions = autoHealer.HealContainerIssues(ctx, containerChecks)
		}

		// Standard predictions and actions
		pred.UpdateHistory(metrics)
		predictions := pred.PredictIssues(metrics)

		// Build a unified action list for UI (auto-healer actions + predictor planned actions)
		plannedActions := make([]diagnostics.HealingAction, 0, len(predictions))
		for _, p := range predictions {
			desc := "AI planned action from predictor"
			if p.Reason != "" {
				desc = fmt.Sprintf("AI planned action from predictor (reason=%s)", p.Reason)
			}
			plannedActions = append(plannedActions, diagnostics.HealingAction{
				ActionType:    p.Action,
				PodName:       p.PodName,
				Namespace:     p.PodNamespace,
				ContainerName: "",
				Description:   desc,
				Status:        "PLANNED",
				Timestamp:     cycleTs,
				Result:        "",
			})
		}

		// Publish snapshot for API/UI
		opsStore.UpdateSnapshot(store.Snapshot{
			Timestamp:       cycleTs,
			PodMetrics:      metrics,
			StuckContainers: stuckContainers,
			ContainerChecks: containerChecks,
			RestartPatterns: restartPatterns,
			Predictions:     predictions,
			HealingActions:  append(append([]diagnostics.HealingAction(nil), healingActions...), plannedActions...),
			DryRun:          cfg.dryRun,
		})

		// Append explainability timeline events (high-signal only)

		// Diagnostic explainability: why remediation candidates were selected (DNS/disk/tmp/network)
		for _, cr := range containerChecks {
			sev := "info"
			if strings.ToUpper(cr.OverallStatus) == "CRITICAL" {
				sev = "critical"
			} else if strings.ToUpper(cr.OverallStatus) == "WARNING" {
				sev = "warning"
			}

			parts := make([]string, 0, len(cr.Checks))
			for _, chk := range cr.Checks {
				if strings.ToUpper(chk.Status) == "OK" {
					continue
				}
				fix := ""
				if len(chk.FixActions) > 0 {
					fix = fmt.Sprintf(" fix=%s", strings.Join(chk.FixActions, ","))
				}
				parts = append(parts, fmt.Sprintf("%s status=%s%s details=%s", chk.CheckName, chk.Status, fix, chk.Details))
			}

			if len(parts) > 0 {
				opsStore.AppendTimeline(store.TimelineEvent{
					ID:            fmt.Sprintf("%s-check-%s-%s", correlationID, cr.Namespace, cr.PodName),
					Type:          "diagnostic",
					Severity:      sev,
					Title:         fmt.Sprintf("Diagnostics: %s/%s (%s)", cr.Namespace, cr.PodName, cr.ContainerName),
					Body:          strings.Join(parts, " | "),
					Namespace:     cr.Namespace,
					PodName:       cr.PodName,
					ContainerName: cr.ContainerName,
					Timestamp:     cycleTs,
					CorrelationID: correlationID,
				})
			}
		}

		if len(predictions) > 0 {
			opsStore.AppendTimeline(store.TimelineEvent{
				ID:            fmt.Sprintf("%s-pred", correlationID),
				Type:          "prediction",
				Severity:      "warning",
				Title:         fmt.Sprintf("%d risk predictions generated", len(predictions)),
				Body:          "Predictor raised risks for one or more workloads.",
				Timestamp:     cycleTs,
				CorrelationID: correlationID,
			})

			// Emit per-workload prediction events so the timeline is drilldown-capable
			for _, p := range predictions {
				sev := "info"
				risk := strings.ToUpper(p.Risk)
				if risk == "CRITICAL" {
					sev = "critical"
				} else if risk == "HIGH" || risk == "MEDIUM" || risk == "LOW-MEDIUM" {
					sev = "warning"
				}

				body := ""
				if p.Reason != "" {
					body = fmt.Sprintf("reason=%s", p.Reason)
				}
				if len(p.Issues) > 0 {
					if body != "" {
						body += " | "
					}
					body += strings.Join(p.Issues, " | ")
				}

				opsStore.AppendTimeline(store.TimelineEvent{
					ID:            fmt.Sprintf("%s-pred-%s-%s", correlationID, p.PodNamespace, p.PodName),
					Type:          "prediction",
					Severity:      sev,
					Title:         fmt.Sprintf("%s → %s (ttf=%s)", p.Risk, p.Action, p.TimeToFailure),
					Body:          body,
					Namespace:     p.PodNamespace,
					PodName:       p.PodName,
					Timestamp:     cycleTs,
					CorrelationID: correlationID,
				})
			}
		}
		for _, a := range healingActions {
			sev := "info"
			if a.Status == "FAILED" {
				sev = "critical"
			}
			body := a.Description
			if a.Status != "" {
				body = fmt.Sprintf("status=%s | %s", a.Status, body)
			}
			if a.Result != "" {
				body = fmt.Sprintf("%s | result=%s", body, a.Result)
			}
			opsStore.AppendTimeline(store.TimelineEvent{
				ID:            fmt.Sprintf("%s-action-%s-%s", correlationID, a.Namespace, a.PodName),
				Type:          "action",
				Severity:      sev,
				Title:         a.ActionType,
				Body:          body,
				Namespace:     a.Namespace,
				PodName:       a.PodName,
				ContainerName: a.ContainerName,
				Timestamp:     a.Timestamp,
				CorrelationID: correlationID,
			})
		}

		hasIssues := false
		for _, m := range metrics {
			if m.Restarts > 3 || m.Status != "Running" || m.CPUPercent > 15 || m.MemPercent > 15 {
				hasIssues = true
				break
			}
		}

		if len(stuckContainers) > 0 || len(containerChecks) > 0 || len(restartPatterns) > 0 || len(healingActions) > 0 {
			hasIssues = true
		}

		checksUntilNext := 20 - (i % 20)
		if i%20 == 1 || hasIssues {
			fmt.Printf("🔍 Health check [%s]:\n", time.Now().Format("15:04:05"))
			col.PrintStatus()

			if len(stuckContainers) > 0 {
				diagEngine.PrintDiagnostics(stuckContainers)
			}
			if len(containerChecks) > 0 {
				diagEngine.PrintContainerChecks(containerChecks)
			}
			if len(restartPatterns) > 0 {
				diagEngine.PrintRestartAnalysis(restartPatterns)
			}
			if len(healingActions) > 0 {
				autoHealer.PrintHealingActions(healingActions)
			}
		} else {
			fmt.Printf("[%s] 🟢 OK (%d checks until next full report) - Dashboard: http://localhost:%s\n",
				time.Now().Format("15:04:05"), checksUntilNext, cfg.port)
		}

		if len(predictions) > 0 {
			pred.PrintPredictions(predictions)
			actionEngine.ExecuteActions(predictions)
		}

		time.Sleep(cfg.checkInterval)
	}
}

func createClients() (*kubernetes.Clientset, *metricsclient.Clientset, *rest.Config, error) {
	// Try in-cluster config first (running inside Kubernetes)
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig file (running locally)
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		// Allow override via KUBECONFIG env var
		if kc := os.Getenv("KUBECONFIG"); kc != "" {
			kubeconfig = kc
		}
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	metricsClient, err := metricsclient.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	return clientset, metricsClient, restConfig, nil
}
