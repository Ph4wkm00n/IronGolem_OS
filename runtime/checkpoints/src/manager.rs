//! Checkpoint manager coordinates checkpoint creation and rollback operations.

use std::sync::Arc;

use irongolem_core::{Error, Result, plan::Plan};
use tracing::{info, warn};
use uuid::Uuid;

use crate::store::{Checkpoint, CheckpointStore};

/// Manages checkpoint lifecycle: creation, listing, and rollback.
pub struct CheckpointManager {
    store: Arc<dyn CheckpointStore>,
}

impl CheckpointManager {
    pub fn new(store: Arc<dyn CheckpointStore>) -> Self {
        Self { store }
    }

    /// Create a checkpoint for the current plan state.
    pub async fn create_checkpoint(&self, plan: &Plan) -> Result<Checkpoint> {
        let plan_state = serde_json::to_value(plan).map_err(|e| Error::Checkpoint {
            reason: e.to_string(),
        })?;

        let mut checkpoint = Checkpoint::new(plan.id, plan_state);
        checkpoint.last_completed_step = plan
            .nodes
            .iter()
            .rev()
            .find(|n| n.status == irongolem_core::plan::PlanNodeStatus::Completed)
            .map(|n| n.id);

        self.store.save(&checkpoint).await?;
        info!(
            checkpoint_id = %checkpoint.id,
            plan_id = %plan.id,
            "Checkpoint created"
        );

        Ok(checkpoint)
    }

    /// Rollback a plan to a specific checkpoint.
    pub async fn rollback(&self, checkpoint_id: Uuid) -> Result<Plan> {
        let checkpoint = self
            .store
            .load(checkpoint_id)
            .await?
            .ok_or_else(|| Error::Rollback {
                reason: format!("Checkpoint {checkpoint_id} not found"),
            })?;

        let plan: Plan =
            serde_json::from_value(checkpoint.plan_state).map_err(|e| Error::Rollback {
                reason: e.to_string(),
            })?;

        info!(
            checkpoint_id = %checkpoint_id,
            plan_id = %plan.id,
            "Plan rolled back to checkpoint"
        );

        Ok(plan)
    }

    /// Rollback to the latest checkpoint for a plan.
    pub async fn rollback_to_latest(&self, plan_id: Uuid) -> Result<Plan> {
        let checkpoint = self
            .store
            .load_latest(plan_id)
            .await?
            .ok_or_else(|| Error::Rollback {
                reason: format!("No checkpoints found for plan {plan_id}"),
            })?;

        self.rollback(checkpoint.id).await
    }

    /// Prune old checkpoints, keeping only the most recent ones.
    pub async fn prune(&self, plan_id: Uuid, keep_latest: usize) -> Result<usize> {
        let pruned = self.store.prune(plan_id, keep_latest).await?;
        if pruned > 0 {
            warn!(plan_id = %plan_id, pruned, "Pruned old checkpoints");
        }
        Ok(pruned)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::SqliteCheckpointStore;
    use irongolem_core::plan::{Plan, PlanNode, PlanNodeKind, PlanNodeStatus, PlanStatus};
    use irongolem_core::types::AgentId;

    fn make_tool_node(desc: &str) -> PlanNode {
        PlanNode::new(
            desc,
            PlanNodeKind::ToolCall {
                tool_name: format!("tool_{desc}"),
                input: serde_json::json!({}),
            },
        )
    }

    /// Simulate executing a node by marking it Completed with output.
    fn complete_node(plan: &mut Plan, index: usize) {
        let node = &mut plan.nodes[index];
        node.status = PlanNodeStatus::Completed;
        node.output = Some(serde_json::json!({"result": format!("output_{}", index)}));
    }

    #[tokio::test]
    async fn test_checkpoint_and_rollback() {
        let store = Arc::new(SqliteCheckpointStore::open_in_memory().unwrap());
        let manager = CheckpointManager::new(store);

        let mut plan = Plan::new("checkpoint test", AgentId::new());
        plan.add_node(make_tool_node("a"));
        plan.add_node(make_tool_node("b"));
        plan.add_node(make_tool_node("c"));
        plan.status = PlanStatus::Running;

        // Execute steps 0 and 1
        complete_node(&mut plan, 0);
        complete_node(&mut plan, 1);

        // Checkpoint after 2 steps completed
        let cp = manager.create_checkpoint(&plan).await.unwrap();
        let cp_id = cp.id;

        // Verify checkpoint records the last completed step
        assert_eq!(cp.last_completed_step, Some(plan.nodes[1].id));

        // Execute step 2
        complete_node(&mut plan, 2);
        plan.status = PlanStatus::Completed;
        assert_eq!(plan.nodes[2].status, PlanNodeStatus::Completed);

        // Rollback to checkpoint
        let restored = manager.rollback(cp_id).await.unwrap();

        // Plan should be restored to the state at checkpoint time
        assert_eq!(restored.id, plan.id);
        assert_eq!(restored.status, PlanStatus::Running);
        assert_eq!(restored.nodes[0].status, PlanNodeStatus::Completed);
        assert_eq!(restored.nodes[1].status, PlanNodeStatus::Completed);
        assert_eq!(restored.nodes[2].status, PlanNodeStatus::Pending);
    }

    #[tokio::test]
    async fn test_rollback_to_latest() {
        let store = Arc::new(SqliteCheckpointStore::open_in_memory().unwrap());
        let manager = CheckpointManager::new(store);

        let mut plan = Plan::new("rollback-latest test", AgentId::new());
        plan.add_node(make_tool_node("a"));
        plan.add_node(make_tool_node("b"));
        plan.add_node(make_tool_node("c"));
        plan.status = PlanStatus::Running;

        // Checkpoint 1: after step 0
        complete_node(&mut plan, 0);
        let _cp1 = manager.create_checkpoint(&plan).await.unwrap();

        // Checkpoint 2: after steps 0 and 1
        complete_node(&mut plan, 1);
        let cp2 = manager.create_checkpoint(&plan).await.unwrap();

        // Execute step 2 and mark plan as failed
        plan.nodes[2].status = PlanNodeStatus::Failed;
        plan.nodes[2].error = Some("something broke".into());
        plan.status = PlanStatus::Failed;

        // Rollback to latest should restore to cp2 (after steps 0 and 1)
        let restored = manager.rollback_to_latest(plan.id).await.unwrap();

        assert_eq!(restored.id, plan.id);
        assert_eq!(restored.status, PlanStatus::Running);
        assert_eq!(restored.nodes[0].status, PlanNodeStatus::Completed);
        assert_eq!(restored.nodes[1].status, PlanNodeStatus::Completed);
        assert_eq!(restored.nodes[2].status, PlanNodeStatus::Pending);
        assert!(restored.nodes[2].error.is_none());

        // Verify it used cp2 (the latest) by checking last_completed_step
        assert_eq!(cp2.last_completed_step, Some(plan.nodes[1].id));
    }
}
