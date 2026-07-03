//! IPC wire types for the gateway <-> runtimed NDJSON protocol.
//!
//! Each message is a single line of JSON on stdin/stdout. Requests carry a
//! `request_id` so the gateway can correlate streamed events and a terminal
//! response against the original call.
//!
//! Shape rules (so v0.3 gRPC migration is mechanical):
//! - Flat structs, no enum-with-data on the wire (use a `kind` string + payload).
//! - All ids are UUIDs as strings (serde handles this via Uuid's serde feature).
//! - No optional fields where a stable default exists; prefer empty strings/arrays.

use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::event::Event;
use crate::plan::Plan;
use crate::types::WorkspaceId;

/// Envelope wrapping every NDJSON line. Tagged so the receiver can dispatch.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "kind")]
pub enum Message {
    #[serde(rename = "execute_plan_request")]
    ExecutePlanRequest(ExecutePlanRequest),
    #[serde(rename = "execute_plan_response")]
    ExecutePlanResponse(ExecutePlanResponse),
    #[serde(rename = "event_notification")]
    EventNotification(EventNotification),
    #[serde(rename = "ping_request")]
    PingRequest(PingRequest),
    #[serde(rename = "ping_response")]
    PingResponse(PingResponse),
    /// v0.3 Step 3 — gateway asks runtimed to enumerate provider profiles.
    #[serde(rename = "list_providers_request")]
    ListProvidersRequest(ListProvidersRequest),
    #[serde(rename = "list_providers_response")]
    ListProvidersResponse(ListProvidersResponse),
    /// v0.4 — direct single-turn LLM call for internal subsystems
    /// (commitments extraction). Deliberately NOT plan execution: no plan
    /// events pollute the timeline for hidden background classification.
    #[serde(rename = "llm_call_request")]
    LlmCallRequest(LlmCallRequest),
    #[serde(rename = "llm_call_response")]
    LlmCallResponse(LlmCallResponse),
    #[serde(rename = "shutdown")]
    Shutdown(Shutdown),
}

/// Gateway asks runtimed to execute a plan against a workspace.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutePlanRequest {
    pub request_id: Uuid,
    pub workspace_id: WorkspaceId,
    pub plan: Plan,
}

/// Terminal response for an ExecutePlanRequest.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutePlanResponse {
    pub request_id: Uuid,
    pub status: ExecutionStatus,
    /// Output of the final step in the plan, if any.
    pub output: Option<serde_json::Value>,
    /// Populated when status is Failed.
    pub error: Option<String>,
}

/// Streamed event during plan execution. Carries the request_id so the
/// gateway can route it to the correct in-flight request.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventNotification {
    pub request_id: Uuid,
    pub event: Event,
}

/// Lightweight liveness probe.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PingRequest {
    pub request_id: Uuid,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PingResponse {
    pub request_id: Uuid,
}

/// Graceful shutdown signal from the gateway.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Shutdown {
    pub request_id: Uuid,
}

/// v0.3 Step 3 — request: enumerate provider profiles known to runtimed.
///
/// The payload is intentionally empty (only `request_id`) so the wire
/// shape mirrors `PingRequest`. The response carries the active profile
/// alongside every other profile the binary knows how to instantiate,
/// so the settings UI can render "currently active" and "available"
/// without a separate query.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ListProvidersRequest {
    pub request_id: Uuid,
}

/// v0.3 Step 3 — response. Profiles arrive as opaque `serde_json::Value`
/// so `core` doesn't have to depend on the provider crate; the gateway
/// passes the payload straight through to the HTTP client.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ListProvidersResponse {
    pub request_id: Uuid,
    /// `name` field of the active provider (e.g. "anthropic"). Matches
    /// `ProviderKind` serialized lowercase form.
    pub active: String,
    /// Every known profile, including the active one. Order is stable
    /// across calls so the UI doesn't shuffle the list on refresh.
    pub profiles: Vec<serde_json::Value>,
}

/// v0.4 — request: one system+user turn against the active provider.
///
/// Used by gateway subsystems that need a model verdict without an agent
/// plan (the commitments LLM extractor is the first caller). `purpose`
/// tags the call for audit/event attribution; it is not sent to the model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LlmCallRequest {
    pub request_id: Uuid,
    pub workspace_id: WorkspaceId,
    /// Attribution tag, e.g. "commitments.extract". Never model-visible.
    pub purpose: String,
    /// System framing for the call. Empty string means provider default.
    pub system: String,
    /// The user-turn content.
    pub prompt: String,
    /// Hard output cap. 0 means the provider profile default.
    pub max_tokens: u32,
}

/// v0.4 — terminal response for an LlmCallRequest.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LlmCallResponse {
    pub request_id: Uuid,
    pub status: ExecutionStatus,
    /// Raw model text on success; empty on failure.
    pub content: String,
    /// Populated when status is Failed.
    pub error: Option<String>,
}

/// Terminal status of a plan execution.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ExecutionStatus {
    Completed,
    Failed,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::plan::{PlanNode, PlanNodeKind};
    use crate::types::AgentId;

    #[test]
    fn round_trip_execute_plan_request() {
        let workspace_id = WorkspaceId::new();
        let agent_id = AgentId::new();
        let mut plan = Plan::new("smoke", agent_id);
        plan.add_node(PlanNode::new(
            "echo",
            PlanNodeKind::ToolCall {
                tool_name: "echo".into(),
                input: serde_json::json!({"hello": "world"}),
            },
        ));

        let msg = Message::ExecutePlanRequest(ExecutePlanRequest {
            request_id: Uuid::new_v4(),
            workspace_id,
            plan,
        });

        let line = serde_json::to_string(&msg).unwrap();
        assert!(line.contains("\"kind\":\"execute_plan_request\""));
        let parsed: Message = serde_json::from_str(&line).unwrap();
        match parsed {
            Message::ExecutePlanRequest(req) => {
                assert_eq!(req.workspace_id, workspace_id);
                assert_eq!(req.plan.agent_id, agent_id);
            }
            other => panic!("expected ExecutePlanRequest, got {other:?}"),
        }
    }

    #[test]
    fn round_trip_llm_call() {
        let msg = Message::LlmCallRequest(LlmCallRequest {
            request_id: Uuid::new_v4(),
            workspace_id: WorkspaceId::new(),
            purpose: "commitments.extract".into(),
            system: "You are a hidden background classifier.".into(),
            prompt: "user said: I'll follow up tomorrow".into(),
            max_tokens: 1024,
        });
        let line = serde_json::to_string(&msg).unwrap();
        assert!(line.contains("\"kind\":\"llm_call_request\""));
        let parsed: Message = serde_json::from_str(&line).unwrap();
        match parsed {
            Message::LlmCallRequest(req) => {
                assert_eq!(req.purpose, "commitments.extract");
                assert_eq!(req.max_tokens, 1024);
            }
            other => panic!("expected LlmCallRequest, got {other:?}"),
        }

        let resp = Message::LlmCallResponse(LlmCallResponse {
            request_id: Uuid::new_v4(),
            status: ExecutionStatus::Completed,
            content: "{\"candidates\":[]}".into(),
            error: None,
        });
        let line = serde_json::to_string(&resp).unwrap();
        assert!(line.contains("\"kind\":\"llm_call_response\""));
        assert!(serde_json::from_str::<Message>(&line).is_ok());
    }

    #[test]
    fn round_trip_response_with_error() {
        let msg = Message::ExecutePlanResponse(ExecutePlanResponse {
            request_id: Uuid::new_v4(),
            status: ExecutionStatus::Failed,
            output: None,
            error: Some("boom".into()),
        });
        let line = serde_json::to_string(&msg).unwrap();
        let parsed: Message = serde_json::from_str(&line).unwrap();
        match parsed {
            Message::ExecutePlanResponse(resp) => {
                assert_eq!(resp.status, ExecutionStatus::Failed);
                assert_eq!(resp.error.as_deref(), Some("boom"));
            }
            other => panic!("expected ExecutePlanResponse, got {other:?}"),
        }
    }
}
