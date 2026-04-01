// Package internal implements the heartbeat manager for the IronGolem OS
// Health service.
//
// The heartbeat manager tracks service health through periodic check-ins,
// detects missed heartbeats, and triggers self-healing actions. Services
// transition through the following states:
//
//	Healthy -> QuietlyRecovering -> NeedsAttention -> Paused -> Quarantined
//
// Each transition can trigger automated recovery or escalation.
package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ServiceState represents the health state of a monitored service, matching
// the HeartbeatStatus values from the events package.
type ServiceState string

const (
	StateHealthy           ServiceState = "healthy"
	StateQuietlyRecovering ServiceState = "quietly_recovering"
	StateNeedsAttention    ServiceState = "needs_attention"
	StatePaused            ServiceState = "paused"
	StateQuarantined       ServiceState = "quarantined"
)

// Thresholds for state transitions.
const (
	// HeartbeatInterval is the expected time between heartbeats.
	HeartbeatInterval = 15 * time.Second

	// MissedThresholdRecovering is how many missed heartbeats before moving
	// from Healthy to QuietlyRecovering.
	MissedThresholdRecovering = 2

	// MissedThresholdAttention is the threshold for NeedsAttention.
	MissedThresholdAttention = 5

	// MissedThresholdQuarantine is the threshold for Quarantined.
	MissedThresholdQuarantine = 10
)

// HealingAction represents an automated recovery action.
type HealingAction string

const (
	HealingRestart       HealingAction = "restart"
	HealingScaleUp       HealingAction = "scale_up"
	HealingFailover      HealingAction = "failover"
	HealingNotifyAdmin   HealingAction = "notify_admin"
	HealingQuarantine    HealingAction = "quarantine"
)

// ServiceRecord holds the tracked state for a single monitored service.
type ServiceRecord struct {
	Name            string        `json:"name"`
	State           ServiceState  `json:"state"`
	LastHeartbeat   time.Time     `json:"last_heartbeat"`
	MissedBeats     int           `json:"missed_beats"`
	ConsecutiveOK   int           `json:"consecutive_ok"`
	RegisteredAt    time.Time     `json:"registered_at"`
	Message         string        `json:"message,omitempty"`
	HealingActions  []HealingLog  `json:"healing_actions,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// HealingLog records an automated healing action taken on a service.
type HealingLog struct {
	Action    HealingAction `json:"action"`
	Timestamp time.Time     `json:"timestamp"`
	Reason    string        `json:"reason"`
	Success   bool          `json:"success"`
}

// HeartbeatRequest is the payload services send to report their health.
type HeartbeatRequest struct {
	ServiceName string         `json:"service_name"`
	Status      string         `json:"status"`
	Uptime      time.Duration  `json:"uptime_ns"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	Message     string         `json:"message,omitempty"`
}

// HeartbeatManager tracks service health, detects missed heartbeats, and
// triggers self-healing actions.
type HeartbeatManager struct {
	mu       sync.RWMutex
	services map[string]*ServiceRecord
	logger   *slog.Logger
}

// NewHeartbeatManager creates a new HeartbeatManager.
func NewHeartbeatManager(logger *slog.Logger) *HeartbeatManager {
	return &HeartbeatManager{
		services: make(map[string]*ServiceRecord),
		logger:   logger,
	}
}

// RecordHeartbeat processes a heartbeat from a service, registering it if
// new or updating its state if known.
func (m *HeartbeatManager) RecordHeartbeat(req HeartbeatRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	rec, exists := m.services[req.ServiceName]
	if !exists {
		rec = &ServiceRecord{
			Name:         req.ServiceName,
			State:        StateHealthy,
			RegisteredAt: now,
			Metadata:     make(map[string]any),
		}
		m.services[req.ServiceName] = rec
		m.logger.Info("service registered",
			slog.String("service", req.ServiceName),
		)
	}

	rec.LastHeartbeat = now
	rec.MissedBeats = 0
	rec.ConsecutiveOK++
	rec.Metadata = req.Metrics

	// Transition logic: recovering services need consecutive OKs to go healthy.
	switch rec.State {
	case StateQuietlyRecovering:
		if rec.ConsecutiveOK >= 3 {
			rec.State = StateHealthy
			rec.Message = "recovered"
			m.logger.Info("service recovered",
				slog.String("service", req.ServiceName),
			)
		}
	case StateNeedsAttention:
		rec.State = StateQuietlyRecovering
		rec.ConsecutiveOK = 1
		rec.Message = "heartbeat resumed, recovering"
		m.logger.Info("service starting recovery",
			slog.String("service", req.ServiceName),
		)
	case StatePaused:
		// Paused services need to be explicitly resumed.
	case StateQuarantined:
		// Quarantined services need admin intervention.
	default:
		rec.State = StateHealthy
		rec.Message = ""
	}
}

// GetService returns the current record for a service.
func (m *HeartbeatManager) GetService(name string) (*ServiceRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.services[name]
	if !ok {
		return nil, false
	}
	cp := *rec
	return &cp, true
}

// ListServices returns a snapshot of all monitored services.
func (m *HeartbeatManager) ListServices() []ServiceRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ServiceRecord, 0, len(m.services))
	for _, r := range m.services {
		result = append(result, *r)
	}
	return result
}

// PauseService transitions a service to the Paused state. Paused services
// are excluded from healing actions and alerting.
func (m *HeartbeatManager) PauseService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.services[name]
	if !ok {
		return errServiceNotFound
	}

	rec.State = StatePaused
	rec.Message = "paused by operator"
	m.logger.Info("service paused", slog.String("service", name))
	return nil
}

// ResumeService transitions a service out of the Paused state back to
// QuietlyRecovering so it can prove health before being marked Healthy.
func (m *HeartbeatManager) ResumeService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.services[name]
	if !ok {
		return errServiceNotFound
	}

	if rec.State != StatePaused {
		return errNotPaused
	}

	rec.State = StateQuietlyRecovering
	rec.ConsecutiveOK = 0
	rec.Message = "resumed, waiting for heartbeats"
	m.logger.Info("service resumed", slog.String("service", name))
	return nil
}

var (
	errServiceNotFound = &serviceError{msg: "service not found", code: http.StatusNotFound}
	errNotPaused       = &serviceError{msg: "service is not paused", code: http.StatusConflict}
)

type serviceError struct {
	msg  string
	code int
}

func (e *serviceError) Error() string { return e.msg }
func (e *serviceError) HTTPCode() int { return e.code }

// Run starts the background loop that checks for missed heartbeats and
// triggers state transitions and healing actions.
func (m *HeartbeatManager) Run(ctx context.Context) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	m.logger.Info("heartbeat monitor started",
		slog.Duration("interval", HeartbeatInterval),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.evaluate(now.UTC())
		}
	}
}

// evaluate checks all services for missed heartbeats and transitions
// their states accordingly, triggering healing actions when needed.
func (m *HeartbeatManager) evaluate(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, rec := range m.services {
		if rec.State == StatePaused || rec.State == StateQuarantined {
			continue
		}

		elapsed := now.Sub(rec.LastHeartbeat)
		expectedBeats := int(elapsed / HeartbeatInterval)

		if expectedBeats <= 0 {
			continue
		}

		rec.MissedBeats = expectedBeats
		rec.ConsecutiveOK = 0

		switch {
		case rec.MissedBeats >= MissedThresholdQuarantine:
			if rec.State != StateQuarantined {
				rec.State = StateQuarantined
				rec.Message = "quarantined: too many missed heartbeats"
				m.triggerHealing(rec, HealingQuarantine, "exceeded quarantine threshold")
				m.logger.Error("service quarantined",
					slog.String("service", name),
					slog.Int("missed_beats", rec.MissedBeats),
				)
			}

		case rec.MissedBeats >= MissedThresholdAttention:
			if rec.State != StateNeedsAttention {
				rec.State = StateNeedsAttention
				rec.Message = "needs attention: multiple missed heartbeats"
				m.triggerHealing(rec, HealingNotifyAdmin, "exceeded attention threshold")
				m.triggerHealing(rec, HealingRestart, "attempting automatic restart")
				m.logger.Warn("service needs attention",
					slog.String("service", name),
					slog.Int("missed_beats", rec.MissedBeats),
				)
			}

		case rec.MissedBeats >= MissedThresholdRecovering:
			if rec.State == StateHealthy {
				rec.State = StateQuietlyRecovering
				rec.Message = "recovering: heartbeat delayed"
				m.logger.Info("service entering quiet recovery",
					slog.String("service", name),
					slog.Int("missed_beats", rec.MissedBeats),
				)
			}
		}
	}
}

// triggerHealing records a healing action. In production this would
// actually invoke the action (restart container, notify admin, etc.).
func (m *HeartbeatManager) triggerHealing(rec *ServiceRecord, action HealingAction, reason string) {
	entry := HealingLog{
		Action:    action,
		Timestamp: time.Now().UTC(),
		Reason:    reason,
		Success:   true, // Placeholder; real impl would track outcome.
	}
	rec.HealingActions = append(rec.HealingActions, entry)

	m.logger.Info("healing action triggered",
		slog.String("service", rec.Name),
		slog.String("action", string(action)),
		slog.String("reason", reason),
	)
}

// --- HTTP Handlers ---

// Handler provides HTTP handlers for the health service API.
type Handler struct {
	logger *slog.Logger
	mgr    *HeartbeatManager
}

// NewHandler creates a new Handler.
func NewHandler(logger *slog.Logger, mgr *HeartbeatManager) *Handler {
	return &Handler{logger: logger, mgr: mgr}
}

// HealthCheck responds with the service health status.
func (h *Handler) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "health",
		"time":    time.Now().UTC(),
	})
}

// RecordHeartbeat handles POST /api/v1/heartbeats.
func (h *Handler) RecordHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if req.ServiceName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "service_name is required",
		})
		return
	}

	h.mgr.RecordHeartbeat(req)

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// ListServices handles GET /api/v1/services.
func (h *Handler) ListServices(w http.ResponseWriter, _ *http.Request) {
	services := h.mgr.ListServices()
	writeJSON(w, http.StatusOK, map[string]any{
		"services": services,
		"count":    len(services),
	})
}

// ServiceStatus handles GET /api/v1/services/{name}/status.
func (h *Handler) ServiceStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	rec, ok := h.mgr.GetService(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "service not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// PauseService handles POST /api/v1/services/{name}/pause.
func (h *Handler) PauseService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.mgr.PauseService(name); err != nil {
		code := http.StatusInternalServerError
		if se, ok := err.(*serviceError); ok {
			code = se.HTTPCode()
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"service": name,
		"state":   string(StatePaused),
	})
}

// ResumeService handles POST /api/v1/services/{name}/resume.
func (h *Handler) ResumeService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.mgr.ResumeService(name); err != nil {
		code := http.StatusInternalServerError
		if se, ok := err.(*serviceError); ok {
			code = se.HTTPCode()
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"service": name,
		"state":   string(StateQuietlyRecovering),
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
