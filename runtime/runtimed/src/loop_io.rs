//! NDJSON request/response handling. Keeps stdin/stdout out of the unit
//! tests by accepting an async writer for outbound messages and pure data
//! for the inbound request.

use std::sync::Arc;

use irongolem_core::{
    Error,
    ipc::{
        EventNotification, ExecutePlanRequest, ExecutePlanResponse, ExecutionStatus,
        ListProvidersRequest, ListProvidersResponse, LlmCallRequest, LlmCallResponse, Message,
        PingRequest, PingResponse,
    },
};
use irongolem_workflow::PlanEngine;
use tokio::io::AsyncWriteExt;
use tokio::sync::Mutex;
use uuid::Uuid;

use crate::executor::RealStepExecutor;
use crate::provider::{LlmProvider, all_known_profiles};

/// Result for the test harness: what was emitted in order while handling a
/// request, paired with the terminal response.
#[derive(Debug)]
pub struct ProcessResult {
    pub events: Vec<EventNotification>,
    pub response: ExecutePlanResponse,
}

/// Execute one ExecutePlanRequest against a fresh PlanEngine and return the
/// emitted events plus the terminal response.
///
/// v0.1 emits events after the plan completes (or fails). True streaming
/// during execution is future work — see the PlanEngine event-log refactor
/// note in the plan file.
pub async fn process_request(
    req: ExecutePlanRequest,
    executor: Arc<RealStepExecutor>,
) -> ProcessResult {
    let mut plan = req.plan;
    let request_id = req.request_id;
    let workspace_id = req.workspace_id;

    let engine = PlanEngine::new(executor);
    let exec_result = engine.execute(&mut plan, workspace_id).await;
    let runtime_events = engine.events().await;

    let events: Vec<EventNotification> = runtime_events
        .into_iter()
        .map(|event| EventNotification { request_id, event })
        .collect();

    let response = match exec_result {
        Ok(()) => {
            // Take the output of the last completed node, if any.
            let output = plan.nodes.last().and_then(|n| n.output.clone());
            ExecutePlanResponse {
                request_id,
                status: ExecutionStatus::Completed,
                output,
                error: None,
            }
        }
        Err(e) => ExecutePlanResponse {
            request_id,
            status: ExecutionStatus::Failed,
            output: None,
            error: Some(e.to_string()),
        },
    };

    ProcessResult { events, response }
}

/// Write a `Message` as a single NDJSON line followed by `\n`.
pub async fn write_message<W: AsyncWriteExt + Unpin>(
    w: &Arc<Mutex<W>>,
    msg: &Message,
) -> Result<(), Error> {
    let line = serde_json::to_string(msg).map_err(Error::from)?;
    let mut guard = w.lock().await;
    guard
        .write_all(line.as_bytes())
        .await
        .map_err(|e| Error::Internal(format!("stdout write: {e}")))?;
    guard
        .write_all(b"\n")
        .await
        .map_err(|e| Error::Internal(format!("stdout newline: {e}")))?;
    guard
        .flush()
        .await
        .map_err(|e| Error::Internal(format!("stdout flush: {e}")))?;
    Ok(())
}

/// Build the response for a Ping.
pub fn ping_response(req: &PingRequest) -> PingResponse {
    PingResponse {
        request_id: req.request_id,
    }
}

/// Build the response for a ListProviders. The active provider is the
/// one the binary booted with; the profile list covers every provider
/// the binary can instantiate so the UI can render available choices.
pub fn list_providers_response(
    req: &ListProvidersRequest,
    active: &dyn LlmProvider,
) -> Result<ListProvidersResponse, Error> {
    let profiles = all_known_profiles()
        .into_iter()
        .map(|p| serde_json::to_value(&p).map_err(Error::from))
        .collect::<Result<Vec<_>, _>>()?;
    Ok(ListProvidersResponse {
        request_id: req.request_id,
        active: active.profile().name.clone(),
        profiles,
    })
}

/// Handle a v0.4 LlmCallRequest: one system+user turn against the active
/// provider, no plan events. Failures come back as a Failed response —
/// never a dropped request — so the gateway's extractor can degrade to
/// its heuristic path deterministically.
///
/// The current `LlmProvider::complete` surface takes a single prompt
/// string; a non-empty `system` is prepended as a framing block. When the
/// trait grows a structured system/messages parameter this is the one
/// call site to update. `max_tokens` defers to the provider profile
/// default for the same reason.
pub async fn llm_call_response(
    req: &LlmCallRequest,
    provider: &dyn LlmProvider,
) -> LlmCallResponse {
    let prompt = if req.system.is_empty() {
        req.prompt.clone()
    } else {
        format!("{}\n\n{}", req.system, req.prompt)
    };

    match provider.complete(&prompt, None).await {
        Ok(content) => LlmCallResponse {
            request_id: req.request_id,
            status: ExecutionStatus::Completed,
            content,
            error: None,
        },
        Err(e) => LlmCallResponse {
            request_id: req.request_id,
            status: ExecutionStatus::Failed,
            content: String::new(),
            error: Some(e.to_string()),
        },
    }
}

/// Build a synthetic ExecutePlanResponse for an internal failure (e.g. a
/// malformed request that couldn't even be parsed). Used by the I/O loop
/// when no valid request_id is available; pass `Uuid::nil()` as a fallback.
pub fn error_response(request_id: Uuid, reason: impl Into<String>) -> ExecutePlanResponse {
    ExecutePlanResponse {
        request_id,
        status: ExecutionStatus::Failed,
        output: None,
        error: Some(reason.into()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use irongolem_core::{
        event::EventKind,
        plan::{Plan, PlanNode, PlanNodeKind},
        types::{AgentId, WorkspaceId},
    };
    use irongolem_sandbox::LocalSandboxHost;
    use std::sync::Arc;

    use crate::provider::MockProvider;

    fn make_executor() -> Arc<RealStepExecutor> {
        Arc::new(RealStepExecutor::new(
            Arc::new(LocalSandboxHost::with_builtins()),
            Arc::new(MockProvider::new("pong")),
        ))
    }

    #[test]
    fn list_providers_response_carries_active_and_all_profiles() {
        // v0.3 Step 3 — the IPC verb must surface the active provider name
        // plus every profile the binary knows how to instantiate. Settings
        // UI uses this for "currently active / available" rendering.
        let mock = MockProvider::new("pong");
        let req = ListProvidersRequest {
            request_id: Uuid::nil(),
        };
        let resp = list_providers_response(&req, &mock).expect("response builds");
        assert_eq!(resp.active, "mock");
        let names: Vec<String> = resp
            .profiles
            .iter()
            .map(|v| v["name"].as_str().unwrap_or("?").to_string())
            .collect();
        assert!(names.contains(&"mock".to_string()));
        assert!(names.contains(&"anthropic".to_string()));
        assert!(names.contains(&"openai".to_string()));
    }

    #[tokio::test]
    async fn echo_request_round_trip() {
        let workspace_id = WorkspaceId::new();
        let mut plan = Plan::new("echo plan", AgentId::new());
        plan.add_node(PlanNode::new(
            "echo",
            PlanNodeKind::ToolCall {
                tool_name: "echo".into(),
                input: serde_json::json!({"hello": "world"}),
            },
        ));

        let req = ExecutePlanRequest {
            request_id: Uuid::new_v4(),
            workspace_id,
            plan,
        };
        let result = process_request(req, make_executor()).await;

        assert_eq!(result.response.status, ExecutionStatus::Completed);
        assert_eq!(
            result.response.output.as_ref().unwrap(),
            &serde_json::json!({"hello": "world"})
        );

        // Events should include PlanCreated, PlanStepStarted, PlanStepCompleted, PlanCompleted in order.
        let kinds: Vec<&'static str> = result
            .events
            .iter()
            .map(|n| match n.event.kind {
                EventKind::PlanCreated { .. } => "PlanCreated",
                EventKind::PlanStepStarted { .. } => "PlanStepStarted",
                EventKind::PlanStepCompleted { .. } => "PlanStepCompleted",
                EventKind::PlanCompleted { .. } => "PlanCompleted",
                _ => "other",
            })
            .collect();
        assert_eq!(
            kinds,
            vec![
                "PlanCreated",
                "PlanStepStarted",
                "PlanStepCompleted",
                "PlanCompleted"
            ]
        );

        for n in &result.events {
            assert_eq!(n.request_id, result.response.request_id);
        }
    }

    #[tokio::test]
    async fn llm_call_returns_mock_response() {
        let mut plan = Plan::new("llm plan", AgentId::new());
        plan.add_node(PlanNode::new(
            "ask",
            PlanNodeKind::LlmCall {
                prompt: "ping".into(),
                model: None,
            },
        ));
        let req = ExecutePlanRequest {
            request_id: Uuid::new_v4(),
            workspace_id: WorkspaceId::new(),
            plan,
        };
        let result = process_request(req, make_executor()).await;
        assert_eq!(result.response.status, ExecutionStatus::Completed);
        assert_eq!(
            result.response.output.as_ref().unwrap(),
            &serde_json::json!({"text": "pong"})
        );
    }

    #[tokio::test]
    async fn direct_llm_call_completes_with_provider_content() {
        let mock = MockProvider::new("{\"candidates\":[]}");
        let req = LlmCallRequest {
            request_id: Uuid::new_v4(),
            workspace_id: WorkspaceId::new(),
            purpose: "commitments.extract".into(),
            system: "hidden classifier".into(),
            prompt: "transcript here".into(),
            max_tokens: 0,
        };
        let resp = llm_call_response(&req, &mock).await;
        assert_eq!(resp.request_id, req.request_id);
        assert_eq!(resp.status, ExecutionStatus::Completed);
        assert_eq!(resp.content, "{\"candidates\":[]}");
        assert!(resp.error.is_none());
    }

    #[tokio::test]
    async fn direct_llm_call_failure_is_a_failed_response_not_a_drop() {
        struct FailingProvider(crate::provider::ProviderProfile);
        #[async_trait::async_trait]
        impl LlmProvider for FailingProvider {
            async fn complete(
                &self,
                _prompt: &str,
                _model: Option<&str>,
            ) -> irongolem_core::Result<String> {
                Err(Error::Internal("provider exploded".into()))
            }
            fn profile(&self) -> &crate::provider::ProviderProfile {
                &self.0
            }
        }

        let mock = MockProvider::new("unused");
        let failing = FailingProvider(mock.profile().clone());
        let req = LlmCallRequest {
            request_id: Uuid::new_v4(),
            workspace_id: WorkspaceId::new(),
            purpose: "commitments.extract".into(),
            system: String::new(),
            prompt: "transcript".into(),
            max_tokens: 0,
        };
        let resp = llm_call_response(&req, &failing).await;
        assert_eq!(resp.status, ExecutionStatus::Failed);
        assert!(resp.content.is_empty());
        assert!(resp.error.as_deref().unwrap_or("").contains("exploded"));
    }
}
