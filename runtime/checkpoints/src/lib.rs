//! # IronGolem Checkpoints
//!
//! State snapshots and rollback management. Checkpoints enable resumption
//! after crashes, rollback to known-good states, and replay for debugging.

pub mod manager;
pub mod store;

pub use manager::CheckpointManager;
pub use store::{Checkpoint, CheckpointStore};
