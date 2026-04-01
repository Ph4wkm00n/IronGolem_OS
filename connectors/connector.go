// Package connectors defines the connector interface and shared types for
// IronGolem OS channel connectors.
package connectors

import (
	"context"
	"time"
)

// HealthState represents the health of a connector.
type HealthState string

const (
	HealthHealthy      HealthState = "healthy"
	HealthDegraded     HealthState = "degraded"
	HealthRecovering   HealthState = "recovering"
	HealthDisconnected HealthState = "disconnected"
	HealthExpired      HealthState = "credential_expired"
)

// ConnectorType identifies the kind of connector.
type ConnectorType string

const (
	TypeEmail      ConnectorType = "email"
	TypeCalendar   ConnectorType = "calendar"
	TypeTelegram   ConnectorType = "telegram"
	TypeSlack      ConnectorType = "slack"
	TypeDiscord    ConnectorType = "discord"
	TypeWhatsApp   ConnectorType = "whatsapp"
	TypeFilesystem ConnectorType = "filesystem"
	TypeBrowser    ConnectorType = "browser"
	TypeWebhook    ConnectorType = "webhook"
	TypeFeishu     ConnectorType = "feishu"
)

// Message represents a normalized message from any connector.
type Message struct {
	ID          string            `json:"id"`
	ConnectorID string            `json:"connector_id"`
	Type        ConnectorType     `json:"connector_type"`
	Direction   Direction         `json:"direction"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// Direction indicates whether a message is inbound or outbound.
type Direction string

const (
	Inbound  Direction = "inbound"
	Outbound Direction = "outbound"
)

// Connector is the interface that all IronGolem OS connectors must implement.
type Connector interface {
	// Type returns the connector type identifier.
	Type() ConnectorType

	// Connect initializes the connector with the given configuration.
	Connect(ctx context.Context, config map[string]string) error

	// Disconnect cleanly shuts down the connector.
	Disconnect(ctx context.Context) error

	// Health returns the current health state of the connector.
	Health(ctx context.Context) HealthState

	// Send delivers an outbound message through this connector.
	Send(ctx context.Context, msg *Message) error

	// Receive returns a channel that yields inbound messages.
	Receive(ctx context.Context) (<-chan *Message, error)

	// Capabilities returns the list of capabilities this connector supports.
	Capabilities() []string
}

// ConnectorInfo describes a registered connector.
type ConnectorInfo struct {
	ID           string        `json:"id"`
	Type         ConnectorType `json:"type"`
	Health       HealthState   `json:"health"`
	ConnectedAt  *time.Time    `json:"connected_at,omitempty"`
	LastActivity *time.Time    `json:"last_activity,omitempty"`
	Capabilities []string      `json:"capabilities"`
}
