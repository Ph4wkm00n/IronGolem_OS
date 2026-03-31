# Workstreams

Nine parallel workstreams drive the implementation of IronGolem OS. Each
workstream delivers specific capabilities across phases.

## Workstream 1: Tenancy and Identity

| Deliverable | Description |
|------------|-------------|
| Tenant model | Core multi-tenant data model |
| Workspace hierarchy | Solo, household, team workspace structures |
| Household/team roles | Role-based access within workspaces |
| Trusted device model | Device pairing and session management |
| Access tokens | Token lifecycle and session policies |
| Isolation tests | Cross-tenant boundary validation |

## Workstream 2: Five-Layer Permissions

| Deliverable | Description |
|------------|-------------|
| Identity gateway checks | Layer 1: authentication and session validation |
| Global tool policy engine | Layer 2: deployment-wide tool restrictions |
| Per-agent permissions | Layer 3: agent-specific capability boundaries |
| Per-channel restrictions | Layer 4: channel-specific policy enforcement |
| Admin-only gating | Layer 5: privileged action approval |
| Policy summary renderer | UI component for plain-language policy display |

## Workstream 3: Rust Runtime Hardening

| Deliverable | Description |
|------------|-------------|
| Plan graph execution | DAG-based workflow execution engine |
| Checkpointing | State snapshot and persistence |
| Step replay | Deterministic re-execution for debugging |
| Rollback state manager | Restore to previous known-good state |
| Verifier contract | Quality gate interface and execution |
| WASM plugin host stub | Extensibility foundation |
| Risk metadata propagation | Risk scoring through every action |

## Workstream 4: Go Services and Control Plane

| Deliverable | Description |
|------------|-------------|
| Gateway service | Message ingress/egress for all channels |
| Connector manager | Connector lifecycle and fleet management |
| Scheduler | Cron, interval, and one-time task execution |
| Heartbeat manager | Health check-in monitoring and alerting |
| Tenancy-aware admin APIs | Multi-tenant management endpoints |
| Health aggregation | System-wide health status compilation |
| Notification dispatch | Cross-channel notification delivery |

## Workstream 5: Experience Layer

| Deliverable | Description |
|------------|-------------|
| Onboarding wizard | Guided first-run setup |
| Recipe gallery | Browse and activate automation templates |
| Inbox and approvals | Decision queue with approval workflows |
| Health center | Heartbeat status and recovery display |
| Security center | Blocked actions and quarantine management |
| Memory explorer | Knowledge graph browsing with evidence |
| Research center | Tracked topics and research briefs |
| Admin console | Team workspace management |
| Squad management | Assistant squad configuration and monitoring |

## Workstream 6: Knowledge Graph Memory

| Deliverable | Description |
|------------|-------------|
| Graph schema | Core entity and relationship types |
| Entity extraction pipeline | Automatic entity recognition from events |
| Preference graph | Learned user preferences from behavior |
| Source relationships | People, tasks, sources, topic connections |
| Freshness/confidence model | Scoring for information currency and reliability |
| Contradiction markers | Conflicting information detection |
| Graph query APIs | Programmatic access to graph data |

## Workstream 7: Research and Optimization

| Deliverable | Description |
|------------|-------------|
| Tracked topics | User-subscribed topic monitoring |
| Source scoring pipeline | Trust ranking for information sources |
| Contradiction detector | Cross-source conflict identification |
| Provider abstraction | Multi-LLM provider interface |
| Prompt caching metrics | Cache hit rate and cost tracking |
| Reasoning-depth control | Adjustable reasoning intensity |
| Benchmark harness | Quality comparison framework |

## Workstream 8: Security and Defense

| Deliverable | Description |
|------------|-------------|
| Prompt injection corpus | Detection training data and scoring |
| SSRF allowlist enforcement | Outbound request destination control |
| Shell denylist / approval | Dangerous command filtering |
| Anomaly detector | Behavioral anomaly scoring engine |
| Quarantine workflows | Isolation and review processes |
| Incident evidence store | Forensic data retention |

## Workstream 9: Observability

| Deliverable | Description |
|------------|-------------|
| Trace model | Span-based execution tracing |
| Log schema | Structured log format and contracts |
| Cache hit metrics | Prompt cache performance tracking |
| Connector health metrics | Per-connector health signals |
| OTLP exporter | OpenTelemetry Protocol export |
| Replay tooling | Execution trace replay for debugging |
| Audit export tooling | Compliance-ready audit reports |

## Canonical Reference

See [specs/04-implementation-plan-v2.md](../specs/04-implementation-plan-v2.md).
