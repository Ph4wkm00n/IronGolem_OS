//! `runtimed` — line-delimited JSON daemon spawned as a child process by the
//! gateway. Reads `Message` envelopes from stdin, writes responses and
//! streamed events to stdout. Tracing output goes to stderr.

use std::sync::Arc;

use irongolem_core::{Error, ipc::Message};
use irongolem_runtimed::{
    RealStepExecutor, build_provider,
    loop_io::{
        error_response, list_providers_response, ping_response, process_request, write_message,
    },
    provider::{LlmProvider, ProviderConfig},
};
use irongolem_sandbox::LocalSandboxHost;
use tokio::io::{AsyncBufReadExt, BufReader, stdin, stdout};
use tokio::sync::Mutex;
use tokio::task::JoinSet;
use tracing::{error, info, warn};
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_writer(std::io::stderr)
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    let provider_cfg = ProviderConfig::from_env();
    info!(provider = ?provider_cfg.kind, "runtimed starting");

    // v0.3 Step 3: keep a handle to the active provider so the
    // `ListProviders` IPC verb can return its profile name without
    // reaching back into the executor's internal state.
    let provider: Arc<dyn LlmProvider> = build_provider(provider_cfg);
    let executor = Arc::new(RealStepExecutor::new(
        Arc::new(LocalSandboxHost::with_builtins()),
        Arc::clone(&provider),
    ));

    let stdin_reader = BufReader::new(stdin());
    let stdout_writer = Arc::new(Mutex::new(stdout()));
    let mut in_flight: JoinSet<()> = JoinSet::new();

    let mut lines = stdin_reader.lines();
    while let Some(line) = lines.next_line().await? {
        if line.trim().is_empty() {
            continue;
        }

        let msg: Message = match serde_json::from_str(&line) {
            Ok(m) => m,
            Err(e) => {
                warn!(error = %e, line = %line, "failed to parse incoming line");
                let resp = error_response(Uuid::nil(), format!("parse error: {e}"));
                if let Err(write_err) =
                    write_message(&stdout_writer, &Message::ExecutePlanResponse(resp)).await
                {
                    error!(error = %write_err, "failed to write error response");
                }
                continue;
            }
        };

        match msg {
            Message::ExecutePlanRequest(req) => {
                let executor = Arc::clone(&executor);
                let stdout_writer = Arc::clone(&stdout_writer);
                in_flight.spawn(async move {
                    let result = process_request(req, executor).await;
                    for n in result.events {
                        if let Err(e) =
                            write_message(&stdout_writer, &Message::EventNotification(n)).await
                        {
                            error!(error = %e, "failed to emit event notification");
                            return;
                        }
                    }
                    if let Err(e) = write_message(
                        &stdout_writer,
                        &Message::ExecutePlanResponse(result.response),
                    )
                    .await
                    {
                        error!(error = %e, "failed to emit execute response");
                    }
                });
            }
            Message::PingRequest(req) => {
                let resp = ping_response(&req);
                if let Err(e) = write_message(&stdout_writer, &Message::PingResponse(resp)).await {
                    error!(error = %e, "ping response write failed");
                }
            }
            Message::ListProvidersRequest(req) => {
                match list_providers_response(&req, provider.as_ref()) {
                    Ok(resp) => {
                        if let Err(e) =
                            write_message(&stdout_writer, &Message::ListProvidersResponse(resp))
                                .await
                        {
                            error!(error = %e, "list_providers response write failed");
                        }
                    }
                    Err(e) => {
                        error!(error = %e, "list_providers response build failed");
                    }
                }
            }
            Message::Shutdown(_) => {
                info!("shutdown requested, draining in-flight requests");
                break;
            }
            other => {
                warn!(?other, "ignoring unsolicited message");
            }
        }
    }

    // Drain in-flight requests before exiting so callers always see their
    // terminal response — vital for the integration test, which spawns the
    // binary, sends one request, then closes stdin.
    while let Some(joined) = in_flight.join_next().await {
        if let Err(e) = joined {
            error!(error = %e, "in-flight task panicked");
        }
    }

    Ok(())
}

#[allow(dead_code)]
fn _ensure_error_type_used(e: Error) {
    let _ = e;
}
