//! LLM provider abstraction. The runtime owns LLM calls directly so step
//! execution stays a single layer.
//!
//! v0.1 ships two providers:
//! - `MockProvider` returns a configurable canned response (default `"pong"`).
//!   Used by integration tests and the smoke harness.
//! - `AnthropicProvider` makes a real Messages API call using `reqwest`.
//!
//! Selection is driven by the `IRONGOLEM_LLM_PROVIDER` env var.

use std::sync::Arc;

use async_trait::async_trait;
use irongolem_core::{Error, Result};
use serde::Deserialize;

#[async_trait]
pub trait LlmProvider: Send + Sync {
    /// Send a prompt and return the model's textual reply.
    async fn complete(&self, prompt: &str, model: Option<&str>) -> Result<String>;
}

#[derive(Debug, Clone)]
pub struct ProviderConfig {
    pub kind: ProviderKind,
    pub api_key: Option<String>,
    pub default_model: String,
    pub mock_response: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ProviderKind {
    Mock,
    Anthropic,
}

impl ProviderConfig {
    /// Build a config from environment variables.
    pub fn from_env() -> Self {
        let kind = match std::env::var("IRONGOLEM_LLM_PROVIDER").as_deref() {
            Ok("anthropic") => ProviderKind::Anthropic,
            // Default mock keeps the binary safe to run without credentials.
            _ => ProviderKind::Mock,
        };
        Self {
            kind,
            api_key: std::env::var("ANTHROPIC_API_KEY").ok(),
            default_model: std::env::var("IRONGOLEM_LLM_MODEL")
                .unwrap_or_else(|_| "claude-sonnet-4-6".into()),
            mock_response: std::env::var("IRONGOLEM_LLM_MOCK_RESPONSE")
                .unwrap_or_else(|_| "pong".into()),
        }
    }
}

pub fn build_provider(cfg: ProviderConfig) -> Arc<dyn LlmProvider> {
    match cfg.kind {
        ProviderKind::Mock => Arc::new(MockProvider {
            response: cfg.mock_response,
        }),
        ProviderKind::Anthropic => Arc::new(AnthropicProvider::new(
            cfg.api_key.unwrap_or_default(),
            cfg.default_model,
        )),
    }
}

/// Deterministic mock provider. Returns the same response for every prompt.
/// Used by integration tests and the smoke harness.
pub struct MockProvider {
    pub response: String,
}

#[async_trait]
impl LlmProvider for MockProvider {
    async fn complete(&self, _prompt: &str, _model: Option<&str>) -> Result<String> {
        Ok(self.response.clone())
    }
}

pub struct AnthropicProvider {
    client: reqwest::Client,
    api_key: String,
    default_model: String,
}

impl AnthropicProvider {
    pub fn new(api_key: String, default_model: String) -> Self {
        Self {
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(60))
                .build()
                .expect("reqwest client"),
            api_key,
            default_model,
        }
    }
}

#[async_trait]
impl LlmProvider for AnthropicProvider {
    async fn complete(&self, prompt: &str, model: Option<&str>) -> Result<String> {
        if self.api_key.is_empty() {
            return Err(Error::PlanExecution {
                reason: "ANTHROPIC_API_KEY not set".into(),
            });
        }

        let model = model.unwrap_or(&self.default_model);
        let body = serde_json::json!({
            "model": model,
            "max_tokens": 1024,
            "messages": [{ "role": "user", "content": prompt }],
        });

        let resp = self
            .client
            .post("https://api.anthropic.com/v1/messages")
            .header("x-api-key", &self.api_key)
            .header("anthropic-version", "2023-06-01")
            .header("content-type", "application/json")
            .json(&body)
            .send()
            .await
            .map_err(|e| Error::PlanExecution {
                reason: format!("anthropic request failed: {e}"),
            })?;

        let status = resp.status();
        let raw = resp.text().await.unwrap_or_default();
        if !status.is_success() {
            return Err(Error::PlanExecution {
                reason: format!("anthropic returned {status}: {raw}"),
            });
        }

        let parsed: MessagesResponse =
            serde_json::from_str(&raw).map_err(|e| Error::PlanExecution {
                reason: format!("anthropic response parse: {e}; body: {raw}"),
            })?;

        Ok(parsed
            .content
            .into_iter()
            .filter_map(|b| if b.kind == "text" { Some(b.text) } else { None })
            .collect::<Vec<_>>()
            .join("\n"))
    }
}

#[derive(Debug, Deserialize)]
struct MessagesResponse {
    content: Vec<MessageBlock>,
}

#[derive(Debug, Deserialize)]
struct MessageBlock {
    #[serde(rename = "type")]
    kind: String,
    #[serde(default)]
    text: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn mock_returns_configured_response() {
        let p = MockProvider {
            response: "pong".into(),
        };
        assert_eq!(p.complete("anything", None).await.unwrap(), "pong");
    }

    #[test]
    fn provider_config_defaults_to_mock() {
        // Set the env to mock to make this test hermetic in case the host
        // process already has it set.
        // Safety: setting environment variables is unsafe in Rust 2024 because
        // it can race with reads on other threads; this test is single-threaded
        // and runs before from_env reads the same variable.
        unsafe {
            std::env::set_var("IRONGOLEM_LLM_PROVIDER", "mock");
            std::env::set_var("IRONGOLEM_LLM_MOCK_RESPONSE", "echo");
        }
        let cfg = ProviderConfig::from_env();
        assert_eq!(cfg.kind, ProviderKind::Mock);
        assert_eq!(cfg.mock_response, "echo");
    }
}
