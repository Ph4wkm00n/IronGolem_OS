//! Checkpoint manager coordinates checkpoint creation and rollback operations.

use std::sync::Arc;

use irongolem_core::{plan::Plan, Error, Result};
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
        let plan_state = serde_json::to_value(plan)
            .map_err(|e| Error::Checkpoint { reason: e.to_string() })?;

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

        let plan: Plan = serde_json::from_value(checkpoint.plan_state)
            .map_err(|e| Error::Rollback { reason: e.to_string() })?;

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
