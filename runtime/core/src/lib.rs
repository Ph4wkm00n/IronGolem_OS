//! # IronGolem Core
//!
//! The core crate for IronGolem OS runtime. Provides foundational types for
//! plan graph execution, event sourcing, policy enforcement, and risk metadata.

pub mod error;
pub mod event;
pub mod plan;
pub mod policy;
pub mod risk;
pub mod types;

pub use error::{Error, Result};
pub use event::{Event, EventKind};
pub use plan::{Plan, PlanNode, PlanNodeKind, PlanStatus};
pub use policy::{Action, Permission, PolicyDecision};
pub use risk::{RiskLevel, RiskMetadata};
pub use types::{AgentId, ChannelId, ConnectorId, SessionId, TenantId, UserId, WorkspaceId};
