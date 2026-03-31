# IronGolem OS Roadmap and Release Plan v2

## Roadmap intent
This v2 roadmap aligns IronGolem OS with the revised platform direction: stronger tenancy, layered security, heartbeat-driven reliability, assistant squads, knowledge graph memory, broader channels, prompt optimization, and observability maturity.

## Phase 0: Architecture and design alignment (Month 0-2)
Goals:
- ratify v2 architecture
- validate tenancy and security models
- update product prototypes to reflect assistant squads, heartbeats, and security center

Deliverables:
- v2 architecture decisions
- permission model spec
- tenancy model spec
- updated UI prototype pack
- observability baseline plan
- source-of-truth event contracts

Exit criteria:
- all product docs aligned to v2
- security review of five-layer model completed
- team signoff on local and team deployment paths

## Phase 1: Trustworthy local core (Month 2-5)
Goals:
- ship a solid solo-mode foundation
- deliver visible trust features from day one

Scope:
- Rust runtime baseline
- Go control plane baseline
- desktop shell via Tauri
- guided onboarding
- recipe gallery v1
- inbox, approvals, timeline
- connector support for email, calendar, Telegram, filesystem
- self-healing baseline with retries and heartbeat checks
- security center baseline with injection filtering and blocked action reporting
- provider abstraction and basic telemetry

Exit criteria:
- solo mode install is reliable
- first recipes are usable by non-technical testers
- heartbeat status visible in Health Center

## Phase 2: Team-grade architecture (Month 5-8)
Goals:
- add multi-tenant team mode
- make governance and administration production-capable

Scope:
- PostgreSQL team mode
- tenant-aware API and data boundaries
- role-based administration
- five-layer permission UI and enforcement
- shared assistant squads
- admin console v1
- connector scope controls
- OTLP-ready tracing path
- knowledge graph v1

Exit criteria:
- tenant isolation test suite passes
- admin users can audit and manage policies visually
- team workspaces can share squads safely

## Phase 3: Adaptive intelligence (Month 8-11)
Goals:
- deepen autonomy under guardrails
- improve research and optimization systems

Scope:
- research center v1
- tracked topic monitoring
- contradiction detection
- preference graph learning
- prompt caching and reasoning-depth tuning
- replay benchmarks
- optimization engine v1
- shadow mode learning controls

Exit criteria:
- research briefs are useful and evidence-backed
- optimization loop improves at least one core recipe materially
- learning loop remains explainable and reversible

## Phase 4: Defense and resilience hardening (Month 11-14)
Goals:
- make IronGolem OS operationally resilient and security mature

Scope:
- quarantine mode
- SSRF protections and destination allowlists
- dangerous command restrictions
- anomaly scoring improvements
- config rollback center
- connector canaries
- stronger incident workflows
- audit export packages

Exit criteria:
- containment workflows verified in simulations
- rollback paths proven in failure tests
- incident center understandable by non-expert admins

## Phase 5: Channel and ecosystem expansion (Month 14-18)
Goals:
- broaden utility without breaking trust model

Scope:
- Slack, Discord, WhatsApp, Feishu/Lark, browser automation, webhooks
- plugin SDK planning or alpha
- richer squad templates
- multilingual UX
- fleet management for advanced deployments

Exit criteria:
- connector expansion does not degrade baseline reliability
- channel-specific policy controls remain comprehensible

## Milestone themes
- Phase 0: Align
- Phase 1: Trust
- Phase 2: Govern
- Phase 3: Adapt
- Phase 4: Defend
- Phase 5: Expand

## KPI ladder

### Phase 1
- time to first recipe completion
- install success rate
- approval speed
- heartbeat visibility comprehension

### Phase 2
- policy comprehension score
- admin setup success
- tenant isolation incident count
- squad adoption rate

### Phase 3
- brief usefulness score
- reduction in user edits
- prompt cache efficiency
- model/provider optimization gains

### Phase 4
- anomaly false positive rate
- containment success rate
- rollback success rate
- incident triage time

### Phase 5
- connector adoption by channel
- plugin or extension readiness
- multi-workspace retention
