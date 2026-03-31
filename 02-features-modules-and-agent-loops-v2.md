# IronGolem OS Features, Modules, and Agent Loops v2

## Alignment summary
This v2 module map aligns IronGolem OS with the revised architecture: Rust secure runtime, Go control plane, TypeScript frontend, multi-tenant team mode, five-layer security, heartbeat-driven operations, knowledge graph memory, assistant squads, broader multi-channel support, and stronger observability.

## Product surfaces
- Home
- Inbox
- Recipes
- Research
- Memory
- Health
- Security
- Settings
- Admin / Team Console

## Core architectural domains

### 1. Rust runtime domain
Purpose:
- trusted execution of plans
- policy enforcement adapters
- secure tool orchestration
- checkpointing and rollback
- memory graph write-path
- verifier and evaluator execution
- WASM plugin host

Key submodules:
- plan engine
- execution state machine
- checkpoint manager
- rollback manager
- verifier runtime
- sandbox host
- risk primitives

### 2. Go control-plane domain
Purpose:
- channel gateways
- connector workers
- scheduler and recurring jobs
- tenant-aware APIs
- health aggregation
- heartbeats and reminders
- event fan-out and streaming
- admin control endpoints

Key submodules:
- gateway service
- scheduler service
- connector manager
- fleet manager
- health service
- tenancy service

### 3. TypeScript experience domain
Purpose:
- onboarding and guided setup
- recipe browsing and activation
- approvals and inbox
- research center
- memory explorer
- health and security centers
- admin console

Key submodules:
- web app shell
- desktop shell via Tauri
- mobile-responsive approval surfaces
- command palette
- policy explainer layer

## Multi-tenant model
IronGolem OS v2 supports:
- solo mode with local SQLite and single workspace
- household mode with shared workspace and role boundaries
- team mode with PostgreSQL-backed multi-tenant isolation

Isolation boundaries:
- tenant
- workspace
- user
- channel
- agent session

## Five-layer permission model
The v2 platform explicitly applies five permission layers to every action:
1. Gateway identity and authentication
2. Global tool policy
3. Per-agent permissions
4. Per-channel restrictions
5. Owner/admin-only controls

UI representations:
- policy card summary
- connector scope labels
- risk badges
- approval requirements
- admin-only shield states

## Module catalog

### Identity and tenancy module
Responsibilities:
- account model
- trusted devices
- workspace and tenant boundaries
- household and team roles
- admin privileges

### Connector module
Responsibilities:
- channel connectors
- SaaS connectors
- local tools
- event normalization
- token lifecycle
- connector allowlists
- connector-specific policy boundaries

Supported categories:
- messaging
- email
- calendar
- filesystem
- browser
- docs and knowledge sources
- generic webhooks/APIs

### Agent runtime module
Responsibilities:
- parse goal
- select recipe or plan
- call tools
- ask for approval when needed
- coordinate delegation
- verify outputs
- record evidence and execution trace

### Assistant squads module
Purpose:
Offer pre-composed multi-agent teams as a user-friendly abstraction.

Examples:
- Inbox Squad: classifier, drafter, verifier
- Research Squad: scout, synthesizer, verifier
- Ops Squad: planner, executor, reporter
- Security Squad: monitor, defender, explainer
- Executive Assistant Squad: planner, scheduler, briefer

### Policy and capability module
Responsibilities:
- capability assignment
- risk scoring
- approval thresholds
- allowed destinations and commands
- provider policies
- per-channel restrictions
- owner-only escalation

### Scheduler and heartbeat module
Responsibilities:
- cron and interval jobs
- active hours
- morning/evening digests
- heartbeat check-ins
- missed-check remediation
- schedule lane management

### Memory graph and knowledge graph module
Responsibilities:
- event-sourced memory
- preference graph
- person/topic/source/task relationships
- freshness scoring
- confidence weighting
- evidence backlinks
- contradiction markers

### Research module
Responsibilities:
- topic tracking
- source ingestion
- source trust scoring
- contradiction detection
- change monitoring
- report synthesis
- provider-aware research strategies

### Healing module
Responsibilities:
- connector heartbeat monitoring
- retry and backoff policies
- last-known-good configuration restore
- rollback execution
- self-test routines
- incident summarization

### Defense module
Responsibilities:
- prompt injection detection
- SSRF protection policies
- suspicious content isolation
- dangerous command filtering
- anomaly scoring
- quarantine actions
- incident evidence retention

### Improvement module
Responsibilities:
- prompt experiments
- provider comparisons
- prompt caching strategies
- reasoning-depth tuning
- replay benchmarks
- rollback on regression

### Observability module
Responsibilities:
- traces
- logs
- event timeline
- OTLP export
- audit reports
- health dashboards
- cache hit and latency metrics

## Feature map

### User-visible features
- Guided onboarding with role/persona presets
- Recipe gallery with safety summaries
- Assistant chat
- Approval inbox
- Action timeline
- Health center with heartbeats and recoveries
- Security center with blocked/quarantined actions
- Memory explorer with evidence graph
- Research center with tracked topics and changes
- Team admin console
- Desktop notifications and omnichannel approvals

### Advanced features
- Multi-tenant workspaces
- Assistant squads
- Prompt caching controls
- Provider reasoning controls
- OTLP observability export
- Replay and simulation
- Canary connectors
- Quarantine mode
- Plugin SDK roadmap

## Agent roles
- Planner
- Executor
- Verifier
- Researcher
- Defender
- Healer
- Optimizer
- Narrator
- Router

## Agent loop definitions

### Self-healing loop
Triggers:
- missed connector heartbeat
- repeated workflow failure
- dependency health drop
- policy-safe rollback candidate

Actions:
- retry
- restart connector
- rotate strategy
- restore config
- rollback to prior stable step
- escalate to user/admin

### Self-learning loop
Triggers:
- repeated approvals or rejections
- user edits to drafts
- consistent preferences over time
- recurring routines in behavior data

Actions:
- update preference graph
- adjust recommendation defaults
- propose recipe refinements in shadow mode
- improve scheduling or summaries

### Self-improving loop
Triggers:
- low approval rate
- high edit distance
- cost spikes
- latency spikes
- quality regressions

Actions:
- compare prompts
- compare providers
- adjust reasoning depth
- use prompt caching where safe
- promote best candidate
- rollback on regression

### Auto-research loop
Triggers:
- tracked topic updates
- source freshness expiry
- user subscription topics
- competitor or API change watches

Actions:
- fetch approved sources
- rank trust
- detect contradictions
- create briefs
- propose actions
- write evidence-linked memory nodes

### Self-defending loop
Triggers:
- injection indicators
- destination allowlist failures
- unusual send volume
- privilege escalation attempts
- suspicious cross-tenant access patterns

Actions:
- deny or sandbox
- quarantine
- reduce privileges
- require admin review
- produce explainable incident summary

## Example flow: team research brief
1. User activates Research Squad on a tracked vendor.
2. Scheduler creates recurring jobs.
3. Researcher fetches trusted sources.
4. Knowledge graph updates relationships and freshness.
5. Verifier checks contradictions and confidence.
6. Narrator creates brief for Inbox and Research Center.
7. Defender checks source hygiene and isolation boundaries.

## Example flow: connector failure
1. Heartbeat detects calendar connector failure.
2. Healing module retries and validates token freshness.
3. If retry fails, last-known-good path restores connector config.
4. Health Center shows “Recovered” or “Needs attention.”
5. Timeline logs the incident and action taken.
