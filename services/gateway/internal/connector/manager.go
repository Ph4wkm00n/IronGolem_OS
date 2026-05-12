// Package connector manages the lifecycle and health of external connectors
// (email, Slack, Telegram, calendar, etc.) within the Gateway service.
//
// The ConnectorManager tracks each connector's health state and detects
// missed heartbeats to trigger degradation or disconnection. It also owns
// the inbound-message pump: each registered InboundSource gets a goroutine
// that drains its Receive(ctx) channel into the gateway's inbound handler.
package connector

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// InboundMessage is the gateway-internal shape pumped from a connector
// into the inbound handler. Kept independent of the connectors module so
// the gateway has no module dependency on it.
type InboundMessage struct {
	// ConnectorID identifies the source connector instance.
	ConnectorID string
	// ChannelID is the per-connector channel/thread/chat identifier.
	ChannelID string
	// UserID identifies the sending user, if known.
	UserID string
	// Content is the verbatim message body.
	Content string
	// TenantID scopes the message for multi-tenant routing.
	TenantID string
	// WorkspaceID maps to runtimed::WorkspaceId.
	WorkspaceID string
	// Metadata is opaque per-connector context (sender phone, message id,
	// etc.). Optional; survives to the audit trail unchanged.
	Metadata map[string]string
}

// OutboundMessage is the reply shape the pump hands back to the connector's
// Send method once the runtime returns a response.
type OutboundMessage struct {
	ChannelID string
	Content   string
	Metadata  map[string]string
}

// InboundSource is what the manager pumps from. Real connector adapters
// in connectors/<name>/ implement this with a small wrapper around
// connectors.Connector. Test code uses an in-memory channel.
type InboundSource interface {
	// Receive starts (or returns the already-running) inbound channel.
	// The returned channel must close when ctx is cancelled.
	Receive(ctx context.Context) (<-chan InboundMessage, error)
	// Send delivers a reply back to the originating channel.
	Send(ctx context.Context, msg OutboundMessage) error
}

// InboundHandler is the callback the pump invokes per received message.
// It returns the reply that should flow back through the source's Send
// (empty string = no reply). The runtime client + planner sit behind it.
type InboundHandler func(ctx context.Context, msg InboundMessage) (reply string, err error)

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

// Manager tracks connector health, handles heartbeats, supports the
// connect/disconnect lifecycle, and pumps inbound messages from registered
// InboundSources into the gateway's handler.
type Manager struct {
	mu     sync.RWMutex
	states map[string]*ConnectorState
	logger *slog.Logger
	stopCh chan struct{}

	// inboundHandler is the callback the pumps fire per received message.
	// Set by SetInboundHandler — nil until wired up at boot.
	inboundHandler InboundHandler

	// sources holds registered InboundSources keyed by connector id, along
	// with their pump goroutine context so we can cancel on Disconnect /
	// DisconnectAll.
	sources map[string]*sourceState

	// pumpWG tracks running pump goroutines so shutdown can wait for them
	// to drain cleanly.
	pumpWG sync.WaitGroup
}

type sourceState struct {
	src    InboundSource
	cancel context.CancelFunc
}

// NewManager creates a ConnectorManager and starts the background health
// checker goroutine.
func NewManager(logger *slog.Logger) *Manager {
	m := &Manager{
		states:  make(map[string]*ConnectorState),
		sources: make(map[string]*sourceState),
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
	go m.healthCheckLoop()
	return m
}

// SetInboundHandler installs the callback the pump invokes per received
// message. Idempotent; subsequent calls replace the handler. Must be set
// before RegisterSource is called for any pump to deliver messages.
func (m *Manager) SetInboundHandler(h InboundHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inboundHandler = h
}

// RegisterSource starts a pump goroutine that drains src.Receive into the
// inbound handler. Registers the connector in the health table if it
// wasn't already present. Returns the spawned pump's cancel function for
// callers that want fine-grained control; the manager also tracks it for
// DisconnectAll.
func (m *Manager) RegisterSource(id string, src InboundSource) error {
	if src == nil {
		return errors.New("connector: nil source")
	}

	m.mu.Lock()
	if _, dup := m.sources[id]; dup {
		m.mu.Unlock()
		return errors.New("connector: source already registered")
	}
	handler := m.inboundHandler
	m.mu.Unlock()

	if handler == nil {
		return errors.New("connector: inbound handler not set; call SetInboundHandler first")
	}

	// Ensure the health state exists; idempotent if Connect was already called.
	m.Connect(id)

	pumpCtx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	m.sources[id] = &sourceState{src: src, cancel: cancel}
	m.mu.Unlock()

	m.pumpWG.Add(1)
	go m.runPump(pumpCtx, id, src, handler)
	return nil
}

// runPump drives one connector's Receive channel into the inbound handler
// for the lifetime of pumpCtx. On Receive error the pump logs and exits —
// the connector will surface the failure via its health state.
func (m *Manager) runPump(ctx context.Context, id string, src InboundSource, handler InboundHandler) {
	defer m.pumpWG.Done()

	ch, err := src.Receive(ctx)
	if err != nil {
		m.logger.Error("connector receive open failed",
			slog.String("connector_id", id),
			slog.String("error", err.Error()),
		)
		return
	}
	m.logger.Info("inbound pump started", slog.String("connector_id", id))

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				m.logger.Info("inbound pump channel closed", slog.String("connector_id", id))
				return
			}
			// Treat the inbound message as an activity heartbeat too —
			// otherwise a healthy producer can drift to degraded.
			_ = m.RecordHeartbeat(id)

			reply, err := handler(ctx, msg)
			if err != nil {
				m.logger.Warn("inbound handler error",
					slog.String("connector_id", id),
					slog.String("error", err.Error()),
				)
				continue
			}
			if reply == "" {
				continue
			}
			outbound := OutboundMessage{
				ChannelID: msg.ChannelID,
				Content:   reply,
				Metadata:  msg.Metadata,
			}
			// Cap outbound send so a wedged Send can't pin the pump.
			sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			if err := src.Send(sendCtx, outbound); err != nil {
				m.logger.Warn("connector send failed",
					slog.String("connector_id", id),
					slog.String("error", err.Error()),
				)
			}
			cancel()

		case <-ctx.Done():
			m.logger.Info("inbound pump stopped", slog.String("connector_id", id))
			return
		}
	}
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

// Disconnect marks a connector as disconnected and stops its inbound pump
// (if one was registered).
func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	state, ok := m.states[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("connector not found")
	}
	state.Health = HealthDisconnected
	state.Message = "gracefully disconnected"
	pump, hasPump := m.sources[id]
	if hasPump {
		delete(m.sources, id)
	}
	m.mu.Unlock()

	if hasPump {
		pump.cancel()
	}
	m.logger.Info("connector disconnected", slog.String("connector_id", id))
	return nil
}

// DisconnectAll marks every connector as disconnected and tears down every
// inbound pump. Called during graceful shutdown.
func (m *Manager) DisconnectAll() {
	close(m.stopCh)

	m.mu.Lock()
	pumps := make([]*sourceState, 0, len(m.sources))
	for _, s := range m.sources {
		pumps = append(pumps, s)
	}
	m.sources = make(map[string]*sourceState)
	for id, state := range m.states {
		state.Health = HealthDisconnected
		state.Message = "service shutdown"
		m.logger.Info("connector disconnected on shutdown", slog.String("connector_id", id))
	}
	m.mu.Unlock()

	for _, p := range pumps {
		p.cancel()
	}
	m.pumpWG.Wait()
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
