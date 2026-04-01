package telemetry

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// maxStoredTraces is the maximum number of distinct traces kept in memory.
	maxStoredTraces = 1000

	// maxStoredSpans is the maximum total spans kept in memory.
	maxStoredSpans = 10000
)

// TraceInfo summarizes a trace for list views.
type TraceInfo struct {
	TraceID   string    `json:"trace_id"`
	RootName  string    `json:"root_name"`
	SpanCount int       `json:"span_count"`
	StartTime time.Time `json:"start_time"`
	Duration  string    `json:"duration,omitempty"`
	Status    string    `json:"status"`
}

// TraceDetail contains all spans for a single trace.
type TraceDetail struct {
	TraceID string     `json:"trace_id"`
	Spans   []SpanData `json:"spans"`
}

// InMemoryExporter stores completed spans in memory, indexed by trace ID.
// It implements SpanExporter and provides query methods for the trace UI.
type InMemoryExporter struct {
	mu      sync.RWMutex
	spans   []SpanData        // all spans, ordered by arrival
	byTrace map[string][]int  // trace_id -> indices into spans
}

// NewInMemoryExporter creates a new in-memory span exporter.
func NewInMemoryExporter() *InMemoryExporter {
	return &InMemoryExporter{
		spans:   make([]SpanData, 0, 256),
		byTrace: make(map[string][]int),
	}
}

// global exporter reference for the HTTP handlers
var (
	globalExporter   *InMemoryExporter
	globalExporterMu sync.RWMutex
)

// ExportSpan records a completed span.
func (e *InMemoryExporter) ExportSpan(span SpanData) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Evict oldest spans if at capacity.
	if len(e.spans) >= maxStoredSpans {
		e.evictOldest()
	}

	idx := len(e.spans)
	e.spans = append(e.spans, span)
	e.byTrace[span.TraceID] = append(e.byTrace[span.TraceID], idx)

	// Update global exporter reference.
	globalExporterMu.Lock()
	globalExporter = e
	globalExporterMu.Unlock()
}

// Shutdown is a no-op for the in-memory exporter.
func (e *InMemoryExporter) Shutdown() {}

// evictOldest removes the oldest 25% of spans. Caller must hold e.mu.
func (e *InMemoryExporter) evictOldest() {
	cutoff := len(e.spans) / 4
	if cutoff == 0 {
		cutoff = 1
	}

	// Collect trace IDs that may be fully evicted.
	evictedTraces := make(map[string]bool)
	for i := 0; i < cutoff; i++ {
		evictedTraces[e.spans[i].TraceID] = true
	}

	e.spans = e.spans[cutoff:]

	// Rebuild the index.
	e.byTrace = make(map[string][]int, len(e.byTrace))
	for i, span := range e.spans {
		e.byTrace[span.TraceID] = append(e.byTrace[span.TraceID], i)
	}
}

// ListTraces returns recent traces, newest first, up to limit.
func (e *InMemoryExporter) ListTraces(limit int) []TraceInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Collect unique traces with their first and last timestamps.
	type traceMeta struct {
		traceID   string
		rootName  string
		startTime time.Time
		endTime   time.Time
		count     int
		hasError  bool
	}

	traces := make(map[string]*traceMeta)
	for _, span := range e.spans {
		tm, ok := traces[span.TraceID]
		if !ok {
			tm = &traceMeta{
				traceID:   span.TraceID,
				startTime: span.StartTime,
			}
			traces[span.TraceID] = tm
		}
		tm.count++
		if span.ParentSpanID == "" {
			tm.rootName = span.Name
		}
		if span.StartTime.Before(tm.startTime) {
			tm.startTime = span.StartTime
		}
		if !span.EndTime.IsZero() && span.EndTime.After(tm.endTime) {
			tm.endTime = span.EndTime
		}
		if span.Status == SpanStatusError {
			tm.hasError = true
		}
	}

	// Sort by start time descending.
	sorted := make([]*traceMeta, 0, len(traces))
	for _, tm := range traces {
		sorted = append(sorted, tm)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].startTime.After(sorted[j].startTime)
	})

	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}

	result := make([]TraceInfo, len(sorted))
	for i, tm := range sorted {
		status := "OK"
		if tm.hasError {
			status = "ERROR"
		}
		var duration string
		if !tm.endTime.IsZero() {
			duration = tm.endTime.Sub(tm.startTime).String()
		}
		result[i] = TraceInfo{
			TraceID:   tm.traceID,
			RootName:  tm.rootName,
			SpanCount: tm.count,
			StartTime: tm.startTime,
			Duration:  duration,
			Status:    status,
		}
	}
	return result
}

// GetTrace returns all spans for a given trace ID.
func (e *InMemoryExporter) GetTrace(traceID string) *TraceDetail {
	e.mu.RLock()
	defer e.mu.RUnlock()

	indices, ok := e.byTrace[traceID]
	if !ok || len(indices) == 0 {
		return nil
	}

	spans := make([]SpanData, len(indices))
	for i, idx := range indices {
		spans[i] = e.spans[idx]
	}

	// Sort spans by start time.
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].StartTime.Before(spans[j].StartTime)
	})

	return &TraceDetail{
		TraceID: traceID,
		Spans:   spans,
	}
}

// --- HTTP Handlers ---

// TraceHandlers returns an http.Handler that serves trace API endpoints:
//
//	GET /api/v1/traces          - list recent traces
//	GET /api/v1/traces/{id}     - get all spans for a trace
func TraceHandlers() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/traces", handleListTraces)
	mux.HandleFunc("/api/v1/traces/", handleGetTrace)
	return mux
}

func handleListTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	globalExporterMu.RLock()
	exp := globalExporter
	globalExporterMu.RUnlock()

	if exp == nil {
		writeJSON(w, http.StatusOK, []TraceInfo{})
		return
	}

	traces := exp.ListTraces(100)
	writeJSON(w, http.StatusOK, traces)
}

func handleGetTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract trace ID from path: /api/v1/traces/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/traces/")
	traceID := strings.TrimSuffix(path, "/")
	if traceID == "" {
		http.Error(w, "trace ID required", http.StatusBadRequest)
		return
	}

	globalExporterMu.RLock()
	exp := globalExporter
	globalExporterMu.RUnlock()

	if exp == nil {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}

	detail := exp.GetTrace(traceID)
	if detail == nil {
		http.Error(w, "trace not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
