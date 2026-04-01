//! Error types for the IronGolem runtime.

use thiserror::Error;

/// Result type alias using the IronGolem error type.
pub type Result<T> = std::result::Result<T, Error>;

/// Core error type for IronGolem OS runtime operations.
#[derive(Debug, Error)]
pub enum Error {
    #[error("plan execution failed: {reason}")]
    PlanExecution { reason: String },

    #[error("plan node '{node_id}' not found")]
    NodeNotFound { node_id: String },

    #[error("checkpoint failed: {reason}")]
    Checkpoint { reason: String },

    #[error("rollback failed: {reason}")]
    Rollback { reason: String },

    #[error("policy denied action: {reason}")]
    PolicyDenied { reason: String },

    #[error("verification failed: {reason}")]
    Verification { reason: String },

    #[error("sandbox error: {reason}")]
    Sandbox { reason: String },

    #[error("memory graph error: {reason}")]
    MemoryGraph { reason: String },

    #[error("connector error for '{connector_id}': {reason}")]
    Connector {
        connector_id: String,
        reason: String,
    },

    #[error("serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("database error: {0}")]
    Database(String),

    #[error("internal error: {0}")]
    Internal(String),
}
