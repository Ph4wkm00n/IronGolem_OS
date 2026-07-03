// Package runtime defines the NDJSON wire types for gateway <-> runtimed IPC.
// These types mirror runtime/core/src/ipc.rs on the Rust side exactly. Any
// change here MUST be made symmetrically in Rust or the round-trip will break.
package runtime

import (
	"encoding/json"
	"time"
)

// Kind names mirror serde's #[serde(tag = "kind")] tags in Rust.
const (
	KindExecutePlanRequest    = "execute_plan_request"
	KindExecutePlanResponse   = "execute_plan_response"
	KindEventNotification     = "event_notification"
	KindPingRequest           = "ping_request"
	KindPingResponse          = "ping_response"
	KindListProvidersRequest  = "list_providers_request"  // v0.3 Step 3
	KindListProvidersResponse = "list_providers_response" // v0.3 Step 3
	KindShutdown              = "shutdown"
)

// Envelope is the raw form of every NDJSON line. Read this first to dispatch
// on Kind, then unmarshal the same bytes into the typed message.
type Envelope struct {
	Kind string `json:"kind"`
}

// ExecutePlanRequest asks runtimed to execute a plan.
type ExecutePlanRequest struct {
	Kind        string `json:"kind"` // KindExecutePlanRequest
	RequestID   string `json:"request_id"`
	WorkspaceID string `json:"workspace_id"`
	Plan        Plan   `json:"plan"`
}

// ExecutionStatus mirrors irongolem-core::ipc::ExecutionStatus.
type ExecutionStatus string

const (
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
)

// ExecutePlanResponse is the terminal response for an ExecutePlanRequest.
type ExecutePlanResponse struct {
	Kind      string          `json:"kind"` // KindExecutePlanResponse
	RequestID string          `json:"request_id"`
	Status    ExecutionStatus `json:"status"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// EventNotification is streamed during plan execution. Each one carries the
// originating request_id so the gateway can route it.
type EventNotification struct {
	Kind      string          `json:"kind"` // KindEventNotification
	RequestID string          `json:"request_id"`
	Event     json.RawMessage `json:"event"`
}

// PingRequest / PingResponse — liveness probe.
type PingRequest struct {
	Kind      string `json:"kind"` // KindPingRequest
	RequestID string `json:"request_id"`
}

type PingResponse struct {
	Kind      string `json:"kind"` // KindPingResponse
	RequestID string `json:"request_id"`
}

// Shutdown tells runtimed to drain in-flight requests and exit.
type Shutdown struct {
	Kind      string `json:"kind"` // KindShutdown
	RequestID string `json:"request_id"`
}

// ListProvidersRequest asks runtimed to enumerate provider profiles it
// knows how to instantiate (v0.3 Step 3 of Plans/modular-puzzling-blum.md).
// The payload mirrors PingRequest — only a request_id — so wire shape
// stays minimal and the response carries everything.
type ListProvidersRequest struct {
	Kind      string `json:"kind"` // KindListProvidersRequest
	RequestID string `json:"request_id"`
}

// ListProvidersResponse carries the active provider name and every
// known profile so the settings UI can show "currently active" alongside
// available alternatives without a separate query. Profiles arrive as
// raw JSON to keep `services/pkg/runtime` free of provider-side types.
type ListProvidersResponse struct {
	Kind      string            `json:"kind"` // KindListProvidersResponse
	RequestID string            `json:"request_id"`
	Active    string            `json:"active"`
	Profiles  []json.RawMessage `json:"profiles"`
}

// Plan mirrors irongolem-core::plan::Plan.
type Plan struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	AgentID     string     `json:"agent_id"`
	Nodes       []PlanNode `json:"nodes"`
	Status      string     `json:"status"`
	Risk        Risk       `json:"risk"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// PlanNode mirrors irongolem-core::plan::PlanNode.
type PlanNode struct {
	ID           string          `json:"id"`
	Description  string          `json:"description"`
	Kind         PlanNodeKind    `json:"kind"`
	Status       string          `json:"status"`
	Dependencies []string        `json:"dependencies"`
	Risk         Risk            `json:"risk"`
	Output       json.RawMessage `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
}

// PlanNodeKind mirrors irongolem-core::plan::PlanNodeKind. Serde uses
// #[serde(tag = "type")] so the wire form is {"type": "ToolCall", ...fields}.
// We carry every variant's optional fields and only set the ones for the
// chosen Type. The Rust side rejects unknown variants on deserialize.
type PlanNodeKind struct {
	Type string `json:"type"`

	// ToolCall fields
	ToolName string          `json:"tool_name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`

	// LlmCall fields
	Prompt string `json:"prompt,omitempty"`
	Model  string `json:"model,omitempty"`

	// ApprovalGate / Delegation
	DescriptionField string `json:"description,omitempty"`
	TargetAgent      string `json:"target_agent,omitempty"`
	Goal             string `json:"goal,omitempty"`

	// Verify
	TargetNodeID string `json:"target_node_id,omitempty"`
	// Checkpoint has no fields.
}

// PlanNodeKind type discriminants — mirror Rust enum variant names.
const (
	NodeKindToolCall     = "ToolCall"
	NodeKindLlmCall      = "LlmCall"
	NodeKindApprovalGate = "ApprovalGate"
	NodeKindDelegation   = "Delegation"
	NodeKindVerify       = "Verify"
	NodeKindCheckpoint   = "Checkpoint"
)

// PlanNodeStatus values — mirror Rust's snake_case enum.
const (
	NodeStatusPending         = "pending"
	NodeStatusRunning         = "running"
	NodeStatusWaitingApproval = "waiting_approval"
	NodeStatusCompleted       = "completed"
	NodeStatusFailed          = "failed"
	NodeStatusSkipped         = "skipped"
	NodeStatusRolledBack      = "rolled_back"
)

// PlanStatus values — mirror Rust's snake_case enum.
const (
	PlanStatusPending    = "pending"
	PlanStatusRunning    = "running"
	PlanStatusPaused     = "paused"
	PlanStatusCompleted  = "completed"
	PlanStatusFailed     = "failed"
	PlanStatusRolledBack = "rolled_back"
)

// Risk mirrors irongolem-core::risk::RiskMetadata.
type Risk struct {
	Level       string   `json:"level"`
	Score       float64  `json:"score"`
	Categories  []string `json:"categories"`
	Explanation string   `json:"explanation,omitempty"`
}

// Risk level values — Rust serializes RiskLevel with rename_all = "lowercase".
const (
	RiskLevelNone     = "none"
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

// DefaultRisk returns the zero-risk metadata that matches RiskMetadata::default().
func DefaultRisk() Risk {
	return Risk{Level: RiskLevelNone, Score: 0.0, Categories: []string{}}
}

// RuntimeEvent is the subset of irongolem-core::event::Event fields the
// gateway needs to read. The `Kind` field is a tagged union — Rust uses
// #[serde(tag = "type", content = "data")], so wire form is
// {"type": "PlanCompleted", "data": {...}}.
type RuntimeEvent struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	WorkspaceID   string    `json:"workspace_id"`
	UserID        string    `json:"user_id,omitempty"`
	AgentID       string    `json:"agent_id,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
	ChannelID     string    `json:"channel_id,omitempty"`
	Kind          EventKind `json:"kind"`
	ParentEventID string    `json:"parent_event_id,omitempty"`
}

// EventKind carries the discriminator and untyped data; the gateway can
// inspect Type without needing to decode Data unless it cares.
type EventKind struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Event type names mirror irongolem-core::event::EventKind variants.
const (
	EventPlanCreated       = "PlanCreated"
	EventPlanStepStarted   = "PlanStepStarted"
	EventPlanStepCompleted = "PlanStepCompleted"
	EventPlanStepFailed    = "PlanStepFailed"
	EventPlanCompleted     = "PlanCompleted"
	EventPlanRolledBack    = "PlanRolledBack"
	EventToolCalled        = "ToolCalled"
	EventToolResult        = "ToolResult"
	EventCheckpointCreated = "CheckpointCreated"
)
