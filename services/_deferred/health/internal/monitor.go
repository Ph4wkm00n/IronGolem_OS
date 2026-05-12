// Package internal implements the system monitor for the IronGolem OS
// Health service. The SystemMonitor periodically checks all registered
// services, tracks resource usage, and exposes a summary endpoint for
// the Health Center dashboard.
package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// ConnectorStatus represents the health state of a connector.
type ConnectorStatus struct {
	Name          string    `json:"name"`
	Healthy       bool      `json:"healthy"`
	LastChecked   time.Time `json:"last_checked"`
	Latency       time.Duration `json:"latency_ns"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	ConsecutiveFails int    `json:"consecutive_fails"`
}

// ResourceUsage tracks simple CPU and memory metrics.
type ResourceUsage struct {
	MemoryAllocMB   float64 `json:"memory_alloc_mb"`
	MemoryTotalMB   float64 `json:"memory_total_mb"`
	MemorySysMB     float64 `json:"memory_sys_mb"`
	NumGoroutines   int     `json:"num_goroutines"`
	NumCPU          int     `json:"num_cpu"`
	GCPauseTotalMs  float64 `json:"gc_pause_total_ms"`
	CollectedAt     time.Time `json:"collected_at"`
}

// StatusSummary provides an aggregate view for the Health Center dashboard.
type StatusSummary struct {
	TotalServices  int            `json:"total_services"`
	Healthy        int            `json:"healthy"`
	Recovering     int            `json:"recovering"`
	NeedsAttention int            `json:"needs_attention"`
	Paused         int            `json:"paused"`
	Quarantined    int            `json:"quarantined"`
	Resources      ResourceUsage  `json:"resources"`
	Connectors     []ConnectorStatus `json:"connectors"`
	CheckedAt      time.Time      `json:"checked_at"`
}

// ConnectorChecker is a function that checks the health of a connector.
// Implementations should return nil if the connector is healthy.
type ConnectorChecker func(ctx context.Context) error

// MonitorConfig holds configuration for the SystemMonitor.
type MonitorConfig struct {
	// PollInterval is how often the monitor refreshes its view.
	// Default: 15s.
	PollInterval time.Duration

	// ConnectorTimeout is the max duration for a connector health check.
	// Default: 5s.
	ConnectorTimeout time.Duration
}

func (c *MonitorConfig) applyDefaults() {
	if c.PollInterval <= 0 {
		c.PollInterval = 15 * time.Second
	}
	if c.ConnectorTimeout <= 0 {
		c.ConnectorTimeout = 5 * time.Second
	}
}

// SystemMonitor periodically checks all registered services and connectors,
// collects resource usage, and maintains a summary for the dashboard.
type SystemMonitor struct {
	mu         sync.RWMutex
	hbMgr      *HeartbeatManager
	config     MonitorConfig
	logger     *slog.Logger
	connectors map[string]ConnectorChecker
	connStatus map[string]*ConnectorStatus
	lastUsage  ResourceUsage
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewSystemMonitor creates a SystemMonitor backed by the given HeartbeatManager.
func NewSystemMonitor(logger *slog.Logger, hbMgr *HeartbeatManager, config MonitorConfig) *SystemMonitor {
	config.applyDefaults()
	return &SystemMonitor{
		hbMgr:      hbMgr,
		config:     config,
		logger:     logger,
		connectors: make(map[string]ConnectorChecker),
		connStatus: make(map[string]*ConnectorStatus),
		stopCh:     make(chan struct{}),
	}
}

// RegisterConnector adds a connector health checker to the monitor.
func (m *SystemMonitor) RegisterConnector(name string, checker ConnectorChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectors[name] = checker
	m.connStatus[name] = &ConnectorStatus{
		Name:    name,
		Healthy: true,
	}
}

// Summary returns the current status summary.
func (m *SystemMonitor) Summary(ctx context.Context) StatusSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hbSummary := m.hbMgr.SystemSummary(ctx)

	connectors := make([]ConnectorStatus, 0, len(m.connStatus))
	for _, cs := range m.connStatus {
		connectors = append(connectors, *cs)
	}

	return StatusSummary{
		TotalServices:  hbSummary.TotalServices,
		Healthy:        hbSummary.Healthy,
		Recovering:     hbSummary.Recovering,
		NeedsAttention: hbSummary.NeedAttention,
		Paused:         hbSummary.Paused,
		Quarantined:    hbSummary.Quarantined,
		Resources:      m.lastUsage,
		Connectors:     connectors,
		CheckedAt:      time.Now().UTC(),
	}
}

// Run starts the background monitoring loop. It blocks until the context
// is cancelled or Stop is called.
func (m *SystemMonitor) Run(ctx context.Context) {
	m.wg.Add(1)
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()

	m.logger.Info("system monitor started",
		slog.Duration("poll_interval", m.config.PollInterval),
	)

	// Do an initial poll immediately.
	m.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll(ctx)
		}
	}
}

// Stop signals the monitor loop to exit and waits for it to finish.
func (m *SystemMonitor) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.wg.Wait()
}

// poll collects resource usage and checks connector health.
func (m *SystemMonitor) poll(ctx context.Context) {
	m.collectResourceUsage()
	m.checkConnectors(ctx)
}

// collectResourceUsage reads Go runtime memory stats.
func (m *SystemMonitor) collectResourceUsage() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	usage := ResourceUsage{
		MemoryAllocMB:  float64(memStats.Alloc) / 1024 / 1024,
		MemoryTotalMB:  float64(memStats.TotalAlloc) / 1024 / 1024,
		MemorySysMB:    float64(memStats.Sys) / 1024 / 1024,
		NumGoroutines:  runtime.NumGoroutine(),
		NumCPU:         runtime.NumCPU(),
		GCPauseTotalMs: float64(memStats.PauseTotalNs) / 1e6,
		CollectedAt:    time.Now().UTC(),
	}

	m.mu.Lock()
	m.lastUsage = usage
	m.mu.Unlock()
}

// checkConnectors polls all registered connectors.
func (m *SystemMonitor) checkConnectors(ctx context.Context) {
	m.mu.RLock()
	checkers := make(map[string]ConnectorChecker, len(m.connectors))
	for k, v := range m.connectors {
		checkers[k] = v
	}
	m.mu.RUnlock()

	for name, checker := range checkers {
		checkCtx, cancel := context.WithTimeout(ctx, m.config.ConnectorTimeout)
		start := time.Now()
		err := checker(checkCtx)
		latency := time.Since(start)
		cancel()

		m.mu.Lock()
		cs := m.connStatus[name]
		if cs == nil {
			cs = &ConnectorStatus{Name: name}
			m.connStatus[name] = cs
		}
		cs.LastChecked = time.Now().UTC()
		cs.Latency = latency

		if err != nil {
			cs.Healthy = false
			cs.ErrorMessage = err.Error()
			cs.ConsecutiveFails++
			m.logger.WarnContext(ctx, "connector health check failed",
				slog.String("connector", name),
				slog.String("error", err.Error()),
				slog.Int("consecutive_fails", cs.ConsecutiveFails),
			)
		} else {
			cs.Healthy = true
			cs.ErrorMessage = ""
			cs.ConsecutiveFails = 0
		}
		m.mu.Unlock()
	}
}

// HandleSummary returns an http.HandlerFunc for GET /api/v1/health/summary.
func (m *SystemMonitor) HandleSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary := m.Summary(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(summary)
	}
}

// HandleConnectors returns an http.HandlerFunc for GET /api/v1/health/connectors.
func (m *SystemMonitor) HandleConnectors() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		connectors := make([]ConnectorStatus, 0, len(m.connStatus))
		for _, cs := range m.connStatus {
			connectors = append(connectors, *cs)
		}
		m.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"connectors": connectors,
			"count":      len(connectors),
			"checked_at": time.Now().UTC(),
		})
		_ = r.Context() // use ctx to satisfy lint
	}
}

// HandleResources returns an http.HandlerFunc for GET /api/v1/health/resources.
func (m *SystemMonitor) HandleResources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		usage := m.lastUsage
		m.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(usage)
		_ = r.Context() // use ctx to satisfy lint
	}
}
