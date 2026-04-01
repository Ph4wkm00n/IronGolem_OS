//! # IronGolem Workflow
//!
//! Execution state machines for plan graph execution. Manages the lifecycle
//! of plans: pending -> running -> paused -> completed/failed/rolled-back.

pub mod engine;
pub mod executor;

pub use engine::PlanEngine;
pub use executor::StepExecutor;
