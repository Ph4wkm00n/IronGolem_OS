# IronGolem OS

> A secure, self-healing autonomous assistant you can host yourself.

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Phase_0_Planning-yellow.svg)](#project-status)

## What is IronGolem OS?

IronGolem OS is a self-hosted autonomous assistant platform that operates
continuously, improves safely over time, explains itself clearly, and defends
its environment proactively. It is designed to be understandable for non-technical
users while remaining rigorous enough for operators and security-conscious admins.

Unlike typical AI assistants, IronGolem OS runs **five governed autonomous loops**
that make it genuinely self-sustaining:

- **Self-Healing** - Detects failures, retries, restores configs, rolls back
- **Self-Learning** - Learns preferences from your approvals and edits
- **Self-Improving** - Optimizes prompts, providers, and reasoning depth
- **Auto-Research** - Tracks topics, fetches sources, detects contradictions
- **Self-Defending** - Detects injection, sandboxes threats, quarantines risks

## Key Features

- **Recipe Gallery** - Pre-built automation templates with safety summaries
- **Assistant Squads** - Multi-agent teams (Inbox, Research, Ops, Security)
- **5-Layer Security** - Gateway, tool policy, agent perms, channel restrictions, admin controls
- **Knowledge Graph** - Evidence-backed memory with freshness and confidence scoring
- **Health Center** - Heartbeat monitoring with calm, informative status displays
- **Multi-Tenant** - Solo (SQLite), Household, and Team (PostgreSQL) deployment modes
- **Multi-Channel** - Email, Calendar, Telegram, Slack, Discord, WhatsApp, and more

## Architecture

IronGolem OS uses a three-domain architecture:

| Domain | Technology | Responsibility |
|--------|-----------|---------------|
| **Trusted Runtime** | Rust | Plan execution, policy enforcement, checkpointing, WASM plugins |
| **Control Plane** | Go | Gateways, connectors, scheduler, health monitoring, tenancy |
| **Experience Layer** | TypeScript/React | Web app, Tauri desktop shell, mobile-responsive surfaces |

See [Architecture Overview](docs/architecture/overview.md) for details.

## Deployment Modes

| Mode | Database | Use Case |
|------|----------|----------|
| Solo | SQLite | Personal desktop assistant |
| Household | SQLite | Shared family workspace with role boundaries |
| Team | PostgreSQL | Multi-tenant organization with workspace isolation |

## Documentation

### Architecture
- [Architecture Overview](docs/architecture/overview.md)
- [Rust Runtime](docs/architecture/rust-runtime.md)
- [Go Control Plane](docs/architecture/go-control-plane.md)
- [TypeScript Experience](docs/architecture/typescript-experience.md)
- [Data Layer](docs/architecture/data-layer.md)
- [Security Model](docs/architecture/security-model.md)

### Implementation Plan
- [Implementation Overview](docs/implementation/README.md)
- [Phases 0-5](docs/implementation/phase-0-alignment.md)
- [Workstreams](docs/implementation/workstreams.md)
- [Milestones](docs/implementation/milestones.md)

### UI/UX Design
- [Design Overview](docs/design/README.md)
- [UX Mission & Pillars](docs/design/ux-mission-and-pillars.md)
- [Design Patterns](docs/design/design-patterns.md)

### Guides
- [Getting Started](docs/guides/getting-started.md)
- [Autonomous Loops](docs/guides/autonomous-loops.md)
- [Connector Development](docs/guides/connector-development.md)
- [Agent Roles](docs/guides/agent-roles.md)

### Specifications (v2)
- [Product Requirements](docs/specs/01-product-requirements-document-v2.md)
- [Features & Modules](docs/specs/02-features-modules-and-agent-loops-v2.md)
- [Roadmap](docs/specs/03-roadmap-and-release-plan-v2.md)
- [Implementation Plan](docs/specs/04-implementation-plan-v2.md)
- [UI/UX Design Guide](docs/specs/05-ui-ux-design-guide-v2.md)
- [Handbook](docs/specs/IronGolemOS_handbook.md)

## Project Status

**Phase 0: Architecture and Design Alignment** (active)

IronGolem OS is in the planning and architecture phase. The product
requirements, feature specifications, roadmap, implementation plan, and UX
design guide are complete. Implementation begins in Phase 1.

See the [Roadmap](docs/specs/03-roadmap-and-release-plan-v2.md) for the full
18-month delivery timeline.

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Licensed under the [Apache License 2.0](LICENSE).
