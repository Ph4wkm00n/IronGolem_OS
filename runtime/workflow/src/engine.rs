//! Plan execution engine. Drives plan graphs through their lifecycle,
//! coordinating step execution, checkpointing, and policy checks.

use std::sync::Arc;

use irongolem_core::{
    event::{Event, EventKind},
    plan::{Plan, PlanNodeStatus, PlanStatus},
    types::WorkspaceId,
    Error, Result,
};
use tokio::sync::Mutex;
use tracing::{info, warn};

use crate::executor::StepExecutor;

/// The plan execution engine drives plan graphs through their lifecycle.
pub struct PlanEngine {
    executor: Arc<dyn StepExecutor>,
    event_log: Arc<Mutex<Vec<Event>>>,
}

impl PlanEngine {
    pub fn new(executor: Arc<dyn StepExecutor>) -> Self {
        Self {
            executor,
            event_log: Arc::new(Mutex::new(Vec::new())),
        }
    }

    /// Execute a plan to completion, step by step.
    pub async fn execute(&self, plan: &mut Plan, workspace_id: WorkspaceId) -> Result<()> {
        plan.status = PlanStatus::Running;
        self.emit_event(workspace_id, EventKind::PlanCreated {
            plan_id: plan.id,
            description: plan.description.clone(),
        }).await;

        while let Some(next_node) = plan.next_pending_node() {
            let next_node_id = next_node.id;

            // Check if dependencies are satisfied
            let node = plan.find_node(next_node_id)
                .ok_or(Error::NodeNotFound { node_id: next_node_id.to_string() })?;
            let deps_met = node.dependencies.iter().all(|dep_id| {
                plan.find_node(*dep_id)
                    .map(|n| n.status == PlanNodeStatus::Completed)
                    .unwrap_or(false)
            });

            if !deps_met {
                // Skip nodes whose dependencies aren't met yet
                continue;
            }

            // Mark as running
            let node = plan.find_node_mut(next_node_id)
                .ok_or(Error::NodeNotFound { node_id: next_node_id.to_string() })?;
            node.status = PlanNodeStatus::Running;

            self.emit_event(workspace_id, EventKind::PlanStepStarted {
                plan_id: plan.id,
                step_id: next_node_id,
            }).await;

            // Execute the step
            match self.executor.execute_step(plan, next_node_id).await {
                Ok(output) => {
                    let node = plan.find_node_mut(next_node_id)
                        .ok_or(Error::NodeNotFound { node_id: next_node_id.to_string() })?;
                    node.status = PlanNodeStatus::Completed;
                    node.output = Some(output.clone());

                    self.emit_event(workspace_id, EventKind::PlanStepCompleted {
                        plan_id: plan.id,
                        step_id: next_node_id,
                        output,
                    }).await;
                    info!(plan_id = %plan.id, step_id = %next_node_id, "Step completed");
                }
                Err(e) => {
                    let node = plan.find_node_mut(next_node_id)
                        .ok_or(Error::NodeNotFound { node_id: next_node_id.to_string() })?;
                    node.status = PlanNodeStatus::Failed;
                    node.error = Some(e.to_string());

                    self.emit_event(workspace_id, EventKind::PlanStepFailed {
                        plan_id: plan.id,
                        step_id: next_node_id,
                        error: e.to_string(),
                    }).await;
                    warn!(plan_id = %plan.id, step_id = %next_node_id, error = %e, "Step failed");

                    plan.status = PlanStatus::Failed;
                    return Err(e);
                }
            }
        }

        plan.status = PlanStatus::Completed;
        self.emit_event(workspace_id, EventKind::PlanCompleted { plan_id: plan.id }).await;
        info!(plan_id = %plan.id, "Plan completed successfully");

        Ok(())
    }

    /// Get a copy of the event log.
    pub async fn events(&self) -> Vec<Event> {
        self.event_log.lock().await.clone()
    }

    async fn emit_event(&self, workspace_id: WorkspaceId, kind: EventKind) {
        let event = Event::new(workspace_id, kind);
        self.event_log.lock().await.push(event);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use irongolem_core::plan::{PlanNode, PlanNodeKind, PlanNodeStatus};
    use irongolem_core::types::AgentId;
    use uuid::Uuid;
    use crate::executor::NoopExecutor;

    /// An executor that always fails with a given error message.
    struct FailingExecutor {
        message: String,
    }

    impl FailingExecutor {
        fn new(message: impl Into<String>) -> Self {
            Self {
                message: message.into(),
            }
        }
    }

    #[async_trait::async_trait]
    impl StepExecutor for FailingExecutor {
        async fn execute_step(
            &self,
            _plan: &Plan,
            _node_id: Uuid,
        ) -> irongolem_core::Result<serde_json::Value> {
            Err(Error::PlanExecution {
                reason: self.message.clone(),
            })
        }
    }

    fn make_tool_node(desc: &str) -> PlanNode {
        PlanNode::new(
            desc,
            PlanNodeKind::ToolCall {
                tool_name: format!("tool_{desc}"),
                input: serde_json::json!({}),
            },
        )
    }

    #[tokio::test]
    async fn test_execute_simple_plan() {
        let engine = PlanEngine::new(Arc::new(NoopExecutor));
        let ws = WorkspaceId::new();
        let mut plan = Plan::new("three-step plan", AgentId::new());

        plan.add_node(make_tool_node("a"));
        plan.add_node(make_tool_node("b"));
        plan.add_node(make_tool_node("c"));

        engine.execute(&mut plan, ws).await.unwrap();

        assert_eq!(plan.status, PlanStatus::Completed);
        for node in &plan.nodes {
            assert_eq!(node.status, PlanNodeStatus::Completed);
            assert!(node.output.is_some());
        }

        // Verify events were emitted
        let events = engine.events().await;
        // PlanCreated + 3*(PlanStepStarted + PlanStepCompleted) + PlanCompleted = 8
        assert_eq!(events.len(), 8);
    }

    #[tokio::test]
    async fn test_execute_plan_with_dependencies() {
        let engine = PlanEngine::new(Arc::new(NoopExecutor));
        let ws = WorkspaceId::new();
        let mut plan = Plan::new("dependency plan", AgentId::new());

        let node_a = make_tool_node("a");
        let node_b = make_tool_node("b");
        let a_id = node_a.id;
        let b_id = node_b.id;

        // Node C depends on both A and B
        let node_c = make_tool_node("c")
            .with_dependency(a_id)
            .with_dependency(b_id);

        plan.add_node(node_a);
        plan.add_node(node_b);
        plan.add_node(node_c);

        engine.execute(&mut plan, ws).await.unwrap();

        assert_eq!(plan.status, PlanStatus::Completed);
        // All nodes should complete
        for node in &plan.nodes {
            assert_eq!(node.status, PlanNodeStatus::Completed);
        }

        // C (index 2) must have completed after A and B
        // Since next_pending_node iterates in order, A runs first, then B, then C
        let events = engine.events().await;
        let step_completed_ids: Vec<Uuid> = events
            .iter()
            .filter_map(|e| match &e.kind {
                EventKind::PlanStepCompleted { step_id, .. } => Some(*step_id),
                _ => None,
            })
            .collect();

        assert_eq!(step_completed_ids.len(), 3);
        // C must be last
        assert_eq!(step_completed_ids[2], plan.nodes[2].id);
    }

    #[tokio::test]
    async fn test_plan_failure_stops_execution() {
        let engine = PlanEngine::new(Arc::new(FailingExecutor::new("tool broke")));
        let ws = WorkspaceId::new();
        let mut plan = Plan::new("failing plan", AgentId::new());

        plan.add_node(make_tool_node("a"));
        plan.add_node(make_tool_node("b"));
        plan.add_node(make_tool_node("c"));

        let result = engine.execute(&mut plan, ws).await;
        assert!(result.is_err());

        assert_eq!(plan.status, PlanStatus::Failed);
        // First node should be Failed
        assert_eq!(plan.nodes[0].status, PlanNodeStatus::Failed);
        assert!(plan.nodes[0].error.is_some());
        // Remaining nodes stay Pending
        assert_eq!(plan.nodes[1].status, PlanNodeStatus::Pending);
        assert_eq!(plan.nodes[2].status, PlanNodeStatus::Pending);
    }

    #[tokio::test]
    async fn test_events_emitted_for_each_step() {
        let engine = PlanEngine::new(Arc::new(NoopExecutor));
        let ws = WorkspaceId::new();
        let mut plan = Plan::new("event test plan", AgentId::new());

        plan.add_node(make_tool_node("a"));
        plan.add_node(make_tool_node("b"));

        let plan_id = plan.id;
        let node_a_id = plan.nodes[0].id;
        let node_b_id = plan.nodes[1].id;

        engine.execute(&mut plan, ws).await.unwrap();

        let events = engine.events().await;
        let kinds: Vec<String> = events
            .iter()
            .map(|e| match &e.kind {
                EventKind::PlanCreated { .. } => "PlanCreated".to_string(),
                EventKind::PlanStepStarted { step_id, .. } => {
                    format!("PlanStepStarted:{step_id}")
                }
                EventKind::PlanStepCompleted { step_id, .. } => {
                    format!("PlanStepCompleted:{step_id}")
                }
                EventKind::PlanCompleted { .. } => "PlanCompleted".to_string(),
                other => format!("{other:?}"),
            })
            .collect();

        assert_eq!(kinds[0], "PlanCreated");
        assert_eq!(kinds[1], format!("PlanStepStarted:{node_a_id}"));
        assert_eq!(kinds[2], format!("PlanStepCompleted:{node_a_id}"));
        assert_eq!(kinds[3], format!("PlanStepStarted:{node_b_id}"));
        assert_eq!(kinds[4], format!("PlanStepCompleted:{node_b_id}"));
        assert_eq!(kinds[5], "PlanCompleted");

        // Verify all events reference the correct plan_id
        for event in &events {
            match &event.kind {
                EventKind::PlanCreated { plan_id: pid, .. }
                | EventKind::PlanStepStarted { plan_id: pid, .. }
                | EventKind::PlanStepCompleted { plan_id: pid, .. }
                | EventKind::PlanCompleted { plan_id: pid } => {
                    assert_eq!(*pid, plan_id);
                }
                _ => {}
            }
        }
    }
}
