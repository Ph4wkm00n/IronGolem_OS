# Repository Structure

IronGolem OS is organized as a monorepo to enable shared schemas, coordinated
releases, and consistent tooling across the Rust, Go, and TypeScript domains.

## Top-Level Layout

```
irongolem-os/
  apps/                    # User-facing applications
    web/                   # React web application
    desktop/               # Tauri desktop shell
    admin-console/         # Team administration UI

  services/                # Go microservices
    gateway/               # Channel ingress/egress
    scheduler/             # Task scheduling (cron, interval, one-time)
    health/                # Heartbeat monitoring and health aggregation
    defense/               # Security monitoring and quarantine
    research/              # Automated research operations
    optimizer/             # Prompt and provider optimization
    tenancy/               # Multi-tenant APIs and boundary enforcement

  runtime/                 # Rust crates
    core/                  # Plan graph execution engine
    workflow/              # Execution state machines
    sandbox/               # Tool isolation and resource limits
    memory/                # Knowledge graph storage and queries
    verifier/              # Output quality gates
    checkpoints/           # State snapshots and rollback

  connectors/              # Channel and service connectors
    email/
    calendar/
    telegram/
    slack/
    discord/
    whatsapp/
    filesystem/
    browser/
    docs/

  packages/                # Shared packages across domains
    ui/                    # Reusable UI component library
    design-tokens/         # Design system tokens (colors, spacing, typography)
    schema/                # Shared data contracts and event schemas
    policy-sdk/            # Policy authoring and evaluation
    telemetry/             # Observability utilities and OTLP helpers
    provider-sdk/          # Multi-provider LLM abstraction

  infra/                   # Infrastructure and deployment
    docker/                # Docker configurations
    k8s/                   # Kubernetes manifests
    terraform/             # Infrastructure as code
    otel/                  # OpenTelemetry collector configs

  docs/                    # All documentation
    specs/                 # v2 canonical specifications
    architecture/          # Architecture deep dives
    implementation/        # Implementation plan and phases
    design/                # UI/UX design system
    guides/                # Developer and contributor guides
```

## Build System

Each domain uses its native build tooling:
- **Rust**: Cargo workspace with crates under `runtime/`
- **Go**: Go modules with services under `services/`
- **TypeScript**: pnpm workspace with packages under `apps/` and `packages/`

## Shared Packages

The `packages/` directory contains code shared across domains:

| Package | Purpose | Used By |
|---------|---------|---------|
| `schema` | Event contracts, data models, API types | All domains |
| `ui` | React component library | `apps/web`, `apps/desktop`, `apps/admin-console` |
| `design-tokens` | Design system values | `packages/ui`, all apps |
| `policy-sdk` | Policy definition and evaluation | Runtime, control plane, frontend |
| `telemetry` | Tracing, logging, metrics helpers | All services |
| `provider-sdk` | LLM provider abstraction | Runtime, research, optimizer |

## Canonical Reference

See [specs/04-implementation-plan-v2.md](../specs/04-implementation-plan-v2.md).
