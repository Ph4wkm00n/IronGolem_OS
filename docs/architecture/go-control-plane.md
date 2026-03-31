# Go Control Plane Domain

The Go control plane handles **operations and orchestration** - everything that
connects, schedules, monitors, and manages the platform's external interactions.

## Purpose

- Channel gateways (ingress/egress for all messaging)
- Connector workers and fleet management
- Scheduler and recurring task orchestration
- Heartbeat manager and health aggregation
- Tenancy-aware APIs and admin endpoints
- Event fan-out and streaming
- Notification dispatch

## Services

### Gateway Service (`services/gateway/`)
Entry point for all external communication. Handles:
- Channel message ingress (Telegram, email, Slack, etc.)
- Outbound message delivery
- Authentication and identity verification (security layer 1)

### Connector Manager (`services/gateway/`)
Manages the lifecycle of channel and SaaS connectors:
- Token lifecycle and credential rotation
- Event normalization across connector types
- Connector-specific policy boundary enforcement
- Health signal emission for heartbeat monitoring

### Scheduler Service (`services/scheduler/`)
Orchestrates time-based automation:
- One-time scheduled tasks
- Recurring schedules (cron and interval patterns)
- Lane-based concurrency management
- Active hours and workspace-specific time windows
- Morning/evening digest generation

### Health Service (`services/health/`)
Aggregates health signals from all components:
- Heartbeat check-ins from connectors and services
- Recovery status tracking
- Incident summarization for the Health Center UI
- Suppress-on-OK behavior to reduce noise

### Defense Service (`services/defense/`)
Active security monitoring:
- Prompt injection detection
- SSRF protection and destination allowlists
- Unusual volume detection
- Anomaly scoring
- Quarantine workflow execution

### Tenancy Service (`services/tenancy/`)
Multi-tenant boundary enforcement:
- Workspace isolation
- Per-tenant data boundaries
- Role-based administration
- Cross-tenant access prevention

### Research Service (`services/research/`)
Automated research operations:
- Source fetching from approved endpoints
- Trust scoring and ranking
- Contradiction detection
- Brief synthesis

### Optimizer Service (`services/optimizer/`)
Continuous improvement operations:
- Prompt experiments and A/B comparison
- Provider performance comparison
- Reasoning depth tuning
- Cache effectiveness tracking

## Design Principles

- `context.Context` propagated everywhere for cancellation and tenant isolation
- Structured logging with `slog` or equivalent
- Table-driven tests preferred
- Every service emits health signals for heartbeat monitoring
- Connector failures degrade gracefully, never silently

## Canonical Reference

See the Go control-plane domain section in
[specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md).
