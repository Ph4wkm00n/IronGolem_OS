# CLAUDE.md - IronGolem OS Development Guide

This file provides context for Claude Code when working on IronGolem OS.

## Project Overview

IronGolem OS is a self-hosted autonomous assistant platform with self-healing,
self-learning, auto-research, self-defending, and self-improving capabilities.
It targets non-technical users while maintaining enterprise-grade rigor.

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Runtime | **Rust** | Trusted execution: plan graphs, policy enforcement, checkpointing, WASM plugins |
| Control Plane | **Go** | Orchestration: gateways, connectors, scheduler, health, tenancy APIs |
| Frontend | **TypeScript/React** | Web app with Tailwind CSS |
| Desktop | **Tauri** | Local-first desktop shell wrapping the web app |
| Data (Solo) | **SQLite** | Single-user local storage |
| Data (Team) | **PostgreSQL** | Multi-tenant with per-workspace isolation |
| History | **Event sourcing** | Canonical source of truth for all actions |

## Repository Structure

See [docs/implementation/repository-structure.md](docs/implementation/repository-structure.md) for the full monorepo layout.

```
apps/          - web, desktop (Tauri), admin-console
services/      - Go microservices (gateway, scheduler, health, defense, research, optimizer, tenancy)
runtime/       - Rust crates (core, workflow, sandbox, memory, verifier, checkpoints)
connectors/    - Channel connectors (email, calendar, telegram, slack, etc.)
packages/      - Shared packages (ui, design-tokens, schema, policy-sdk, telemetry, provider-sdk)
infra/         - Docker, k8s, terraform, otel configs
docs/          - All documentation
```

## Architecture Rules

1. **Three-domain separation**: Rust runtime, Go control plane, TypeScript experience. Never mix responsibilities across domains.
2. **Every autonomous action must be inspectable** and produce an audit trail.
3. **Every loop must have pause, shadow, and rollback controls.**
4. **Five-layer security on every action**: gateway identity -> global tool policy -> per-agent permissions -> per-channel restrictions -> admin-only controls.
5. **Event sourcing** is the canonical history model. Never bypass it.
6. **Local-first** where practical, **multi-tenant** where needed.
7. **Trust before power** - default to least privilege.

## Canonical Specifications

These are the source-of-truth documents. Always consult them for authoritative details:

- **Product requirements**: `docs/specs/01-product-requirements-document-v2.md`
- **Features & modules**: `docs/specs/02-features-modules-and-agent-loops-v2.md`
- **Roadmap**: `docs/specs/03-roadmap-and-release-plan-v2.md`
- **Implementation plan**: `docs/specs/04-implementation-plan-v2.md`
- **UI/UX design**: `docs/specs/05-ui-ux-design-guide-v2.md`
- **Handbook (consolidated)**: `docs/specs/IronGolemOS_handbook.md`

## Coding Conventions

### Rust
- Standard Rust idioms; run `cargo clippy` before committing
- No `unwrap()` in production code; use `Result` types with proper error handling
- All `unsafe` blocks require justification comments
- Crate-level documentation for every public module

### Go
- Follow Effective Go and standard project layout
- Propagate `context.Context` for cancellation and tenant isolation
- Use structured logging (`slog` or equivalent)
- Table-driven tests preferred

### TypeScript
- React with functional components and hooks
- Strict TypeScript mode; no `any` in production code
- Tailwind CSS with design tokens from `packages/design-tokens`
- Components follow progressive disclosure pattern

## Testing Expectations

| Category | What to test |
|----------|-------------|
| Functional | Recipe activation, approval flows, connector messaging, squad delegation |
| Isolation | Tenant boundaries, workspace isolation, per-channel policy enforcement |
| Reliability | Failure injection, heartbeat timeouts, rollback verification |
| Security | Prompt injection suites, SSRF simulations, cross-tenant access attempts |
| UX | Onboarding success, policy comprehension, health center interpretation |

## Branch Conventions

- `main` - stable releases
- `develop` - integration branch
- `feature/*` - feature branches
- `fix/*` - bug fixes
- `docs/*` - documentation changes

## Key Concepts

- **Recipes**: User-facing automation templates with safety summaries
- **Squads**: Pre-composed multi-agent teams (Inbox, Research, Ops, Security, Executive Assistant)
- **Agent Roles**: Planner, Executor, Verifier, Researcher, Defender, Healer, Optimizer, Narrator, Router
- **Five Loops**: Self-healing, self-learning, self-improving, auto-research, self-defending
- **Heartbeats**: Periodic health check-ins with states: Healthy, Quietly Recovering, Needs Attention, Paused, Quarantined
- **Deployment Modes**: Solo (SQLite), Household (shared SQLite), Team (PostgreSQL multi-tenant)
