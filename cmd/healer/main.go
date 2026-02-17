package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"k8s-healer/internal/actions"
	"k8s-healer/internal/api"
	"k8s-healer/internal/collector"
	"k8s-healer/internal/diagnostics"
	"k8s-healer/internal/predictor"

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

	fmt.Println("🤖 K8s AI Healer v4.0 - COMPLETE SYSTEM WITH API")
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

	// Start HTTP API Server
	apiServer := api.NewAPIServer(autoHealer, diagEngine, cfg.port)
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

		// Standard metrics collection
		metrics, err := col.GetAllPodMetrics(ctx)
		if err != nil {
			fmt.Printf("Error getting metrics: %v\n", err)
			time.Sleep(cfg.checkInterval)
			continue
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

		// Standard predictions and actions
		pred.UpdateHistory(metrics)
		predictions := pred.PredictIssues(metrics)

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
