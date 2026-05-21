# v0.3 — Adoption + Hardening (from openclaw/hermes competitive scan)

> **Status: SHIPPED** as `v1.2.0` on 2026-05-20 via [PR #63](https://github.com/Ph4wkm00n/IronGolem_OS/pull/63). All eight steps merged. See [`CHANGELOG.md`](../CHANGELOG.md) for the per-step release log. This plan file is preserved as the historical record of the design decisions that drove the release; subsequent edits should land in the v0.4 plan rather than here.

## Context

v1.1.0 closed the v0.2 plan (foundation hardening: real Telegram, workspace claims, F6 real-API for Inbox/Home/Health, Layer-4 channel policies, LLM smoke gate, release polish). A source-code comparison vs `openclaw/openclaw` (TS, 1.22M LOC) and `NousResearch/hermes-agent` (Python, 822K LOC) — both real codebases despite physically-impossible star counts (372k/153k) — surfaced seven architectural patterns worth adopting and ten distinct weaknesses to track.

Goal of v0.3: absorb the highest-leverage of those seven patterns into IronGolem's three-domain split without inheriting either competitor's monolith risk, while hardening the frontend's silent-fail patterns that PH4wkm00n has been living with since v0.2.

Out of scope for v0.3 (deliberately deferred to v0.4+): shared content-addressable checkpoint store (full Rust redesign — sequence with WASM sandbox wiring), full plugin system + supply-chain mitigations (depends on `runtime/sandbox` WASM completion), per-connector process isolation, dark mode + comprehensive a11y pass. These earn their own plan once their prerequisites are done.

## Confirmed Decisions

1. **Commitments are a new event-kind and a new top-level v2 route**, not a tab inside Inbox. Lifecycle is different: Inbox = in-flight approvals (resolved within minutes-to-hours), Commitments = time-bound future obligations (resolved within hours-to-days). Bundling them would corrupt both data models.
2. **ProviderProfile is Rust-side**, declared at `runtime/runtimed/src/provider.rs`, with the existing `LlmProvider` trait kept as the runtime interface. A `ProviderProfile` struct adds declarative metadata (auth, endpoints, request quirks, fallback models) consumed by both the runtimed dispatcher AND the gateway's settings handler so the UI can render provider cards.
3. **Audit subsystem runs in the Go gateway**, not in the Rust runtime, despite my v1 reasoning that runtime-side audits resist gateway compromise better. Reason: the threat model in v0.3 is misconfiguration drift + accidental scope escape, not active gateway compromise. Gateway-side keeps the implementation tractable. Note in `## Decisions` so v0.4+ can revisit if the threat model evolves.
4. **Audits use a simple `time.Ticker` goroutine in gateway main**, NOT the parked scheduler at `services/_deferred/scheduler/`. Unparking the scheduler is a v0.4 step that needs its own plan; coupling it to v0.3 expands scope by ~2x.
5. **Frontend hardening is its own step, not bolted onto Commitments UI**. Skeletons + error boundaries + component dedup applies to ALL eight v2 pages; doing it inline alongside the new Commitments page would skip the existing pages that need it most (Home/Inbox/Health all silent-fail today).
6. **Connector breadth (Slack + Discord + Signal) is the largest single step** and ships LAST in v0.3 so the registration-extension foundation (Step 1) is locked in before three new connectors exercise it.

## Sequenced Integration

### Step 1 — Connector Registration Extension (S, no deps)

**Adopts:** hermes `gateway/platform_registry.py:PlatformEntry`. Extends IronGolem's `connectors/connector.go` registration with the `CheckFn / RequiredEnv / InstallHint / SetupFn` fields that make `doctor` + setup wizard UX work without per-connector special cases.

- **Modify:** `connectors/connector.go` — extend `ConnectorRegistration` struct with `CheckFn func() bool`, `RequiredEnv []string`, `InstallHint string`, `ValidateConfig func(cfg) error`. Keep existing `Factory func(cfg) Connector` as-is.
- **Modify:** `connectors/registry.go` (create if absent) — registry list with iterator + lookup by ID.
- **Modify:** `services/gateway/internal/connector/manager.go` — `RegisterSource` learns to call `CheckFn` before accepting registration; reject with descriptive error if check fails.
- **Modify:** `connectors/telegram/` — populate the four new fields on the existing telegram registration (`CheckFn` = `os.Getenv("IRONGOLEM_TELEGRAM_BOT_TOKEN") != ""`, `RequiredEnv = ["IRONGOLEM_TELEGRAM_BOT_TOKEN"]`, etc.).
- **Modify:** `connectors/email/` + `connectors/webhook/` — same population pass.
- **New:** `services/gateway/cmd/doctor/main.go` — operator binary; iterates registry, runs `CheckFn` per connector, prints structured status (`telegram: OK | email: missing IMAP_HOST | ...`). Mirrors `mint-token` / `smoke-telegram` operator-tooling pattern.
- **Modify:** `infra/docker/Dockerfile.services` — add `doctor` to the `go build` list.
- **Modify:** `.github/workflows/release.yml` — add `doctor:./gateway/cmd/doctor` to the matrix builds.
- **Verification:** `go build ./...` clean; `irongolem-doctor` binary exits 0 with all three connectors marked OK when env is set, exits non-zero with structured messages when env is missing.

### Step 2 — Hook Decision Types Schema (S, no deps, parallelizable with Step 1)

**Adopts:** openclaw `src/plugins/hook-decision-types.ts`. Locks in the `Allow | Deny | Modify | Observe` taxonomy ahead of any plugin system so future plugin authors can't invent ad-hoc decision types.

- **New:** `packages/schema/src/hooks.ts` — `HookDecision` enum + `HookContext` interface (correlation_id, phase, agent_id, workspace_id) + `HookResult { decision: HookDecision, reason?: string, modifiedPayload?: unknown }`.
- **Modify:** `packages/schema/src/index.ts` — re-export from `hooks.ts`.
- **New:** `runtime/core/src/hook.rs` — mirror the TS types as Rust structs/enums. Wire into the existing `policy.rs` module's `PolicyDecision` so audit + policy decisions share a vocabulary.
- **Modify:** `runtime/core/src/lib.rs` — `pub use hook::{HookDecision, HookContext, HookResult};`
- **Verification:** `cargo build --workspace`; `pnpm --filter @irongolem/schema build`. ISC: `Grep` for `HookDecision` returns hits in both Rust and TS.

### Step 3 — ProviderProfile + OpenAI as Profile #2 (M, no deps)

**Adopts:** hermes `providers/base.py:ProviderProfile` declarative dataclass. Lifts IronGolem's hardcoded-Anthropic seam into a profile vocabulary BEFORE a second provider lands, so the second provider validates the abstraction instead of retrofitting it.

- **Modify:** `runtime/runtimed/src/provider.rs`:
  - Add `ProviderProfile` struct: `name`, `auth_type` (enum: ApiKey | OAuth | Bedrock), `base_url`, `default_headers: HashMap`, `fixed_temperature: Option<f32>`, `default_max_tokens: u32`, `fallback_models: Vec<String>`, `models_url: Option<String>`.
  - Keep existing `LlmProvider` trait; add `fn profile(&self) -> &ProviderProfile;` to it.
  - Refactor existing Anthropic provider to be `AnthropicProvider { profile: ProviderProfile, api_key: String, client: reqwest::Client }`. Move headers/URL/temperature defaults from inline code into the profile.
  - Add `OpenAIProvider` as Profile #2. Reads `OPENAI_API_KEY`. `base_url = "https://api.openai.com/v1"`. Same `complete()` method signature.
  - Extend `ProviderConfig::from_env()` to recognize `IRONGOLEM_LLM_PROVIDER=openai` and instantiate `OpenAIProvider`.
- **New:** `packages/schema/src/provider.ts` — TS mirror of `ProviderProfile` for the settings UI.
- **Modify:** `services/gateway/internal/handler/` — new file `provider.go` exposing `GET /api/v1/providers` returning the active profile (read from runtimed via a new `ListProviders` IPC verb) + available alternatives.
- **Modify:** `runtime/core/src/ipc.rs` — add `ListProvidersRequest` / `ListProvidersResponse` to `Message` enum.
- **New:** `scripts/smoke-llm-openai.sh` — mirrors `scripts/smoke-llm.sh`; opt-in via `OPENAI_API_KEY`.
- **Modify:** `.github/workflows/llm-smoke.yml` — add a second job matrix entry for OpenAI. Skip-clean when secret missing (same exit-2 pattern as Anthropic).
- **Verification:** `cargo test --workspace`. `IRONGOLEM_LLM_PROVIDER=openai OPENAI_API_KEY=... cargo run --bin runtimed` exits 0 on Ping. Gateway `/api/v1/providers` returns the active profile. ISC: `Grep "fixed_temperature"` returns ≥2 hits (Anthropic + OpenAI profiles, no inline magic numbers).

### Step 4 — Commitments Subsystem (L, depends on Step 2 for HookDecision)

**Adopts:** openclaw `src/commitments/`. Adds user-facing future-obligation tracking distinct from IronGolem's runtime-health Heartbeats. Lifecycle: extraction-at-turn-close → queue → due-window fire → dismiss/snooze/expire.

- **New:** `services/gateway/internal/commitments/` Go package:
  - `types.go` — `CommitmentKind` enum (`event_check_in | deadline_check | care_check_in | open_loop`), `CommitmentSensitivity` enum (`routine | personal | care`), `CommitmentStatus` (`pending | sent | dismissed | snoozed | expired`), `Commitment` struct with `DedupeKey`, `DueWindow {EarliestMs, LatestMs, Timezone}`, `Confidence`.
  - `store.go` — `Store` interface + SQLite impl. New table `gateway_commitments` migrated via `persist/db.go`.
  - `extractor.go` — given a turn pair (user + assistant text), prompts the active LLM provider to extract candidate commitments. Dedupe against existing pending via `DedupeKey`. Confidence threshold gates auto-insert.
  - `runtime.go` — `time.Ticker`-driven (60s) goroutine; scans pending commitments for `DueWindow.EarliestMs <= now <= LatestMs`; fires via existing connector outbound, emits `CommitmentFired` event.
- **Modify:** `services/gateway/internal/persist/db.go` — append `gateway_commitments` migration. Indexes: `(workspace_id, status, earliest_ms)`, `(dedupe_key)`.
- **Modify:** `packages/schema/src/events.ts` — add `CommitmentExtractedEvent`, `CommitmentFiredEvent`, `CommitmentDismissedEvent`, `CommitmentSnoozedEvent`, `CommitmentExpiredEvent` discriminated-union members.
- **New:** `services/gateway/internal/handler/commitments.go` — `GET /api/v2/commitments`, `POST /api/v2/commitments/{id}/dismiss`, `POST /api/v2/commitments/{id}/snooze`, `DELETE /api/v2/commitments/{id}` (admin only).
- **Modify:** `services/gateway/cmd/main.go` — register routes; instantiate commitments runtime goroutine.
- **Modify:** `services/gateway/internal/middleware/policy.go` — add `routeMapping` entries for the new endpoints. New permission: `commitment.write`, `commitment.read`, `commitment.admin`.
- **Modify:** `services/gateway/internal/handler/messages.go` (or wherever outbound dispatch lives) — on assistant-turn close, enqueue extraction job (async, non-blocking).
- **Verification:** `go test ./...`. Synthetic input: send a Telegram message saying "remind me Tuesday at 6pm to call mom" → extraction creates a `care_check_in` commitment with the appropriate `DueWindow` → `time.Ticker` fires → outbound message lands → `CommitmentFiredEvent` in events table. ISC: SQL `SELECT COUNT(*) FROM gateway_commitments WHERE status='sent'` ≥ 1 after the smoke run.

### Step 5 — Audit Probe Subsystem (M, depends on Step 1 for connector probes)

**Adopts:** openclaw `src/security/audit-*.ts` (taxonomy, not the 80+ file scale). Adds continuous-security-testing-as-runtime to IronGolem's 5-layer policy enforcement.

- **New:** `services/gateway/internal/audit/` Go package:
  - `audit.go` — `Probe` interface: `Run(ctx context.Context) Finding`. `Finding` has `ProbeID`, `Severity` (info | warning | critical), `Evidence map[string]any`, `Reason`, `Timestamp`. Registry pattern (mirrors connector registry from Step 1).
  - `probes/workspace_skill_escape.go` — placeholder until skills land; returns `info` finding `"no skill system in v0.3, probe passes vacuously"`. Pre-locks-in the probe ID + Finding shape.
  - `probes/channel_dm_policy.go` — checks channel_policies table for orphan rules (channel_id no longer in connector registry) + inconsistencies (allow + deny on the same action).
  - `probes/trust_model.go` — checks: HMAC secret loaded; auth middleware in chain; policy middleware in chain. If any missing, `critical` finding.
  - `probes/connector_health_drift.go` — uses Step 1's `CheckFn` per registered connector; finding `warning` if a registered connector's CheckFn returns false (env drifted).
  - `runtime.go` — `time.Ticker` (5min tick) iterates probes, emits structured findings via OTel + `AuditFinding` events.
- **Modify:** `services/gateway/internal/persist/db.go` — new table `gateway_audit_findings` (id, probe_id, severity, evidence, reason, ts).
- **Modify:** `packages/schema/src/events.ts` — `AuditFindingEvent` discriminated-union member.
- **New:** `services/gateway/internal/handler/audit.go` — `GET /api/v2/audit/findings` (paginated, severity-filtered).
- **Modify:** `services/gateway/cmd/main.go` — instantiate audit runtime.
- **Verification:** unit tests per probe; `audit_trust_model.go` returns `critical` if HMAC secret env unset (test by unsetting env, invoking probe in unit test). ISC: `curl localhost:8080/api/v2/audit/findings` returns ≥1 JSON object after first tick.

### Step 6 — Frontend UX Hardening (M, no deps — parallelizable with backend steps)

**Frontend-only step.** Fixes silent-fail patterns surfaced by the explore agent across Home/Inbox/Health (today they `.catch(() => mockFallback)` with no user signal). Establishes the convention before adding new Commitments + Audit pages in Step 7.

- **New:** `apps/web/src/components/RouteSkeleton.tsx` — generic loading skeleton (card-grid silhouette using `@irongolem/ui` Tokens for fill colors). Accepts `variant: "cards" | "list" | "timeline"` for shape.
- **New:** `apps/web/src/components/RouteErrorBoundary.tsx` — React error boundary; renders a `<SafetyCard>` ("This route crashed — see gateway logs at...") with a "retry" button. One per v2 route.
- **New:** `apps/web/src/components/RouteError.tsx` — explicit error state for caught-but-non-crash failures (e.g., real-API mode returns 5xx). Replaces today's silent mock fallback when `VITE_API_MODE_<ROUTE>=real`. Mock mode still falls through silently (intentional dev affordance).
- **Modify:** `apps/web/src/lib/api.ts` — wrap `v2.{home, inbox, health, recipes, research, memory, security, settings}.load()` calls so the page receives `{ status: "loading" | "ok" | "error", data?, error? }` instead of bare data. Optional discriminated union shape; pages opt in per-route.
- **Modify:** `apps/web/src/pages/v2/Home.tsx` + `Inbox.tsx` + `Health.tsx` — adopt the new `{ status, data, error }` envelope. Show `<RouteSkeleton variant="..." />` during loading. Show `<RouteError />` on real-mode failure. Wrap each in `<RouteErrorBoundary>`.
- **Modify:** `apps/web/src/pages/v2/registry.ts` — wrap each lazy-loaded route in `<RouteErrorBoundary>` automatically (so new routes inherit the safety net).
- **New:** `apps/web/eslint-rules/no-inline-component-redefine.cjs` — custom ESLint rule that warns when a TSX file defines a local `SafetyCard | RiskBadge | StatusChip | SourcePill | Timeline | PolicyCard | HeartbeatStatus | ResearchCard` (these MUST be imported from `@irongolem/ui`). Wired into `.eslintrc`. Existing inline definitions: replace with imports in this step.
- **Modify:** `packages/ui/src/index.ts` — ensure `StatusChip` + `SourcePill` are exported (currently rolled inline in pages per explore agent finding at Home.tsx:62-100, Inbox.tsx:57-93).
- **Modify:** `tests/visual/` — re-capture baselines for the three modified pages after skeleton + error-state work. Capture two NEW baselines per page (skeleton state, error state) by feeding `VITE_API_MODE_<ROUTE>=real` against a deliberately-down gateway.
- **Verification:** `pnpm --filter @irongolem/web build` clean. `pnpm --filter @irongolem/web lint` returns 0 inline-redefine violations. Visual regression via `scripts/visual-capture.sh` clean against new baselines. Manually: with `VITE_API_MODE_INBOX=real` and gateway off, Inbox shows error state instead of silently displaying mock data.

### Step 7 — Commitments + Audit Frontend (M, depends on Steps 4 + 5 + 6)

- **New:** `apps/web/src/pages/v2/Commitments.tsx` — new top-level v2 route `/commitments`. Layout: filter chips by kind/sensitivity/status, list of `<CommitmentCard>` each showing dedupeKey-derived title, due window relative ("in 2h 14m"), source turn-link, action buttons (dismiss, snooze, view source).
- **New:** `apps/web/src/pages/v2/Audit.tsx` — new top-level v2 route `/audit`. Severity-filtered findings list. Group by probe ID. Drill-down panel shows evidence map as key-value table + reason string.
- **Modify:** `apps/web/src/pages/v2/registry.ts` — register both routes (lazy-loaded, wrapped in error boundary from Step 6).
- **Modify:** `apps/web/src/components/WorkspaceTopbar.tsx` (or whatever nav lives in `pages/v2/_shared/`) — add `/commitments` + `/audit` nav entries.
- **Modify:** `apps/web/src/lib/api.ts` — add `v2.commitments.list()` + `v2.commitments.dismiss(id)` + `v2.commitments.snooze(id, until)` + `v2.audit.findings({ severity, page })`. Mock data in `_mocks/commitments.ts` and `_mocks/audit.ts`.
- **Modify:** `packages/ui/src/index.ts` — add `<CommitmentCard>` + `<AuditFindingCard>` components. Use existing design tokens for severity-tone mapping.
- **Verification:** Both routes load with mocks at `pnpm --filter @irongolem/web dev`. With `VITE_API_MODE_COMMITMENTS=real` + backend running, Commitments page populates from gateway. Visual baselines captured for both routes. ISC: navigate to `/commitments`, dismiss one, refresh — dismissed card no longer appears (event sourcing round-trip verified).

### Step 8 — Connector Breadth: Slack + Discord + Signal (XL, depends on Step 1)

**Adopts** the *outcome* of openclaw's 41-extension / hermes's 30-platform connector counts, using IronGolem's existing telegram connector as template. Locks in three high-priority channels.

- **New:** `connectors/slack/` — adapter, OAuth-app config (bot token + signing secret), Events API webhook handler, `CheckFn` (token + signing secret present), `RequiredEnv = ["SLACK_BOT_TOKEN", "SLACK_SIGNING_SECRET"]`, `InstallHint = "Create a Slack app at https://api.slack.com/apps and set tokens"`.
- **New:** `connectors/discord/` — adapter, bot token config, gateway WebSocket or REST polling (whichever matches IronGolem's existing inbound pattern), full registration shape.
- **New:** `connectors/signal/` — adapter. Signal CLI bridge (`signal-cli`) is the realistic v0.3 path; native libsignal is a separate v0.4+ project. `CheckFn` = `signal-cli` binary present in PATH.
- **New:** per-connector smoke scripts: `scripts/smoke-slack.sh`, `scripts/smoke-discord.sh`, `scripts/smoke-signal.sh`. Each follows `scripts/smoke-telegram.sh` template, opt-in via env, exits 2 when secret missing.
- **Modify:** `services/gateway/cmd/main.go` — instantiate three new sources in the registration block (conditional on `CheckFn`).
- **Modify:** `.github/workflows/release.yml` — no new binaries (connectors are part of the gateway image), but mention in release notes generator.
- **Modify:** `infra/docker/docker-compose.yml` — env-var examples for the three new connectors.
- **Modify:** `README.md` — supported-channels table now reads telegram + email + webhook + slack + discord + signal.
- **Verification:** Per-channel: send inbound message via that channel, gateway logs ingestion, event appears in events table tagged with correct connector ID. ISC: `SELECT DISTINCT source_service FROM gateway_events` returns ≥6 distinct values after all three smokes run.

## Weakness Mitigations Tracked in This Plan

| ID | Source | Status | Where mitigated |
|---|---|---|---|
| W1 | openclaw: no trust separation | ALREADY MITIGATED | Three-domain Rust/Go/TS split since v0.1 |
| W2 | openclaw: plugin marketplace supply-chain risk | DEFERRED to v0.4+ | Depends on plugin system actually existing; punted with WASM sandbox completion |
| W3 | openclaw: plugin SDK complexity (300+ files) | DEFERRED to v0.4+ | Same as W2; v0.3 only locks in HookDecision vocabulary (Step 2) |
| W4 | openclaw: session-state history, not event-sourced | ALREADY MITIGATED | events table is canonical (explore agent confirmed via handler reads) |
| W5 | openclaw: audit in-process, can't audit itself during compromise | PARTIAL — see Decision 3 | Audit runs gateway-side in v0.3; runtime-side audits revisited if threat model evolves |
| W6 | hermes: 642KB cli.py monolith | ALREADY MITIGATED | Go's package boundaries enforce subpackage limits |
| W7 | hermes: Python no compile-time safety | ALREADY MITIGATED | Rust + Go give compile-time guarantees |
| W8 | hermes: 30+ adapters share gateway process | PARTIAL — DEFERRED | Connectors live in their own Go module; per-connector process isolation is v0.4+ |
| W9 | hermes: weak workspace model | ALREADY MITIGATED | Workspace-scoped HMAC claims since v0.2 Step 2 |
| W10 | hermes: credentials in inference process | PARTIAL — Step 3 reinforces | ProviderProfile keeps secrets in Rust runtimed; gateway sees scoped IPC verbs only |

## Verification — Gates

| Gate | What it checks | When |
|---|---|---|
| **Gate 1: Build + CI green** | `cargo test --workspace`, `go test ./...`, `pnpm --filter @irongolem/web build`, `pnpm lint` (with new no-inline-component-redefine rule) | After every step |
| **Gate 2: OpenAI smoke** | `OPENAI_API_KEY=... bash scripts/smoke-llm-openai.sh` exit 0 against `gpt-4o-mini` or equivalent cheap model | End of Step 3 |
| **Gate 3: Commitments round-trip** | Synthetic Telegram → extraction → ticker fires → outbound delivers → `CommitmentFiredEvent` in events table | End of Step 4 |
| **Gate 4: Audit findings produce** | `curl /api/v2/audit/findings` returns ≥1 finding within 6 minutes of gateway start (one tick interval) | End of Step 5 |
| **Gate 5: Frontend hardening** | With backend down + `VITE_API_MODE_<X>=real`, each of Home/Inbox/Health shows `<RouteError />` not stale mock; visual baselines clean | End of Step 6 |
| **Gate 6: Commitments + Audit UI** | Both new routes load; mock + real both work; dismiss roundtrip verified | End of Step 7 |
| **Gate 7: All six connectors live** | All six smoke scripts pass when env present, skip cleanly when env absent | End of Step 8 |
| **Gate 8: Release tag** | `v1.2.0` tagged; release workflow produces binaries including `doctor`; release notes generated | Plan close |

## Risk Register

| ID | Risk | Resolution |
|---|---|---|
| R1 | Commitments extractor cost. Every turn pair runs an LLM call. | Confidence-threshold gate (don't insert below 0.6); cap extraction to one call per N seconds per workspace; opt-out env. |
| R2 | OpenAI provider untested against IronGolem's plan-graph format (assistant tool calls). | Step 3 ships `smoke-llm-openai.sh` BEFORE switching default; doc in README that Anthropic remains default for v0.3. |
| R3 | Connector breadth (Step 8) is the largest scope; could slip release. | Step 8 is LAST; if it slips, v0.3 ships without Signal (keep Slack + Discord which are higher-leverage). |
| R4 | ESLint inline-redefine rule false positives. | Custom rule whitelists `@irongolem/ui` import line itself; rule name is config-flagged so it can be `eslint-disable-next-line` per occurrence if a real exception arises. |
| R5 | Audit goroutine starvation if a probe hangs. | Each probe gets `context.WithTimeout(15s)`; tick-loop logs + skips slow probes; never blocks the next tick. |
| R6 | Migration of existing v2 pages to `{ status, data, error }` envelope is invasive. | Envelope is opt-in per-route; pages adopt incrementally; un-migrated pages keep working in v0.3 with the existing pattern. |
| R7 | Audit probe `workspace_skill_escape` is vacuous in v0.3 (no skill system yet). | Acceptable. Probe ID + Finding shape lock in early so v0.4's skill system inherits the contract instead of inventing one. |

## Critical Files

### To create
- `apps/web/src/components/RouteSkeleton.tsx`
- `apps/web/src/components/RouteErrorBoundary.tsx`
- `apps/web/src/components/RouteError.tsx`
- `apps/web/src/pages/v2/Commitments.tsx`
- `apps/web/src/pages/v2/Audit.tsx`
- `apps/web/eslint-rules/no-inline-component-redefine.cjs`
- `connectors/slack/` (multiple files)
- `connectors/discord/` (multiple files)
- `connectors/signal/` (multiple files)
- `connectors/registry.go`
- `packages/schema/src/hooks.ts`
- `packages/schema/src/provider.ts`
- `runtime/core/src/hook.rs`
- `services/gateway/cmd/doctor/main.go`
- `services/gateway/internal/audit/audit.go` + `runtime.go` + `probes/*.go`
- `services/gateway/internal/commitments/types.go` + `store.go` + `extractor.go` + `runtime.go`
- `services/gateway/internal/handler/audit.go`
- `services/gateway/internal/handler/commitments.go`
- `services/gateway/internal/handler/provider.go`
- `scripts/smoke-llm-openai.sh`
- `scripts/smoke-slack.sh`, `smoke-discord.sh`, `smoke-signal.sh`

### To modify
- `apps/web/src/App.tsx` — wire error boundaries into route layer
- `apps/web/src/lib/api.ts` — `{ status, data, error }` envelope + new endpoints
- `apps/web/src/pages/v2/Home.tsx` + `Inbox.tsx` + `Health.tsx` — adopt envelope + skeletons + error states
- `apps/web/src/pages/v2/registry.ts` — register Commitments + Audit; auto-wrap in error boundary
- `connectors/connector.go` — extend registration struct
- `connectors/{email,telegram,webhook}/` — populate new registration fields
- `packages/schema/src/events.ts` — Commitments + Audit event kinds
- `packages/schema/src/index.ts` — re-export hooks + provider
- `packages/ui/src/index.ts` — export StatusChip, SourcePill, CommitmentCard, AuditFindingCard
- `runtime/core/src/ipc.rs` — `ListProviders` message
- `runtime/core/src/lib.rs` — re-export hook types
- `runtime/runtimed/src/provider.rs` — ProviderProfile + OpenAI provider
- `services/gateway/cmd/main.go` — register new routes, instantiate audit + commitments runtimes, register Slack/Discord/Signal sources
- `services/gateway/internal/middleware/policy.go` — routeMapping entries for new endpoints
- `services/gateway/internal/handler/messages.go` — enqueue commitment extraction on assistant-turn close
- `services/gateway/internal/persist/db.go` — migrations for `gateway_commitments` + `gateway_audit_findings`
- `.github/workflows/release.yml` — `doctor` binary in matrix; release notes mention new connectors
- `.github/workflows/llm-smoke.yml` — OpenAI job
- `infra/docker/Dockerfile.services` — add `doctor` binary build
- `infra/docker/docker-compose.yml` — env-var examples for new connectors
- `CHANGELOG.md` — v0.3 entry
- `README.md` — supported-channels table

## How v0.3 Composes with v0.2 Surface

| v0.2 surface | v0.3 extension |
|---|---|
| Telegram connector (Step 1) | Extended with `CheckFn`/`RequiredEnv`/`InstallHint` (v0.3 Step 1); joined by Slack + Discord + Signal (v0.3 Step 8) |
| WorkspaceID HMAC claims (Step 2) | Commitments + Audit endpoints scope by workspace via existing middleware |
| Inbox F6 real-API (Step 3) | Commitments page is sibling — same envelope pattern, different lifecycle |
| Channel policy store (Step 4) | Audit probe `channel_dm_policy` reads this store to find orphans |
| LLM smoke gate (Step 5) | Joined by `llm-smoke-openai.sh` job (v0.3 Step 3) |
| Home + Health F6 real-API (Step 6) | Both hardened with skeleton + error states + envelope (v0.3 Step 6) |
| Release polish (Step 7) | v0.3 ships `doctor` binary alongside `mint-token` + `smoke-telegram` |

## Deferred to v0.4+

- Shared content-addressable git checkpoint store (from hermes `tools/checkpoint_manager.py`). Requires full `runtime/checkpoints/` redesign; sequence with WASM sandbox work.
- Plugin system (the actual lifecycle/loader/marketplace from openclaw `src/plugins/`). Hook decision types from Step 2 are the foundation; full system depends on `runtime/sandbox` WASM completion.
- Per-connector process isolation (W8 mitigation). Architectural change; needs its own plan.
- Dark mode + comprehensive a11y audit. Tackled together once design tokens get a darkPalette + once route hardening (Step 6) stabilizes the shape.
- Scheduler unpark from `services/_deferred/scheduler/`. Currently audits + commitments use simple goroutine tickers; if v0.4+ adds delayed recipes, the scheduler comes back.
- Browser automation tool (openclaw `extensions/browser`, hermes `tools/browser_*.py` camofox/CDP stack). Out of v0.3 scope; depends on sandbox WASM.
- ~38 additional messaging platforms (openclaw extension count = 41, hermes platform count = 30, IronGolem post-v0.3 = 6). The connector pattern scales linearly; further channels are added one-per-quarter, not packed into a single release.

## Estimated effort

- Steps 1, 2: ~half-day each (S each)
- Step 3: ~2-3 days (M — Rust refactor + OpenAI provider + IPC verb + UI hookup)
- Step 4: ~4-5 days (L — extraction prompt design + dedup logic + tick loop + UI Step 7 dependency)
- Step 5: ~2 days (M — probe pattern, 4 initial probes)
- Step 6: ~2-3 days (M — three pages migrated + ESLint rule + visual baselines)
- Step 7: ~2 days (M — two pages, well-defined components)
- Step 8: ~5-7 days (XL — three connectors, OAuth flows, smoke scripts each)

Total: ~3 weeks single-developer continuous; reduces to ~1.5 weeks with two devs parallelizing Steps 1/2/3 + Step 6 vs Steps 4/5 + Step 8.
