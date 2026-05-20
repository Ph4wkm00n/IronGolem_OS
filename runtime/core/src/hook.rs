//! Hook decision types.
//!
//! v0.3 Step 2 of `Plans/modular-puzzling-blum.md`. Mirror of
//! `packages/schema/src/hooks.ts`. Locks in the
//! `Allow | Deny | Modify | Observe` taxonomy ahead of any plugin system
//! (deferred to v0.4+ alongside `runtime/sandbox` WASM completion) so
//! future plugin authors cannot invent ad-hoc decision types.
//!
//! Vocabulary aligns with [`crate::policy::PolicyEffect`]:
//! - [`HookDecision::Allow`] ~ `PolicyEffect::Allow`
//! - [`HookDecision::Deny`]  ~ `PolicyEffect::Deny`
//! - `Modify` and `Observe` are hook-only; the policy engine never
//!   rewrites payloads and never runs purely observer-side without an
//!   effect.

use serde::{Deserialize, Serialize};

use crate::types::{AgentId, TenantId, WorkspaceId};

/// What a single hook returns.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum HookDecision {
    /// Allow the operation to proceed unchanged.
    Allow,
    /// Block the operation; `reason` becomes the user-facing explanation.
    Deny,
    /// Proceed using the hook's modified payload.
    Modify,
    /// No effect; the hook only observed (logged / emitted telemetry).
    Observe,
}

impl HookDecision {
    /// User-facing label, used in audit-trail rendering. Stays aligned
    /// with `hookDecisionLabel` in `packages/schema/src/hooks.ts`.
    pub fn label(self) -> &'static str {
        match self {
            HookDecision::Allow => "Allowed",
            HookDecision::Deny => "Blocked",
            HookDecision::Modify => "Modified",
            HookDecision::Observe => "Observed",
        }
    }

    /// Whether this decision short-circuits hook chain evaluation.
    /// `Deny` short-circuits; `Modify` keeps going (later hooks see the
    /// modified payload); `Allow` and `Observe` always continue.
    pub fn short_circuits(self) -> bool {
        matches!(self, HookDecision::Deny)
    }
}

/// The lifecycle phase a hook fires at.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum HookPhase {
    BeforeAgentStart,
    BeforeAgentReply,
    BeforeToolCall,
    AfterToolCall,
    BeforeInstall,
}

/// Context passed to every hook invocation. Minimal by design — hooks
/// that need richer state should accept a typed payload alongside this
/// envelope rather than expanding it.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HookContext {
    /// Correlation across all events in this turn / plan execution.
    pub correlation_id: String,
    /// Lifecycle phase firing.
    pub phase: HookPhase,
    /// Which agent is in scope. `None` for install-time hooks that
    /// pre-date any agent assignment.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub agent_id: Option<AgentId>,
    /// Workspace owning this invocation.
    pub workspace_id: WorkspaceId,
    /// Tenant owning the workspace.
    pub tenant_id: TenantId,
}

/// Result of a single hook invocation.
///
/// - `decision == Allow`: proceed; ignore `reason` and `modified_payload`.
/// - `decision == Deny`: block; `reason` surfaces as the block message.
/// - `decision == Modify`: proceed with `modified_payload` REPLACING
///   the original. `modified_payload` is required.
/// - `decision == Observe`: no effect; the hook only observed.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HookResult {
    pub decision: HookDecision,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modified_payload: Option<serde_json::Value>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn short_circuit_only_deny() {
        assert!(HookDecision::Deny.short_circuits());
        assert!(!HookDecision::Allow.short_circuits());
        assert!(!HookDecision::Modify.short_circuits());
        assert!(!HookDecision::Observe.short_circuits());
    }

    #[test]
    fn labels_match_schema_ts() {
        // These strings are duplicated in packages/schema/src/hooks.ts —
        // if either side drifts, audit-trail rendering breaks. Keep in
        // sync with hookDecisionLabel there.
        assert_eq!(HookDecision::Allow.label(), "Allowed");
        assert_eq!(HookDecision::Deny.label(), "Blocked");
        assert_eq!(HookDecision::Modify.label(), "Modified");
        assert_eq!(HookDecision::Observe.label(), "Observed");
    }

    #[test]
    fn decision_serializes_lowercase() {
        // Wire format must match the TS string-union exactly.
        assert_eq!(serde_json::to_string(&HookDecision::Allow).unwrap(), "\"allow\"");
        assert_eq!(serde_json::to_string(&HookDecision::Deny).unwrap(), "\"deny\"");
        assert_eq!(serde_json::to_string(&HookDecision::Modify).unwrap(), "\"modify\"");
        assert_eq!(serde_json::to_string(&HookDecision::Observe).unwrap(), "\"observe\"");
    }

    #[test]
    fn phase_serializes_kebab() {
        assert_eq!(
            serde_json::to_string(&HookPhase::BeforeAgentStart).unwrap(),
            "\"before-agent-start\""
        );
        assert_eq!(
            serde_json::to_string(&HookPhase::BeforeToolCall).unwrap(),
            "\"before-tool-call\""
        );
    }

    #[test]
    fn result_skips_optional_fields_on_serialize() {
        let r = HookResult {
            decision: HookDecision::Allow,
            reason: None,
            modified_payload: None,
        };
        let s = serde_json::to_string(&r).unwrap();
        assert_eq!(s, "{\"decision\":\"allow\"}");
    }
}
