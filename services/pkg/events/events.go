// Package events defines the shared event contracts for IronGolem OS.
// These types mirror the Rust runtime event types to ensure consistent
// event sourcing across the control plane and trusted execution layers.
package events

import (
	"encoding/json"
	"time"
)

// EventKind enumerates all event types in the system.
type EventKind string

const (
	// Lifecycle events
	EventKindServiceStarted  EventKind = "service.started"
	EventKindServiceStopped  EventKind = "service.stopped"
	EventKindHeartbeat       EventKind = "service.heartbeat"

	// Message events
	EventKindMessageInbound  EventKind = "message.inbound"
	EventKindMessageOutbound EventKind = "message.outbound"
	EventKindMessageRouted   EventKind = "message.routed"
	EventKindMessageFailed   EventKind = "message.failed"

	// Connector events
	EventKindConnectorUp     EventKind = "connector.up"
	EventKindConnectorDown   EventKind = "connector.down"
	EventKindConnectorError  EventKind = "connector.error"

	// Agent events
	EventKindAgentSpawned    EventKind = "agent.spawned"
	EventKindAgentCompleted  EventKind = "agent.completed"
	EventKindAgentFailed     EventKind = "agent.failed"

	// Workflow events
	EventKindJobCreated      EventKind = "job.created"
	EventKindJobStarted      EventKind = "job.started"
	EventKindJobCompleted    EventKind = "job.completed"
	EventKindJobFailed       EventKind = "job.failed"

	// Security events
	EventKindThreatDetected  EventKind = "security.threat_detected"
	EventKindPolicyDenied    EventKind = "security.policy_denied"
	EventKindQuarantined     EventKind = "security.quarantined"
	EventKindQuarantineReleased EventKind = "security.quarantine_released"
	EventKindIncidentCreated EventKind = "security.incident_created"
	EventKindIncidentResolved EventKind = "security.incident_resolved"
	EventKindConfigRollback  EventKind = "security.config_rollback"
	EventKindCommandBlocked  EventKind = "security.command_blocked"

	// Tenancy events
	EventKindTenantCreated   EventKind = "tenancy.tenant_created"
	EventKindWorkspaceCreated EventKind = "tenancy.workspace_created"

	// Self-healing events
	EventKindHealingTriggered EventKind = "healing.triggered"
	EventKindHealingResolved  EventKind = "healing.resolved"

	// Recipe events
	EventKindRecipeActivated   EventKind = "recipe.activated"
	EventKindRecipeDeactivated EventKind = "recipe.deactivated"

	// Approval events
	EventKindApprovalRequested EventKind = "approval.requested"
	EventKindApprovalApproved  EventKind = "approval.approved"
	EventKindApprovalDenied    EventKind = "approval.denied"
	EventKindApprovalExpired   EventKind = "approval.expired"
)

// HeartbeatStatus represents the health state of a service or agent.
type HeartbeatStatus string

const (
	HeartbeatHealthy           HeartbeatStatus = "healthy"
	HeartbeatQuietlyRecovering HeartbeatStatus = "quietly_recovering"
	HeartbeatNeedsAttention    HeartbeatStatus = "needs_attention"
	HeartbeatPaused            HeartbeatStatus = "paused"
	HeartbeatQuarantined       HeartbeatStatus = "quarantined"
)

// Event is the canonical event envelope used across the entire platform.
// All autonomous actions produce events, ensuring a complete audit trail
// as required by the architecture rules.
type Event struct {
	// ID is a globally unique event identifier.
	ID string `json:"id"`

	// Kind identifies the type of event.
	Kind EventKind `json:"kind"`

	// TenantID scopes the event to a tenant for isolation.
	TenantID string `json:"tenant_id"`

	// WorkspaceID scopes the event within a tenant.
	WorkspaceID string `json:"workspace_id,omitempty"`

	// SourceService identifies the originating service.
	SourceService string `json:"source_service"`

	// CorrelationID links related events across services.
	CorrelationID string `json:"correlation_id,omitempty"`

	// CausationID points to the event that caused this one.
	CausationID string `json:"causation_id,omitempty"`

	// Timestamp records when the event was created.
	Timestamp time.Time `json:"timestamp"`

	// Payload carries the event-specific data.
	Payload json.RawMessage `json:"payload"`

	// Metadata holds additional contextual information.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Version tracks the event schema version for evolution.
	Version int `json:"version"`
}

// HeartbeatPayload is the payload for heartbeat events.
type HeartbeatPayload struct {
	ServiceName string          `json:"service_name"`
	Status      HeartbeatStatus `json:"status"`
	Uptime      time.Duration   `json:"uptime_ns"`
	Message     string          `json:"message,omitempty"`
	Metrics     map[string]any  `json:"metrics,omitempty"`
}

// MessagePayload is the payload for message events.
type MessagePayload struct {
	ChannelID   string `json:"channel_id"`
	ConnectorID string `json:"connector_id"`
	UserID      string `json:"user_id,omitempty"`
	Content     string `json:"content"`
	Direction   string `json:"direction"` // "inbound" or "outbound"
}

// ThreatPayload is the payload for security threat events.
type ThreatPayload struct {
	ThreatType  string  `json:"threat_type"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Blocked     bool    `json:"blocked"`
}

// NewEvent creates a new Event with the given kind and source, setting
// the timestamp and default version.
func NewEvent(kind EventKind, tenantID, source string, payload json.RawMessage) Event {
	return Event{
		ID:            generateID(),
		Kind:          kind,
		TenantID:      tenantID,
		SourceService: source,
		Timestamp:     time.Now().UTC(),
		Payload:       payload,
		Version:       1,
	}
}

// generateID produces a simple unique identifier. In production this would
// use a proper UUID library; here we use a timestamp-based approach.
func generateID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}
