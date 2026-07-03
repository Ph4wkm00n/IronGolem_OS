---
project: IronGolem_OS
task: "Project ISA — IronGolem OS (active task: v0.4 adoption wave)"
effort: E3
effort_source: classifier
phase: complete
progress: 38/38
mode: interactive
started: 2026-07-02T22:25:39Z
updated: 2026-07-02T22:25:39Z
---

# IronGolem OS — Ideal State Artifact

## Problem

IronGolem OS is a self-hosted autonomous assistant platform (Rust runtime / Go control plane / TS experience) whose v0.3 release (tagged v1.2.0) absorbed seven architectural patterns from openclaw/openclaw and NousResearch/hermes-agent. Three gaps remain from that comparison: (1) the hermes shared content-addressable checkpoint store was never ported — `runtime/checkpoints/` does not exist; (2) Slack/Discord/Signal connectors ship outbound-only, with inbound paths explicitly stubbed for v0.4; (3) the commitments extractor is heuristic-only, pending a direct-LLM IPC verb in runtimed. Both upstream repos have ~7 weeks of drift since the May 16 scan that may contain further adoptable patterns, and the build system lacks fast-feedback targets.

## Vision

An operator upgrades to v0.4 and the platform closes its loop with the outside world: messages arriving on Slack/Discord/Signal flow inbound through the same normalized envelope as Telegram, commitments are extracted by a real model instead of regexes, every agent action can be checkpointed and restored from a deduplicated store, and the audit subsystem watches more of the surface area that actually fails in the wild. A contributor runs `make check` and gets an answer in seconds, not minutes.

## Out of Scope

- Re-adopting any v0.3 pattern (connector registry, HookDecision schema, ProviderProfile, commitments data model, audit framework skeleton) — those shipped in v1.2.0.
- A full plugin execution system (WASM host, install flow, marketplace) — only the permission-manifest schema groundwork lands in this wave.
- Per-connector process isolation and credential isolation — tracked weaknesses, gated on later waves.
- Browser automation multi-backend (hermes `browser_*`) — no product surface needs it yet.
- New frontend routes beyond what the adopted backends require.

## Constraints

- Three-domain separation is immovable: Rust owns trusted execution (checkpoints, IPC verbs), Go owns orchestration (connectors, commitments, audit), TS owns experience. No domain mixing.
- Event sourcing remains the canonical history model; new subsystems emit events, never bypass.
- No `unwrap()` in production Rust; `Result` + proper error types. Table-driven Go tests. Strict TS.
- Connectors must degrade gracefully: missing env/binary → clean skip, never crash the gateway.
- All changes keep `make build` and `make test` green across all three stacks.

## Goal

Ship the v0.4 adoption wave: port the hermes checkpoint shared-store into a new Rust crate, complete inbound paths for Slack/Discord/Signal, add a direct-LLM IPC verb enabling LLM-based commitment extraction with heuristic fallback, expand the audit probe taxonomy with the highest-value openclaw probes, land plugin permission-manifest groundwork, and add fast-feedback build targets — all verified by the existing three-stack test matrix.

## Criteria

### Research
- [x] ISC-1: Fresh openclaw clone examined; structured report with exact file paths exists covering plugins, audit taxonomy, commitments, inbound channels, post-May-16 drift
- [x] ISC-2: Fresh hermes-agent clone examined; structured report with exact file paths exists covering checkpoint_manager, ProviderProfile drift, PlatformEntry drift, scheduler
- [x] ISC-3: Adopt/skip decision recorded in ## Decisions for every candidate pattern surfaced by ISC-1/ISC-2

### Checkpoint shared-store (hermes port, Rust)
- [x] ISC-4: `runtime/checkpoints/` crate exists and is a member of the Cargo workspace
- [x] ISC-5: Content-addressable object store: identical content written twice yields one stored object (dedup unit test passes)
- [x] ISC-6: Store layout uses `refs/<id>` + content-addressed objects, mirroring hermes shared-store design
- [x] ISC-7: Public API supports create / restore / list checkpoint operations with `Result` error handling
- [x] ISC-8: GC/prune removes unreferenced objects; referenced objects survive (unit test passes)
- [x] ISC-9: `cargo test -p <checkpoints-crate>` passes
- [x] ISC-10: Anti: no `unwrap()` in the new crate's non-test code (`rg 'unwrap\(\)'` clean outside `#[cfg(test)]`)

### Inbound connectors
- [x] ISC-11: Slack inbound via Socket Mode worker: apps.connections.open → wss, immediate envelope ack, clean outbound-only skip when app token absent (refined per Advisor decision 2026-07-02)
- [x] ISC-12: Slack `message` events normalize into the existing inbound envelope (same shape Telegram uses)
- [x] ISC-13: Discord inbound path implemented (Gateway WS or documented interim mechanism) delivering MESSAGE_CREATE into the inbound envelope
- [x] ISC-14: Signal inbound path implemented via signal-cli receive loop with clean skip when binary/env absent
- [x] ISC-15: Table-driven Go tests cover each inbound normalization path
- [x] ISC-16: Anti: inbound handlers reject unauthenticated/unsigned payloads (negative test passes)

### LLM commitment extraction
- [x] ISC-17: New direct-LLM IPC verb exists in `runtime/core/src/ipc.rs` and is handled by runtimed
- [x] ISC-18: Go client method in `runtime/client.go` exercises the verb
- [x] ISC-19: Commitments LLM extractor produces structured commitments from a transcript when enabled
- [x] ISC-20: Extractor falls back to the heuristic path when runtime/LLM unavailable (test passes)
- [x] ISC-21: LLM extraction is opt-in via config; default remains heuristic
- [x] ISC-22: Go tests for extractor selection + fallback pass

### Audit probe expansion
- [x] ISC-23: ≥3 new probes ported from openclaw's taxonomy (selected from explorer report) under `services/gateway/internal/audit/probes/`
- [x] ISC-24: New probes registered in the audit ticker and persist findings to `gateway_audit_findings`
- [x] ISC-25: Each new probe has table-driven tests covering info/warning/critical paths

### Plugin groundwork
- [x] ISC-26: Plugin permission-manifest schema exists in `packages/schema` (TS) aligned with a Rust counterpart type
- [x] ISC-27: Manifest validation rejects undeclared permissions (unit tests in both languages pass)

### Build improvements
- [x] ISC-28: `make check` target exists: fast typecheck + `go vet` + `cargo clippy` without full builds
- [x] ISC-29: `make fmt` target exists formatting all three stacks
- [x] ISC-30: `make build` passes after all changes
- [x] ISC-31: `make test` passes across Rust, Go, connectors, web
- [x] ISC-32: `cargo clippy --workspace` reports no warnings introduced by this wave
- [x] ISC-33: `go vet ./...` clean in services/ and connectors/
- [x] ISC-34: `pnpm lint` clean

### Cross-cutting
- [x] ISC-35: Anti: no v0.3 pattern duplicated (no second registry/provider seam/hook schema)
- [x] ISC-36: Anti: no Rust/Go/TS responsibility crosses domains (checkpoints stay in Rust; connectors stay in Go)
- [x] ISC-37: CHANGELOG gains a v0.4 wave entry documenting adoptions with upstream attributions
- [x] ISC-38: `graphify update .` run after code changes completes successfully

## Test Strategy

| isc | type | check | threshold | tool |
|-----|------|-------|-----------|------|
| ISC-1..2 | research | explorer reports returned with file paths | non-empty, path-cited | Agent(Explore) |
| ISC-3 | doc | Decisions section lists adopt/skip per pattern | all candidates covered | Read |
| ISC-4..9 | unit | cargo test on new crate | all pass | Bash |
| ISC-10,16,35,36 | anti | rg / negative tests | zero hits / tests pass | Bash |
| ISC-11..15 | unit | go test connectors + gateway | all pass | Bash |
| ISC-17..22 | unit | cargo test + go test | all pass | Bash |
| ISC-23..25 | unit | go test ./internal/audit/... | all pass | Bash |
| ISC-26..27 | unit | pnpm test schema + cargo test | all pass | Bash |
| ISC-28..34 | build | make targets run | exit 0 | Bash |
| ISC-37 | doc | CHANGELOG contains v0.4 entry | present | Read |
| ISC-38 | tool | graphify update . | exit 0 | Bash |

## Features

| name | description | satisfies | depends_on | parallelizable |
|------|-------------|-----------|------------|----------------|
| research-refresh | Explore both upstream clones + IronGolem gap map | ISC-1..3 | — | true |
| checkpoint-store | Rust crate: content-addressable shared checkpoint store | ISC-4..10 | research-refresh | true |
| inbound-slack | Slack Events API inbound with signature verification | ISC-11,12,15,16 | research-refresh | true |
| inbound-discord | Discord inbound message path | ISC-13,15,16 | research-refresh | true |
| inbound-signal | signal-cli daemon receive loop | ISC-14,15,16 | research-refresh | true |
| llm-ipc-verb | Direct-LLM IPC verb Rust + Go client | ISC-17,18 | research-refresh | true |
| llm-extractor | Commitments LLM extractor + fallback | ISC-19..22 | llm-ipc-verb | false |
| audit-probes | Port top openclaw probes | ISC-23..25 | research-refresh | true |
| plugin-manifest | Permission-manifest schema TS+Rust | ISC-26,27 | research-refresh | true |
| build-targets | make check / make fmt + hygiene | ISC-28..34 | — | true |
| release-docs | CHANGELOG entry + graphify refresh | ISC-37,38 | all | false |

## Decisions

- 2026-07-02 — Seed-generated draft from README, CHANGELOG (v1.2.0–v1.2.2), docs/specs, last 15 commits, and the 2026-05-16 competitive-scan memory. ISCs seeded for the v0.4 adoption wave; refine via `Skill('ISA', 'interview')` if the wave scope shifts.
- 2026-07-02 — Tier E3 from mode classifier (fail-safe after timeout); session /effort=high supports comprehensive execution. Delegation: Forge auto-include + 3 Explore agents.
- 2026-07-02 — refined: ISC-4 means "shared-store backend module exists in the existing `irongolem-checkpoints` crate" — gap explorer found the crate already exists (SQLite-backed); the port adds a second `CheckpointStore` backend, not a new crate. ISC-6 layout: `refs/irongolem/<hash16>` + per-project `indexes/<hash16>` over one bare git object DB (hermes v2 design).
- 2026-07-02 — ADOPT/SKIP register (ISC-3), from the three explorer reports:
  - ADOPT hermes checkpoint v2 shared store → git-backed CAS backend beside `SqliteCheckpointStore`; plumbing-only writes (write-tree/commit-tree/update-ref CAS), config-isolation env (`GIT_CONFIG_GLOBAL=/dev/null`), commit-hash/path validation, oversize drop, keep-last-N prune. Shell to `git`; clean-skip when binary absent.
  - ADOPT openclaw audit probes (portable subset): `exposure_composition` (open channel policy × agent exec perms — openclaw `security.exposure.open_channels_with_exec`), `fs_permissions` (state DB/config world-readable/writable — `fs.*.perms_*`), `gateway_exposure` (non-loopback bind without auth — `gateway.bind_no_auth`). SKIP sandbox/plugin/model probes — no Docker sandbox or plugin loader shipped yet.
  - ADOPT openclaw commitments extraction design into new `LLMExtractor`: sensitivity-scaled confidence thresholds (care 0.86 / default 0.72), stable dedupe keys, JSON-only output with brace-matching fallback, self-disable after terminal LLM failure, heuristic fallback. Requires new `llm_call_request/response` IPC verb (flat struct, `kind`-tagged per ipc.rs:7-10 rules).
  - ADOPT inbound paths as the connector comments planned: Slack Events API webhook + signing verification (not Socket Mode — fits gateway HTTP server, no new WS dep for Slack), Discord Gateway WS minimal client (identify/heartbeat/dispatch), Signal `signal-cli receive --output=json` NDJSON loop. Plus the long-lived connector Worker pattern both stubs asked for.
  - ADOPT plugin-sdk permission enforcement: replace `AllowAllPolicyChecker` with manifest-permission validation; undeclared permission → reject. Mirror permission labels Rust↔TS with a sync test (same pattern as hook.rs:113).
  - ADOPT build/CI fixes: `make fmt`, `make check`, connectors folded into `make build`, remove CI TS `|| true` failure-swallowing, add TS lint to CI.
  - SKIP (v0.5 candidates, recorded): hermes PlatformEntry extensions (is_connected/platform_hint/config-bridge fns — revisit with scheduler unpark), deferred-loading registry (Go blank imports are cheap; startup latency not a problem), Chronos cron patterns (scheduler still parked in services/_deferred/), ProviderProfile reasoning_config hooks (no reasoning-tunable provider wired), openclaw install-policy exec oracle + ClawHub-style trust gate (no plugin install flow exists yet — strongest v0.5 candidate once WASM sandbox lands), DaemonThreadPool (goroutines already cover it).
- 2026-07-02 — Advisor (Rule 2, pre-BUILD): (a) shell-to-git port is the right parity call; test contended `update-ref <ref> <new> <old>` CAS explicitly; gitoxide migration is a follow-up, not this wave. (b) refined: ISC-11 — Slack inbound switches from Events API webhook to **Socket Mode** worker (apps.connections.open → wss, envelope ack): consistent with the Worker pattern and adds zero inbound HTTP exposure, which our own new exposure probes would otherwise flag. `signingSecret` stays reserved for a future HTTP-fronted deployment. (c) Ordering: CI-unswallow first → checkpoint store → permission enforcement + llm_call together → ingress workers → audit probes. (d) LLM extractor falls back to heuristic on failure OR low confidence, threshold as a tested boundary. (e) Commit per component, no batch commit. Advisor's "ISA mismatch" flag was an --auto-state artifact (stale task ISA loaded instead of this project ISA); the real ISA covers the wave.
- 2026-07-02 — Forge (codex exec) stalled 600s with zero output and was killed by the watchdog — same root cause family as the pnpm TLS failure (session env ships `NODE_EXTRA_CA_CERTS` with a literal unexpanded `$HOME`, breaking Node-based tools' TLS). Checkpoint store reassigned to a Claude-family general-purpose agent; Forge auto-include binding was honored (invoked, failed environmentally). Inbound agent stalled mid-work and was resumed from transcript.
- 2026-07-02 — CI `|| true` removal immediately exposed two real latent failures: packages/i18n locales typed `typeof en` against an `as const` source (every translated string a type error) and workspace-wide `eslint: command not found` (lint scripts existed, dependency never installed). Fixed: mapped-type `{ [K in keyof typeof en]: string }` keeps key-completeness; eslint 9 + typescript-eslint flat config added at root.
- 2026-07-02 — Priority order if budget forces descoping: checkpoint-store → build-targets → llm-ipc-verb/extractor → audit-probes → inbound-slack → inbound-discord/signal → plugin-manifest. Descoped items become `[DEFERRED-VERIFY]` with follow-up IDs, never silently dropped.

## Verification

- ISC-11/12: Bash — `go test ./slack/` ok (Socket Mode envelope ack + normalization tests pass, incl. malformed frames)
- ISC-13: Bash — `go test ./discord/` ok (Gateway IDENTIFY/intents/MESSAGE_CREATE normalization tests pass)
- ISC-14: Bash — `go test ./signal/` ok 0.342s (NDJSON receive normalization, 11 cases incl. hostile input)
- ISC-15: Read — table-driven `inbound_test.go` present in slack/, discord/, signal/
- ISC-16: Bash — malformed/unauthenticated event cases return (nil,false), suites green; workers panic-recover (worker_test.go)
- ISC-28: Bash — `make check` exit 0 (cargo check + go vet ×2 + pnpm typecheck)
- ISC-29: Bash — `make fmt` ran all three stacks; `gofmt -l` now returns 0 files
- ISC-34: Bash — `pnpm lint` exit 0 (0 errors, 3 exhaustive-deps warnings by design)
- ISC-3: Read — ADOPT/SKIP register in ## Decisions covers all candidates from three explorer reports
- ISC-4..9: Bash — `cargo test -p irongolem-checkpoints` 15 passed (dedup via count-objects, CAS conflict, restore round-trip, prune, git-absent)
- ISC-10: Bash — `rg unwrap\(\)` first hit line 917; test module starts line 900 — zero production unwraps
- ISC-17: Bash — cargo test runtimed 11 passed incl. direct_llm_call_completes/failure tests; verb dispatched in main.rs
- ISC-18: Read — Client.LlmCall in services/gateway/internal/runtime/client.go; services build green
- ISC-19..22: Bash — `go test ./gateway/internal/commitments/` ok (14 LLM extractor cases: boundaries 0.72/0.719, 0.86/0.859, cooldown, parse-fallback, enum rejection)
- ISC-23..25: Bash — `go test ./gateway/internal/audit/...` ok (3 probes, 16 cases incl. negative/critical paths); registered in main.go ticker
- ISC-26..27: Bash — bun test plugin-sdk 8 passed; cargo test core plugin 4 passed (wire-format sync, undeclared rejection)
- ISC-30: Bash — `make build` exit 0 (now includes connectors)
- ISC-31: Bash — `make test` exit 0, zero FAIL across Rust/Go/connectors/web
- ISC-32: Bash — `cargo clippy --workspace -- -D warnings` Finished clean
- ISC-33: Bash — `go vet ./...` clean in services and connectors
- ISC-35: Read — extended existing registry/audit/commitments/provider seams; no duplicate subsystems created
- ISC-36: Read — checkpoints Rust-only; connectors/probes/extractor Go-only; schema/plugin-sdk TS with Rust mirror types only
- ISC-37: Read — CHANGELOG.md:9 "v0.4 — Adoption Wave 2" with per-component attributions
- ISC-38: Bash — `graphify update .` exit 0 "Code graph updated"

## Changelog

- 2026-07-02 — conjectured: Slack inbound should use the Events API webhook (as the v0.3 stub comments planned, signingSecret reserved for it). refuted by: advisor exposure analysis — a public HTTP endpoint adds exactly the inbound surface the new exposure_composition/gateway_exposure probes exist to flag, and it breaks the one-Worker-pattern consistency across the three connectors. learned: when a wave ships both ingress AND exposure auditing, choose the ingress that the audit would score best — the subsystems must not indict each other. criterion now: ISC-11 refined to Socket Mode worker with clean outbound-only skip.

## Decisions (post-VERIFY)

- 2026-07-02 — Advisor final pass: no fatal gaps. Actioned: CHANGELOG labels GitSharedStore "staged" (crate complete, plan-execution wiring is a release-gate item) and inbound as unit-verified with live-protocol smoke deferred (no Slack/Discord/Signal credentials in this environment). Branch pushed; PR #89 opened for server-side CI validation. Follow-up (release gate before tagging v0.4): [1] live handshake smoke per connector, [2] Checkpoint plan-node → GitSharedStore wiring.
- 2026-07-02 — Delegation floor: met via 3× Explore (research) + inbound implementation agent + checkpoint implementation agent. Forge was invoked per the E3 auto-include binding but stalled on codex exec (environment: Node TLS breakage from unexpanded $HOME in NODE_EXTRA_CA_CERTS + 600s subagent stall watchdog); work reassigned to a Claude-family agent.
- 2026-07-03 — RELEASED: PR #89 squash-merged to main (537b972) after 3 CI-fix commits (fmt drift, build-before-typecheck ordering, bun on runner — server CI caught all three; local runs had masked them). Tagged v1.3.0; release workflow published 27 binary assets; notes enriched with wave highlights. Release-gate items for production posture remain: live connector handshakes, Checkpoint plan-node wiring.
