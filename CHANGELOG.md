# Changelog

All notable changes to IronGolem OS, organized by gateway-architecture milestone. Each subsection links to the plan step + PR that landed it.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions track the gateway-architecture milestones (`v0.1.x`, `v0.2.x`); the pre-existing `v1.0.0` repo tag references the initial OSS-scaffolding milestone and predates this changelog.

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
