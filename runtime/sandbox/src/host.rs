//! Sandbox host manages sandboxed execution environments for tools.
//! Future: Will support WASM plugin execution.

use async_trait::async_trait;
use irongolem_core::Result;

use crate::capability::SandboxConfig;

/// Trait for sandbox hosts that execute tools in isolated environments.
#[async_trait]
pub trait SandboxHost: Send + Sync {
    /// Execute a tool call within a sandboxed environment.
    async fn execute(
        &self,
        tool_name: &str,
        input: &serde_json::Value,
        config: &SandboxConfig,
    ) -> Result<serde_json::Value>;
}

/// A local sandbox host that runs tools in the current process with
/// capability checks but without full isolation. Suitable for solo mode.
pub struct LocalSandboxHost;

#[async_trait]
impl SandboxHost for LocalSandboxHost {
    async fn execute(
        &self,
        tool_name: &str,
        input: &serde_json::Value,
        _config: &SandboxConfig,
    ) -> Result<serde_json::Value> {
        tracing::info!(tool = tool_name, "Executing tool in local sandbox");
        // Stub: real implementation will dispatch to registered tool handlers
        Ok(serde_json::json!({
            "tool": tool_name,
            "input": input,
            "status": "executed",
        }))
    }
}
