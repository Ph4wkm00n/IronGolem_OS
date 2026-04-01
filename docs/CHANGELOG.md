# Changelog

All notable changes to IronGolem OS are documented in this file. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [0.1.0] - 2026-04-01

Initial open-source release.

### Phase 1 -- Trustworthy Local Core

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

### Phase 2 -- Team-Grade Architecture

- PostgreSQL multi-tenant team mode with per-workspace isolation
- Tenant-aware API and data boundaries
- Role-based administration and five-layer permission enforcement
- Shared assistant squads (Inbox, Research, Ops, Security, Executive Assistant)
- Admin console v1
- Connector scope controls (per-channel restrictions)
- OTLP-ready tracing pipeline

### Phase 3 -- Adaptive Intelligence

- Knowledge graph memory with confidence scoring and freshness tracking
- Self-learning loop: preference capture, prompt refinement, feedback integration
- Research center: tracked topics, source fetching, contradiction detection
- Optimizer service: prompt caching, A/B experiments, benchmark tooling
- Auto-research loop with scheduled briefs

### Phase 4 -- Defense and Resilience

- Self-defending loop: anomaly detection, quarantine, rollback
- Defense service with allowlist/blocklist management
- Canary checks and pre-deployment verification
- Incident timeline with full audit trail
- Fleet service for multi-instance monitoring (Team mode)

### Phase 5 -- Channel and Ecosystem Expansion

- Additional connectors: Slack, Discord, WhatsApp, Feishu/Lark, CalDAV, browser automation
- Webhook and generic REST connector
- Plugin SDK for community-built connectors
- Desktop app distribution (macOS, Windows, Linux)
- Documentation and onboarding improvements
