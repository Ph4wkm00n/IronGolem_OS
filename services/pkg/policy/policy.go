// Package policy implements the five-layer security model for IronGolem OS.
//
// Every autonomous action passes through five policy layers in order:
//
//  1. Gateway Identity - authenticates the request source
//  2. Global Tool Policy - enforces system-wide tool restrictions
//  3. Per-Agent Permissions - checks what the acting agent is allowed to do
//  4. Per-Channel Restrictions - applies channel-specific rules
//  5. Admin-Only Controls - enforces admin overrides and emergency stops
//
// The default engine short-circuits on the first denial. All decisions
// are logged for the audit trail.
package policy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// PolicyLayer identifies which security layer produced a decision.
type PolicyLayer int

const (
	LayerGatewayIdentity    PolicyLayer = iota + 1 // Layer 1
	LayerGlobalToolPolicy                           // Layer 2
	LayerPerAgentPermission                         // Layer 3
	LayerPerChannelRestrict                         // Layer 4
	LayerAdminControls                              // Layer 5
)

// String returns a human-readable name for the policy layer.
func (l PolicyLayer) String() string {
	switch l {
	case LayerGatewayIdentity:
		return "gateway_identity"
	case LayerGlobalToolPolicy:
		return "global_tool_policy"
	case LayerPerAgentPermission:
		return "per_agent_permission"
	case LayerPerChannelRestrict:
		return "per_channel_restriction"
	case LayerAdminControls:
		return "admin_controls"
	default:
		return "unknown"
	}
}

// Decision is the outcome of a policy evaluation.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
	DecisionAudit Decision = "audit" // allow but flag for review
)

// Permission describes a single capability an entity may or may not have.
type Permission struct {
	Resource string `json:"resource"` // e.g. "connector.email", "tool.web_search"
	Action   string `json:"action"`   // e.g. "read", "write", "execute"
}

// String returns a human-readable representation.
func (p Permission) String() string {
	return p.Resource + ":" + p.Action
}

// EvalRequest bundles all the context needed for a policy evaluation.
type EvalRequest struct {
	TenantID    string
	WorkspaceID string
	UserID      string
	AgentRole   string
	ChannelID   string
	Permission  Permission
	Metadata    map[string]string
}

// EvalResult captures the final decision and which layer produced it.
type EvalResult struct {
	Decision  Decision    `json:"decision"`
	Layer     PolicyLayer `json:"layer"`
	Reason    string      `json:"reason"`
	Timestamp time.Time   `json:"timestamp"`
}

// PolicyEngine evaluates an action request against all five security layers.
type PolicyEngine interface {
	// Evaluate checks the request against all policy layers in order.
	// It returns the result from the first layer that denies, or an allow
	// if all layers pass.
	Evaluate(ctx context.Context, req EvalRequest) (EvalResult, error)
}

// LayerChecker evaluates a single policy layer. Implementations are
// pluggable so each layer can be configured independently.
type LayerChecker interface {
	Layer() PolicyLayer
	Check(ctx context.Context, req EvalRequest) (Decision, string, error)
}

// DefaultPolicyEngine evaluates all five layers sequentially, short-circuiting
// on the first denial.
type DefaultPolicyEngine struct {
	layers []LayerChecker
	logger *slog.Logger
}

// NewDefaultPolicyEngine creates an engine with the standard five layers
// and Layer 4 in its v0.1 disabled-with-reason mode (no store). Use
// NewDefaultPolicyEngineWithStore to plug a real per-channel policy
// store in once one is available.
//
// Pass nil for logger to use the default.
func NewDefaultPolicyEngine(logger *slog.Logger) *DefaultPolicyEngine {
	return NewDefaultPolicyEngineWithStore(logger, nil)
}

// NewDefaultPolicyEngineWithStore creates an engine whose Layer 4 (per-
// channel restrictions) consults the supplied ChannelPolicyStore. v0.2
// Step 4 of Plans/v0.2-foundation.md introduced this constructor so the
// gateway can wire its SQLite-backed store at boot without forcing every
// other policy consumer to thread a store through that may not be ready.
//
// Pass nil for store to fall back to the v0.1 disabled-with-reason
// behavior (Step 7 of the v0.1 plan).
func NewDefaultPolicyEngineWithStore(logger *slog.Logger, store ChannelPolicyStore) *DefaultPolicyEngine {
	if logger == nil {
		logger = slog.Default()
	}
	return &DefaultPolicyEngine{
		layers: []LayerChecker{
			&gatewayIdentityChecker{},
			&globalToolPolicyChecker{},
			&perAgentPermissionChecker{},
			&perChannelRestrictionChecker{store: store},
			&adminControlsChecker{},
		},
		logger: logger,
	}
}

// Evaluate implements PolicyEngine by walking layers 1-5 in order.
func (e *DefaultPolicyEngine) Evaluate(ctx context.Context, req EvalRequest) (EvalResult, error) {
	for _, lc := range e.layers {
		decision, reason, err := lc.Check(ctx, req)
		if err != nil {
			e.logger.ErrorContext(ctx, "policy layer error",
				slog.String("layer", lc.Layer().String()),
				slog.String("permission", req.Permission.String()),
				slog.String("error", err.Error()),
			)
			return EvalResult{
				Decision:  DecisionDeny,
				Layer:     lc.Layer(),
				Reason:    fmt.Sprintf("layer error: %v", err),
				Timestamp: time.Now().UTC(),
			}, err
		}

		e.logger.DebugContext(ctx, "policy layer evaluated",
			slog.String("layer", lc.Layer().String()),
			slog.String("decision", string(decision)),
			slog.String("permission", req.Permission.String()),
		)

		if decision == DecisionDeny {
			return EvalResult{
				Decision:  DecisionDeny,
				Layer:     lc.Layer(),
				Reason:    reason,
				Timestamp: time.Now().UTC(),
			}, nil
		}
	}

	return EvalResult{
		Decision:  DecisionAllow,
		Layer:     LayerAdminControls, // passed all layers
		Reason:    "all layers passed",
		Timestamp: time.Now().UTC(),
	}, nil
}

// --- Layer 1: Gateway Identity ---

type gatewayIdentityChecker struct{}

func (c *gatewayIdentityChecker) Layer() PolicyLayer { return LayerGatewayIdentity }

func (c *gatewayIdentityChecker) Check(_ context.Context, req EvalRequest) (Decision, string, error) {
	if req.TenantID == "" {
		return DecisionDeny, "missing tenant identity", nil
	}
	if req.UserID == "" && req.AgentRole == "" {
		return DecisionDeny, "no authenticated principal", nil
	}
	return DecisionAllow, "", nil
}

// --- Layer 2: Global Tool Policy ---

// blockedTools is the system-wide deny list. In production this would be
// loaded from configuration.
var blockedTools = map[string]bool{
	"tool.shell_exec":    true,
	"tool.raw_sql":       true,
	"tool.network_scan":  true,
}

type globalToolPolicyChecker struct{}

func (c *globalToolPolicyChecker) Layer() PolicyLayer { return LayerGlobalToolPolicy }

func (c *globalToolPolicyChecker) Check(_ context.Context, req EvalRequest) (Decision, string, error) {
	key := req.Permission.Resource
	if blockedTools[key] {
		return DecisionDeny, fmt.Sprintf("tool %q is globally blocked", key), nil
	}
	return DecisionAllow, "", nil
}

// --- Layer 3: Per-Agent Permissions ---

// agentAllowedActions maps agent roles to their permitted action kinds.
// In production this would be a database-backed policy store.
var agentAllowedActions = map[string]map[string]bool{
	"executor":   {"read": true, "write": true, "execute": true},
	"verifier":   {"read": true},
	"researcher": {"read": true, "execute": true},
	"narrator":   {"read": true},
	"defender":   {"read": true, "execute": true},
	"healer":     {"read": true, "write": true, "execute": true},
	"optimizer":  {"read": true, "write": true},
	"planner":    {"read": true, "write": true},
	"router":     {"read": true},
}

type perAgentPermissionChecker struct{}

func (c *perAgentPermissionChecker) Layer() PolicyLayer { return LayerPerAgentPermission }

func (c *perAgentPermissionChecker) Check(_ context.Context, req EvalRequest) (Decision, string, error) {
	// Skip agent check for direct user actions.
	if req.AgentRole == "" {
		return DecisionAllow, "", nil
	}

	allowed, exists := agentAllowedActions[req.AgentRole]
	if !exists {
		return DecisionDeny, fmt.Sprintf("unknown agent role %q", req.AgentRole), nil
	}
	if !allowed[req.Permission.Action] {
		return DecisionDeny, fmt.Sprintf("agent role %q cannot perform action %q", req.AgentRole, req.Permission.Action), nil
	}
	return DecisionAllow, "", nil
}

// --- Layer 4: Per-Channel Restrictions ---

// ChannelRule is the persisted shape of a per-channel policy entry.
// Each row says "on channel X, action Y is decided as Z, with this human
// reason". v0.2 keeps the shape narrow on purpose — v0.3 adds rate-limit
// fields per (channel, action) without breaking the row format.
type ChannelRule struct {
	ChannelID string
	Action    string
	Decision  Decision
	Reason    string
}

// ChannelPolicyStore loads per-channel rules from durable storage. The
// gateway's SQLite-backed implementation lives in
// services/gateway/internal/policy/store.go; tests and embedded uses can
// supply an in-memory fake. A nil store is treated as "no policies
// configured" — Layer 4 falls back to the v0.1 disabled-with-reason
// behavior so an empty workspace doesn't accidentally deny every request.
type ChannelPolicyStore interface {
	// Lookup returns the rule for (channelID, action) when one exists.
	// `ok=false` means no matching rule; the caller decides the fallback.
	Lookup(ctx context.Context, channelID, action string) (rule ChannelRule, ok bool, err error)
	// HasRules reports whether the store has any rules at all. The
	// checker uses this to distinguish "no rule for this channel" (still
	// allow) from "store is fully empty / not provisioned" (fall back to
	// the env-flagged disabled behavior).
	HasRules(ctx context.Context) (bool, error)
}

type perChannelRestrictionChecker struct {
	store ChannelPolicyStore
}

func (c *perChannelRestrictionChecker) Layer() PolicyLayer { return LayerPerChannelRestrict }

// Check evaluates Layer 4 (per-channel restrictions). Decision matrix:
//
//	store != nil + matching rule → use the rule's Decision + Reason
//	store != nil + no rule       → allow ("no channel rule applies")
//	store == nil OR empty store  → fall back to the v0.1 env-flagged
//	                                disabled-with-reason behavior (Step 7
//	                                of the v0.1 plan)
//
// IRONGOLEM_LAYER4_ENABLED is still the explicit operator switch. With
// the env unset, an unconfigured store says "disabled in v0.2"; with the
// env set to "true" and no store, the layer denies — refusing to silently
// pass through requests when the operator has explicitly asked for
// enforcement.
func (c *perChannelRestrictionChecker) Check(ctx context.Context, req EvalRequest) (Decision, string, error) {
	if c.store == nil {
		return layer4Fallback(req), reasonForFallback(req), nil
	}

	hasRules, err := c.store.HasRules(ctx)
	if err != nil {
		// Treat store errors as fail-closed when the layer is enabled; in
		// disabled mode, log + allow so a transient db blip doesn't
		// take down every request.
		if isLayer4Enabled() {
			return DecisionDeny, "layer4 store error: " + err.Error(), nil
		}
		return DecisionAllow, "layer4 store error (disabled): " + err.Error(), nil
	}
	if !hasRules {
		return layer4Fallback(req), reasonForFallback(req), nil
	}

	if req.ChannelID == "" {
		// Layer 4 only applies to channel-scoped operations.
		return DecisionAllow, "layer4: no channel context", nil
	}

	rule, ok, err := c.store.Lookup(ctx, req.ChannelID, req.Permission.Action)
	if err != nil {
		return DecisionDeny, "layer4 lookup error: " + err.Error(), nil
	}
	if !ok {
		// Store is configured but has no rule for this (channel, action).
		// Default to allow — Layer 4 is a deny-list, not an allow-list,
		// in v0.2. Allow-list mode is a v0.3 concern.
		return DecisionAllow, "layer4: no channel rule applies", nil
	}
	return rule.Decision, rule.Reason, nil
}

// layer4Fallback is the v0.1 Step 7 behavior, preserved for the
// nil-store / empty-store cases. With env unset → allow; with env set →
// deny ("not implemented") so operators can't silently leave Layer 4 off.
func layer4Fallback(_ EvalRequest) Decision {
	if isLayer4Enabled() {
		return DecisionDeny
	}
	return DecisionAllow
}

func reasonForFallback(req EvalRequest) string {
	if isLayer4Enabled() {
		return "layer4 store not provisioned"
	}
	if req.ChannelID == "" {
		return "layer4 disabled in v0.2 (no channel context)"
	}
	return "layer4 disabled in v0.2"
}

func isLayer4Enabled() bool {
	return os.Getenv("IRONGOLEM_LAYER4_ENABLED") == "true"
}

// --- Layer 5: Admin-Only Controls ---

type adminControlsChecker struct{}

func (c *adminControlsChecker) Layer() PolicyLayer { return LayerAdminControls }

func (c *adminControlsChecker) Check(_ context.Context, req EvalRequest) (Decision, string, error) {
	// Check admin-only metadata flags. In production this would consult
	// a feature-flag or emergency-stop service.
	if req.Metadata != nil {
		if req.Metadata["emergency_stop"] == "true" {
			return DecisionDeny, "system is in emergency stop mode", nil
		}
	}
	return DecisionAllow, "", nil
}
