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

	"k8s-healer/internal/store"
)

//go:embed web/*
var webFS embed.FS

type APIServer struct {
	store *store.OpsStore
	port  string
}

type StatusResponse struct {
	Status        string                       `json:"status"`
	Timestamp     time.Time                    `json:"timestamp"`
	TotalActions  int                          `json:"total_actions"`
	RecentActions []any                        `json:"recent_actions"`
	SystemHealth  string                       `json:"system_health"`
}


func NewAPIServer(opsStore *store.OpsStore, port string) *APIServer {
	return &APIServer{
		store: opsStore,
		port:  port,
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
		"service":   "k8s-ai-healer",
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
