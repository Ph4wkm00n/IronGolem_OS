//! Step executor trait and implementations. Each plan node kind has a
//! corresponding execution strategy.

use async_trait::async_trait;
use irongolem_core::{plan::Plan, Result};
use uuid::Uuid;

/// Trait for executing individual plan steps.
#[async_trait]
pub trait StepExecutor: Send + Sync {
    /// Execute a single step in a plan, returning its output.
    async fn execute_step(&self, plan: &Plan, node_id: Uuid) -> Result<serde_json::Value>;
}

/// A no-op executor for testing that returns empty objects for all steps.
pub struct NoopExecutor;

#[async_trait]
impl StepExecutor for NoopExecutor {
    async fn execute_step(&self, _plan: &Plan, _node_id: Uuid) -> Result<serde_json::Value> {
        Ok(serde_json::json!({"status": "completed"}))
    }
}
