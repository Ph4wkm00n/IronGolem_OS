//! `RealStepExecutor` — the concrete `StepExecutor` that finally dispatches
//! every `PlanNodeKind` variant for v0.1. This is what replaces `NoopExecutor`
//! in production paths.

use std::sync::Arc;

use async_trait::async_trait;
use irongolem_core::{
    Error, Result,
    plan::{Plan, PlanNodeKind},
};
use irongolem_sandbox::{LocalSandboxHost, SandboxConfig, SandboxHost};
use irongolem_verifier::{Verifier, checks::NonEmptyVerifier};
use irongolem_workflow::StepExecutor;
use uuid::Uuid;

use crate::provider::LlmProvider;

/// Dispatches each plan-node kind to the appropriate handler.
///
/// - `ToolCall` -> `SandboxHost`
/// - `LlmCall` -> `LlmProvider`
/// - `Verify` -> `NonEmptyVerifier` (placeholder until per-node verifier
///   configuration arrives in a later step)
/// - `Checkpoint` -> creates a checkpoint via the supplied callback if any,
///   otherwise returns a no-op marker (callers wire checkpoint persistence
///   in higher layers when needed)
/// - `ApprovalGate`, `Delegation` -> explicit "not implemented in v0.1" error
pub struct RealStepExecutor {
    pub sandbox: Arc<LocalSandboxHost>,
    pub llm: Arc<dyn LlmProvider>,
    pub default_sandbox_config: SandboxConfig,
}

impl RealStepExecutor {
    pub fn new(sandbox: Arc<LocalSandboxHost>, llm: Arc<dyn LlmProvider>) -> Self {
        Self {
            sandbox,
            llm,
            default_sandbox_config: default_sandbox_config(),
        }
    }
}

fn default_sandbox_config() -> SandboxConfig {
    // v0.1 default: no network access for arbitrary tool calls. Specific tools
    // such as `http_get` require the SandboxConfig to be widened at the call
    // site (or by the gateway when it constructs the plan).
    SandboxConfig::default()
}

#[async_trait]
impl StepExecutor for RealStepExecutor {
    async fn execute_step(&self, plan: &Plan, node_id: Uuid) -> Result<serde_json::Value> {
        let node = plan.find_node(node_id).ok_or(Error::NodeNotFound {
            node_id: node_id.to_string(),
        })?;

        match &node.kind {
            PlanNodeKind::ToolCall { tool_name, input } => {
                self.sandbox
                    .execute(tool_name, input, &self.default_sandbox_config)
                    .await
            }
            PlanNodeKind::LlmCall { prompt, model } => {
                let text = self.llm.complete(prompt, model.as_deref()).await?;
                Ok(serde_json::json!({ "text": text }))
            }
            PlanNodeKind::Verify { target_node_id } => {
                let target = plan.find_node(*target_node_id).ok_or(Error::NodeNotFound {
                    node_id: target_node_id.to_string(),
                })?;
                let output = target.output.as_ref().ok_or(Error::Verification {
                    reason: format!("target node {target_node_id} has no output yet"),
                })?;
                let result = NonEmptyVerifier.verify(output).await?;
                if !result.passed {
                    return Err(Error::Verification {
                        reason: format!("non_empty check failed: {:?}", result.suggestions),
                    });
                }
                Ok(serde_json::json!({ "passed": true, "verifier": "non_empty" }))
            }
            PlanNodeKind::Checkpoint => {
                // Persistence is left to a higher layer (the gateway can attach
                // a CheckpointManager and call it on this event). The runtime
                // surfaces a marker so the engine still records the step.
                Ok(serde_json::json!({ "checkpoint": "noted" }))
            }
            PlanNodeKind::ApprovalGate { .. } => Err(Error::PlanExecution {
                reason: "ApprovalGate not implemented in v0.1".into(),
            }),
            PlanNodeKind::Delegation { .. } => Err(Error::PlanExecution {
                reason: "Delegation not implemented in v0.1".into(),
            }),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use irongolem_core::types::WorkspaceId;
    use irongolem_core::{
        plan::{PlanNode, PlanNodeKind, PlanNodeStatus},
        types::AgentId,
    };
    use irongolem_workflow::PlanEngine;

    use crate::provider::MockProvider;

    fn executor() -> Arc<RealStepExecutor> {
        Arc::new(RealStepExecutor::new(
            Arc::new(LocalSandboxHost::with_builtins()),
            Arc::new(MockProvider::new("pong")),
        ))
    }

    #[tokio::test]
    async fn tool_call_dispatches_through_registry() {
        let exec = executor();
        let engine = PlanEngine::new(exec.clone());
        let mut plan = Plan::new("tool", AgentId::new());
        plan.add_node(PlanNode::new(
            "echo",
            PlanNodeKind::ToolCall {
                tool_name: "echo".into(),
                input: serde_json::json!({"x": 1}),
            },
        ));

        engine.execute(&mut plan, WorkspaceId::new()).await.unwrap();
        assert_eq!(plan.nodes[0].status, PlanNodeStatus::Completed);
        assert_eq!(
            plan.nodes[0].output.as_ref().unwrap(),
            &serde_json::json!({"x": 1})
        );
    }

    #[tokio::test]
    async fn llm_call_returns_provider_text() {
        let exec = executor();
        let engine = PlanEngine::new(exec.clone());
        let mut plan = Plan::new("llm", AgentId::new());
        plan.add_node(PlanNode::new(
            "ask",
            PlanNodeKind::LlmCall {
                prompt: "ping".into(),
                model: None,
            },
        ));

        engine.execute(&mut plan, WorkspaceId::new()).await.unwrap();
        assert_eq!(
            plan.nodes[0].output.as_ref().unwrap(),
            &serde_json::json!({"text": "pong"})
        );
    }

    #[tokio::test]
    async fn approval_gate_is_not_implemented_in_v0_1() {
        let exec = executor();
        let engine = PlanEngine::new(exec.clone());
        let mut plan = Plan::new("approval", AgentId::new());
        plan.add_node(PlanNode::new(
            "wait",
            PlanNodeKind::ApprovalGate {
                description: "stop here".into(),
            },
        ));

        let err = engine
            .execute(&mut plan, WorkspaceId::new())
            .await
            .unwrap_err();
        match err {
            Error::PlanExecution { reason } => assert!(reason.contains("ApprovalGate")),
            other => panic!("expected PlanExecution error, got {other:?}"),
        }
    }
}
