//! Sandbox host manages sandboxed execution environments for tools.
//! Future: Will support WASM plugin execution.

use std::sync::Arc;

use async_trait::async_trait;
use irongolem_core::{Error, Result};

use crate::capability::SandboxConfig;
use crate::registry::ToolRegistry;

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
pub struct LocalSandboxHost {
    registry: Arc<ToolRegistry>,
}

impl LocalSandboxHost {
    /// Construct a host backed by the supplied registry.
    pub fn new(registry: Arc<ToolRegistry>) -> Self {
        Self { registry }
    }

    /// Construct a host seeded with the v0.1 built-in tools.
    pub fn with_builtins() -> Self {
        Self::new(Arc::new(ToolRegistry::with_builtins()))
    }

    pub fn registry(&self) -> Arc<ToolRegistry> {
        Arc::clone(&self.registry)
    }
}

#[async_trait]
impl SandboxHost for LocalSandboxHost {
    async fn execute(
        &self,
        tool_name: &str,
        input: &serde_json::Value,
        config: &SandboxConfig,
    ) -> Result<serde_json::Value> {
        tracing::info!(tool = tool_name, "Executing tool in local sandbox");
        let tool = self.registry.get(tool_name).ok_or_else(|| Error::Sandbox {
            reason: format!("tool not registered: {tool_name}"),
        })?;
        tool.invoke(input, config).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn registered_tool_dispatches() {
        let host = LocalSandboxHost::with_builtins();
        let input = serde_json::json!({"a": 1});
        let out = host
            .execute("echo", &input, &SandboxConfig::default())
            .await
            .unwrap();
        assert_eq!(out, input);
    }

    #[tokio::test]
    async fn unregistered_tool_errors() {
        let host = LocalSandboxHost::with_builtins();
        let err = host
            .execute("nope", &serde_json::Value::Null, &SandboxConfig::default())
            .await
            .unwrap_err();
        match err {
            Error::Sandbox { reason } => assert!(reason.contains("not registered")),
            other => panic!("expected Sandbox error, got {other:?}"),
        }
    }
}
