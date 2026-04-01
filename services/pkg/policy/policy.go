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

// NewDefaultPolicyEngine creates an engine with the standard five layers.
// Pass nil for logger to use the default.
func NewDefaultPolicyEngine(logger *slog.Logger) *DefaultPolicyEngine {
	if logger == nil {
		logger = slog.Default()
	}
	return &DefaultPolicyEngine{
		layers: []LayerChecker{
			&gatewayIdentityChecker{},
			&globalToolPolicyChecker{},
			&perAgentPermissionChecker{},
			&perChannelRestrictionChecker{},
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

type perChannelRestrictionChecker struct{}

func (c *perChannelRestrictionChecker) Layer() PolicyLayer { return LayerPerChannelRestrict }

func (c *perChannelRestrictionChecker) Check(_ context.Context, req EvalRequest) (Decision, string, error) {
	// In production, this loads channel-specific policies from the data store.
	// For now, allow all actions on channels that are specified.
	if req.ChannelID == "" {
		// No channel context means this is an internal operation; allow.
		return DecisionAllow, "", nil
	}
	return DecisionAllow, "", nil
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
