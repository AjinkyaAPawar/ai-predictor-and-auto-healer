package store

import (
	"encoding/json"
	"sync"
	"time"

	"ai-predictor-healer/internal/collector"
	"ai-predictor-healer/internal/diagnostics"
	"ai-predictor-healer/internal/predictor"
)

type TimelineEvent struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`        // observation|diagnostic|prediction|action|system
	Severity      string    `json:"severity"`    // info|warning|critical
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	Namespace     string    `json:"namespace,omitempty"`
	PodName       string    `json:"pod_name,omitempty"`
	ContainerName string    `json:"container_name,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id,omitempty"`
}

type Snapshot struct {
	Timestamp       time.Time                         `json:"timestamp"`
	PodMetrics      []collector.PodMetrics            `json:"pod_metrics"`
	StuckContainers []diagnostics.DiagnosticResult    `json:"stuck_containers"`
	ContainerChecks []diagnostics.ContainerCheckResult `json:"container_checks"`
	RestartPatterns []diagnostics.RestartPattern      `json:"restart_patterns"`
	Predictions     []predictor.PredictionResult      `json:"predictions"`
	HealingActions  []diagnostics.HealingAction       `json:"healing_actions"`
	DryRun          bool                              `json:"dry_run"`
}

type subscriber struct {
	ch chan []byte
}

type OpsStore struct {
	mu sync.RWMutex

	lastSnapshot Snapshot
	timeline     []TimelineEvent

	subsMu sync.Mutex
	subs   map[*subscriber]struct{}

	maxTimeline int
}

func New(maxTimeline int) *OpsStore {
	if maxTimeline <= 0 {
		maxTimeline = 1000
	}
	return &OpsStore{
		subs:        make(map[*subscriber]struct{}),
		maxTimeline: maxTimeline,
	}
}

func (s *OpsStore) UpdateSnapshot(snapshot Snapshot) {
	s.mu.Lock()
	s.lastSnapshot = snapshot
	s.mu.Unlock()

	s.broadcast(map[string]any{
		"type":      "snapshot",
		"timestamp": snapshot.Timestamp,
	})
}

func (s *OpsStore) GetSnapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSnapshot
}

func (s *OpsStore) AppendTimeline(events ...TimelineEvent) {
	if len(events) == 0 {
		return
	}

	s.mu.Lock()
	s.timeline = append(s.timeline, events...)
	if len(s.timeline) > s.maxTimeline {
		s.timeline = s.timeline[len(s.timeline)-s.maxTimeline:]
	}
	s.mu.Unlock()

	s.broadcast(map[string]any{
		"type":      "timeline",
		"count":     len(events),
		"timestamp": time.Now(),
	})
}

func (s *OpsStore) GetTimeline(limit int) []TimelineEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit >= len(s.timeline) {
		out := make([]TimelineEvent, len(s.timeline))
		copy(out, s.timeline)
		return out
	}

	start := len(s.timeline) - limit
	out := make([]TimelineEvent, limit)
	copy(out, s.timeline[start:])
	return out
}

func (s *OpsStore) Subscribe() (recv <-chan []byte, cancel func()) {
	sub := &subscriber{ch: make(chan []byte, 32)}

	s.subsMu.Lock()
	s.subs[sub] = struct{}{}
	s.subsMu.Unlock()

	cancel = func() {
		s.subsMu.Lock()
		if _, ok := s.subs[sub]; ok {
			delete(s.subs, sub)
			close(sub.ch)
		}
		s.subsMu.Unlock()
	}

	return sub.ch, cancel
}

func (s *OpsStore) broadcast(payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}

	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for sub := range s.subs {
		select {
		case sub.ch <- b:
		default:
			// Drop if client is slow; UI will refetch snapshot.
		}
	}
}
