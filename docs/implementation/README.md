# Implementation Plan

Build IronGolem OS as a secure, multi-mode autonomous assistant platform with a
Rust trusted runtime, Go control plane, and TypeScript experience layer.

## Core Platform Decisions

- **Rust** remains the trusted execution core
- **Go** owns connector-heavy, concurrency-heavy, tenancy-aware services
- **TypeScript** owns product UX
- **SQLite** is solo-mode default; **PostgreSQL** is team-mode default
- **Event sourcing** remains the canonical history model
- Every user-relevant action must be traceable and explainable

## Delivery Strategy

1. Build solo mode first without compromising future team mode
2. Land permission and observability foundations before broad connector expansion
3. Test every autonomous loop in shadow mode before broader rollout
4. Keep advanced controls available but hidden behind progressive disclosure

## Phase Index

| Phase | Theme | Timeline | Focus |
|-------|-------|----------|-------|
| [Phase 0](phase-0-alignment.md) | Align | Month 0-2 | Architecture and design alignment |
| [Phase 1](phase-1-trust.md) | Trust | Month 2-5 | Trustworthy local core |
| [Phase 2](phase-2-govern.md) | Govern | Month 5-8 | Team-grade architecture |
| [Phase 3](phase-3-adapt.md) | Adapt | Month 8-11 | Adaptive intelligence |
| [Phase 4](phase-4-defend.md) | Defend | Month 11-14 | Defense and resilience |
| [Phase 5](phase-5-expand.md) | Expand | Month 14-18 | Channel and ecosystem expansion |

## Additional Documents

- [Workstreams](workstreams.md) - Nine parallel workstreams with deliverables
- [Milestones](milestones.md) - Engineering milestones A through E
- [Testing Strategy](testing-strategy.md) - Five test categories
- [Repository Structure](repository-structure.md) - Monorepo layout

## Canonical Reference

See [specs/04-implementation-plan-v2.md](../specs/04-implementation-plan-v2.md)
for the full source-of-truth implementation plan.
