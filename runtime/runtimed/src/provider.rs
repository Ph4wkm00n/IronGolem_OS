//! LLM provider abstraction. The runtime owns LLM calls directly so step
//! execution stays a single layer.
//!
//! ## v0.3 Step 3 — ProviderProfile + OpenAI as Profile #2
//!
//! Per `Plans/modular-puzzling-blum.md` Step 3, every provider now carries
//! a declarative [`ProviderProfile`] that captures the API surface
//! (auth type, base URL, static headers, temperature default, fallback
//! models). Adopted from `NousResearch/hermes-agent`
//! `providers/base.py:ProviderProfile` — the same dataclass that
//! "replaces 20+ boolean flags in the transport".
//!
//! The trait keeps the same `complete()` signature; the addition is
//! `profile()` so the gateway can list providers and the UI can render
//! provider cards without reaching into provider-specific code.
//!
//! Providers shipped in v0.3:
//! - `MockProvider`        — canned response, used by tests + smoke.
//! - `AnthropicProvider`   — `/v1/messages` (existing path, Profile #1).
//! - `OpenAIProvider`      — `/v1/chat/completions` (new, Profile #2).
//!
//! Selection is driven by `IRONGOLEM_LLM_PROVIDER` env var
//! (`mock | anthropic | openai`). Mock stays the default so the binary
//! is safe to run without credentials.

use std::collections::BTreeMap;
use std::sync::Arc;

use async_trait::async_trait;
use irongolem_core::{Error, Result};
use serde::{Deserialize, Serialize};

/// Declarative provider metadata. Serialized to the gateway via the
/// `ListProviders` IPC verb so the UI can render provider cards + the
/// settings handler can show "currently active" without poking through
/// provider-specific code.
///
/// Secrets are **never** populated here. `default_headers` is for
/// static, non-sensitive values (e.g. Anthropic's `anthropic-version`).
/// Auth headers go on the request at send-time using credentials read
/// from env.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderProfile {
    /// Stable wire identifier — matches `ProviderKind` serialized form.
    /// Example: `"anthropic"`, `"openai"`, `"mock"`.
    pub name: String,
    /// Human-readable label for the settings UI.
    pub display_name: String,
    /// How the provider authenticates. Only `ApiKey` is wired in v0.3;
    /// `OAuth` and `Bedrock` are reserved so we don't have to bump the
    /// schema when they land.
    pub auth_type: AuthType,
    /// Inference base URL. Empty string for `mock`.
    pub base_url: String,
    /// Optional explicit models endpoint. When unset, callers fall back
    /// to `{base_url}/models`.
    pub models_url: Option<String>,
    /// Static, non-sensitive headers sent on every request. Use BTreeMap
    /// so the serialized JSON is deterministic for test fixtures.
    pub default_headers: BTreeMap<String, String>,
    /// Default temperature. `None` means "do not send the field" — some
    /// providers refuse the parameter (Kimi/Moonshot) and others apply
    /// their own default when omitted. Adopted semantics from hermes-
    /// agent's `OMIT_TEMPERATURE` sentinel.
    pub fixed_temperature: Option<f32>,
    /// Default `max_tokens` value. Kept conservative so smoke tests
    /// don't spend more than ~$0.001 per run.
    pub default_max_tokens: u32,
    /// Curated model list shown in the UI when live `models_url` fetch
    /// fails. v0.3 ships a small, opinionated set per provider.
    pub fallback_models: Vec<String>,
    /// Name of the env var that holds the API key. Reported by the
    /// settings UI so the operator knows what to set; the secret itself
    /// is never serialized.
    pub api_key_env: String,
}

/// Authentication mode declared by a `ProviderProfile`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthType {
    /// Bearer or `x-api-key` style; secret read from env at boot.
    ApiKey,
    /// OAuth2 device-code / authorization-code flow. Not wired in v0.3.
    Oauth,
    /// AWS SDK SigV4-signed (Bedrock). Not wired in v0.3.
    Bedrock,
    /// No authentication — only used by `MockProvider`.
    None,
}

#[async_trait]
pub trait LlmProvider: Send + Sync {
    /// Send a prompt and return the model's textual reply.
    async fn complete(&self, prompt: &str, model: Option<&str>) -> Result<String>;

    /// Static metadata describing this provider. The gateway and UI
    /// read this via the `ListProviders` IPC verb.
    fn profile(&self) -> &ProviderProfile;
}

#[derive(Debug, Clone)]
pub struct ProviderConfig {
    pub kind: ProviderKind,
    /// Active credential when [`Self::kind`] is a real provider. Kept
    /// outside [`ProviderProfile`] so secrets never serialize.
    pub api_key: Option<String>,
    pub default_model: String,
    pub mock_response: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ProviderKind {
    Mock,
    Anthropic,
    OpenAi,
}

impl ProviderKind {
    /// The env var holding the API key for this provider. Used by the
    /// settings UI to surface what's missing without leaking the value.
    pub fn api_key_env(self) -> &'static str {
        match self {
            ProviderKind::Mock => "",
            ProviderKind::Anthropic => "ANTHROPIC_API_KEY",
            ProviderKind::OpenAi => "OPENAI_API_KEY",
        }
    }

    /// Default model id when `IRONGOLEM_LLM_MODEL` is unset.
    fn default_model(self) -> &'static str {
        match self {
            ProviderKind::Mock => "mock",
            ProviderKind::Anthropic => "claude-sonnet-4-6",
            ProviderKind::OpenAi => "gpt-4o-mini",
        }
    }
}

impl ProviderConfig {
    /// Build a config from environment variables.
    ///
    /// `IRONGOLEM_LLM_PROVIDER` selects the kind. The corresponding API
    /// key env var is read lazily; if a real provider is selected but
    /// its key isn't set, `complete()` returns a clean
    /// `Error::PlanExecution` instead of panicking at boot.
    pub fn from_env() -> Self {
        let kind = match std::env::var("IRONGOLEM_LLM_PROVIDER").as_deref() {
            Ok("anthropic") => ProviderKind::Anthropic,
            Ok("openai") => ProviderKind::OpenAi,
            // Default mock keeps the binary safe to run without credentials.
            _ => ProviderKind::Mock,
        };
        let api_key = match kind {
            ProviderKind::Mock => None,
            other => std::env::var(other.api_key_env()).ok(),
        };
        Self {
            kind,
            api_key,
            default_model: std::env::var("IRONGOLEM_LLM_MODEL")
                .unwrap_or_else(|_| kind.default_model().into()),
            mock_response: std::env::var("IRONGOLEM_LLM_MOCK_RESPONSE")
                .unwrap_or_else(|_| "pong".into()),
        }
    }
}

pub fn build_provider(cfg: ProviderConfig) -> Arc<dyn LlmProvider> {
    match cfg.kind {
        ProviderKind::Mock => Arc::new(MockProvider::new(cfg.mock_response)),
        ProviderKind::Anthropic => Arc::new(AnthropicProvider::new(
            cfg.api_key.unwrap_or_default(),
            cfg.default_model,
        )),
        ProviderKind::OpenAi => Arc::new(OpenAiProvider::new(
            cfg.api_key.unwrap_or_default(),
            cfg.default_model,
        )),
    }
}

/// Builds every known provider profile (active key not required). Used
/// by the `ListProviders` IPC verb so the UI can show the operator
/// "anthropic and openai exist; openai is missing OPENAI_API_KEY".
pub fn all_known_profiles() -> Vec<ProviderProfile> {
    vec![
        mock_profile("pong"),
        anthropic_profile("claude-sonnet-4-6"),
        openai_profile("gpt-4o-mini"),
    ]
}

// ---------------------------------------------------------------------
// Mock provider
// ---------------------------------------------------------------------

/// Deterministic mock provider. Returns the same response for every prompt.
/// Used by integration tests and the smoke harness.
pub struct MockProvider {
    profile: ProviderProfile,
    response: String,
}

impl MockProvider {
    pub fn new(response: impl Into<String>) -> Self {
        Self {
            profile: mock_profile(""),
            response: response.into(),
        }
    }
}

#[async_trait]
impl LlmProvider for MockProvider {
    async fn complete(&self, _prompt: &str, _model: Option<&str>) -> Result<String> {
        Ok(self.response.clone())
    }
    fn profile(&self) -> &ProviderProfile {
        &self.profile
    }
}

fn mock_profile(_default_model: &str) -> ProviderProfile {
    ProviderProfile {
        name: "mock".into(),
        display_name: "Mock (canned response)".into(),
        auth_type: AuthType::None,
        base_url: String::new(),
        models_url: None,
        default_headers: BTreeMap::new(),
        fixed_temperature: None,
        default_max_tokens: 1024,
        fallback_models: vec!["mock".into()],
        api_key_env: String::new(),
    }
}

// ---------------------------------------------------------------------
// Anthropic provider
// ---------------------------------------------------------------------

pub struct AnthropicProvider {
    profile: ProviderProfile,
    client: reqwest::Client,
    api_key: String,
    default_model: String,
}

impl AnthropicProvider {
    pub fn new(api_key: String, default_model: String) -> Self {
        Self {
            profile: anthropic_profile(&default_model),
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(60))
                .build()
                .expect("reqwest client"),
            api_key,
            default_model,
        }
    }
}

fn anthropic_profile(default_model: &str) -> ProviderProfile {
    let mut headers = BTreeMap::new();
    headers.insert("anthropic-version".into(), "2023-06-01".into());
    headers.insert("content-type".into(), "application/json".into());
    ProviderProfile {
        name: "anthropic".into(),
        display_name: "Anthropic Claude".into(),
        auth_type: AuthType::ApiKey,
        base_url: "https://api.anthropic.com/v1".into(),
        models_url: None,
        default_headers: headers,
        fixed_temperature: None,
        default_max_tokens: 1024,
        fallback_models: vec![
            "claude-sonnet-4-6".into(),
            "claude-haiku-4-5-20251001".into(),
            "claude-opus-4-7".into(),
            default_model.to_string(),
        ]
        .into_iter()
        .collect::<std::collections::BTreeSet<_>>()
        .into_iter()
        .collect(),
        api_key_env: "ANTHROPIC_API_KEY".into(),
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
            "max_tokens": self.profile.default_max_tokens,
            "messages": [{ "role": "user", "content": prompt }],
        });

        let mut req = self
            .client
            .post(format!("{}/messages", self.profile.base_url))
            .header("x-api-key", &self.api_key);
        for (k, v) in &self.profile.default_headers {
            req = req.header(k, v);
        }

        let resp = req.json(&body).send().await.map_err(|e| Error::PlanExecution {
            reason: format!("anthropic request failed: {e}"),
        })?;

        let status = resp.status();
        let raw = resp.text().await.unwrap_or_default();
        if !status.is_success() {
            return Err(Error::PlanExecution {
                reason: format!("anthropic returned {status}: {raw}"),
            });
        }

        let parsed: AnthropicMessagesResponse =
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
    fn profile(&self) -> &ProviderProfile {
        &self.profile
    }
}

#[derive(Debug, Deserialize)]
struct AnthropicMessagesResponse {
    content: Vec<AnthropicMessageBlock>,
}

#[derive(Debug, Deserialize)]
struct AnthropicMessageBlock {
    #[serde(rename = "type")]
    kind: String,
    #[serde(default)]
    text: String,
}

// ---------------------------------------------------------------------
// OpenAI provider (Profile #2, v0.3 Step 3)
// ---------------------------------------------------------------------

pub struct OpenAiProvider {
    profile: ProviderProfile,
    client: reqwest::Client,
    api_key: String,
    default_model: String,
}

impl OpenAiProvider {
    pub fn new(api_key: String, default_model: String) -> Self {
        Self {
            profile: openai_profile(&default_model),
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(60))
                .build()
                .expect("reqwest client"),
            api_key,
            default_model,
        }
    }
}

fn openai_profile(default_model: &str) -> ProviderProfile {
    let mut headers = BTreeMap::new();
    headers.insert("content-type".into(), "application/json".into());
    ProviderProfile {
        name: "openai".into(),
        display_name: "OpenAI".into(),
        auth_type: AuthType::ApiKey,
        base_url: "https://api.openai.com/v1".into(),
        models_url: None,
        default_headers: headers,
        // OpenAI accepts temperature; smoke runs the call with the
        // provider default to keep cost down and replies deterministic-
        // enough for the smoke assertion.
        fixed_temperature: Some(0.2),
        default_max_tokens: 1024,
        fallback_models: vec![
            "gpt-4o-mini".into(),
            "gpt-4o".into(),
            "gpt-4.1-mini".into(),
            default_model.to_string(),
        ]
        .into_iter()
        .collect::<std::collections::BTreeSet<_>>()
        .into_iter()
        .collect(),
        api_key_env: "OPENAI_API_KEY".into(),
    }
}

#[async_trait]
impl LlmProvider for OpenAiProvider {
    async fn complete(&self, prompt: &str, model: Option<&str>) -> Result<String> {
        if self.api_key.is_empty() {
            return Err(Error::PlanExecution {
                reason: "OPENAI_API_KEY not set".into(),
            });
        }

        let model = model.unwrap_or(&self.default_model);
        let mut body = serde_json::json!({
            "model": model,
            "max_tokens": self.profile.default_max_tokens,
            "messages": [{ "role": "user", "content": prompt }],
        });
        if let Some(temp) = self.profile.fixed_temperature {
            body["temperature"] = serde_json::json!(temp);
        }

        let mut req = self
            .client
            .post(format!("{}/chat/completions", self.profile.base_url))
            .bearer_auth(&self.api_key);
        for (k, v) in &self.profile.default_headers {
            req = req.header(k, v);
        }

        let resp = req.json(&body).send().await.map_err(|e| Error::PlanExecution {
            reason: format!("openai request failed: {e}"),
        })?;

        let status = resp.status();
        let raw = resp.text().await.unwrap_or_default();
        if !status.is_success() {
            return Err(Error::PlanExecution {
                reason: format!("openai returned {status}: {raw}"),
            });
        }

        let parsed: OpenAiChatResponse =
            serde_json::from_str(&raw).map_err(|e| Error::PlanExecution {
                reason: format!("openai response parse: {e}; body: {raw}"),
            })?;

        // OpenAI always returns at least one choice on success; defensive
        // join just in case a future API revision returns multiple.
        Ok(parsed
            .choices
            .into_iter()
            .map(|c| c.message.content)
            .collect::<Vec<_>>()
            .join("\n"))
    }
    fn profile(&self) -> &ProviderProfile {
        &self.profile
    }
}

#[derive(Debug, Deserialize)]
struct OpenAiChatResponse {
    choices: Vec<OpenAiChatChoice>,
}

#[derive(Debug, Deserialize)]
struct OpenAiChatChoice {
    message: OpenAiChatMessage,
}

#[derive(Debug, Deserialize)]
struct OpenAiChatMessage {
    #[serde(default)]
    content: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn mock_returns_configured_response() {
        let p = MockProvider::new("pong");
        assert_eq!(p.complete("anything", None).await.unwrap(), "pong");
    }

    #[test]
    fn mock_profile_shape() {
        let p = MockProvider::new("x");
        let prof = p.profile();
        assert_eq!(prof.name, "mock");
        assert_eq!(prof.auth_type, AuthType::None);
        assert!(prof.api_key_env.is_empty());
    }

    #[test]
    fn anthropic_profile_shape() {
        let p = AnthropicProvider::new("dummy".into(), "claude-sonnet-4-6".into());
        let prof = p.profile();
        assert_eq!(prof.name, "anthropic");
        assert_eq!(prof.auth_type, AuthType::ApiKey);
        assert_eq!(prof.api_key_env, "ANTHROPIC_API_KEY");
        assert_eq!(prof.base_url, "https://api.anthropic.com/v1");
        assert!(prof.default_headers.contains_key("anthropic-version"));
        assert!(prof.fallback_models.contains(&"claude-sonnet-4-6".to_string()));
    }

    #[test]
    fn openai_profile_shape() {
        let p = OpenAiProvider::new("dummy".into(), "gpt-4o-mini".into());
        let prof = p.profile();
        assert_eq!(prof.name, "openai");
        assert_eq!(prof.auth_type, AuthType::ApiKey);
        assert_eq!(prof.api_key_env, "OPENAI_API_KEY");
        assert_eq!(prof.base_url, "https://api.openai.com/v1");
        assert_eq!(prof.fixed_temperature, Some(0.2));
        assert!(prof.fallback_models.iter().any(|m| m.starts_with("gpt-")));
    }

    #[test]
    fn provider_config_defaults_to_mock() {
        // Safety: setting env is unsafe in 2024 edition because it can race
        // with reads on other threads; this test is single-threaded.
        unsafe {
            std::env::set_var("IRONGOLEM_LLM_PROVIDER", "mock");
            std::env::set_var("IRONGOLEM_LLM_MOCK_RESPONSE", "echo");
        }
        let cfg = ProviderConfig::from_env();
        assert_eq!(cfg.kind, ProviderKind::Mock);
        assert_eq!(cfg.mock_response, "echo");
    }

    #[test]
    fn provider_config_routes_openai() {
        unsafe {
            std::env::set_var("IRONGOLEM_LLM_PROVIDER", "openai");
            std::env::set_var("OPENAI_API_KEY", "sk-test");
            std::env::remove_var("IRONGOLEM_LLM_MODEL");
        }
        let cfg = ProviderConfig::from_env();
        assert_eq!(cfg.kind, ProviderKind::OpenAi);
        assert_eq!(cfg.api_key.as_deref(), Some("sk-test"));
        assert_eq!(cfg.default_model, "gpt-4o-mini");
        // Restore so the previous test doesn't see the openai routing.
        unsafe {
            std::env::set_var("IRONGOLEM_LLM_PROVIDER", "mock");
            std::env::remove_var("OPENAI_API_KEY");
        }
    }

    #[test]
    fn provider_kind_api_key_env() {
        assert_eq!(ProviderKind::Anthropic.api_key_env(), "ANTHROPIC_API_KEY");
        assert_eq!(ProviderKind::OpenAi.api_key_env(), "OPENAI_API_KEY");
        assert_eq!(ProviderKind::Mock.api_key_env(), "");
    }

    #[test]
    fn provider_kind_wire_format_lowercase() {
        // The IPC wire encodes kind as a lowercase string. Both the
        // settings UI and the gateway dispatch on these strings, so any
        // drift here breaks the cross-language contract.
        assert_eq!(serde_json::to_string(&ProviderKind::Mock).unwrap(), "\"mock\"");
        assert_eq!(serde_json::to_string(&ProviderKind::Anthropic).unwrap(), "\"anthropic\"");
        assert_eq!(serde_json::to_string(&ProviderKind::OpenAi).unwrap(), "\"openai\"");
    }

    #[test]
    fn all_known_profiles_includes_three() {
        let profs = all_known_profiles();
        let names: Vec<&str> = profs.iter().map(|p| p.name.as_str()).collect();
        assert!(names.contains(&"mock"));
        assert!(names.contains(&"anthropic"));
        assert!(names.contains(&"openai"));
    }

    #[test]
    fn anthropic_no_api_key_returns_error() {
        let p = AnthropicProvider::new(String::new(), "claude-sonnet-4-6".into());
        let rt = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap();
        let err = rt.block_on(p.complete("hi", None)).unwrap_err();
        assert!(err.to_string().contains("ANTHROPIC_API_KEY"));
    }

    #[test]
    fn openai_no_api_key_returns_error() {
        let p = OpenAiProvider::new(String::new(), "gpt-4o-mini".into());
        let rt = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap();
        let err = rt.block_on(p.complete("hi", None)).unwrap_err();
        assert!(err.to_string().contains("OPENAI_API_KEY"));
    }
}
