# Changelog

All notable changes to IronGolem OS, organized by gateway-architecture milestone. Each subsection links to the plan step + PR that landed it.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions track the gateway-architecture milestones (`v0.1.x`, `v0.2.x`); the pre-existing `v1.0.0` repo tag references the initial OSS-scaffolding milestone and predates this changelog.

---

## v0.3 — Adoption + Hardening

Plan: [`Plans/modular-puzzling-blum.md`](Plans/modular-puzzling-blum.md). Source-code comparison vs `openclaw/openclaw` (TS, 1.22M LOC) and `NousResearch/hermes-agent` (Python, 822K LOC) surfaced seven architectural patterns worth adopting and ten distinct weaknesses to track. v0.3 absorbs the highest-leverage patterns into IronGolem's three-domain split while hardening the frontend's silent-fail patterns inherited from v0.2.

### Step 1 — Connector registration extension

Adopts hermes `gateway/platform_registry.py:PlatformEntry`. Extends connector registration with `CheckFn` / `RequiredEnv` / `InstallHint` / `ValidateConfig` so doctor + setup-wizard UX work without per-connector special cases.

- **`connectors/registry.go`** — package-level `Registration` struct + thread-safe registry (`Register` / `MustRegister` / `Get` / `List`).
- **`connectors/{telegram,email,webhook}/metadata.go`** — each connector self-registers via `init()`.
- **`services/gateway/cmd/doctor/main.go`** — operator binary. Text + JSON output, exit-code-on-failure. Built into the Docker image and the release matrix alongside `mint-token` and `smoke-telegram`.

### Step 2 — Hook decision types schema

Adopts openclaw `src/plugins/hook-decision-types.ts`. Locks in the `Allow | Deny | Modify | Observe` taxonomy ahead of any plugin system.

- **`packages/schema/src/hooks.ts`** + **`runtime/core/src/hook.rs`** — `HookDecision` / `HookPhase` / `HookContext` / `HookResult` aligned across Rust and TS. 5 unit tests cover short-circuit behavior, label drift, wire-format lowercase serialization, kebab-case phases.

### Step 3 — ProviderProfile + OpenAI as Profile #2

Adopts hermes `providers/base.py:ProviderProfile`. Lifts the hardcoded-Anthropic seam into declarative profiles BEFORE a second provider lands.

- **`runtime/runtimed/src/provider.rs`** — new `ProviderProfile` (name, display_name, auth_type, base_url, default_headers, fixed_temperature, default_max_tokens, fallback_models, api_key_env). Refactored `AnthropicProvider` to carry a profile. Added `OpenAiProvider` (`/v1/chat/completions`, bearer auth).
- **`ListProviders` IPC verb** in `runtime/core/src/ipc.rs`; gateway client in `runtime/client.go`; handler in `handler/provider.go` serving `GET /api/v1/providers`.
- **`scripts/smoke-llm-openai.sh`** + **`.github/workflows/llm-smoke.yml`** — `IRONGOLEM_LLM_PROVIDER=openai` matrix job; skips cleanly (exit 2 → green) when secret unset.

### Step 4 — Commitments subsystem (backend)

Adopts openclaw `src/commitments/`. User-facing future-obligation tracking distinct from runtime-health heartbeats. Lifecycle: extraction → queue → fire → dismiss / snooze / expire.

- **`services/gateway/internal/commitments/`** — types, SQLite store with dedup by `workspace_id + dedupe_key`, heuristic regex+keyword extractor (LLM extractor lands in v0.4 once runtimed exposes a direct-LLM IPC verb), 60s ticker with 10min expire grace.
- **`gateway_commitments` table** + handler endpoints (`GET`, `dismiss`, `snooze`, admin `DELETE`).
- **Post-reply hook** on `Handler` lets the commitments runtime hook into every successful assistant turn for async extraction.

### Step 5 — Audit probe subsystem

Adopts openclaw `src/security/audit-*` taxonomy. Continuous-security-testing-as-runtime on top of the existing 5-layer enforcement.

- **`services/gateway/internal/audit/`** — `Probe` interface, `Finding` shape, `Severity` enum (info / warning / critical), Registry, 5min ticker with panic-recovery and invalid-severity normalization.
- **Four probes**: `workspace_skill_escape` (vacuous placeholder for v0.4 skill system), `trust_model` (HMAC secret presence + placeholder detection), `channel_dm_policy` (orphan + conflicting rule detection), `connector_health_drift` (env drift via Step 1's `CheckFn`).
- **`gateway_audit_findings` table** + **`GET /api/v2/audit/findings?severity=&limit=`**.
- New event kind **`audit.finding`** — emitted for non-info findings only.

### Step 6 — Frontend UX hardening

Retires the v0.2 silent-fail (`.catch(() => mockFallback)`).

- **`RouteSkeleton`** (cards / list / timeline variants), **`RouteErrorBoundary`** (per-route React class boundary), **`RouteError`** (caught-failure UI with retry).
- **`useRouteData<T>` hook** in `lib/route-data.ts` — `{ status, data, error, reload }` envelope. Supports seeded mode (Home/Inbox/Health) and skeleton-first (Commitments + Audit).
- **Home / Inbox / Health migrated** to envelope; real-mode failures surface via `<RouteError>` instead of stale mock data. Routes auto-wrapped in `<RouteErrorBoundary>` via `registry.tsx`.

### Step 7 — Commitments + Audit frontend

- **`/commitments`** route — filter chips by status, list of cards (kind + sensitivity + due-window + routing), Snooze 4h + Dismiss actions, mock + real-API hookup.
- **`/audit`** route — severity-filtered findings list with drill-down evidence panel.
- Nav entries added to `WorkspaceTopbar`.

### Step 8 — Connector breadth: Slack + Discord + Signal

Triples coverage from 3 to 6 connectors.

- **`connectors/slack/`** — `chat.postMessage` outbound + boot-time `auth.test`. Events API inbound stubbed (v0.4).
- **`connectors/discord/`** — `channels/{id}/messages` outbound + boot-time `/users/@me` validation. Gateway WebSocket inbound stubbed (v0.4).
- **`connectors/signal/`** — `signal-cli` shell bridge. CheckFn requires the account env AND the binary on PATH.
- Smoke scripts at `scripts/smoke-{slack,discord,signal}.sh`. Each exits 2 cleanly when env unset.
- `doctor --format=json` now enumerates all six connectors.

### Weaknesses tracked

10 weaknesses surfaced from the competitive scan: 5 ALREADY MITIGATED by IronGolem's three-domain split + event-sourcing + workspace-scoped HMAC claims (W1, W4, W6, W7, W9); the remaining 5 (W2/W3 plugin supply-chain, W5 audit isolation, W8 per-connector process isolation, W10 credential isolation) are gated on v0.4+ prerequisites and documented in the plan.

### Deferred to v0.4+

Shared content-addressable checkpoint store, plugin system (depends on WASM sandbox completion), per-connector process isolation, LLM-based commitment extractor (needs direct-LLM IPC verb), Slack Events API receiver, Discord Gateway WebSocket, Signal `signal-cli daemon` inbound, dark mode + comprehensive a11y, scheduler unpark, ESLint `no-inline-component-redefine` rule, StatusChip + SourcePill promotion to `@irongolem/ui`, visual baseline re-capture.

---

## v0.2 — Foundation Hardening (in progress)

Plan: [`Plans/v0.2-foundation.md`](Plans/v0.2-foundation.md). v0.1's spine handles requests; v0.2 hardens identity, real connectors, real-API frontend integration, and durable policy enforcement.

### Step 7 — Release polish

- **Visual baselines**: regenerated against the real production Vite build via `scripts/visual-capture.sh` so `make test-visual` is a meaningful regression gate.
- **This `CHANGELOG.md`** — first authoritative version log covering v0.1 + v0.2.

### Step 6 — Home + Health real-API ([#53](https://github.com/Ph4wkm00n/IronGolem_OS/pull/53))

Applied the Step 3 Inbox contract template to the two remaining routes whose backend signals already exist.

- **`GET /api/v1/home`** — workspace + heartbeat (probed live from gateway HTTP + db Ping + connector health) + events (from the SQLite event store, mapped to `EventItem` shape). Teams / trust / safety / research findings ship as structurally-correct stubs until the deferred services graduate.
- **`GET /api/v1/health/status`** — components probed from gateway, SQLite store, runtimed, and per-connector rows. `healEvents` + `predictive` ship as empty arrays (not `null`) so frontend `.map` calls don't blow up.
- **Frontend**: `Home.tsx` got an `events`-replace reducer action + `useEffect`; `Health.tsx` exposed state setters + dispatched all three on mount.
- **Tests**: full-shape coverage, workspace isolation, connector row inclusion, empty-array contract — 4 new tests.

### Step 5 — Real-LLM CI smoke ([#52](https://github.com/Ph4wkm00n/IronGolem_OS/pull/52))

- **`scripts/smoke-llm.sh`** + **`.github/workflows/llm-smoke.yml`** validate the Anthropic provider end-to-end without breaking the default mock path. Manual-dispatch + nightly cron. Pins `claude-haiku-4-5-20251001` for cost control (estimated < $0.001 / run).
- Exits 2 ("skipped") when `ANTHROPIC_API_KEY` is unset so forks see a clean signal instead of a red CI cross.

### Step 4 — Layer-4 channel policy store ([#51](https://github.com/Ph4wkm00n/IronGolem_OS/pull/51))

Replaces the v0.1 Step 7 env-flagged stub with durable per-channel enforcement.

- **`ChannelPolicyStore` interface** in `pkg/policy` + **`SQLiteChannelPolicyStore`** in `gateway/internal/policy`. Schema `channel_policies(channel_id, action, decision, reason, created_at)` with `PRIMARY KEY (channel_id, action)`.
- **Decision matrix**: empty store + env unset → allow with "disabled in v0.2" reason; rule installed → rule's decision verbatim; store error + env=true → fail-closed deny.
- 8 new/updated tests covering deny-by-rule, allow-by-rule, missing-rule fallback, error-fail-closed, and end-to-end engine wiring.

### Step 3 — F6 Inbox real-API ([#50](https://github.com/Ph4wkm00n/IronGolem_OS/pull/50))

First live frontend↔backend integration.

- **`GET /api/v1/inbox`** returns `message.inbound` audit events for the authenticated tenant + workspace, shaped to match the frontend `Item` type. Tenant + workspace come **exclusively** from HMAC token claims — body / query identity fields are not consulted.
- Frontend: `api.v2.inbox.list()` hits `/inbox` when `VITE_API_MODE_INBOX=real`. `Inbox.tsx` added a `replace` reducer action + `useEffect` that swaps the mock seed for the real list.
- 4 new tests including workspace isolation, content truncation, future-stamp clamping.

### Step 2 — WorkspaceID through HMAC token claims ([#49](https://github.com/Ph4wkm00n/IronGolem_OS/pull/49))

- **Token wire format**: 5 fields → 6 fields. `<tenant>:<workspace>:<user>:<role>:<channel>:<exp>.<mac>`. v0.1 tokens hard-fail at verify; no compat layer.
- Handler reads tenant + workspace from claims (never body) — fixes a tenant/workspace spoofing avenue.
- `mint-token` gained `--workspace` flag, defaults to the nil UUID for solo mode back-compat.

### Step 1 — Real Telegram connector wire-up ([#48](https://github.com/Ph4wkm00n/IronGolem_OS/pull/48))

Finishes the Step 8 Gate 3 prose intent: a real `connectors.telegram.Connector` now registers with the gateway's pump at boot.

- **Adapter**: `connector/telegram_source.go` wraps `*telegram.Connector` as `InboundSource`. Single seam translating between `connectors.Message` and gateway-local message types.
- **Cross-module dep**: `replace` directive in `services/go.mod` so the gateway can import the connectors module without restructuring either.
- **Gate 4 smoke**: `scripts/smoke-telegram.sh` + `cmd/smoke-telegram` harness with an httptest impersonator. Asserts `sendMessage(chat_id=X, text="pong")` round-trips end-to-end.

#### Bug fixes surfaced by Step 1

- **`connectors/telegram/telegram.go`** — latent RWMutex deadlock in `Connect()`: held `Lock()` across an HTTP call that needed `RLock()` on the same mutex. Latent because nothing previously called `Connect()`. Refactored to three-stage install→drop→commit pattern.
- **`auth_test.go`** — two tests pinned `time.Date(2026, 5, 13, 12:00 UTC)` and added 5–15 minutes for `ExpiresAt`. The calendar drifted past those exps; switched both to wall-clock `time.Now()`.

### Plan landing ([#47](https://github.com/Ph4wkm00n/IronGolem_OS/pull/47))

`Plans/v0.2-foundation.md` — sized + dependency-graphed roadmap. Same plan-then-build cadence as v0.1.

---

## v0.1 — Critical Moves Foundation

Plan: [`Plans/create-a-plan-to-glowing-nest.md`](Plans/create-a-plan-to-glowing-nest.md). Built the spine: HMAC-authenticated HTTP inbound → deterministic plan synthesis → `runtimed` execution over NDJSON IPC → reply with audit trail in SQLite.

### Step 8 — Verification harness + smoke-e2e ([#46](https://github.com/Ph4wkm00n/IronGolem_OS/pull/46))

The capstone PR. Three gates that prove the spine works end-to-end:

- **Gate 1** — Rust unit tests (sandbox host + runtimed NDJSON round-trip).
- **Gate 2** — `services/gateway/internal/runtime/client_test.go::TestClient_ExecuteEcho` spawns the real `runtimed` binary, sends a single-node ToolCall echo plan, asserts output equality + event ordering (`PlanCreated → PlanStepStarted → PlanStepCompleted → PlanCompleted`).
- **Gate 3** — `scripts/smoke-e2e.sh` boots the full stack with mock provider, mints a token via the new `gateway/cmd/mint-token` helper, posts inbound, asserts the reply and the audit event landed.

#### Bug fixes surfaced by Gate 3

- **`handler/handler.go`** — stamp default `workspace_id` (nil UUID) when an inbound message arrives without one (Rust deserializer rejects empty strings).
- **`runtime/client.go`** — fail-fast on `ExecutePlanResponse` carrying the nil request_id (runtimed's canonical parse-error marker). Previously the original caller would hang to its 30s timeout.

### Step 7 — Scope cuts + HMAC auth ([#45](https://github.com/Ph4wkm00n/IronGolem_OS/pull/45))

Security hardening + scope narrowing.

- **Scope cuts**: parked 7 services under `services/_deferred/` (defense, fleet, health, optimizer, research, scheduler, tenancy); deleted 6 connector stubs (browser, discord, feishu, filesystem, slack, whatsapp). `docker-compose.yml` + `Dockerfile.services` trimmed to gateway + web only.
- **HMAC bearer-token auth**: replaced `X-Tenant-ID` / `X-User-ID` header-trust with `Authorization: Bearer <token>`. Wire format `<tenant>:<user>:<role>:<channel>:<exp>.<hex_hmac>`. Fails closed at boot if `IRONGOLEM_HMAC_SECRET` is unset. `/healthz` is the only auth-exempt path.
- **Layer-4 fix**: env-flagged the per-channel restriction stub. Default `allow` with "disabled in v0.1" reason; `IRONGOLEM_LAYER4_ENABLED=true` → deny with "not implemented". No silent allow.
- 11 new tests covering the auth + chain integration.

### Step 4-6 — Backend foundation ([#44](https://github.com/Ph4wkm00n/IronGolem_OS/pull/44))

Single PR landing F1–F8 (frontend integration) plus backend Steps 4–6 in one coherent slice — they share contracts (`lib/api.ts` mock seam, runtime IPC types) so splitting would force one half to land against a moving target on the other.

- **Step 4 — Gateway runtime client**: `services/gateway/internal/runtime/client.go` spawns `runtimed` at boot, multiplexes Execute / Ping over NDJSON stdin/stdout, restarts on crash with exponential backoff (max 5 attempts). UUID v4 request_ids via `crypto/rand`.
- **Step 5 — Telegram inbound → plan synth**: deterministic 1-node `LlmCall` plan per inbound message. `handler.HandleInbound` is the shared entry point for HTTP `MessageInbound` and the connector receive-pump.
- **Step 6 — Persist gateway stores**: `services/gateway/internal/persist/db.go` owns the single `*sql.DB`. SQLite-backed event/recipe/approval/squad stores swap in. Default path `~/.irongolem/gateway.db`. Pure-Go `modernc.org/sqlite` driver (no cgo).
- **F1–F8 frontend integration**: landing zone, Tailwind ↔ design-tokens bridge, component-dedup audit, mock-data seam with `VITE_API_MODE_<ROUTE>` overrides, 8 v2 routes ported with React.lazy code-splitting, visual regression infrastructure (`make test-visual` + `scripts/png-diff.ts` zero-dep PNG diff), real-API smoke probe (`make check-real-api`).

### Steps 1-3 — Rust runtime + IPC contract

Pre-dating this changelog format but tracked in `Plans/create-a-plan-to-glowing-nest.md`.

- **Step 1**: IPC contract — NDJSON wire types in `runtime/core/src/ipc.rs` + `services/pkg/runtime/types.go`.
- **Step 2**: `runtimed` binary at `runtime/runtimed/`. `RealStepExecutor` dispatches on `PlanNodeKind` (LlmCall → Anthropic, ToolCall → sandbox).
- **Step 3**: `LocalSandboxHost` with tool registry. Two built-ins (`echo`, `http_get`).

---

## Pre-v0.1

### v1.0.0 — Initial OSS scaffolding (April 2026)

Initial public-ready repo state: monorepo layout, design tokens package, schema package, web shell, foundational CI workflows. Architecture details are documented in `docs/specs/` and `docs/implementation/`.

This tag predates the gateway-architecture redesign tracked in the v0.1+v0.2 plans above and uses a different versioning semantic — it captures "the project is open-source ready" rather than "the gateway architecture is at milestone X". The `v0.x` series resets the count against the architecture work.
