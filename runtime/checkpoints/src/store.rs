//! Checkpoint storage trait and types.

use async_trait::async_trait;
use chrono::{DateTime, Utc};
use irongolem_core::Result;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// A checkpoint captures the state of plan execution at a point in time.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Checkpoint {
    /// Unique checkpoint identifier.
    pub id: Uuid,
    /// Plan this checkpoint belongs to.
    pub plan_id: Uuid,
    /// Which step was the last completed step.
    pub last_completed_step: Option<Uuid>,
    /// Serialized plan state at checkpoint time.
    pub plan_state: serde_json::Value,
    /// When this checkpoint was created.
    pub created_at: DateTime<Utc>,
    /// Optional label for this checkpoint.
    pub label: Option<String>,
}

impl Checkpoint {
    pub fn new(plan_id: Uuid, plan_state: serde_json::Value) -> Self {
        Self {
            id: Uuid::new_v4(),
            plan_id,
            last_completed_step: None,
            plan_state,
            created_at: Utc::now(),
            label: None,
        }
    }
}

/// Trait for checkpoint persistence backends.
#[async_trait]
pub trait CheckpointStore: Send + Sync {
    /// Save a checkpoint.
    async fn save(&self, checkpoint: &Checkpoint) -> Result<()>;

    /// Load the latest checkpoint for a plan.
    async fn load_latest(&self, plan_id: Uuid) -> Result<Option<Checkpoint>>;

    /// Load a specific checkpoint by ID.
    async fn load(&self, checkpoint_id: Uuid) -> Result<Option<Checkpoint>>;

    /// List all checkpoints for a plan, newest first.
    async fn list_for_plan(&self, plan_id: Uuid) -> Result<Vec<Checkpoint>>;

    /// Delete checkpoints older than a given checkpoint for a plan.
    async fn prune(&self, plan_id: Uuid, keep_latest: usize) -> Result<usize>;
}
