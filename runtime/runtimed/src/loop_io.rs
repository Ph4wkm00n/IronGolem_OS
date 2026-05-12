//! NDJSON request/response handling. Keeps stdin/stdout out of the unit
//! tests by accepting an async writer for outbound messages and pure data
//! for the inbound request.

use std::sync::Arc;

use irongolem_core::{
    Error,
    ipc::{
        EventNotification, ExecutePlanRequest, ExecutePlanResponse, ExecutionStatus, Message,
        PingRequest, PingResponse,
    },
};
use irongolem_workflow::PlanEngine;
use tokio::io::AsyncWriteExt;
use tokio::sync::Mutex;
use uuid::Uuid;

use crate::executor::RealStepExecutor;

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
            Arc::new(MockProvider {
                response: "pong".into(),
            }),
        ))
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
}
