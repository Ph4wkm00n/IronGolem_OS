# Getting Started

This guide walks you through setting up a development environment for
IronGolem OS.

## Prerequisites

| Tool | Version | Domain |
|------|---------|--------|
| Rust | Latest stable | Runtime |
| Go | 1.22+ | Control plane |
| Node.js | 20+ | Frontend |
| pnpm | 9+ | Package management |
| SQLite | 3.40+ | Solo mode data |
| Docker | 24+ | Containerized development (optional) |
| PostgreSQL | 16+ | Team mode development (optional) |

## Clone the Repository

```bash
git clone https://github.com/Ph4wkm00n/IronGolem_OS.git
cd IronGolem_OS
```

## Domain-Specific Setup

### Rust Runtime

```bash
cd runtime
cargo build
cargo test
cargo clippy
```

### Go Control Plane

```bash
cd services
go mod download
go build ./...
go test ./...
```

### TypeScript Frontend

```bash
cd apps/web
pnpm install
pnpm dev       # Development server
pnpm build     # Production build
pnpm lint      # Lint check
```

### Tauri Desktop (requires Rust + Node.js)

```bash
cd apps/desktop
pnpm install
pnpm tauri dev
```

## Running Locally (Solo Mode)

```bash
# 1. Start Go services
cd services && go run ./cmd/server

# 2. Start web frontend
cd apps/web && pnpm dev

# 3. Open http://localhost:3000
```

## Running with Docker

```bash
docker compose up
```

## Project Structure

See [implementation/repository-structure.md](../implementation/repository-structure.md)
for the full monorepo layout.

## Next Steps

- Read the [Architecture Overview](../architecture/overview.md)
- Explore the [Autonomous Loops](autonomous-loops.md)
- Review the [Contributing Guide](../../CONTRIBUTING.md)
- Check the [Implementation Plan](../implementation/README.md) for current priorities

## Getting Help

- Open an issue on GitHub for bugs or questions
- Check existing issues before creating new ones
- See [CONTRIBUTING.md](../../CONTRIBUTING.md) for contribution guidelines
