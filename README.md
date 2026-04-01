```
 ___                  ____       _                    ___  ____
|_ _|_ __ ___  _ __ / ___| ___ | | ___ _ __ ___     / _ \/ ___|
 | || '__/ _ \| '_ \| |  _ / _ \| |/ _ \ '_ ` _ \  | | | \___ \
 | || | | (_) | | | | |_| | (_) | |  __/ | | | | | | |_| |___) |
|___|_|  \___/|_| |_|\____|\___/|_|\___|_| |_| |_|  \___/|____/

         Self-hosted autonomous assistant platform
```

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://img.shields.io/badge/CI-passing-brightgreen.svg)](#)
[![Status](https://img.shields.io/badge/Release-v0.1.0-orange.svg)](#changelog)

## What is IronGolem OS?

IronGolem OS is a self-hosted autonomous assistant platform you can run on your own hardware. It operates continuously, improves safely over time, explains every action it takes, and defends its environment proactively. Designed for non-technical users who want powerful automation without giving up control, it ships with five governed autonomous loops -- self-healing, self-learning, self-improving, auto-research, and self-defending -- so you get a genuinely self-sustaining assistant with full transparency.

## Features

| Feature | Description |
|---------|-------------|
| **Recipe Gallery** | Pre-built automation templates with plain-language safety summaries |
| **Assistant Squads** | Multi-agent teams (Inbox, Research, Ops, Security, Executive Assistant) |
| **5-Layer Security** | Gateway identity, tool policy, agent perms, channel restrictions, admin controls |
| **Knowledge Graph** | Evidence-backed memory with freshness and confidence scoring |
| **Health Center** | Heartbeat monitoring with calm, informative status displays |
| **Research Center** | Tracked topics, source fetching, contradiction detection |
| **Self-Healing** | Automatic failure detection, retries, config restoration, rollback |
| **Multi-Tenant** | Solo (SQLite), Household, and Team (PostgreSQL) deployment modes |
| **Multi-Channel** | Email, Calendar, Telegram, Slack, Discord, WhatsApp, and more |
| **Event Sourcing** | Full audit trail for every autonomous action |

## Architecture

```
                    +--------------------------+
                    |    Experience Layer       |
                    |  TypeScript/React + Tauri |
                    +------------+-------------+
                                 |
              +------------------+------------------+
              |          Go Control Plane            |
    +---------+---------+---------+---------+---------+
    | Gateway | Sched.  | Health  | Defense | Research|
    | :8080   | :8081   | :8082   | :8083   | :8085   |
    +---------+---------+---------+---------+---------+
    | Optim.  | Fleet   | Tenancy |
    | :8086   | :8087   | :8088   |
    +---------+---------+---------+
              |
    +---------+----------+
    |    Rust Runtime     |
    | Plan graphs, WASM,  |
    | Policy, Checkpoints |
    +-----+---------+----+
          |         |
    +-----+--+ +----+------+
    | SQLite | | PostgreSQL |
    | (Solo) | |  (Team)    |
    +--------+ +-----------+
```

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/Ph4wkm00n/IronGolem_OS.git
cd IronGolem_OS
docker compose -f infra/docker/docker-compose.yml up -d
```

Open `http://localhost:3000` in your browser. The gateway API is at `http://localhost:8080`.

### Manual Setup

```bash
# Prerequisites: Rust 1.75+, Go 1.22+, Node 20+, pnpm 9+

# Build everything
make build

# Run in development mode (Go services + web app)
make dev

# Or run components individually
make build-rust       # Rust runtime
make build-go         # Go services
make build-web        # Web frontend
```

See the [Deployment Guide](docs/guides/deployment-guide.md) for Solo, Household, and Team mode instructions.

## Screenshots

> Screenshots will be added after the UI stabilizes. See the [UI/UX Design Guide](docs/specs/05-ui-ux-design-guide-v2.md) for current mockups.

## Documentation

| Guide | Audience |
|-------|----------|
| [User Guide](docs/guides/user-guide.md) | End users and non-technical operators |
| [Deployment Guide](docs/guides/deployment-guide.md) | System administrators |
| [API Reference](docs/guides/api-reference.md) | Developers and integrators |
| [Getting Started](docs/guides/getting-started.md) | New contributors |
| [Architecture Overview](docs/architecture/overview.md) | Engineers |
| [Connector Development](docs/guides/connector-development.md) | Plugin authors |
| [Changelog](docs/CHANGELOG.md) | Everyone |

## Community

- [Contributing Guide](CONTRIBUTING.md) -- how to submit patches and proposals
- [Code of Conduct](CODE_OF_CONDUCT.md) -- expected behavior in project spaces
- [Security Policy](SECURITY.md) -- how to report vulnerabilities responsibly

## Built With

| Technology | Role |
|-----------|------|
| **Rust** | Trusted runtime -- plan graphs, policy enforcement, checkpointing, WASM plugins |
| **Go** | Control plane -- gateways, connectors, scheduler, health, tenancy APIs |
| **TypeScript / React** | Web application with Tailwind CSS |
| **Tauri** | Local-first desktop shell |
| **SQLite** | Single-user and household storage |
| **PostgreSQL** | Multi-tenant team storage |
| **OpenTelemetry** | Observability and tracing |
| **Docker** | Containerized deployment |

## License

Licensed under the [Apache License 2.0](LICENSE).
