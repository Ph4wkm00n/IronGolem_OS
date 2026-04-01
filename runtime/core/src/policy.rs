//! Policy types for the five-layer permission model.
//!
//! Every action passes through five layers:
//! 1. Gateway identity and authentication
//! 2. Global tool policy
//! 3. Per-agent permissions
//! 4. Per-channel restrictions
//! 5. Owner/admin-only controls

use serde::{Deserialize, Serialize};

use crate::types::{AgentId, ChannelId, UserId};

/// An action that a policy decision applies to.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Action {
    /// What tool or operation is being requested.
    pub tool_name: String,
    /// Who is requesting the action.
    pub user_id: UserId,
    /// Which agent is executing.
    pub agent_id: Option<AgentId>,
    /// Which channel originated the request.
    pub channel_id: Option<ChannelId>,
    /// Parameters for the action.
    pub params: serde_json::Value,
}

/// A permission entry in the policy engine.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Permission {
    /// Which policy layer this permission belongs to (1-5).
    pub layer: PolicyLayer,
    /// The tool or action this permission applies to.
    pub tool_pattern: String,
    /// Whether this is an allow or deny rule.
    pub effect: PolicyEffect,
    /// Whether approval is required even if allowed.
    pub requires_approval: bool,
    /// Human-readable description for the UI policy card.
    pub description: String,
}

/// The five permission layers.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PolicyLayer {
    /// Layer 1: Gateway identity and authentication.
    GatewayIdentity = 1,
    /// Layer 2: Global tool policy.
    GlobalToolPolicy = 2,
    /// Layer 3: Per-agent permissions.
    AgentPermissions = 3,
    /// Layer 4: Per-channel restrictions.
    ChannelRestrictions = 4,
    /// Layer 5: Owner/admin-only controls.
    AdminControls = 5,
}

/// Whether a policy allows or denies an action.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PolicyEffect {
    Allow,
    Deny,
}

/// The result of evaluating an action against all five policy layers.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PolicyDecision {
    /// Whether the action is allowed.
    pub allowed: bool,
    /// Whether approval is required before execution.
    pub requires_approval: bool,
    /// Which layer denied the action, if denied.
    pub denied_by_layer: Option<PolicyLayer>,
    /// Human-readable explanation for the UI.
    pub explanation: String,
    /// Results from each layer evaluation.
    pub layer_results: Vec<LayerResult>,
}

/// Result of evaluating a single policy layer.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LayerResult {
    pub layer: PolicyLayer,
    pub passed: bool,
    pub explanation: String,
}

impl PolicyDecision {
    /// Create an "allowed" decision.
    pub fn allow(explanation: impl Into<String>) -> Self {
        Self {
            allowed: true,
            requires_approval: false,
            denied_by_layer: None,
            explanation: explanation.into(),
            layer_results: Vec::new(),
        }
    }

    /// Create a "denied" decision.
    pub fn deny(layer: PolicyLayer, explanation: impl Into<String>) -> Self {
        Self {
            allowed: false,
            requires_approval: false,
            denied_by_layer: Some(layer),
            explanation: explanation.into(),
            layer_results: Vec::new(),
        }
    }

    /// Create an "allowed but requires approval" decision.
    pub fn allow_with_approval(explanation: impl Into<String>) -> Self {
        Self {
            allowed: true,
            requires_approval: true,
            denied_by_layer: None,
            explanation: explanation.into(),
            layer_results: Vec::new(),
        }
    }
}
