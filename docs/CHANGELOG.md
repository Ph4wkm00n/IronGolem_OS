# Changelog

> **This file has been superseded.** The canonical release log lives at
> [`/CHANGELOG.md`](../CHANGELOG.md) in the repo root and is updated per
> step in each gateway-architecture milestone (v0.1, v0.2, v0.3, …).
>
> This file remains as a historical snapshot of the pre-v0.1 phase plan.
> All releases from `v0.1.0` onward are documented in the root changelog.

---

## Historical phase plan (pre-v0.1)

The four-phase plan below predates the milestone-based release log. It
describes the *intended* OSS-scaffolding sequence at project inception,
not the actual ship history. For what shipped and when, read
[`/CHANGELOG.md`](../CHANGELOG.md).

### Phase 1 — Trustworthy Local Core

- Rust runtime baseline: plan graphs, policy enforcement, checkpointing, WASM sandbox
- Go control plane: gateway, scheduler, health, defense services
- Tauri desktop shell wrapping the React web app
- Guided onboarding wizard for first-time users
- Recipe gallery v1 with safety summaries
- Inbox with approval/reject workflows and activity timeline
- Connector support: email (IMAP/SMTP), Google Calendar, Telegram, local filesystem
- Self-healing baseline: automatic retries, config restoration, heartbeat checks
- Security center baseline: prompt injection filtering, blocked action reporting
- Provider abstraction layer for LLM backends
- Basic OpenTelemetry tracing

### Phase 2 — Team-Grade Architecture

- PostgreSQL multi-tenant team mode with per-workspace isolation
- Tenant-aware API and data boundaries
- Role-based administration and five-layer permission enforcement
- Shared assistant squads (Inbox, Research, Ops, Security, Executive Assistant)
- Admin console v1
- Connector scope controls (per-channel restrictions)
- OTLP-ready tracing pipeline

### Phase 3 — Adaptive Intelligence

- Knowledge graph memory with confidence scoring and freshness tracking
- Self-learning loop: preference capture, prompt refinement, feedback integration
- Research center: tracked topics, source fetching, contradiction detection
- Optimizer service: prompt caching, A/B experiments, benchmark tooling
- Auto-research loop with scheduled briefs

### Phase 4 — Defense and Resilience

- Self-defending loop: anomaly detection, quarantine, rollback
- Defense service with allowlist/blocklist management
- Canary checks and pre-deployment verification
- Incident timeline with full audit trail
- Fleet service for multi-instance monitoring (Team mode)

### Phase 5 — Channel and Ecosystem Expansion

- Additional connectors: Slack, Discord, WhatsApp, Feishu/Lark, CalDAV, browser automation
- Webhook and generic REST connector
- Plugin SDK for community-built connectors
- Desktop app distribution (macOS, Windows, Linux)
- Documentation and onboarding improvements
