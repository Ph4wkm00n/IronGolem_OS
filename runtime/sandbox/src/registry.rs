//! Tool registry. Maps tool names to handlers and built-in tools.
//!
//! v0.1 ships two built-in tools to prove the registry shape:
//! `echo` returns input verbatim, `http_get` makes a GET request against an
//! allow-listed destination from the SandboxConfig.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;
use irongolem_core::{Error, Result};
use serde_json::Value;

use crate::capability::{Capability, SandboxConfig};

/// A single executable tool with a name and capability needs.
#[async_trait]
pub trait Tool: Send + Sync {
    async fn invoke(&self, input: &Value, config: &SandboxConfig) -> Result<Value>;
}

/// Registry mapping tool names to handler implementations.
#[derive(Default)]
pub struct ToolRegistry {
    tools: HashMap<String, Arc<dyn Tool>>,
}

impl ToolRegistry {
    pub fn new() -> Self {
        Self::default()
    }

    /// Build a registry seeded with the v0.1 built-in tools.
    pub fn with_builtins() -> Self {
        let mut reg = Self::new();
        reg.register("echo", Arc::new(EchoTool));
        reg.register("http_get", Arc::new(HttpGetTool::default()));
        reg
    }

    pub fn register(&mut self, name: impl Into<String>, tool: Arc<dyn Tool>) {
        self.tools.insert(name.into(), tool);
    }

    pub fn get(&self, name: &str) -> Option<Arc<dyn Tool>> {
        self.tools.get(name).cloned()
    }

    pub fn names(&self) -> Vec<String> {
        let mut names: Vec<String> = self.tools.keys().cloned().collect();
        names.sort();
        names
    }
}

/// Echo tool: returns its input verbatim. Used as the canary tool for
/// integration tests proving the registry wiring works end-to-end.
pub struct EchoTool;

#[async_trait]
impl Tool for EchoTool {
    async fn invoke(&self, input: &Value, _config: &SandboxConfig) -> Result<Value> {
        Ok(input.clone())
    }
}

/// HTTP GET tool: makes a GET request against a URL that must appear in
/// `SandboxConfig.allowed_destinations`. Requires the NetworkAccess capability.
pub struct HttpGetTool {
    client: reqwest::Client,
}

impl Default for HttpGetTool {
    fn default() -> Self {
        Self {
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(15))
                .build()
                .expect("reqwest client builds with default settings"),
        }
    }
}

#[async_trait]
impl Tool for HttpGetTool {
    async fn invoke(&self, input: &Value, config: &SandboxConfig) -> Result<Value> {
        if !config.capabilities.contains(&Capability::NetworkAccess) {
            return Err(Error::Sandbox {
                reason: "http_get requires NetworkAccess capability".into(),
            });
        }

        let url = input
            .get("url")
            .and_then(|u| u.as_str())
            .ok_or_else(|| Error::Sandbox {
                reason: "http_get requires 'url' string input".into(),
            })?;

        if !config
            .allowed_destinations
            .iter()
            .any(|allowed| url.starts_with(allowed))
        {
            return Err(Error::Sandbox {
                reason: format!("destination not in allowlist: {url}"),
            });
        }

        let resp = self
            .client
            .get(url)
            .send()
            .await
            .map_err(|e| Error::Sandbox {
                reason: format!("http_get request failed: {e}"),
            })?;
        let status = resp.status().as_u16();
        let body = resp.text().await.map_err(|e| Error::Sandbox {
            reason: format!("http_get body read failed: {e}"),
        })?;

        Ok(serde_json::json!({
            "status": status,
            "body": body,
        }))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn echo_returns_input_verbatim() {
        let reg = ToolRegistry::with_builtins();
        let tool = reg.get("echo").expect("echo registered");
        let input = serde_json::json!({"hello": "world"});
        let out = tool
            .invoke(&input, &SandboxConfig::default())
            .await
            .unwrap();
        assert_eq!(out, input);
    }

    #[tokio::test]
    async fn unknown_tool_returns_none() {
        let reg = ToolRegistry::with_builtins();
        assert!(reg.get("does_not_exist").is_none());
    }

    #[tokio::test]
    async fn http_get_rejects_without_network_capability() {
        let tool = HttpGetTool::default();
        let cfg = SandboxConfig::default();
        let input = serde_json::json!({"url": "https://example.com"});
        let err = tool.invoke(&input, &cfg).await.unwrap_err();
        assert!(matches!(err, Error::Sandbox { .. }));
    }

    #[tokio::test]
    async fn http_get_rejects_disallowed_destination() {
        let tool = HttpGetTool::default();
        let cfg = SandboxConfig {
            capabilities: vec![Capability::NetworkAccess],
            allowed_destinations: vec!["https://allowed.example.com".into()],
            ..SandboxConfig::default()
        };
        let input = serde_json::json!({"url": "https://other.example.com/foo"});
        let err = tool.invoke(&input, &cfg).await.unwrap_err();
        match err {
            Error::Sandbox { reason } => assert!(reason.contains("not in allowlist")),
            other => panic!("expected Sandbox error, got {other:?}"),
        }
    }

    #[tokio::test]
    async fn registry_lists_builtin_names() {
        let reg = ToolRegistry::with_builtins();
        let names = reg.names();
        assert!(names.contains(&"echo".to_string()));
        assert!(names.contains(&"http_get".to_string()));
    }
}
