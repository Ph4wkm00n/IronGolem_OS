// Package connector manages the lifecycle and health of external connectors
// (email, Slack, Telegram, calendar, etc.) within the Gateway service.
//
// The ConnectorManager tracks each connector's health state and detects
// missed heartbeats to trigger degradation or disconnection.
package connector

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Health represents the runtime health state of a connector.
type Health string

const (
	HealthHealthy      Health = "healthy"
	HealthDegraded     Health = "degraded"
	HealthRecovering   Health = "recovering"
	HealthDisconnected Health = "disconnected"
)

// HeartbeatTimeout is the maximum time between heartbeats before a connector
// is marked degraded.
const HeartbeatTimeout = 30 * time.Second

// DegradedTimeout is how long a connector can stay degraded before it is
// marked disconnected.
const DegradedTimeout = 2 * time.Minute

// ConnectorState holds the runtime state for a single connector.
type ConnectorState struct {
	ID            string    `json:"id"`
	Health        Health    `json:"health"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	MissedBeats   int       `json:"missed_beats"`
	Message       string    `json:"message,omitempty"`
}

// Manager tracks connector health, handles heartbeats, and supports the
// connect/disconnect lifecycle.
type Manager struct {
	mu     sync.RWMutex
	states map[string]*ConnectorState
	logger *slog.Logger
	stopCh chan struct{}
}

// NewManager creates a ConnectorManager and starts the background health
// checker goroutine.
func NewManager(logger *slog.Logger) *Manager {
	m := &Manager{
		states: make(map[string]*ConnectorState),
		logger: logger,
		stopCh: make(chan struct{}),
	}
	go m.healthCheckLoop()
	return m
}

// Connect registers a connector or transitions it back to healthy if it
// was previously disconnected.
func (m *Manager) Connect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	if existing, ok := m.states[id]; ok {
		existing.Health = HealthHealthy
		existing.LastHeartbeat = now
		existing.MissedBeats = 0
		existing.Message = "reconnected"
		m.logger.Info("connector reconnected", slog.String("connector_id", id))
		return
	}

	m.states[id] = &ConnectorState{
		ID:            id,
		Health:        HealthHealthy,
		ConnectedAt:   now,
		LastHeartbeat: now,
		MissedBeats:   0,
	}
	m.logger.Info("connector registered", slog.String("connector_id", id))
}

// Disconnect marks a connector as disconnected.
func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.states[id]
	if !ok {
		return errors.New("connector not found")
	}

	state.Health = HealthDisconnected
	state.Message = "gracefully disconnected"
	m.logger.Info("connector disconnected", slog.String("connector_id", id))
	return nil
}

// DisconnectAll marks every connector as disconnected. Called during
// graceful shutdown.
func (m *Manager) DisconnectAll() {
	close(m.stopCh)

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, state := range m.states {
		state.Health = HealthDisconnected
		state.Message = "service shutdown"
		m.logger.Info("connector disconnected on shutdown", slog.String("connector_id", id))
	}
}

// RecordHeartbeat updates the last heartbeat time for a connector and
// resets its missed-beat counter.
func (m *Manager) RecordHeartbeat(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.states[id]
	if !ok {
		return errors.New("connector not found")
	}

	if state.Health == HealthDisconnected {
		return errors.New("connector is disconnected; reconnect first")
	}

	wasRecovering := state.Health == HealthDegraded || state.Health == HealthRecovering
	state.LastHeartbeat = time.Now().UTC()
	state.MissedBeats = 0

	if wasRecovering {
		state.Health = HealthRecovering
		state.Message = "recovering after missed heartbeats"
		m.logger.Info("connector recovering",
			slog.String("connector_id", id),
		)
	} else {
		state.Health = HealthHealthy
		state.Message = ""
	}

	return nil
}

// PromoteRecovering transitions a connector from Recovering to Healthy
// after it has sent consecutive heartbeats without interruption. Called
// by the health check loop.
func (m *Manager) promoteRecovering(id string, state *ConnectorState) {
	if state.Health == HealthRecovering && state.MissedBeats == 0 {
		elapsed := time.Since(state.LastHeartbeat)
		if elapsed < HeartbeatTimeout {
			state.Health = HealthHealthy
			state.Message = ""
			m.logger.Info("connector promoted to healthy",
				slog.String("connector_id", id),
			)
		}
	}
}

// Status returns the current state of a connector.
func (m *Manager) Status(id string) (ConnectorState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.states[id]
	if !ok {
		return ConnectorState{}, false
	}
	return *state, true
}

// List returns a snapshot of all connector states.
func (m *Manager) List() []ConnectorState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ConnectorState, 0, len(m.states))
	for _, s := range m.states {
		result = append(result, *s)
	}
	return result
}

// healthCheckLoop runs periodically to detect missed heartbeats and
// transition connector health states.
func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.evaluateHealth()
		}
	}
}

// evaluateHealth checks every connector's last heartbeat time and updates
// health states accordingly.
func (m *Manager) evaluateHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	for id, state := range m.states {
		if state.Health == HealthDisconnected {
			continue
		}

		elapsed := now.Sub(state.LastHeartbeat)

		switch {
		case elapsed > DegradedTimeout:
			if state.Health != HealthDisconnected {
				state.Health = HealthDisconnected
				state.Message = "disconnected due to prolonged heartbeat absence"
				m.logger.Warn("connector auto-disconnected",
					slog.String("connector_id", id),
					slog.Duration("silence", elapsed),
				)
			}
		case elapsed > HeartbeatTimeout:
			state.MissedBeats++
			if state.Health == HealthHealthy {
				state.Health = HealthDegraded
				state.Message = "missed heartbeat"
				m.logger.Warn("connector degraded",
					slog.String("connector_id", id),
					slog.Int("missed_beats", state.MissedBeats),
				)
			}
		default:
			m.promoteRecovering(id, state)
		}
	}
}
