// Package internal implements heartbeat tracking and self-healing triggers
// for the IronGolem OS Health service.
//
// The HeartbeatManager monitors all registered services and agents, detects
// missed heartbeats, transitions health states, and invokes self-healing
// actions when services degrade.
package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

// HeartbeatState maps to the five heartbeat states defined in the IronGolem
// OS specification.
type HeartbeatState string

const (
	StateHealthy           HeartbeatState = "healthy"
	StateQuietlyRecovering HeartbeatState = "quietly_recovering"
	StateNeedsAttention    HeartbeatState = "needs_attention"
	StatePaused            HeartbeatState = "paused"
	StateQuarantined       HeartbeatState = "quarantined"
)

// HeartbeatConfig holds tunable parameters for the heartbeat monitor.
type HeartbeatConfig struct {
	// Timeout is the maximum duration between heartbeats before a service
	// is considered unhealthy. Default: 30s.
	Timeout time.Duration

	// CheckInterval is how often the monitor evaluates all services.
	// Default: 10s.
	CheckInterval time.Duration

	// QuietRecoveryWindow is how long a service stays in QuietlyRecovering
	// before being promoted back to Healthy. Default: 60s.
	QuietRecoveryWindow time.Duration

	// AttentionThreshold is the number of consecutive missed heartbeats
	// before transitioning from QuietlyRecovering to NeedsAttention.
	// Default: 3.
	AttentionThreshold int

	// QuarantineThreshold is the number of consecutive missed heartbeats
	// before transitioning to Quarantined. Default: 10.
	QuarantineThreshold int
}

func (c *HeartbeatConfig) applyDefaults() {
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if c.CheckInterval <= 0 {
		c.CheckInterval = 10 * time.Second
	}
	if c.QuietRecoveryWindow <= 0 {
		c.QuietRecoveryWindow = 60 * time.Second
	}
	if c.AttentionThreshold <= 0 {
		c.AttentionThreshold = 3
	}
	if c.QuarantineThreshold <= 0 {
		c.QuarantineThreshold = 10
	}
}

// ServiceRecord holds the health tracking state for a single service or agent.
type ServiceRecord struct {
	ServiceName   string         `json:"service_name"`
	State         HeartbeatState `json:"state"`
	LastHeartbeat time.Time      `json:"last_heartbeat"`
	LastStatus    string         `json:"last_status"`
	MissedBeats   int            `json:"missed_beats"`
	RecoveredAt   *time.Time     `json:"recovered_at,omitempty"`
	Message       string         `json:"message,omitempty"`
	Metrics       map[string]any `json:"metrics,omitempty"`
	RegisteredAt  time.Time      `json:"registered_at"`
	HealingCount  int            `json:"healing_count"`
	LastHealingAt *time.Time     `json:"last_healing_at,omitempty"`
}

// SelfHealingTrigger is invoked when a service's health deteriorates beyond
// the quiet-recovery threshold. Implementations perform corrective actions
// such as restarting services, reallocating resources, or notifying operators.
type SelfHealingTrigger interface {
	// OnServiceDegraded is called when a service transitions to NeedsAttention.
	OnServiceDegraded(ctx context.Context, record ServiceRecord) error

	// OnServiceQuarantined is called when a service transitions to Quarantined.
	OnServiceQuarantined(ctx context.Context, record ServiceRecord) error

	// OnServiceRecovered is called when a service returns to Healthy.
	OnServiceRecovered(ctx context.Context, record ServiceRecord) error
}

// SystemSummaryResponse is the response for the overall health endpoint.
type SystemSummaryResponse struct {
	OverallState  HeartbeatState    `json:"overall_state"`
	TotalServices int               `json:"total_services"`
	Healthy       int               `json:"healthy"`
	Recovering    int               `json:"recovering"`
	NeedAttention int               `json:"need_attention"`
	Paused        int               `json:"paused"`
	Quarantined   int               `json:"quarantined"`
	CheckedAt     time.Time         `json:"checked_at"`
	ServiceStates map[string]string `json:"service_states"`
}

// HeartbeatManager tracks heartbeats for all services and agents, detects
// missed beats, and triggers self-healing when appropriate.
type HeartbeatManager struct {
	mu      sync.RWMutex
	records map[string]*ServiceRecord
	config  HeartbeatConfig
	logger  *slog.Logger
	healer  SelfHealingTrigger
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewHeartbeatManager creates a HeartbeatManager with the given configuration.
func NewHeartbeatManager(logger *slog.Logger, config HeartbeatConfig) *HeartbeatManager {
	config.applyDefaults()
	return &HeartbeatManager{
		records: make(map[string]*ServiceRecord),
		config:  config,
		logger:  logger,
		healer:  &logOnlyHealer{logger: logger},
		stopCh:  make(chan struct{}),
	}
}

// SetHealer replaces the self-healing trigger implementation.
func (m *HeartbeatManager) SetHealer(healer SelfHealingTrigger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healer = healer
}

// RecordHeartbeat processes an incoming heartbeat from a service.
func (m *HeartbeatManager) RecordHeartbeat(ctx context.Context, payload events.HeartbeatPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	rec, exists := m.records[payload.ServiceName]

	if !exists {
		rec = &ServiceRecord{
			ServiceName:  payload.ServiceName,
			State:        StateHealthy,
			RegisteredAt: now,
		}
		m.records[payload.ServiceName] = rec
		m.logger.InfoContext(ctx, "new service registered",
			slog.String("service", payload.ServiceName),
		)
	}

	rec.LastHeartbeat = now
	rec.LastStatus = string(payload.Status)
	rec.Metrics = payload.Metrics
	rec.Message = payload.Message

	// If the service was degraded and is now checking in, transition to
	// quietly recovering.
	switch rec.State {
	case StateNeedsAttention, StateQuarantined:
		rec.State = StateQuietlyRecovering
		rec.RecoveredAt = &now
		rec.MissedBeats = 0
		rec.Message = "heartbeat resumed; entering quiet recovery"
		m.logger.InfoContext(ctx, "service entering quiet recovery",
			slog.String("service", payload.ServiceName),
		)
	case StateQuietlyRecovering:
		// Stay in recovering; the monitor loop will promote to healthy
		// after the recovery window.
		rec.MissedBeats = 0
	case StatePaused:
		// Paused services resume to quietly recovering.
		rec.State = StateQuietlyRecovering
		rec.RecoveredAt = &now
		rec.MissedBeats = 0
	default:
		rec.State = StateHealthy
		rec.MissedBeats = 0
	}
}

// PauseService manually pauses monitoring for a service.
func (m *HeartbeatManager) PauseService(ctx context.Context, serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.records[serviceName]
	if !ok {
		return errServiceNotFound(serviceName)
	}

	rec.State = StatePaused
	rec.Message = "manually paused"
	m.logger.InfoContext(ctx, "service paused",
		slog.String("service", serviceName),
	)
	return nil
}

// QuarantineService manually quarantines a service.
func (m *HeartbeatManager) QuarantineService(ctx context.Context, serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.records[serviceName]
	if !ok {
		return errServiceNotFound(serviceName)
	}

	rec.State = StateQuarantined
	rec.Message = "manually quarantined"
	m.logger.InfoContext(ctx, "service quarantined",
		slog.String("service", serviceName),
	)
	return nil
}

// ListAll returns a snapshot of all service records.
func (m *HeartbeatManager) ListAll(_ context.Context) []ServiceRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ServiceRecord, 0, len(m.records))
	for _, rec := range m.records {
		result = append(result, *rec)
	}
	return result
}

// SystemSummary returns an aggregate health view of all monitored services.
func (m *HeartbeatManager) SystemSummary(_ context.Context) SystemSummaryResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := SystemSummaryResponse{
		TotalServices: len(m.records),
		CheckedAt:     time.Now().UTC(),
		ServiceStates: make(map[string]string, len(m.records)),
	}

	for name, rec := range m.records {
		summary.ServiceStates[name] = string(rec.State)
		switch rec.State {
		case StateHealthy:
			summary.Healthy++
		case StateQuietlyRecovering:
			summary.Recovering++
		case StateNeedsAttention:
			summary.NeedAttention++
		case StatePaused:
			summary.Paused++
		case StateQuarantined:
			summary.Quarantined++
		}
	}

	// Overall state is the worst state across all services.
	switch {
	case summary.Quarantined > 0:
		summary.OverallState = StateQuarantined
	case summary.NeedAttention > 0:
		summary.OverallState = StateNeedsAttention
	case summary.Recovering > 0:
		summary.OverallState = StateQuietlyRecovering
	case summary.Paused > 0 && summary.Healthy == 0:
		summary.OverallState = StatePaused
	default:
		summary.OverallState = StateHealthy
	}

	return summary
}

// Run starts the background monitoring loop. It blocks until the context
// is cancelled or Stop is called.
func (m *HeartbeatManager) Run(ctx context.Context) {
	m.wg.Add(1)
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	m.logger.Info("heartbeat monitor started",
		slog.Duration("timeout", m.config.Timeout),
		slog.Duration("check_interval", m.config.CheckInterval),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluate(ctx)
		}
	}
}

// Stop signals the monitor loop to exit and waits for it to finish.
func (m *HeartbeatManager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.wg.Wait()
}

// evaluate checks all service records for missed heartbeats and transitions
// their states accordingly.
func (m *HeartbeatManager) evaluate(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	for name, rec := range m.records {
		if rec.State == StatePaused {
			continue
		}

		elapsed := now.Sub(rec.LastHeartbeat)

		switch rec.State {
		case StateHealthy:
			if elapsed > m.config.Timeout {
				rec.MissedBeats++
				rec.State = StateQuietlyRecovering
				rec.Message = "missed heartbeat; entering quiet recovery"
				m.logger.WarnContext(ctx, "service missed heartbeat",
					slog.String("service", name),
					slog.Duration("silence", elapsed),
				)
			}

		case StateQuietlyRecovering:
			if elapsed > m.config.Timeout {
				rec.MissedBeats++
				if rec.MissedBeats >= m.config.AttentionThreshold {
					rec.State = StateNeedsAttention
					rec.Message = "multiple missed heartbeats"
					m.logger.WarnContext(ctx, "service needs attention",
						slog.String("service", name),
						slog.Int("missed_beats", rec.MissedBeats),
					)
					// Trigger self-healing.
					rec.HealingCount++
					healTime := now
					rec.LastHealingAt = &healTime
					m.triggerHealing(ctx, *rec, false)
				}
			} else if rec.RecoveredAt != nil && now.Sub(*rec.RecoveredAt) > m.config.QuietRecoveryWindow {
				// Promote back to healthy after sustained recovery.
				rec.State = StateHealthy
				rec.RecoveredAt = nil
				rec.MissedBeats = 0
				rec.Message = ""
				m.logger.InfoContext(ctx, "service promoted to healthy",
					slog.String("service", name),
				)
				m.triggerRecovered(ctx, *rec)
			}

		case StateNeedsAttention:
			if elapsed > m.config.Timeout {
				rec.MissedBeats++
				if rec.MissedBeats >= m.config.QuarantineThreshold {
					rec.State = StateQuarantined
					rec.Message = "quarantined due to prolonged absence"
					m.logger.ErrorContext(ctx, "service quarantined",
						slog.String("service", name),
						slog.Int("missed_beats", rec.MissedBeats),
					)
					rec.HealingCount++
					healTime := now
					rec.LastHealingAt = &healTime
					m.triggerHealing(ctx, *rec, true)
				}
			}

		case StateQuarantined:
			// Quarantined services stay quarantined until a heartbeat arrives
			// (handled in RecordHeartbeat) or manual intervention.
		}
	}
}

// triggerHealing invokes the self-healing interface. The caller holds the
// lock, so we snapshot the record and fire asynchronously.
func (m *HeartbeatManager) triggerHealing(ctx context.Context, rec ServiceRecord, quarantined bool) {
	payload, _ := json.Marshal(map[string]any{
		"service":      rec.ServiceName,
		"state":        rec.State,
		"missed_beats": rec.MissedBeats,
		"quarantined":  quarantined,
	})
	evt := events.NewEvent(events.EventKindHealingTriggered, "", "health", payload)
	m.logger.InfoContext(ctx, "self-healing triggered",
		slog.String("event_id", evt.ID),
		slog.String("service", rec.ServiceName),
		slog.Bool("quarantined", quarantined),
	)

	go func() {
		if quarantined {
			if err := m.healer.OnServiceQuarantined(ctx, rec); err != nil {
				m.logger.ErrorContext(ctx, "quarantine healing failed",
					slog.String("service", rec.ServiceName),
					slog.String("error", err.Error()),
				)
			}
		} else {
			if err := m.healer.OnServiceDegraded(ctx, rec); err != nil {
				m.logger.ErrorContext(ctx, "degraded healing failed",
					slog.String("service", rec.ServiceName),
					slog.String("error", err.Error()),
				)
			}
		}
	}()
}

// triggerRecovered notifies the healer that a service recovered.
func (m *HeartbeatManager) triggerRecovered(ctx context.Context, rec ServiceRecord) {
	payload, _ := json.Marshal(map[string]string{
		"service": rec.ServiceName,
	})
	evt := events.NewEvent(events.EventKindHealingResolved, "", "health", payload)
	m.logger.InfoContext(ctx, "service recovery confirmed",
		slog.String("event_id", evt.ID),
		slog.String("service", rec.ServiceName),
	)

	go func() {
		if err := m.healer.OnServiceRecovered(ctx, rec); err != nil {
			m.logger.ErrorContext(ctx, "recovery notification failed",
				slog.String("service", rec.ServiceName),
				slog.String("error", err.Error()),
			)
		}
	}()
}

func errServiceNotFound(name string) error {
	return &serviceNotFoundError{name: name}
}

type serviceNotFoundError struct {
	name string
}

func (e *serviceNotFoundError) Error() string {
	return "service not found: " + e.name
}

// logOnlyHealer is the default SelfHealingTrigger that only logs events.
// In production, this is replaced by an implementation that can restart
// services, reassign agents, or notify administrators.
type logOnlyHealer struct {
	logger *slog.Logger
}

func (h *logOnlyHealer) OnServiceDegraded(ctx context.Context, rec ServiceRecord) error {
	h.logger.WarnContext(ctx, "healing: service degraded (log-only)",
		slog.String("service", rec.ServiceName),
		slog.Int("missed_beats", rec.MissedBeats),
	)
	return nil
}

func (h *logOnlyHealer) OnServiceQuarantined(ctx context.Context, rec ServiceRecord) error {
	h.logger.ErrorContext(ctx, "healing: service quarantined (log-only)",
		slog.String("service", rec.ServiceName),
		slog.Int("missed_beats", rec.MissedBeats),
	)
	return nil
}

func (h *logOnlyHealer) OnServiceRecovered(ctx context.Context, rec ServiceRecord) error {
	h.logger.InfoContext(ctx, "healing: service recovered (log-only)",
		slog.String("service", rec.ServiceName),
	)
	return nil
}
