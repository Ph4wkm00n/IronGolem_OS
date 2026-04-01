//! Core identity and reference types used throughout IronGolem OS.
//!
//! These types enforce the isolation boundary hierarchy:
//! Tenant -> Workspace -> User -> Channel -> Agent Session

use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Strongly-typed wrapper for entity identifiers.
macro_rules! define_id {
    ($name:ident, $doc:literal) => {
        #[doc = $doc]
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
        pub struct $name(pub Uuid);

        impl $name {
            pub fn new() -> Self {
                Self(Uuid::new_v4())
            }

            pub fn from_uuid(id: Uuid) -> Self {
                Self(id)
            }
        }

        impl Default for $name {
            fn default() -> Self {
                Self::new()
            }
        }

        impl std::fmt::Display for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                write!(f, "{}", self.0)
            }
        }
    };
}

define_id!(TenantId, "Identifies a tenant in multi-tenant mode.");
define_id!(WorkspaceId, "Identifies a workspace within a tenant.");
define_id!(UserId, "Identifies a user within a workspace.");
define_id!(ChannelId, "Identifies a communication channel.");
define_id!(ConnectorId, "Identifies a connector instance.");
define_id!(AgentId, "Identifies an agent instance.");
define_id!(SessionId, "Identifies an agent session.");

/// Deployment mode determines database backend and isolation behavior.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DeploymentMode {
    /// Single-user, SQLite-backed, one workspace.
    Solo,
    /// Shared workspace with role boundaries, SQLite-backed.
    Household,
    /// Multi-tenant, PostgreSQL-backed with per-workspace isolation.
    Team,
}

/// Agent roles in the IronGolem OS system.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AgentRole {
    Planner,
    Executor,
    Verifier,
    Researcher,
    Defender,
    Healer,
    Optimizer,
    Narrator,
    Router,
}

impl std::fmt::Display for AgentRole {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let name = match self {
            Self::Planner => "Planner",
            Self::Executor => "Executor",
            Self::Verifier => "Verifier",
            Self::Researcher => "Researcher",
            Self::Defender => "Defender",
            Self::Healer => "Healer",
            Self::Optimizer => "Optimizer",
            Self::Narrator => "Narrator",
            Self::Router => "Router",
        };
        write!(f, "{name}")
    }
}
