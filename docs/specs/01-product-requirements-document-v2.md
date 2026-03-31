# IronGolem OS Product Requirements Document v2

## Document status
- Product: IronGolem OS
- Version: v2
- Category: Self-hosted autonomous assistant platform
- Positioning: A secure, self-healing autonomous operations assistant for non-technical users, teams, and operators
- Architecture baseline: Rust runtime + Go control plane + TypeScript frontend
- Revision theme: Incorporates the best GoClaw-inspired platform capabilities while preserving IronGolem OS's user-friendly, security-first product identity

## Revision summary
This v2 PRD updates IronGolem OS to explicitly incorporate the strongest platform features observed in GoClaw-like systems:
- multi-tenant PostgreSQL team architecture
- single-binary and low-footprint deployment philosophy
- five-layer permission architecture
- agent teams, delegation, and shared task boards
- heartbeat-based monitoring and recovery
- prompt caching and provider-specific reasoning controls
- stronger observability and OTLP support
- knowledge graph-based memory enrichment
- broader multi-channel connector strategy
- stronger security hardening including SSRF protection, shell deny rules, and encrypted secrets

IronGolem OS still differentiates itself through a superior UI/UX layer, plain-language governance, progressive autonomy, explainable timelines, and a non-technical-friendly control surface.

## Strategic context
OpenClaw is publicly described as a self-hosted gateway and assistant platform connecting messaging channels to AI agents, with local data, scheduled tasks, tool calling, multi-agent routing, and Node-based deployment guidance. GoClaw appears to strengthen that category with Go-native concurrency, multi-tenancy, production observability, broader provider support, deeper permissioning, heartbeat monitoring, knowledge graph support, and a single-binary deployment model.

IronGolem OS v2 is intended to combine those platform strengths with a safer and more approachable user experience.

## Vision
IronGolem OS is a local-first, self-hosted autonomous assistant platform that can operate continuously, improve safely, explain itself clearly, and defend its environment proactively. It should feel understandable enough for a non-technical user and rigorous enough for an operator or security-conscious admin.

## Product goals
1. Deliver a calm, recipe-first experience for non-technical users.
2. Match or exceed GoClaw-style production reliability and deployment simplicity.
3. Implement layered security and autonomous self-defense by default.
4. Support autonomous research, learning, healing, and optimization loops.
5. Scale from desktop solo mode to multi-tenant team mode.
6. Provide broad multi-channel and multi-provider support without making configuration overwhelming.

## Non-goals
- offensive cybersecurity actions
- fully uncontrolled agent autonomy
- a developer-only terminal product
- requiring users to understand raw policy code to use the system safely
- shipping every channel, model, and connector in the first release

## Product principles
- Trust before power.
- Default to least privilege.
- Local-first where practical, multi-tenant where needed.
- Every autonomous action must be inspectable.
- Every loop must have pause, shadow, and rollback controls.
- Good UI is part of safety.
- Platform rigor should be invisible until needed.

## Personas

### 1. Everyday operator
Needs a helpful assistant for summaries, reminders, email triage, and recurring tasks without technical setup complexity.

### 2. Household coordinator
Needs role-aware automation for calendars, payments, travel, school communication, and home routines.

### 3. Founder / small-team operator
Needs research, inbox support, scheduling, task delegation, notes, and strong auditability.

### 4. Power user / builder
Needs visual workflows, custom tools, advanced settings, and provider flexibility.

### 5. Security-conscious admin
Needs strong permission boundaries, audit evidence, anomaly controls, quarantine, and deployment flexibility.

## User problems
- existing self-hosted assistants are often too technical to manage confidently
- autonomy is powerful but difficult to supervise
- reliability and remediation often require manual intervention
- broad tool access creates security anxiety
- users want background help without losing control

## Core value proposition
IronGolem OS gives users an autonomous assistant they can deploy locally or on their own infrastructure, supervise visually, improve safely over time, and trust to operate within explicit boundaries.

## Product pillars

### Pillar 1: Guided autonomy
- recipe-first entry points
- safety slider per recipe and workspace
- plain-language policies
- approval workflows for higher-risk actions

### Pillar 2: Platform rigor
- Rust secure runtime
- Go connector and control-plane services
- event-driven orchestration
- multi-tenant Postgres for team mode
- low-footprint deployment path

### Pillar 3: Active defense
- five-layer permission enforcement
- prompt injection detection
- SSRF and dangerous command protections
- anomaly scoring
- quarantine mode

### Pillar 4: Continuous adaptation
- self-healing
- self-learning
- self-improving
- auto-research
- heartbeat-driven diagnostics and check-ins

## Functional requirements

### A. Deployment modes
- Solo local mode using SQLite.
- Team mode using PostgreSQL with per-workspace isolation.
- Packaged desktop deployment via Tauri.
- Small-server deployment path with low operational overhead.
- Single-package operational option for reduced setup complexity where feasible.

### B. Identity and tenancy
- workspace isolation for solo, household, and team modes
- tenant-aware context boundaries
- role-based controls
- per-user sessions and isolated memory scopes
- trusted device pairing

### C. Five-layer security architecture
The security model must explicitly support the following layers:
1. gateway and identity authentication
2. global tool policy
3. per-agent permissions
4. per-channel restrictions
5. owner-only or admin-only privileged actions

The UI must translate these layers into understandable summaries.

### D. Connectors and channels
Initial connectors:
- email
- calendar
- filesystem
- Telegram
- Slack or Discord
- browser automation
- webhook/API

Expansion connectors:
- WhatsApp
- Feishu/Lark
- Zalo or region-specific channels
- docs and knowledge sources

All connectors must:
- expose health signals
- support isolated credentials
- emit normalized events
- enforce connector-specific policy boundaries

### E. Agent orchestration and teams
IronGolem OS must support:
- multiple cooperating agent roles
- delegation between agents
- shared task boards for agent teams
- sync and async handoffs
- verifier and quality-gate checkpoints
- managed assistant squads exposed through UI templates

Agent team presets:
- Planner + Researcher + Executor + Verifier
- Inbox triage squad
- Research squad
- Operations squad
- Security and health squad

### F. Provider strategy
- multi-provider model abstraction
- support for strongest commercial and compatible model endpoints
- provider-specific reasoning controls where available
- prompt caching support where available
- cost, latency, and quality telemetry by provider
- policy-based provider selection

### G. Scheduling and heartbeat
- one-time scheduled tasks
- recurring schedules with every/cron patterns
- lane-based concurrency management for scheduled work
- heartbeat check-ins driven by policy-defined windows and active hours
- suppress-on-OK summaries to reduce noise
- delivery of significant heartbeat issues into dashboard and channels

### H. Memory and knowledge graph
- event log as source of truth
- preference graph
- relationship graph between people, tasks, sources, and topics
- knowledge graph extraction for research and personalization
- evidence and source links on memory nodes
- freshness and confidence indicators

### I. Observability and tracing
- timeline of every user-relevant action
- trace spans for LLM calls and tool calls
- cache hit metrics where prompt caching is used
- optional OTLP export in team/advanced mode
- structured logs and incident summaries
- replayable execution traces for debugging and audits

### J. Autonomous loops
IronGolem OS must include five governed loops:
- Self-healing
- Self-learning
- Self-improving
- Auto-research
- Self-defending

Each loop must support:
- scope limits
- pause and resume
- shadow mode
- audit logging
- confidence reporting

### K. Security hardening
Required controls:
- prompt injection detection
- suspicious content isolation
- SSRF protections for web-fetching tools
- shell deny patterns and command approval workflow
- rate limiting
- secret encryption at rest
- tamper-evident audit trails
- per-tool and per-channel allowlists

### L. UX requirements
- guided onboarding
- recipe gallery
- approvals inbox
- health center
- security center
- memory explorer
- research center
- timeline as primary explanation surface
- plain-language policy cards
- progressive disclosure for advanced settings

## Non-functional requirements
- local mode must remain lightweight enough for modest hardware
- startup should feel fast and responsive
- team mode must support reliable multi-tenant isolation
- UI must be accessible and mobile-friendly
- security model must degrade safely under partial failure
- observability must be present from the start, not bolted on later

## Success metrics

### Product metrics
- time to first successful recipe
- approval completion speed
- weekly active automations
- user trust score
- percentage of users staying in guided mode vs abandoning setup
- automation retention after 30 days

### Platform metrics
- idle memory profile in local mode
- startup latency
- connector recovery success rate
- prompt cache effectiveness where enabled
- anomaly detection precision/recall targets
- tenant isolation incident count

## Release criteria for v2-aligned product baseline
- multi-tenant team mode architecture finalized
- five-layer permission model implemented end-to-end
- heartbeat diagnostics appear in health center
- agent teams exposed as visual squads
- knowledge graph visible in memory explorer
- OTLP-ready tracing path exists for advanced deployments
- prompt caching and provider reasoning controls exposed in advanced settings
- security center shows blocked actions, quarantines, and policy coverage
