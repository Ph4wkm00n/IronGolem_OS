# IronGolem OS Full Implementation Plan v2

## Implementation objective
Build IronGolem OS as a secure, multi-mode autonomous assistant platform with a Rust trusted runtime, Go control plane, and TypeScript experience layer, while explicitly supporting multi-tenancy, five-layer permissions, heartbeat operations, assistant squads, knowledge graph memory, prompt optimization, and OTLP-ready observability.

## Updated repository structure
```text
harbormind/
  apps/
    web/
    desktop/
    admin-console/
  services/
    gateway/
    scheduler/
    health/
    defense/
    research/
    optimizer/
    tenancy/
  runtime/
    core/
    workflow/
    sandbox/
    memory/
    verifier/
    checkpoints/
  connectors/
    email/
    calendar/
    telegram/
    slack/
    discord/
    whatsapp/
    filesystem/
    browser/
    docs/
  packages/
    ui/
    design-tokens/
    schema/
    policy-sdk/
    telemetry/
    provider-sdk/
  infra/
    docker/
    k8s/
    terraform/
    otel/
  docs/
```

## Core platform decisions
- Rust remains the trusted execution core.
- Go owns connector-heavy, concurrency-heavy, tenancy-aware services.
- TypeScript owns product UX.
- SQLite is solo-mode default.
- PostgreSQL is team-mode default.
- Event sourcing remains the canonical history model.
- Every user-relevant action must be traceable and explainable.

## Detailed workstreams

### Workstream 1: Tenancy and identity
Deliverables:
- tenant model
- workspace hierarchy
- household/team roles
- trusted device model
- access tokens and session policies
- cross-tenant isolation tests

### Workstream 2: Five-layer permissions
Deliverables:
- identity gateway checks
- global tool policy engine
- per-agent permissions
- per-channel restrictions
- admin-only privileged action gating
- policy summary renderer for UI

### Workstream 3: Rust runtime hardening
Deliverables:
- plan graph execution
- checkpointing
- step replay
- rollback state manager
- verifier contract
- WASM plugin host stub
- risk metadata propagation

### Workstream 4: Go services and control plane
Deliverables:
- gateway service
- connector manager
- scheduler and recurring tasks
- heartbeat manager
- tenancy-aware admin APIs
- health aggregation service
- notification dispatch service

### Workstream 5: Experience layer
Deliverables:
- onboarding wizard
- recipe gallery
- inbox and approvals
- health center
- security center
- memory explorer
- research center
- admin console
- assistant squad management

### Workstream 6: Knowledge graph memory
Deliverables:
- graph schema
- entity extraction pipeline
- preference graph
- source relationships
- freshness/confidence model
- contradiction markers
- graph query APIs

### Workstream 7: Research and optimization
Deliverables:
- tracked topics
- source scoring pipeline
- contradiction detector
- provider abstraction
- prompt caching metrics
- reasoning-depth control framework
- benchmark harness

### Workstream 8: Security and defense
Deliverables:
- prompt injection corpus and detector
- SSRF allowlist enforcement
- shell denylist / approval controls
- anomaly detector
- quarantine workflows
- incident evidence store

### Workstream 9: Observability
Deliverables:
- trace model
- log schema
- cache hit metrics
- connector health metrics
- OTLP exporter
- replay tooling
- audit export tooling

## Engineering milestones

### Milestone A: v2 skeleton
- repositories and shared packages ready
- schema and event contracts frozen for alpha
- tenancy model drafted
- policy engine stub in place
- UI shell updated to v2 IA

### Milestone B: solo product alpha
- onboarding works end to end
- first connector set live
- recipes activate from gallery
- inbox and timeline functional
- heartbeat status visible

### Milestone C: security and reliability alpha
- five-layer permission checks enforced on core actions
- security center shows blocked actions
- self-healing retries and rollback available
- dangerous command protection active in core tools

### Milestone D: team mode beta
- PostgreSQL tenant mode live
- admin console available
- assistant squads shareable inside workspace
- OTLP traces visible in advanced mode

### Milestone E: adaptive systems beta
- knowledge graph explorer working
- research center live
- prompt optimization and caching controls present
- shadow-mode learning supported

## Testing plan

### Functional tests
- recipe activation
- approval flows
- connector messaging
- schedule execution
- squad delegation
- graph updates

### Isolation tests
- tenant boundary validation
- workspace-level connector isolation
- per-channel policy restrictions
- admin-only action gating

### Reliability tests
- connector failure injection
- heartbeat timeout scenarios
- rollback verification
- stale credential recovery tests

### Security tests
- prompt injection suites
- SSRF attack simulations
- command abuse scenarios
- cross-tenant access attempts
- quarantine flow verification

### UX tests
- non-technical onboarding success
- policy comprehension tests
- health center interpretation tests
- security incident comprehension tests

## Delivery strategy
- build solo mode first without compromising future team mode
- land permission and observability foundations before broad connector expansion
- test every autonomous loop in shadow mode before broader rollout
- keep advanced controls available but hidden behind progressive disclosure

## Staffing implications
Add emphasis on:
- security engineering earlier in roadmap
- product design ownership over policy explainability
- platform engineer for observability and deployment packaging
- applied AI engineer for prompt optimization and research evaluation
