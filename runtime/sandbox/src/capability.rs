//! Capability definitions for sandboxed tool execution.

use serde::{Deserialize, Serialize};

/// A capability that a sandboxed tool can request.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Capability {
    /// Read from the local filesystem.
    FileRead,
    /// Write to the local filesystem.
    FileWrite,
    /// Make outbound network requests.
    NetworkAccess,
    /// Execute shell commands.
    ShellExecution,
    /// Access environment variables.
    EnvAccess,
    /// Access database directly.
    DatabaseAccess,
    /// Send messages through connectors.
    ConnectorSend,
    /// Custom capability with a name.
    Custom(String),
}

/// Configuration for a sandboxed execution environment.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SandboxConfig {
    /// Capabilities granted to this sandbox.
    pub capabilities: Vec<Capability>,
    /// Maximum execution time in milliseconds.
    pub timeout_ms: u64,
    /// Maximum memory usage in bytes.
    pub max_memory_bytes: u64,
    /// Allowed network destinations (when NetworkAccess is granted).
    pub allowed_destinations: Vec<String>,
    /// Denied shell patterns (when ShellExecution is granted).
    pub denied_commands: Vec<String>,
}

impl Default for SandboxConfig {
    fn default() -> Self {
        Self {
            capabilities: Vec::new(),
            timeout_ms: 30_000,
            max_memory_bytes: 256 * 1024 * 1024, // 256 MB
            allowed_destinations: Vec::new(),
            denied_commands: vec![
                "rm -rf".to_string(),
                "mkfs".to_string(),
                "dd if=".to_string(),
                ":(){ :|:& };:".to_string(),
            ],
        }
    }
}
