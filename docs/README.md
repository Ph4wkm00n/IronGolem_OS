# IronGolem OS Documentation

Welcome to the IronGolem OS documentation. This guide helps you find what you need.

## Documentation Map

### [Specifications](specs/) (v2 Source of Truth)
The canonical product specifications. All other docs reference these.
- [Product Requirements](specs/01-product-requirements-document-v2.md)
- [Features & Modules](specs/02-features-modules-and-agent-loops-v2.md)
- [Roadmap & Release Plan](specs/03-roadmap-and-release-plan-v2.md)
- [Implementation Plan](specs/04-implementation-plan-v2.md)
- [UI/UX Design Guide](specs/05-ui-ux-design-guide-v2.md)
- [Handbook (Consolidated)](specs/IronGolemOS_handbook.md)

### [Architecture](architecture/)
Technical architecture deep dives.
- [Overview](architecture/overview.md) - Three-domain architecture
- [Rust Runtime](architecture/rust-runtime.md) - Trusted execution core
- [Go Control Plane](architecture/go-control-plane.md) - Orchestration services
- [TypeScript Experience](architecture/typescript-experience.md) - Frontend layer
- [Data Layer](architecture/data-layer.md) - Storage and event sourcing
- [Security Model](architecture/security-model.md) - Five-layer permission system

### [Implementation Plan](implementation/)
Phased delivery plan with workstreams and milestones.
- [Overview](implementation/README.md) - Plan summary and index
- [Phase 0-5 Details](implementation/phase-0-alignment.md) - Per-phase breakdowns
- [Workstreams](implementation/workstreams.md) - Nine parallel workstreams
- [Milestones](implementation/milestones.md) - Engineering milestones A-E
- [Testing Strategy](implementation/testing-strategy.md)
- [Repository Structure](implementation/repository-structure.md)

### [UI/UX Design](design/)
Design system, patterns, and component specifications.
- [Overview](design/README.md) - Design plan summary
- [UX Mission & Pillars](design/ux-mission-and-pillars.md)
- [Information Architecture](design/information-architecture.md)
- [Design Patterns](design/design-patterns.md)
- [Component Catalog](design/component-catalog.md)
- [Mobile & Accessibility](design/mobile-and-accessibility.md)
- [Visual System](design/visual-system.md)

### [Developer Guides](guides/)
Hands-on guides for contributors and developers.
- [Getting Started](guides/getting-started.md)
- [Autonomous Loops](guides/autonomous-loops.md)
- [Connector Development](guides/connector-development.md)
- [Agent Roles](guides/agent-roles.md)

## Quick Links

| I want to... | Go to... |
|--------------|----------|
| Understand the project | [README.md](../README.md) |
| Set up development | [Getting Started](guides/getting-started.md) |
| Understand the architecture | [Architecture Overview](architecture/overview.md) |
| See what's planned | [Roadmap](specs/03-roadmap-and-release-plan-v2.md) |
| Contribute code | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Report a security issue | [SECURITY.md](../SECURITY.md) |
