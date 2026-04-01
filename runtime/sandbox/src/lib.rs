//! # IronGolem Sandbox
//!
//! Tool isolation and resource limits. Each tool call runs in a constrained
//! environment with capability restrictions.

pub mod capability;
pub mod host;

pub use capability::{Capability, SandboxConfig};
pub use host::SandboxHost;
