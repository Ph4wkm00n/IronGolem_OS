//! # IronGolem Core
//!
//! The core crate for IronGolem OS runtime. Provides foundational types for
//! plan graph execution, event sourcing, policy enforcement, and risk metadata.

pub mod error;
pub mod event;
pub mod hook;
pub mod ipc;
pub mod pg_store;
pub mod plan;
pub mod plugin;
pub mod policy;
pub mod risk;
pub mod store;
pub mod types;

pub use error::{Error, Result};
pub use event::{Event, EventKind};
pub use hook::{HookContext, HookDecision, HookPhase, HookResult};
pub use ipc::{
    EventNotification, ExecutePlanRequest, ExecutePlanResponse, ExecutionStatus,
    ListProvidersRequest, ListProvidersResponse, Message, PingRequest, PingResponse, Shutdown,
};
pub use plan::{Plan, PlanNode, PlanNodeKind, PlanStatus};
pub use policy::{Action, Permission, PolicyDecision};
pub use risk::{RiskLevel, RiskMetadata};
pub use store::{EventStore, SqliteEventStore};
pub use types::{AgentId, ChannelId, ConnectorId, SessionId, TenantId, UserId, WorkspaceId};

#[cfg(feature = "postgres")]
pub use pg_store::PgEventStore;
