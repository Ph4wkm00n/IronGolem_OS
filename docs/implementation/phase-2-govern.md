# Phase 2: Team-Grade Architecture

**Timeline**: Month 5-8
**Theme**: Govern

## Goals

- Add multi-tenant team mode
- Make governance and administration production-capable

## Scope

- PostgreSQL team mode with per-workspace isolation
- Tenant-aware API and data boundaries
- Role-based administration
- Five-layer permission UI and enforcement
- Shared assistant squads within workspaces
- Admin console v1
- Connector scope controls
- OTLP-ready tracing path
- Knowledge graph v1

## KPIs

- Policy comprehension score
- Admin setup success rate
- Tenant isolation incident count (target: 0)
- Squad adoption rate

## Exit Criteria

- Tenant isolation test suite passes
- Admin users can audit and manage policies visually
- Team workspaces can share squads safely

## Canonical Reference

See [specs/03-roadmap-and-release-plan-v2.md](../specs/03-roadmap-and-release-plan-v2.md).
