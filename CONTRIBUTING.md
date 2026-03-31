# Contributing to IronGolem OS

Thank you for your interest in contributing to IronGolem OS! This guide will
help you get started.

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Create a feature branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Run tests for the affected domain(s)
6. Commit with clear messages
7. Push to your fork and open a pull request

## Development Setup

See [docs/guides/getting-started.md](docs/guides/getting-started.md) for
environment setup instructions.

### Prerequisites

- **Rust** (latest stable) - for the runtime domain
- **Go 1.22+** - for the control plane domain
- **Node.js 20+** and **pnpm** - for the experience domain
- **SQLite** - for solo mode development
- **PostgreSQL 16+** - for team mode development (optional)
- **Docker** - for containerized development (optional)

## Code Style

### Rust
- Run `cargo fmt` and `cargo clippy` before committing
- No `unwrap()` in production code
- Use `Result` types with descriptive error variants

### Go
- Run `gofmt` and `golangci-lint`
- Follow standard Go project layout
- Propagate `context.Context` through call chains

### TypeScript
- Run `pnpm lint` and `pnpm format`
- Strict TypeScript mode required
- No `any` types in production code
- Use Tailwind CSS with project design tokens

## Pull Request Process

1. **Title**: Use a clear, descriptive title (e.g., "Add email connector heartbeat monitoring")
2. **Description**: Explain what changed and why
3. **Tests**: Include tests for new functionality
4. **Docs**: Update documentation if behavior changes
5. **Scope**: Keep PRs focused; one feature or fix per PR

### PR Labels

| Label | Meaning |
|-------|---------|
| `domain/rust` | Changes to Rust runtime |
| `domain/go` | Changes to Go control plane |
| `domain/ts` | Changes to TypeScript frontend |
| `type/feature` | New feature |
| `type/fix` | Bug fix |
| `type/docs` | Documentation only |
| `type/security` | Security-related change |

## Architecture Decisions

For significant architectural changes, open a discussion issue first. We use
Architecture Decision Records (ADRs) for important decisions. See existing
ADRs in `docs/architecture/` for examples.

## Reporting Issues

- **Bugs**: Use the bug report issue template
- **Features**: Use the feature request issue template
- **Security**: See [SECURITY.md](SECURITY.md) for private disclosure

## Code of Conduct

All contributors must follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

By contributing, you agree that your contributions will be licensed under the
Apache License 2.0.
