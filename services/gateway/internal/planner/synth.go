// Package planner turns inbound messages into runtime-executable plans.
//
// v0.1 plan synthesis is deliberately deterministic, not agentic. Every
// inbound message produces the same plan shape: one LlmCall node whose
// prompt is the operator system prompt concatenated with the user's
// message. This keeps the gateway → runtimed contract simple, makes the
// behavior trivially reproducible, and gives a clean seam for v0.2 to
// swap in real planning (Plan-Verifier-Executor loops, recipe-driven
// synthesis, etc.).
package planner

import (
	"time"

	"github.com/google/uuid"

	ipc "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/runtime"
)

// SystemPrompt is the operator persona for v0.1. Kept short and explicit so
// the round-trip stays predictable in tests and the mock provider can
// match against a stable prefix. Tunable via SynthesizePlan options if a
// future caller needs per-channel personas.
const SystemPrompt = `You are IronGolem, a self-hosted autonomous assistant.
Answer clearly and briefly. If you don't know something, say so.`

// InboundMessage is the gateway-internal shape passed to the planner.
// Mirrors the fields the planner actually needs from connectors.Message —
// the gateway and connectors modules don't share types directly, so this
// is the seam.
type InboundMessage struct {
	// ConnectorID identifies the source connector (telegram, email, …).
	ConnectorID string
	// ChannelID is the per-connector channel/thread identifier.
	ChannelID string
	// UserID identifies the end-user that sent the message, if known.
	UserID string
	// Content is the verbatim message body the user sent.
	Content string
	// TenantID scopes the plan to a tenant for multi-tenant deployments.
	TenantID string
	// WorkspaceID maps to runtimed::WorkspaceId; if empty, callers should
	// supply one before sending to the runtime.
	WorkspaceID string
}

// Options tweak SynthesizePlan output without changing its 1-node shape.
type Options struct {
	// SystemPromptOverride replaces SystemPrompt when non-empty.
	SystemPromptOverride string
	// PlanDescription overrides the default plan description.
	PlanDescription string
	// AgentID is the runtime agent identity recorded on the plan. Defaults
	// to a per-call random UUID — fine for v0.1 single-agent flows, but
	// real squads will pass a stable id here.
	AgentID string
	// Now overrides time.Now() so tests can assert deterministic timestamps.
	Now func() time.Time
}

// SynthesizePlan produces the canonical 1-node LlmCall plan for an
// inbound message. The plan is ready to send to runtimed via the runtime
// client's Execute method.
func SynthesizePlan(msg InboundMessage, opts Options) ipc.Plan {
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	prompt := buildPrompt(opts.SystemPromptOverride, msg.Content)
	desc := opts.PlanDescription
	if desc == "" {
		desc = "v0.1 deterministic synth — single LlmCall for inbound message"
	}
	agentID := opts.AgentID
	if agentID == "" {
		agentID = uuid.NewString()
	}

	createdAt := now().UTC()

	plan := ipc.Plan{
		ID:          uuid.NewString(),
		Description: desc,
		AgentID:     agentID,
		Status:      ipc.PlanStatusPending,
		Risk:        ipc.DefaultRisk(),
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		Nodes: []ipc.PlanNode{
			{
				ID:           uuid.NewString(),
				Description:  "Respond to user message",
				Status:       ipc.NodeStatusPending,
				Dependencies: []string{},
				Risk:         ipc.DefaultRisk(),
				Kind: ipc.PlanNodeKind{
					Type:   ipc.NodeKindLlmCall,
					Prompt: prompt,
					// Model is intentionally omitted so the runtime picks
					// its default provider. Per-message model selection is
					// a v0.2 concern.
				},
			},
		},
	}
	return plan
}

// buildPrompt concatenates the system prompt and user content with a
// blank line so the model sees them as separate sections. v0.2 will
// move this into a proper messages array once the LlmCall wire shape
// supports it.
func buildPrompt(systemPromptOverride, userContent string) string {
	sys := systemPromptOverride
	if sys == "" {
		sys = SystemPrompt
	}
	return sys + "\n\nUser: " + userContent
}
