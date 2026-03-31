# Architecture Overview

IronGolem OS uses a **three-domain architecture** that cleanly separates
concerns across technology boundaries.

## Domain Map

```
┌─────────────────────────────────────────────────────┐
│              TypeScript Experience Domain            │
│     Web App  |  Desktop (Tauri)  |  Admin Console   │
├─────────────────────────────────────────────────────┤
│                Go Control Plane Domain              │
│  Gateway | Scheduler | Health | Connectors | Tenant │
├─────────────────────────────────────────────────────┤
│               Rust Runtime Domain                   │
│  Plan Engine | Policy | Checkpoint | Memory | WASM  │
├─────────────────────────────────────────────────────┤
│                    Data Layer                        │
│        SQLite (Solo)  |  PostgreSQL (Team)           │
│               Event Sourcing (canonical)             │
└─────────────────────────────────────────────────────┘
```

## Domain Responsibilities

| Domain | Language | Owns |
|--------|----------|------|
| **Runtime** | Rust | Plan execution, policy enforcement, checkpointing/rollback, memory graph writes, WASM plugin host, verifier/evaluator logic |
| **Control Plane** | Go | Channel gateways, connector workers, scheduler, heartbeats, tenant-aware APIs, health aggregation, event streaming |
| **Experience** | TypeScript | Web app shell, Tauri desktop, mobile-responsive approval surfaces, command palette, policy explainer |

## Why Three Domains?

- **Rust** for the runtime because plan execution, policy enforcement, and
  rollback need memory safety, performance, and correctness guarantees
- **Go** for the control plane because connector management, scheduling, and
  multi-tenant APIs benefit from Go's concurrency model and deployment simplicity
- **TypeScript** for the frontend because React provides the best ecosystem for
  building accessible, progressive-disclosure UIs with design systems

## Cross-Cutting Concerns

These span all three domains:
- **Event sourcing** - canonical history model used by all domains
- **Observability** - traces, logs, metrics exported via OTLP
- **Security** - five-layer permission model enforced at every boundary
- **Tenancy** - workspace isolation from data to UI

## Detailed Domain Docs

- [Rust Runtime](rust-runtime.md)
- [Go Control Plane](go-control-plane.md)
- [TypeScript Experience](typescript-experience.md)
- [Data Layer](data-layer.md)
- [Security Model](security-model.md)

## Canonical Reference

See [specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md)
for the full module catalog.
