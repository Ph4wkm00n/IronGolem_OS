# IronGolem OS v2 Product \& Platform Handbook

## 0. Overview

### 0.1 Document purpose

This handbook is the single source of truth for IronGolem OS v2: what it is, who it serves, how it works, and how it will be delivered. It merges the v2 PRD, features/modules map, roadmap, implementation plan, and UI/UX guide into one aligned reference.

### 0.2 Audience \& usage

This document is written for product, design, engineering, security, and operations stakeholders, and doubles as a reference for internal alignment and fundraising or partner conversations. It is the narrative and technical backbone behind the clickable product site, architecture diagrams, and any pitch or kickoff decks derived from it.

### 0.3 Executive summary

IronGolem OS is a local-first, self-hosted autonomous assistant platform that runs on a Rust trusted runtime, a Go control plane, and a TypeScript experience layer, with a multi-tenant Postgres architecture for team mode. It provides a recipe-first, approval-centered UX, layered security and governance (five-layer permission model), heartbeat-driven health and self-healing, assistant squads, knowledge graph–backed memory, governed autonomous loops, and OTLP-ready observability.

***

## 1. Product Thesis

### 1.1 Product definition \& positioning

- **Product:** IronGolem OS
- **Category:** Self-hosted autonomous assistant platform
- **Positioning:** A secure, self-healing autonomous operations assistant for non-technical users, teams, and operators.
- **Revision theme:** v2 incorporates the strongest GoClaw-style platform capabilities—multi-tenant Postgres, deeper permissions, heartbeats, knowledge graphs, observability—while retaining IronGolem OS’s friendlier, explainable UX.

IronGolem OS competes in the same broad category as OpenClaw/GoClaw-style systems but differentiates on calm UX, plain-language governance, progressive autonomy, and explainable timelines.

### 1.2 Vision \& goals

**Vision:** IronGolem OS is a local-first, self-hosted autonomous assistant platform that can operate continuously, improve safely, explain itself clearly, and defend its environment proactively. It should feel understandable to a non-technical user and rigorous enough for a security-conscious operator.

**Product goals:**

1. Deliver a calm, recipe-first experience for non-technical users.
2. Match or exceed GoClaw-style reliability and deployment simplicity.
3. Implement layered security and autonomous self-defense by default.
4. Support autonomous research, learning, healing, and optimization loops.
5. Scale from desktop solo mode to multi-tenant team mode.
6. Provide broad multi-channel and multi-provider support without overwhelming configuration.

**Non-goals:**

- Offensive cybersecurity actions.
- Fully uncontrolled agent autonomy.
- A developer-only terminal product.
- Requiring users to understand raw policy code to be safe.
- Shipping every possible channel/model/connector in the first release.


### 1.3 Principles \& personas

**Product principles:**

- Trust before power.
- Default to least privilege.
- Local-first where practical, multi-tenant where needed.
- Every autonomous action must be inspectable.
- Every loop must have pause, shadow, and rollback controls.
- Good UI is part of safety.
- Platform rigor should be invisible until needed.

**Primary personas:**

1. **Everyday operator** – needs help with summaries, reminders, email triage, and recurring tasks, without heavy setup.
2. **Household coordinator** – needs role-aware automation for calendars, payments, travel, school, and home routines.
3. **Founder / small-team operator** – needs research, inbox support, scheduling, delegation, notes, and auditability.
4. **Power user / builder** – wants visual workflows, custom tools, advanced settings, and provider flexibility.
5. **Security-conscious admin** – needs strong permission boundaries, audit evidence, anomaly controls, quarantine, and deployment flexibility.

### 1.4 User problems \& value proposition

**Problems:**

- Existing self-hosted assistants feel too technical and fragile for non-experts.
- Autonomy is powerful but hard to supervise and trust.
- Reliability and remediation often require manual intervention and log-diving.
- Broad tool access creates security anxiety.
- Users want ambient help without losing control or observability.

**Core value proposition:**

IronGolem OS gives users a self-hosted autonomous assistant they can deploy on their own infrastructure, supervise visually, improve safely over time, and trust to operate within explicit boundaries, with clear explanations and rollback paths for autonomous behavior.

***

## 2. Operating Model \& Capabilities

### 2.1 Deployment modes \& tenancy

**Deployment modes:**

- **Solo local mode:** Desktop-focused, SQLite-backed, single workspace.
- **Household mode:** Shared workspace with role boundaries (e.g., family members).
- **Team mode:** PostgreSQL-backed multi-tenant deployment with per-workspace isolation.

**Isolation boundaries:**

- Tenant → Workspace → User → Channel → Agent session.

Each boundary is enforced in data models, policies, and runtime checks so that actions, memory, and events cannot cross isolation lines without explicit policy.

### 2.2 Five-layer security model

Every action passes through five explicit layers:

1. **Gateway identity \& authentication:** Who is this, what device, what session.
2. **Global tool policy:** Which tools are ever allowed, under which global constraints.
3. **Per-agent permissions:** What a specific assistant/agent can do.
4. **Per-channel restrictions:** What is allowed from a given channel (e.g., email vs chat).
5. **Owner/admin-only controls:** High-privilege actions gated to owners/admins.

The UX must translate this stack into understandable summaries via policy cards, connector scope labels, risk badges, approval markers, and admin-only states.

### 2.3 Connectors \& channels

**Initial connectors:**

- Email
- Calendar
- Filesystem
- Telegram
- Slack or Discord (one initially)
- Browser automation
- Webhook/API

**Expansion connectors:**

- WhatsApp
- Feishu/Lark
- Zalo or region-specific channels
- Docs and knowledge sources

All connectors must:

- Expose health signals and heartbeat status.
- Support isolated credentials per workspace or tenant.
- Emit normalized events into the runtime/control plane.
- Enforce connector-specific policy boundaries and allowlists.


### 2.4 Assistant orchestration \& squads

IronGolem OS supports multiple cooperating agent roles and composes them into **assistant squads** as user-facing abstractions.

**Agent roles (examples):**

- Planner, Executor, Verifier, Researcher, Defender, Healer, Optimizer, Narrator, Router.

**Squad examples:**

- Inbox Squad: classifier, drafter, verifier.
- Research Squad: scout, synthesizer, verifier.
- Ops Squad: planner, executor, reporter.
- Security Squad: monitor, defender, explainer.
- Executive Assistant Squad: planner, scheduler, briefer.

Squads support delegation, sync/async handoffs, verification checkpoints, and visual controls to pause, inspect, or edit.

### 2.5 Scheduling, heartbeats, and digests

The scheduler and heartbeat system provide ambient operations:

- One-time and recurring jobs (intervals and cron-style patterns).
- Workspace- and user-specific active hours.
- Morning/evening digests and heartbeat check-ins.
- Lane-based concurrency management for scheduled work.
- Heartbeat states such as Healthy, Quietly Recovering, Needs Attention, Paused, Quarantined.

Heartbeat and scheduling events feed into the **Health Center**, timelines, and optionally channels, always aiming for calm, low-noise communication.

### 2.6 Memory \& knowledge graph

Memory is event-sourced and enriched into graphs:

- **Event log:** Canonical history of actions, tool calls, and significant events.
- **Preference graph:** Encodes user and workspace preferences from repeated behavior.
- **Relationship graph:** Connects people, tasks, sources, and topics.
- **Knowledge graph:** Extracted from research and usage, with evidence links.

Each node includes:

- Evidence back-links.
- Freshness indicators.
- Confidence scores.
- Contradiction markers where sources disagree.


### 2.7 Observability \& auditability

The platform is observability-first:

- Trace spans for LLM calls, tool calls, and workflows.
- Structured logs with incident-friendly summaries.
- Timeline views of all user-relevant actions.
- Cache hit and latency metrics, connector health metrics.
- Optional OTLP export in team/advanced mode.
- Replayable execution traces and audit export packages for debugging and compliance.


### 2.8 Governed autonomous loops

IronGolem OS includes five governed loops:

1. **Self-healing** – handles failures (connectors, workflows, dependencies) via retries, restarts, configuration restore, and rollback, with escalation when needed.
2. **Self-learning** – learns from approvals, edits, and patterns to update preferences and propose better defaults or recipes.
3. **Self-improving** – experiments with prompts/providers, adjusts reasoning depth and caching, and rolls back on regression.
4. **Auto-research** – tracks topics, refreshes sources, detects contradictions, synthesizes briefs, and writes evidence-linked memory nodes.
5. **Self-defending** – detects prompt injection and other suspicious activity, denies or sandboxes actions, quarantines items, and reduces privileges with explainable incident summaries.

Each loop supports:

- Explicit scope limits.
- Pause and resume.
- Shadow mode (observe and learn without acting).
- Audit logging and confidence reporting.

***

## 3. Experience Model (UX)

### 3.1 UX mission \& promises

IronGolem OS v2 must make a more powerful platform feel simpler, safer, and more understandable. Users should feel:

- Informed, not overwhelmed.
- Protected, not restricted.
- Assisted, not replaced.
- In control, even when the system acts in the background.


### 3.2 Information architecture

**Primary navigation:**

- Home
- Inbox
- Recipes
- Research
- Memory
- Health
- Security
- Settings

**Admin / team additions:**

- Workspaces
- Members
- Policies
- Connectors
- Traces

This IA must be consistent with the underlying domains and modules so that each nav item maps cleanly to a conceptual and technical surface.

### 3.3 Key product surfaces

For each main surface, the handbook should capture purpose, key objects, and primary actions:

- **Home:** Overview of health, active recipes, squads, and important alerts.
- **Inbox:** Approvals, decisions, drafts, and research briefs needing attention.
- **Recipes:** Recipe gallery, safety summaries, and activation flows.
- **Research Center:** Tracked topics, briefs, contradictions, and evidence.
- **Memory Explorer:** List and graph views for memory nodes with evidence and freshness.
- **Health Center:** Heartbeats, recoveries, and system status.
- **Security Center:** Blocked actions, suspicious content, quarantined items, policy coverage, recommendations, and audit exports.
- **Settings:** Personal, workspace, and advanced controls.
- **Admin Console:** Workspaces, members, policies, connectors, and traces for admins.


### 3.4 Safety, policy, and trust UI

The UX must make the five-layer security model visible and understandable:

- **Safety blocks** on recipes, squads, and connectors:
    - Can access / Cannot access / Needs approval for / Stops automatically if.
- **Policy explainer:** “Who can trigger this?”, “Which agent acts?”, “Which channel is used?”, “Which tools are allowed?”, “What always needs approval?”.
- **Policy cards:** Layered visual representation of identity, tools, agents, channels, and admin-only controls.
- **Risk badges:** Clear but calm indicators of risk levels or required approvals.


### 3.5 Timelines, heartbeats, and incidents

The **timeline** is the primary explanation surface and must show:

- Actions taken.
- Actions proposed.
- Actions blocked.
- Actions healed.
- Actions quarantined.
- Research updates.
- Squad handoffs.

Heartbeat UX should be calm and informative, with simple language explaining:

- What was checked.
- What changed.
- Whether recovery succeeded.
- Whether user action is needed.

The Security Center surfaces blocked, suspicious, and quarantined events with concise, plain-language cause statements and next steps.

### 3.6 Memory \& research UX

The memory and research experiences should:

- Default to list and card views, with graph visualizations as an optional enhancement.
- Show evidence and freshness on every node.
- Make “why do you know this?” one click away.
- Display research cards with title, summary, confidence, freshness, source count, contradiction markers, and action suggestions.


### 3.7 Advanced mode

Advanced mode exposes platform internals for power users and admins:

- Traces and spans.
- Cache metrics and provider routing decisions.
- Reasoning controls and depth settings.
- Detailed policies and squad internals.

Default users never need advanced mode; it is surfaced via progressive disclosure from the standard interfaces.

### 3.8 Writing and tone guidelines

To keep the product approachable:

- Use plain language.
- Avoid raw platform terms (e.g., “orchestrator,” “vector,” “tenant,” “OTLP”) on default surfaces unless explained.
- Prefer outcome-oriented labels:
    - “Assistant team” instead of “agent squad” (in beginner contexts).
    - “Workspace” instead of “tenant” on user-facing screens.
    - “Safety rules” instead of “policy engine” in onboarding.


### 3.9 Accessibility \& mobile behavior

Accessibility and mobile design are first-class:

- All statuses (especially security and health) must be screen-reader friendly.
- Graph and trace views must have list/table equivalents.
- Policy cards must be usable without hover-only interactions.
- Reduced-motion modes should simplify graph and timeline transitions.
- On mobile:
    - Approvals and incident summaries are prioritized.
    - Heartbeats are compressed into concise summaries.
    - Swipeable cards and bottom nav are used where appropriate.
    - Security alerts may use full-screen sheets to avoid ambiguity.

***

## 4. System Architecture \& Modules

### 4.1 High-level architecture

IronGolem OS is organized into three primary domains:

- **Rust runtime domain (trusted execution):** Executes plans, enforces policies via adapters, orchestrates tools, manages checkpointing/rollback, writes to memory graphs, and hosts verifier/evaluator logic and WASM plugins.
- **Go control-plane domain:** Owns channel gateways, connector workers, scheduler and recurring jobs, tenancy-aware APIs, health aggregation, heartbeats, event fan-out/streaming, and admin control endpoints.
- **TypeScript experience domain:** Delivers the web app, desktop shell (via Tauri), responsive UX, command palette, and policy explainer layers.

State is event-sourced where possible, with SQLite in solo mode and PostgreSQL in team mode, and traces/logs emit into the observability stack.

### 4.2 Runtime domain (Rust)

Key responsibilities:

- Plan graph execution and state machine.
- Checkpointing and rollback manager.
- Verifier runtime for quality checks and gates.
- Sandbox host and isolation for tools.
- Risk metadata propagation through plans.
- WASM plugin host stub and contract.

This domain is the “trusted core” responsible for correctness, safety, and deterministic behavior where possible.

### 4.3 Control-plane domain (Go)

Key responsibilities:

- Gateway and channel ingress/egress.
- Connector management and worker fleets.
- Scheduler and recurring task orchestration.
- Heartbeat manager and health aggregation.
- Tenancy-aware APIs and admin interfaces.
- Notification dispatch and downstream event fan-out.

This domain focuses on concurrency, I/O, tenancy, and operational concerns.

### 4.4 Experience domain (TypeScript)

Key responsibilities:

- Web app shell (navigation, session handling, theming, layout).
- Desktop shell via Tauri for local-first distributions.
- Mobile-responsive approval and incident surfaces.
- Command palette and fast actions.
- Policy explainer and trust overlays.

It consumes contracts from the control plane and runtime and presents them in a way that matches the UX mission.

### 4.5 Multi-tenant model \& data boundaries

The multi-tenant architecture supports:

- Solo mode: local SQLite, one workspace.
- Household/team mode: PostgreSQL with per-tenant and per-workspace isolation.

Isolation levels:

- Tenant: hard data isolation boundary across organizations.
- Workspace: scoped sharing within a tenant (household, team, department).
- User: personal sessions, preferences, and private memory scopes.
- Channel: channel-specific scopes, tools, and restrictions.
- Agent session: per-run execution context and trace.

The implementation includes cross-tenant isolation tests and clear admin tooling to inspect boundaries.

### 4.6 Module catalog

**Identity \& tenancy module**

- Account model, trusted devices, workspace/tenant boundaries, household/team roles, and admin privileges.

**Connector module**

- Channel/SaaS/local tools, token lifecycle, event normalization, connector allowlists, and connector-specific policy boundaries.

**Agent runtime module**

- Goal parsing, recipe/plan selection, tool calling, approval requests, delegation handling, verification, and trace recording.

**Assistant squads module**

- Pre-composed multi-agent teams, templates, squad orchestration, and UX mapping for squads.

**Policy \& capability module**

- Capability assignment, risk scoring, approval thresholds, allowed destinations/commands, provider policies, channel restrictions, and owner-only escalation.

**Scheduler \& heartbeat module**

- Cron/interval jobs, active hours, digests, heartbeat checks, missed-check remediation, and lane management.

**Memory \& knowledge graph module**

- Event-sourced memory, graphs (preference, relationships, knowledge), freshness/confidence, evidence backlinks, and contradictions.

**Research module**

- Topic tracking, source ingestion, trust scoring, contradiction detection, change monitoring, and evidence-linked synthesis.

**Healing module**

- Connector monitoring, retry/backoff, last-known-good restore, rollback, self-tests, and incident summarization.

**Defense module**

- Prompt injection detection, SSRF protection, suspicious content isolation, command filtering, anomaly scoring, quarantine, and evidence retention.

**Improvement module**

- Prompt experiments, provider comparisons, caching strategies, reasoning-depth tuning, replay benchmarks, and regression rollbacks.

**Observability module**

- Traces, logs, event timeline, OTLP export, audit reports, health dashboards, and cache/latency metrics.


### 4.7 Example flows

**Team research brief:**

1. User activates a Research Squad on a tracked vendor/topic.
2. Scheduler creates recurring research jobs.
3. Researcher agent fetches approved, trusted sources.
4. Knowledge graph updates relationships, freshness, and confidence.
5. Verifier checks contradictions and confidence.
6. Narrator produces a brief in Inbox and Research Center, linked to evidence.
7. Defender checks source hygiene and isolation boundaries.

**Connector failure \& self-healing:**

1. Heartbeat detects a failing connector (e.g., calendar).
2. Healing module retries and validates token freshness.
3. On persistent failure, last-known-good configuration is restored.
4. Health Center shows “Recovered” or “Needs attention.”
5. Timeline logs the incident and actions taken for audit.

### 4.8 External integrations \& provider strategy

Provider strategy includes:

- Multi-provider model abstraction.
- Support for strongest commercial and compatible endpoints.
- Provider-specific reasoning controls where available.
- Prompt caching where available and safe.
- Cost, latency, and quality telemetry per provider.
- Policy-based provider selection (e.g., by workspace, recipe, or risk level).

***

## 5. Roadmap, Milestones, and Delivery

### 5.1 Roadmap phases (0–5)

**Phase 0 – Align (Month 0–2)**

- Ratify v2 architecture and tenancy/security models.
- Update product prototypes to reflect squads, heartbeats, and Security Center.
- Produce permission and tenancy model specs, event contracts, observability baseline.

**Phase 1 – Trust (Month 2–5)**

- Ship a robust solo-mode foundation with visible trust.
- Deliver Rust runtime and Go control-plane baselines.
- Desktop shell via Tauri, guided onboarding, recipe gallery v1, inbox/approvals/timeline.
- Core connectors (email, calendar, Telegram, filesystem).
- Self-healing baseline and heartbeat checks.
- Security Center baseline with injection filtering and blocked action reporting.

**Phase 2 – Govern (Month 5–8)**

- Add multi-tenant team mode and production-capable governance.
- PostgreSQL team mode, tenant-aware APIs, role-based administration.
- Five-layer permission UI + enforcement, shared assistant squads, admin console v1.
- Connector scope controls, OTLP-ready tracing path, knowledge graph v1.

**Phase 3 – Adapt (Month 8–11)**

- Deepen autonomy under guardrails and improve research/optimization.
- Research Center v1, tracked topic monitoring, contradiction detection.
- Preference graph learning, prompt caching and reasoning-depth tuning.
- Replay benchmarks, optimization engine v1, shadow-mode learning.

**Phase 4 – Defend (Month 11–14)**

- Make IronGolem OS operationally resilient and security-mature.
- Quarantine mode, SSRF protections, destination allowlists.
- Dangerous command restrictions, improved anomaly scoring.
- Config rollback center, connector canaries, stronger incidents and audits.

**Phase 5 – Expand (Month 14–18)**

- Broaden channels and ecosystem without breaking trust.
- Slack, Discord, WhatsApp, Feishu/Lark, browser automation, webhooks.
- Plugin SDK planning or alpha, richer squad templates, multilingual UX.
- Fleet management for advanced deployments.


### 5.2 Engineering milestones (A–E)

- **Milestone A – v2 skeleton:** Repos and shared packages in place; schemas and event contracts frozen for alpha; tenancy model drafted; policy engine stub; UI shell updated to new IA.
- **Milestone B – Solo product alpha:** Onboarding works end to end; first connector set live; recipes activate from gallery; inbox and timeline functional; heartbeat status visible.
- **Milestone C – Security \& reliability alpha:** Five-layer permission checks on core actions; Security Center shows blocked actions; self-healing and rollback; dangerous command protection in core tools.
- **Milestone D – Team mode beta:** PostgreSQL tenant mode live; admin console; assistant squads shareable; OTLP traces visible in advanced mode.
- **Milestone E – Adaptive systems beta:** Knowledge graph explorer; Research Center; prompt optimization and caching controls; shadow-mode learning supported.


### 5.3 Testing strategy

**Functional tests:**

- Recipe activation, approval flows, connector messaging, scheduled execution, squad delegation, graph updates.

**Isolation tests:**

- Tenant boundary validation, workspace-level connector isolation, per-channel policy checks, admin-only action gating.

**Reliability tests:**

- Connector failure injection, heartbeat timeouts, rollback verification, stale credential recovery.

**Security tests:**

- Prompt injection suites, SSRF simulation, command abuse scenarios, cross-tenant access attempts, quarantine flows.

**UX tests:**

- Non-technical onboarding success, policy comprehension, Health Center interpretation, security incident comprehension.


### 5.4 KPIs \& release criteria

**Product metrics:**

- Time to first successful recipe.
- Approval completion speed.
- Weekly active automations.
- User trust score.
- Percentage of users staying in guided mode vs abandoning setup.
- Automation retention after 30 days.

**Platform metrics:**

- Idle memory profile in local mode.
- Startup latency.
- Connector recovery success rate.
- Prompt cache effectiveness.
- Anomaly detection precision/recall.
- Tenant isolation incident count.

**v2 baseline release criteria:**

- Multi-tenant team mode architecture finalized.
- Five-layer permission model implemented end-to-end.
- Heartbeat diagnostics in Health Center.
- Assistant teams exposed as visual squads.
- Knowledge graph visible in Memory Explorer.
- OTLP-ready tracing path available.
- Prompt caching and provider reasoning controls in advanced settings.
- Security Center surfaces blocked actions, quarantines, and policy coverage.


### 5.5 Staffing \& ownership

IronGolem OS v2 requires:

- Early and sustained **security engineering** focus.
- Strong **product design ownership** over policy explainability and safety UX.
- A **platform engineer** dedicated to observability and deployment packaging.
- An **applied AI engineer** for research evaluation, prompt optimization, and loop tuning.

***

## 6. Governance \& Appendices

### 6.1 Security \& privacy posture

Security hardening includes:

- Prompt injection detection and suspicious-content isolation.
- SSRF protections and destination allowlists for web tools.
- Shell deny patterns and command approval workflows.
- Rate limiting, secret encryption at rest, tamper-evident audit trails.
- Per-tool and per-channel allowlists to keep blast radius small.
- Safe degradation strategies so failures reduce capabilities instead of silently breaking boundaries.


### 6.2 Glossary

Define canonical terms such as:

- Runtime, control plane, experience layer.
- Tenant, workspace, member, channel, session.
- Assistant squad, loop, heartbeat, policy layer.
- Memory graph, knowledge graph, trace, shadow mode, quarantine.



