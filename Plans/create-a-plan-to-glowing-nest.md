# IronGolem OS v0.1 — Critical Moves Plan

## Context

The Council review of IronGolem OS converged on a sharp diagnosis: **the architecture is 2–3 phases ahead of the implementation.** The repo has well-designed scaffolding (Rust plan graph + checkpoints + memory graph, Go control plane with five-layer policy *model*, multi-tenant data abstractions) but the three domains do not actually connect, and most claimed capabilities are unverifiable end-to-end.

Verified current state from exploration:

- **Rust runtime**: `PlanEngine` is real and tested; `LlmCall`/`ToolCall` variants are typed but have no executor (only `NoopExecutor` exists). `LocalSandboxHost::execute` is a stub returning dummy JSON. There is no HTTP/gRPC/FFI server — the runtime is library-only.
- **Go services**: 8 services exist with real handlers but in-memory stores. Gateway has 26 routes and **zero calls into Rust**. Layer-4 per-channel policy is stub-as-allow at `/services/pkg/policy/policy.go:252-260`. Identity is header-based and trivially spoofable.
- **Connectors**: Telegram, Email, Webhook are real custom-HTTP implementations (no SDKs, no tests). Six others are stubs on the kill-list.
- **Frontend**: web app is real with 8 routed pages. Admin console is real. Desktop is Tauri skeleton.
- **No `proto/` directory, no `go.work`, empty `Plans/`**.

This plan executes the Council's top 3 critical moves and the highest-priority scope cuts to produce **one verifiable end-to-end flow**: Telegram message → Gateway → Rust runtime executes a plan with an `LlmCall` node → response sent back to user. Everything else stays out of v0.1.

## Confirmed Decisions

1. **IPC**: Subprocess + stdio NDJSON. Gateway spawns `runtimed` as a child process. Graduate to gRPC at v0.3 when fleet/multi-host arrives. v0.1 message shapes designed proto-friendly so migration is mechanical.
2. **Deferred code**: Move 6 deferred services to `services/_deferred/` (Go ignores leading underscore — clean builds). Delete 6 deferred connectors (Discord, Slack, WhatsApp, Feishu, Browser, Filesystem) outright; git history is the safety net.
3. **LLM routing**: Rust runtime calls the LLM provider directly. The Go `services/gateway/internal/provider/` package becomes vestigial and is removed.

## Sequenced Implementation

### Step 1 — Define IPC contract (S, no deps)

Create the line-delimited NDJSON wire types and mirror them on both sides.

- **New crate**: `runtime/runtime-ipc/` (or `core::ipc` module) with four message types:
  - `ExecutePlanRequest { request_id, plan, workspace_id }`
  - `ExecutePlanResponse { request_id, status, output, error }`
  - `EventNotification { request_id, event }` (streamed during execution)
  - `Shutdown { request_id }`
- **New Go package**: `services/pkg/runtime/types.go` — Go structs with `json:` tags matching serde output exactly.
- **Reuse**: existing `core::Plan`, `core::Event`, `core::WorkspaceId` already serialize via serde; tag-shape must stay flat (no enum-with-data leakage) so v0.3 gRPC migration is mechanical.

### Step 2 — Build `runtimed` binary (M, depends on Step 1)

Wire the first concrete `StepExecutor` and ship the runtime as a process.

- **New binary crate**: `runtime/runtimed/` added to root `Cargo.toml` workspace members.
- **Modify**: `runtime/workflow/src/executor.rs` — replace reliance on `NoopExecutor` with `RealStepExecutor` that dispatches on `PlanNodeKind`:
  - `LlmCall { prompt, model }` → direct HTTP call to Anthropic (Claude) via `reqwest`. Mock provider behind `IRONGOLEM_LLM_PROVIDER=mock` returning literal `"pong"` for tests.
  - `ToolCall { tool_name, input }` → dispatch through `LocalSandboxHost::execute` (Step 3).
  - `Verify { target_node_id }` → wire existing `NonEmptyVerifier` / `SchemaVerifier` from `runtime/verifier/src/checks.rs` (currently unused).
  - `Checkpoint` → `CheckpointManager::create_checkpoint` (already real).
  - `ApprovalGate`, `Delegation` → return `Error::PlanExecution { reason: "not implemented in v0.1" }`.
- **Main loop**: read NDJSON from stdin, spawn `tokio::task` per request, stream `EventNotification` to stdout, terminal `ExecutePlanResponse` on completion.
- **Reuse**: `PlanEngine::execute`, `SqliteEventStore`, `SqliteCheckpointStore` (all real, all tested).

### Step 3 — Real `LocalSandboxHost` with tool registry (S, parallel with Steps 1–2)

Replace the stub at `runtime/sandbox/src/host.rs:23-41` with a registry-backed dispatcher.

- **Modify**: `runtime/sandbox/src/host.rs` — `LocalSandboxHost` holds a `HashMap<String, Arc<dyn Tool>>`.
- **New**: `runtime/sandbox/src/registry.rs` — `Tool` trait:
  ```rust
  #[async_trait] trait Tool: Send + Sync {
      async fn invoke(&self, input: &Value, cap: &SandboxConfig) -> Result<Value>;
  }
  ```
- **Two built-in tools** to prove the registry: `echo` (returns input verbatim) and `http_get` (capability-checked against `SandboxConfig.allowed_destinations`).
- **Out of scope for v0.1**: WASM, fork-exec sandboxing, seccomp. Those are v0.4.
- **Reuse**: existing `runtime/sandbox/src/capability.rs` (`SandboxConfig`, `Capability` enum already complete).

### Step 4 — Gateway runtime client (M, depends on Steps 1–2)

The bridge.

- **New**: `services/gateway/internal/runtime/client.go` — spawns `runtimed` child at gateway boot; one writer goroutine, one reader goroutine; `request_id → chan ExecutePlanResponse` correlation map; restart-on-crash with exponential backoff capped at 5 attempts; health check via `Ping` request_id.
- **Modify**: `services/gateway/cmd/main.go` (lines ~40–60 in the boot sequence) to construct the runtime client and inject it into handlers.

### Step 5 — Telegram inbound → Plan synthesizer (M, depends on Steps 2, 4)

Connect the connector to the runtime.

- **Modify**: `services/gateway/internal/handler/handler.go` — `MessageInbound` becomes the entry point. v0.1 plan synthesis is **deterministic, not agentic**: every inbound message becomes a 1-node plan with a single `LlmCall { prompt: <system_prompt + user_message>, model: None }`.
- **New**: `services/gateway/internal/planner/synth.go` — `SynthesizePlan(msg models.Message) core.Plan`.
- **Modify**: `services/gateway/internal/connector/manager.go` — add a goroutine pump that drains each connector's `Receive(ctx)` channel into `MessageInbound`. Today the gateway only exposes the inbound HTTP endpoint; the connector's outbound channel is never read.
- **Outbound**: response flows through existing `MessageOutbound` handler → existing `connectors/telegram/telegram.go` `Send()` (already real).

### Step 6 — Persist gateway stores (S, parallel with Steps 1–5)

Stop losing state on every restart.

- **Modify**: `services/gateway/cmd/main.go` lines 36–39 — replace `NewInMemory*Store` constructors with SQLite-backed equivalents.
- **Modify**: `services/gateway/internal/handler/{recipes,approvals,timeline}.go` — add `NewSQLite*Store(db *sql.DB)` constructors; existing in-memory implementations already satisfy the same interfaces.
- **Reuse**: `services/pkg/store/drivers.go` SQLite driver (currently behind build tag — flip to default).
- **Single SQLite file** at `~/.irongolem/gateway.db`.

### Step 7 — Scope cuts (S, mechanical, parallel)

| Action | Path | Method |
|---|---|---|
| Move 6 services to `_deferred/` | `services/{scheduler,health,defense,tenancy,research,optimizer,fleet}/` → `services/_deferred/<name>/` | `git mv`; remove from `Makefile` build/test targets; remove from `infra/docker/docker-compose.yml` |
| Delete 6 connector stubs | `connectors/{discord,slack,whatsapp,feishu,browser,filesystem}/` | `git rm -r`; remove from `connectors/registry.go` |
| Replace header identity with HMAC tokens | `services/gateway/internal/middleware/{tenant.go,security.go}` | Token shape `tenant:user:role:channel:exp` HMAC-SHA256 with secret from `IRONGOLEM_HMAC_SECRET` env. Mutual TLS at v0.3. |
| Fix layer-4 stub-as-allow | `services/pkg/policy/policy.go:252-260` | Fence behind `IRONGOLEM_LAYER4_ENABLED` env var (default `false`). When `false`, return `DecisionAllow` with reason `"layer4 disabled in v0.1"`. When `true`, return `DecisionDeny` with `"layer4 store not implemented"`. **No silent allow.** |

### Step 8 — Verification harness (S, depends on all prior)

See "Verification" section below.

## Critical Files

**To create:**
- `runtime/runtime-ipc/` (or `runtime/core/src/ipc.rs`) — wire types
- `runtime/runtimed/src/main.rs` — binary entry point
- `runtime/runtimed/src/executor.rs` — `RealStepExecutor`
- `runtime/sandbox/src/registry.rs` — `Tool` trait + registry
- `services/gateway/internal/runtime/client.go` — IPC bridge
- `services/gateway/internal/planner/synth.go` — plan synthesis
- `services/pkg/runtime/types.go` — Go-side IPC types
- `scripts/smoke-e2e.sh` — end-to-end smoke test

**To modify:**
- `runtime/sandbox/src/host.rs:23-41` — replace `execute` stub with registry dispatch
- `runtime/workflow/src/executor.rs` — `RealStepExecutor` impl
- `services/gateway/cmd/main.go` — boot `runtimed`, swap stores, install HMAC middleware
- `services/gateway/internal/handler/handler.go` — `MessageInbound` invokes synthesizer + runtime client
- `services/gateway/internal/connector/manager.go` — add receive-pump goroutine per connector
- `services/gateway/internal/handler/{recipes,approvals,timeline}.go` — add SQLite-backed constructors
- `services/gateway/internal/middleware/{tenant.go,security.go}` — HMAC token verifier
- `services/pkg/policy/policy.go:252-260` — fence layer-4 behind env flag
- `services/pkg/store/drivers.go` — flip SQLite default
- `Cargo.toml` — add `runtimed` and `runtime-ipc` workspace members; add `reqwest` to `runtimed` deps
- `Makefile` — drop deferred service build/test targets; add `runtimed` build
- `infra/docker/docker-compose.yml` — remove deferred services

**To remove:**
- `services/gateway/internal/provider/` — Rust now calls LLM directly
- `connectors/{discord,slack,whatsapp,feishu,browser,filesystem}/` — git rm

## Reuse Inventory (existing real components)

- `runtime/core/src/plan.rs` — `Plan`, `PlanNode`, `PlanNodeKind`, status types (complete)
- `runtime/core/src/store.rs` — `SqliteEventStore` (complete + tested)
- `runtime/checkpoints/src/{manager.rs,sqlite_store.rs}` — `CheckpointManager`, `SqliteCheckpointStore` (complete + tested)
- `runtime/memory/src/sqlite_store.rs` — `SqliteMemoryStore` with FTS5 (complete + tested)
- `runtime/verifier/src/checks.rs` — `NonEmptyVerifier`, `SchemaVerifier` (complete; just unwired)
- `runtime/sandbox/src/capability.rs` — `SandboxConfig`, `Capability` enum (complete)
- `runtime/workflow/src/engine.rs` — `PlanEngine::execute` (complete + tested)
- `connectors/telegram/telegram.go` — Telegram Bot API client with long-polling (complete)
- `connectors/email/email.go` — IMAP/SMTP client (complete)
- `connectors/webhook/webhook.go` — HTTP listener + outbound (complete)
- `services/pkg/store/drivers.go` — SQLite driver behind build tag (just flip default)

## Verification

Three gates. All must pass before declaring v0.1 done.

**Gate 1 — Rust unit tests**
```
cargo test --workspace
```
New tests required:
- `runtime/sandbox/src/host.rs` — registered tool dispatches; unregistered tool returns `Error::ToolNotFound`
- `runtime/runtimed/src/main.rs` — NDJSON request/response round-trip via in-memory pipes

**Gate 2 — Gateway↔runtime integration test (the linchpin)**

New test: `services/gateway/internal/runtime/client_test.go`. It must:
1. Build and spawn the actual compiled `runtimed` binary as a child process.
2. Send `ExecutePlanRequest` with a single-node plan `PlanNodeKind::ToolCall { tool_name: "echo", input: {"hello": "world"} }`.
3. Assert the response output equals `{"hello": "world"}`.
4. Assert the streamed events include `PlanCreated`, `PlanStepStarted`, `PlanStepCompleted`, `PlanCompleted` in that order.

This proves the contract, the binary, the registry, and the executor wiring all work without involving Telegram or LLM providers.

**Gate 3 — End-to-end smoke test**

New script: `scripts/smoke-e2e.sh`. Sequence:
1. Start the gateway with `IRONGOLEM_LLM_PROVIDER=mock` (Rust-side mock returning literal `"pong"` for any `LlmCall`).
2. Start a local httptest server impersonating Telegram's `sendMessage` endpoint; configure the Telegram connector's `api_base` to point at it.
3. POST a fake Telegram update to `/api/v1/messages/inbound` matching the connector's normalized `Message` shape.
4. `curl /api/v1/events?limit=10` and assert events include `PlanCompleted` with output containing `"pong"`.
5. Assert the impersonated Telegram server received a `sendMessage` call with the correct `chat_id` and `text=pong`.

When this script exits 0, **Telegram → Gateway → Rust LlmCall → response works end-to-end**. Swapping `mock` for `anthropic` and pointing Telegram at the real API is config flips, not code changes — which is the whole point.

## Migration Path Notes

- **IPC graduation**: when v0.3 fleet/multi-host arrives, replace the stdio framer in `services/gateway/internal/runtime/client.go` and `runtime/runtimed/src/main.rs` with `tonic`/`google.golang.org/grpc`. Message shapes stay identical because we designed them flat and proto-friendly. Estimated effort: 1–2 weeks for one engineer.
- **Identity graduation**: HMAC tokens → mutual TLS. Token claims (`tenant:user:role:channel`) become certificate SANs. Same gateway middleware shape, different verifier.
- **Layer-4 policy**: when implemented, the env-fence pattern stays; just flip the default once the channel-restrictions store ships.
- **Deferred services**: graduating one back is `git mv services/_deferred/<name> services/<name>` plus re-add to `Makefile` and `docker-compose.yml`.
