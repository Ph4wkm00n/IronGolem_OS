//! # IronGolem Checkpoints
//!
//! State snapshots and rollback management. Checkpoints enable resumption
//! after crashes, rollback to known-good states, and replay for debugging.

pub mod git_shared_store;
pub mod manager;
pub mod sqlite_store;
pub mod store;

pub use git_shared_store::{
    CheckpointEntry, GitSharedStore, GitStoreError, GitStoreResult, StoreStatus,
};
pub use manager::CheckpointManager;
pub use sqlite_store::SqliteCheckpointStore;
pub use store::{Checkpoint, CheckpointStore};
