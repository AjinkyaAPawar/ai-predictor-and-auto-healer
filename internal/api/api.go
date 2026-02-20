package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"ai-predictor-healer/internal/store"
	"ai-predictor-healer/internal/demo"
)

//go:embed web/*
var webFS embed.FS

type APIServer struct {
	store    *store.OpsStore
	port     string
	demoMode *demo.DemoMode
}

type StatusResponse struct {
	Status        string                       `json:"status"`
	Timestamp     time.Time                    `json:"timestamp"`
	TotalActions  int                          `json:"total_actions"`
	RecentActions []any                        `json:"recent_actions"`
	SystemHealth  string                       `json:"system_health"`
}


func NewAPIServer(opsStore *store.OpsStore, port string, demoMode *demo.DemoMode) *APIServer {
	return &APIServer{
		store:    opsStore,
		port:     port,
		demoMode: demoMode,
	}
}

func (s *APIServer) Start() {
	http.HandleFunc("/", s.handleRoot)
	http.HandleFunc("/app.css", s.handleStatic)
	http.HandleFunc("/app.js", s.handleStatic)
	http.HandleFunc("/status", s.handleStatus)
	http.HandleFunc("/actions", s.handleActions)
	http.HandleFunc("/health", s.handleHealth)

	http.HandleFunc("/api/v1/snapshot", s.handleSnapshotV1)
	http.HandleFunc("/api/v1/timeline", s.handleTimelineV1)
	http.HandleFunc("/api/v1/stream", s.handleStreamV1)
	
	// Demo mode endpoints
	http.HandleFunc("/api/demo/start", s.handleDemoStart)
	http.HandleFunc("/api/demo/stop", s.handleDemoStop)
	http.HandleFunc("/api/demo/status", s.handleDemoStatus)

	fmt.Printf("🌐 API Server starting on port %s\n", s.port)
	fmt.Printf("📊 Access at: http://localhost:%s\n", s.port)

	go func() {
		if err := http.ListenAndServe(":"+s.port, nil); err != nil {
			fmt.Printf("API Server error: %v\n", err)
		}
	}()
}

func (s *APIServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	b, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "UI load error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

func (s *APIServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	name = path.Clean(name)
	if name != "app.css" && name != "app.js" {
		http.NotFound(w, r)
		return
	}

	b, err := webFS.ReadFile("web/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(name, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else if strings.HasSuffix(name, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}
	w.Write(b)
}

func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap := s.store.GetSnapshot()
	history := snap.HealingActions

	var recentActions []any
	if len(history) > 0 {
		start := 0
		if len(history) > 10 {
			start = len(history) - 10
		}
		for _, a := range history[start:] {
			recentActions = append(recentActions, a)
		}
	}

	systemHealth := "HEALTHY"
	if len(recentActions) > 0 {
		criticalCount := 0
		for _, action := range recentActions {
			b, _ := json.Marshal(action)
			if strings.Contains(string(b), "RESTART_POD_NETWORK") || strings.Contains(string(b), "\"Status\":\"FAILED\"") {
				criticalCount++
			}
		}
		if criticalCount > 5 {
			systemHealth = "CRITICAL"
		} else if criticalCount > 0 {
			systemHealth = "WARNING"
		}
	}

	response := StatusResponse{
		Status:        "ACTIVE",
		Timestamp:     time.Now(),
		TotalActions:  len(history),
		RecentActions: recentActions,
		SystemHealth:  systemHealth,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(response)
}

func (s *APIServer) handleActions(w http.ResponseWriter, r *http.Request) {
	snap := s.store.GetSnapshot()
	history := snap.HealingActions

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_actions": len(history),
		"actions":       history,
	})
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "UP",
		"timestamp": time.Now(),
		"service":   "ai-predictor-healer",
		"version":   "4.0",
	})
}

func (s *APIServer) handleSnapshotV1(w http.ResponseWriter, r *http.Request) {
	snap := s.store.GetSnapshot()
	writeJSON(w, snap)
}

func (s *APIServer) handleTimelineV1(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	items := s.store.GetTimeline(limit)
	writeJSON(w, map[string]any{"items": items})
}

func (s *APIServer) handleStreamV1(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	recv, cancel := s.store.Subscribe()
	defer cancel()

	io.WriteString(w, "retry: 2000\n")
	io.WriteString(w, "event: ready\n")
	io.WriteString(w, "data: {}\n\n")
	flusher.Flush()

	ping := time.NewTicker(5 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			io.WriteString(w, "event: ping\n")
			io.WriteString(w, "data: {}\n\n")
			flusher.Flush()
		case msg, ok := <-recv:
			if !ok {
				return
			}
			io.WriteString(w, "event: update\n")
			io.WriteString(w, "data: ")
			w.Write(msg)
			io.WriteString(w, "\n\n")
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(payload)
}

func (s *APIServer) handleDemoStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	scenario := r.URL.Query().Get("scenario")
	if scenario == "" {
		scenario = "memory_leak"
	}
	
	s.demoMode.Start(scenario)
	
	writeJSON(w, map[string]interface{}{
		"status": "started",
		"scenario": scenario,
		"message": "Demo mode activated",
	})
}

func (s *APIServer) handleDemoStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	s.demoMode.Stop()
	
	writeJSON(w, map[string]interface{}{
		"status": "stopped",
		"message": "Demo mode deactivated",
	})
}

func (s *APIServer) handleDemoStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"active": s.demoMode.IsActive(),
	})
}
