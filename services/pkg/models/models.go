// Package models defines the core domain types for IronGolem OS.
// These models represent the isolation boundary hierarchy:
//
//	Tenant -> Workspace -> Channel -> User
//
// Every entity is scoped to a tenant to enforce multi-tenant isolation.
package models

import "time"

// DeploymentMode determines the storage backend and isolation strategy.
type DeploymentMode string

const (
	// DeploymentSolo uses local SQLite for a single user.
	DeploymentSolo DeploymentMode = "solo"

	// DeploymentHousehold uses shared SQLite for a small group.
	DeploymentHousehold DeploymentMode = "household"

	// DeploymentTeam uses PostgreSQL with per-workspace isolation.
	DeploymentTeam DeploymentMode = "team"
)

// AgentRole identifies the function an agent performs within a squad.
type AgentRole string

const (
	AgentRolePlanner    AgentRole = "planner"
	AgentRoleExecutor   AgentRole = "executor"
	AgentRoleVerifier   AgentRole = "verifier"
	AgentRoleResearcher AgentRole = "researcher"
	AgentRoleDefender   AgentRole = "defender"
	AgentRoleHealer     AgentRole = "healer"
	AgentRoleOptimizer  AgentRole = "optimizer"
	AgentRoleNarrator   AgentRole = "narrator"
	AgentRoleRouter     AgentRole = "router"
)

// SquadKind identifies a pre-composed multi-agent team.
type SquadKind string

const (
	SquadInbox              SquadKind = "inbox"
	SquadResearch           SquadKind = "research"
	SquadOps                SquadKind = "ops"
	SquadSecurity           SquadKind = "security"
	SquadExecutiveAssistant SquadKind = "executive_assistant"
)

// Tenant is the top-level isolation boundary. All resources belong to a tenant.
type Tenant struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	DeploymentMode DeploymentMode `json:"deployment_mode"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Disabled       bool           `json:"disabled"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Workspace is an isolated environment within a tenant. In Team mode each
// workspace gets its own database schema for full data isolation.
type Workspace struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Disabled  bool      `json:"disabled"`
}

// UserRole controls what a user can do within a workspace.
type UserRole string

const (
	UserRoleOwner  UserRole = "owner"
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
	UserRoleViewer UserRole = "viewer"
)

// User represents a human operator within a workspace.
type User struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	WorkspaceID string    `json:"workspace_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        UserRole  `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	Disabled    bool      `json:"disabled"`
}

// ChannelKind represents a communication channel type.
type ChannelKind string

const (
	ChannelEmail    ChannelKind = "email"
	ChannelSlack    ChannelKind = "slack"
	ChannelTelegram ChannelKind = "telegram"
	ChannelCalendar ChannelKind = "calendar"
	ChannelWeb      ChannelKind = "web"
)

// Channel is a communication endpoint within a workspace. Channels are
// subject to per-channel policy restrictions (layer 4 of the security model).
type Channel struct {
	ID          string      `json:"id"`
	TenantID    string      `json:"tenant_id"`
	WorkspaceID string      `json:"workspace_id"`
	Kind        ChannelKind `json:"kind"`
	Name        string      `json:"name"`
	ConnectorID string      `json:"connector_id"`
	Config      map[string]string `json:"config,omitempty"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
}

// ConnectorStatus represents the runtime state of a connector.
type ConnectorStatus string

const (
	ConnectorStatusConnected    ConnectorStatus = "connected"
	ConnectorStatusDisconnected ConnectorStatus = "disconnected"
	ConnectorStatusDegraded     ConnectorStatus = "degraded"
	ConnectorStatusError        ConnectorStatus = "error"
)

// Connector represents an external service integration (email provider,
// Slack workspace, Telegram bot, etc.).
type Connector struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id"`
	WorkspaceID string          `json:"workspace_id"`
	Kind        ChannelKind     `json:"kind"`
	Name        string          `json:"name"`
	Status      ConnectorStatus `json:"status"`
	Config      map[string]string `json:"config,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	LastPingAt  time.Time       `json:"last_ping_at"`
}

// RecipeStatus represents the lifecycle state of a recipe.
type RecipeStatus string

const (
	RecipeStatusDraft    RecipeStatus = "draft"
	RecipeStatusActive   RecipeStatus = "active"
	RecipeStatusPaused   RecipeStatus = "paused"
	RecipeStatusArchived RecipeStatus = "archived"
)

// Recipe is a user-facing automation template with a safety summary.
// Recipes are the primary way non-technical users interact with the platform.
type Recipe struct {
	ID            string       `json:"id"`
	TenantID      string       `json:"tenant_id"`
	WorkspaceID   string       `json:"workspace_id"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	SafetySummary string       `json:"safety_summary"`
	SquadKind     SquadKind    `json:"squad_kind"`
	Status        RecipeStatus `json:"status"`
	TriggerConfig map[string]any `json:"trigger_config,omitempty"`
	CreatedBy     string       `json:"created_by"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}
