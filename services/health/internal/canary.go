// Package internal implements connector canary checks for the IronGolem OS
// Health service. Canaries detect connector degradation before full failure
// by periodically verifying connectivity, latency, authentication, and data
// integrity.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

// CanaryType identifies the kind of canary check.
type CanaryType string

const (
	CanaryConnectivity  CanaryType = "connectivity"
	CanaryLatency       CanaryType = "latency"
	CanaryAuth          CanaryType = "auth"
	CanaryDataIntegrity CanaryType = "data_integrity"
)

// CanaryCheck describes a scheduled canary check for a connector.
type CanaryCheck struct {
	// ID uniquely identifies this canary check.
	ID string `json:"id"`

	// ConnectorID is the connector being checked.
	ConnectorID string `json:"connector_id"`

	// Type is the kind of canary check.
	Type CanaryType `json:"type"`

	// Schedule is a cron-like interval string (e.g. "30s", "5m").
	Schedule string `json:"schedule"`

	// Interval is the parsed duration from Schedule.
	Interval time.Duration `json:"-"`

	// LastRun is the timestamp of the most recent execution.
	LastRun *time.Time `json:"last_run,omitempty"`

	// LastResult is the outcome of the most recent execution.
	LastResult *CanaryResult `json:"last_result,omitempty"`

	// ConsecutiveFailures is the number of failures in a row.
	ConsecutiveFailures int `json:"consecutive_failures"`

	// FailureThreshold is how many consecutive failures trigger an alert.
	FailureThreshold int `json:"failure_threshold"`
}

// CanaryResult captures the outcome of a single canary execution.
type CanaryResult struct {
	Passed    bool          `json:"passed"`
	Duration  time.Duration `json:"duration_ns"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// CanaryProbe is the interface that individual canary types implement.
type CanaryProbe interface {
	// Probe executes the canary check and returns the result.
	Probe(ctx context.Context, connectorID string) CanaryResult
}

// ConnectivityCanary verifies that a connection to the connector is alive.
type ConnectivityCanary struct {
	// Checker is called with the connector ID and returns nil if alive.
	Checker func(ctx context.Context, connectorID string) error
}

// Probe checks connectivity to the connector.
func (c *ConnectivityCanary) Probe(ctx context.Context, connectorID string) CanaryResult {
	start := time.Now()
	err := c.Checker(ctx, connectorID)
	dur := time.Since(start)

	result := CanaryResult{
		Passed:    err == nil,
		Duration:  dur,
		Timestamp: time.Now().UTC(),
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// LatencyCanary checks that response time is within an SLA threshold.
type LatencyCanary struct {
	// MaxLatency is the maximum acceptable response time.
	MaxLatency time.Duration

	// Pinger is called to measure round-trip time to the connector.
	Pinger func(ctx context.Context, connectorID string) (time.Duration, error)
}

// Probe measures latency and fails if it exceeds the threshold.
func (c *LatencyCanary) Probe(ctx context.Context, connectorID string) CanaryResult {
	start := time.Now()
	latency, err := c.Pinger(ctx, connectorID)
	dur := time.Since(start)

	result := CanaryResult{
		Duration:  dur,
		Timestamp: time.Now().UTC(),
	}

	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("ping failed: %v", err)
		return result
	}

	if latency > c.MaxLatency {
		result.Passed = false
		result.Error = fmt.Sprintf("latency %v exceeds SLA threshold %v", latency, c.MaxLatency)
		return result
	}

	result.Passed = true
	return result
}

// AuthCanary verifies that credentials for a connector have not expired.
type AuthCanary struct {
	// Verifier checks whether the connector's credentials are still valid.
	Verifier func(ctx context.Context, connectorID string) error
}

// Probe checks authentication validity.
func (c *AuthCanary) Probe(ctx context.Context, connectorID string) CanaryResult {
	start := time.Now()
	err := c.Verifier(ctx, connectorID)
	dur := time.Since(start)

	result := CanaryResult{
		Passed:    err == nil,
		Duration:  dur,
		Timestamp: time.Now().UTC(),
	}
	if err != nil {
		result.Error = fmt.Sprintf("auth verification failed: %v", err)
	}
	return result
}

// DataIntegrityCanary sends test data through a connector and verifies the
// round-trip to detect silent data corruption.
type DataIntegrityCanary struct {
	// RoundTripper sends a test payload and returns nil if the response matches.
	RoundTripper func(ctx context.Context, connectorID string, testPayload string) error
}

// Probe sends test data and verifies round-trip integrity.
func (c *DataIntegrityCanary) Probe(ctx context.Context, connectorID string) CanaryResult {
	start := time.Now()
	testPayload := fmt.Sprintf("canary-integrity-check-%d", time.Now().UnixNano())
	err := c.RoundTripper(ctx, connectorID, testPayload)
	dur := time.Since(start)

	result := CanaryResult{
		Passed:    err == nil,
		Duration:  dur,
		Timestamp: time.Now().UTC(),
	}
	if err != nil {
		result.Error = fmt.Sprintf("data integrity check failed: %v", err)
	}
	return result
}

// CanaryManager manages registration and execution of canary checks.
type CanaryManager struct {
	mu     sync.RWMutex
	checks map[string]*CanaryCheck
	probes map[string]CanaryProbe
	logger *slog.Logger
	stopCh chan struct{}
	wg     sync.WaitGroup

	// AlertCallback is called when a canary fails N consecutive times.
	AlertCallback func(check CanaryCheck)
}

// NewCanaryManager creates a CanaryManager with the given logger.
func NewCanaryManager(logger *slog.Logger) *CanaryManager {
	return &CanaryManager{
		checks: make(map[string]*CanaryCheck),
		probes: make(map[string]CanaryProbe),
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Register adds a canary check with its associated probe.
func (m *CanaryManager) Register(check CanaryCheck, probe CanaryProbe) error {
	if check.ID == "" {
		return fmt.Errorf("canary check ID is required")
	}
	if check.ConnectorID == "" {
		return fmt.Errorf("canary check connector_id is required")
	}
	if check.FailureThreshold <= 0 {
		check.FailureThreshold = 3
	}
	if check.Interval <= 0 {
		check.Interval = 60 * time.Second
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.checks[check.ID] = &check
	m.probes[check.ID] = probe

	m.logger.Info("canary check registered",
		slog.String("id", check.ID),
		slog.String("connector_id", check.ConnectorID),
		slog.String("type", string(check.Type)),
	)

	return nil
}

// List returns a snapshot of all registered canary checks.
func (m *CanaryManager) List() []CanaryCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]CanaryCheck, 0, len(m.checks))
	for _, c := range m.checks {
		result = append(result, *c)
	}
	return result
}

// RunCheck forces execution of a single canary check by ID.
func (m *CanaryManager) RunCheck(ctx context.Context, checkID string) (*CanaryResult, error) {
	m.mu.Lock()
	check, ok := m.checks[checkID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("canary check not found: %s", checkID)
	}
	probe, ok := m.probes[checkID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("canary probe not found: %s", checkID)
	}
	m.mu.Unlock()

	result := probe.Probe(ctx, check.ConnectorID)

	m.mu.Lock()
	now := time.Now().UTC()
	check.LastRun = &now
	check.LastResult = &result

	if result.Passed {
		check.ConsecutiveFailures = 0
	} else {
		check.ConsecutiveFailures++
		m.logger.Warn("canary check failed",
			slog.String("id", check.ID),
			slog.String("connector_id", check.ConnectorID),
			slog.String("type", string(check.Type)),
			slog.Int("consecutive_failures", check.ConsecutiveFailures),
			slog.String("error", result.Error),
		)

		if check.ConsecutiveFailures >= check.FailureThreshold {
			m.emitAlert(*check)
		}
	}
	m.mu.Unlock()

	return &result, nil
}

// emitAlert fires an event and calls the alert callback when a canary
// exceeds the failure threshold. Caller must hold the lock.
func (m *CanaryManager) emitAlert(check CanaryCheck) {
	payload, _ := json.Marshal(map[string]any{
		"canary_id":            check.ID,
		"connector_id":         check.ConnectorID,
		"type":                 check.Type,
		"consecutive_failures": check.ConsecutiveFailures,
		"last_error":           check.LastResult.Error,
	})
	evt := events.NewEvent(events.EventKindConnectorDown, "", "health", payload)

	m.logger.Error("canary alert: connector degradation detected",
		slog.String("event_id", evt.ID),
		slog.String("canary_id", check.ID),
		slog.String("connector_id", check.ConnectorID),
		slog.Int("consecutive_failures", check.ConsecutiveFailures),
	)

	if m.AlertCallback != nil {
		go m.AlertCallback(check)
	}
}

// Run starts the background canary execution loop. It checks each canary
// at its configured interval.
func (m *CanaryManager) Run(ctx context.Context) {
	m.wg.Add(1)
	defer m.wg.Done()

	// Use a base tick of 5 seconds and check which canaries are due.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	m.logger.Info("canary manager started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluateDue(ctx)
		}
	}
}

// evaluateDue runs any canary checks that are past their scheduled interval.
func (m *CanaryManager) evaluateDue(ctx context.Context) {
	m.mu.RLock()
	var due []string
	now := time.Now().UTC()
	for id, check := range m.checks {
		if check.LastRun == nil || now.Sub(*check.LastRun) >= check.Interval {
			due = append(due, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range due {
		if _, err := m.RunCheck(ctx, id); err != nil {
			m.logger.Warn("failed to run canary check",
				slog.String("id", id),
				slog.String("error", err.Error()),
			)
		}
	}
}

// Stop signals the canary manager to exit and waits for completion.
func (m *CanaryManager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.wg.Wait()
}
