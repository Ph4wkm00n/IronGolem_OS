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

        loop {
            let next_node_id = match plan.next_pending_node() {
                Some(node) => node.id,
                None => break,
            };

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
