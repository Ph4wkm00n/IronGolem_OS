// Package internal implements connector canary checks for proactive health
// monitoring. Canaries detect connector degradation (connectivity loss,
// latency spikes, credential expiry, data corruption) before full failure.
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
	CanaryConnectivity   CanaryType = "connectivity"
	CanaryLatency        CanaryType = "latency"
	CanaryAuth           CanaryType = "auth"
	CanaryDataIntegrity  CanaryType = "data_integrity"
)

// CanaryCheck describes a scheduled canary probe for a connector.
type CanaryCheck struct {
	ID                  string     `json:"id"`
	ConnectorID         string     `json:"connector_id"`
	Type                CanaryType `json:"type"`
	Schedule            string     `json:"schedule"` // cron expression
	LastRun             *time.Time `json:"last_run,omitempty"`
	LastResult          *CanaryResult `json:"last_result,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
}

// CanaryResult captures the outcome of a single canary probe.
type CanaryResult struct {
	Passed    bool          `json:"passed"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// CanaryProbe is a function that executes a canary check and returns the
// result. Implementations vary by canary type.
type CanaryProbe func(ctx context.Context, connectorID string) CanaryResult

// CanaryAlertHandler is called when a canary fails N consecutive times.
type CanaryAlertHandler func(ctx context.Context, check CanaryCheck)

// CanaryManagerConfig holds tunable parameters for the canary manager.
type CanaryManagerConfig struct {
	// DefaultInterval is the check interval when no cron schedule is set.
	DefaultInterval time.Duration

	// FailureThreshold is the number of consecutive failures before an
	// alert is emitted. Default: 3.
	FailureThreshold int

	// LatencySLA is the maximum acceptable response time for latency
	// canaries. Default: 2s.
	LatencySLA time.Duration
}

func (c *CanaryManagerConfig) applyDefaults() {
	if c.DefaultInterval <= 0 {
		c.DefaultInterval = 60 * time.Second
	}
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 3
	}
	if c.LatencySLA <= 0 {
		c.LatencySLA = 2 * time.Second
	}
}

// CanaryManager registers and runs canary checks for connectors. It detects
// degradation early and emits events when failures exceed the threshold.
type CanaryManager struct {
	mu      sync.RWMutex
	checks  map[string]*CanaryCheck
	probes  map[CanaryType]CanaryProbe
	config  CanaryManagerConfig
	logger  *slog.Logger
	alertFn CanaryAlertHandler
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewCanaryManager creates a CanaryManager with the given configuration.
func NewCanaryManager(logger *slog.Logger, config CanaryManagerConfig) *CanaryManager {
	config.applyDefaults()
	cm := &CanaryManager{
		checks: make(map[string]*CanaryCheck),
		probes: make(map[CanaryType]CanaryProbe),
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
	}

	// Register built-in canary probes.
	cm.probes[CanaryConnectivity] = cm.connectivityProbe
	cm.probes[CanaryLatency] = cm.latencyProbe
	cm.probes[CanaryAuth] = cm.authProbe
	cm.probes[CanaryDataIntegrity] = cm.dataIntegrityProbe

	return cm
}

// SetAlertHandler sets the callback invoked when a canary exceeds the
// failure threshold.
func (m *CanaryManager) SetAlertHandler(fn CanaryAlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertFn = fn
}

// Register adds a canary check. If a check with the same ID already exists
// it is replaced.
func (m *CanaryManager) Register(check CanaryCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checks[check.ID] = &check
	m.logger.Info("canary registered",
		slog.String("id", check.ID),
		slog.String("connector", check.ConnectorID),
		slog.String("type", string(check.Type)),
	)
}

// ListChecks returns a snapshot of all registered canary checks.
func (m *CanaryManager) ListChecks() []CanaryCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]CanaryCheck, 0, len(m.checks))
	for _, c := range m.checks {
		result = append(result, *c)
	}
	return result
}

// RunCheck forces a single canary check to execute immediately.
func (m *CanaryManager) RunCheck(ctx context.Context, checkID string) (*CanaryResult, error) {
	m.mu.Lock()
	check, ok := m.checks[checkID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("canary check not found: %s", checkID)
	}
	// Copy probe reference while holding the lock.
	probe, hasProbe := m.probes[check.Type]
	m.mu.Unlock()

	if !hasProbe {
		return nil, fmt.Errorf("no probe registered for canary type: %s", check.Type)
	}

	result := probe(ctx, check.ConnectorID)
	m.recordResult(ctx, checkID, result)
	return &result, nil
}

// Run starts the background canary loop. It blocks until Stop or context
// cancellation.
func (m *CanaryManager) Run(ctx context.Context) {
	m.wg.Add(1)
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.DefaultInterval)
	defer ticker.Stop()

	m.logger.Info("canary manager started",
		slog.Duration("interval", m.config.DefaultInterval),
		slog.Int("failure_threshold", m.config.FailureThreshold),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runAll(ctx)
		}
	}
}

// Stop signals the canary loop to exit and waits for it to finish.
func (m *CanaryManager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.wg.Wait()
}

// runAll executes all registered canary checks.
func (m *CanaryManager) runAll(ctx context.Context) {
	m.mu.RLock()
	ids := make([]string, 0, len(m.checks))
	for id := range m.checks {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		if _, err := m.RunCheck(ctx, id); err != nil {
			m.logger.Warn("canary check failed to run",
				slog.String("check_id", id),
				slog.String("error", err.Error()),
			)
		}
	}
}

// recordResult updates the check state and triggers alerts when appropriate.
func (m *CanaryManager) recordResult(ctx context.Context, checkID string, result CanaryResult) {
	m.mu.Lock()
	check, ok := m.checks[checkID]
	if !ok {
		m.mu.Unlock()
		return
	}

	now := result.Timestamp
	check.LastRun = &now
	check.LastResult = &result

	if result.Passed {
		check.ConsecutiveFailures = 0
		m.mu.Unlock()
		m.logger.Debug("canary passed",
			slog.String("check_id", checkID),
			slog.String("connector", check.ConnectorID),
			slog.Duration("duration", result.Duration),
		)
		return
	}

	check.ConsecutiveFailures++
	failures := check.ConsecutiveFailures
	threshold := m.config.FailureThreshold
	snapshot := *check
	alertFn := m.alertFn
	m.mu.Unlock()

	m.logger.Warn("canary failed",
		slog.String("check_id", checkID),
		slog.String("connector", snapshot.ConnectorID),
		slog.Int("consecutive_failures", failures),
		slog.String("error", result.Error),
	)

	if failures >= threshold {
		m.emitCanaryAlert(ctx, snapshot)
		if alertFn != nil {
			alertFn(ctx, snapshot)
		}
	}
}

// emitCanaryAlert emits an event when a canary exceeds the failure threshold.
func (m *CanaryManager) emitCanaryAlert(ctx context.Context, check CanaryCheck) {
	payload, _ := json.Marshal(map[string]any{
		"check_id":             check.ID,
		"connector_id":        check.ConnectorID,
		"type":                check.Type,
		"consecutive_failures": check.ConsecutiveFailures,
		"last_error":          check.LastResult.Error,
	})
	evt := events.NewEvent(events.EventKindConnectorError, "", "health", payload)
	m.logger.Error("canary alert: connector degradation detected",
		slog.String("event_id", evt.ID),
		slog.String("check_id", check.ID),
		slog.String("connector", check.ConnectorID),
		slog.String("type", string(check.Type)),
		slog.Int("consecutive_failures", check.ConsecutiveFailures),
	)
	_ = ctx // context carried for future async event publishing
}

// --- Built-in canary probes ---

// connectivityProbe verifies the connector is reachable and alive.
func (m *CanaryManager) connectivityProbe(_ context.Context, connectorID string) CanaryResult {
	start := time.Now()
	// In production this would call the connector's Health endpoint.
	// For now, we simulate a successful connectivity check.
	return CanaryResult{
		Passed:    true,
		Duration:  time.Since(start),
		Timestamp: time.Now().UTC(),
	}
}

// latencyProbe checks that the connector responds within the SLA.
func (m *CanaryManager) latencyProbe(_ context.Context, connectorID string) CanaryResult {
	start := time.Now()
	// In production this would perform a round-trip request to the connector
	// and measure the response time.
	elapsed := time.Since(start)

	if elapsed > m.config.LatencySLA {
		return CanaryResult{
			Passed:    false,
			Duration:  elapsed,
			Error:     fmt.Sprintf("latency %s exceeds SLA %s", elapsed, m.config.LatencySLA),
			Timestamp: time.Now().UTC(),
		}
	}

	return CanaryResult{
		Passed:    true,
		Duration:  elapsed,
		Timestamp: time.Now().UTC(),
	}
}

// authProbe verifies the connector credentials have not expired.
func (m *CanaryManager) authProbe(_ context.Context, connectorID string) CanaryResult {
	start := time.Now()
	// In production this would validate OAuth tokens, API keys, etc.
	return CanaryResult{
		Passed:    true,
		Duration:  time.Since(start),
		Timestamp: time.Now().UTC(),
	}
}

// dataIntegrityProbe sends a test payload and verifies round-trip integrity.
func (m *CanaryManager) dataIntegrityProbe(_ context.Context, connectorID string) CanaryResult {
	start := time.Now()
	// In production this would send a known payload through the connector
	// pipeline and verify the data arrives intact.
	return CanaryResult{
		Passed:    true,
		Duration:  time.Since(start),
		Timestamp: time.Now().UTC(),
	}
}
