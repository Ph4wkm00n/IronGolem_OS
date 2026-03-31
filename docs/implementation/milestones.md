# Engineering Milestones

Five engineering milestones mark key integration points across the delivery
phases.

## Milestone A: v2 Skeleton

**Target**: End of Phase 0

- Repositories and shared packages ready
- Schema and event contracts frozen for alpha
- Tenancy model drafted
- Policy engine stub in place
- UI shell updated to v2 information architecture

**Verification**: All three domains (Rust, Go, TypeScript) build and pass
basic smoke tests. Event contracts are documented and reviewed.

## Milestone B: Solo Product Alpha

**Target**: End of Phase 1

- Onboarding works end to end
- First connector set live (email, calendar, Telegram, filesystem)
- Recipes activate from gallery
- Inbox and timeline functional
- Heartbeat status visible in Health Center

**Verification**: A non-technical tester can install, onboard, activate a
recipe, and see heartbeat status without developer assistance.

## Milestone C: Security and Reliability Alpha

**Target**: Mid Phase 2

- Five-layer permission checks enforced on core actions
- Security center shows blocked actions
- Self-healing retries and rollback available
- Dangerous command protection active in core tools

**Verification**: Permission bypass attempts fail. Self-healing recovers from
simulated connector failures. Blocked actions appear in Security Center.

## Milestone D: Team Mode Beta

**Target**: End of Phase 2

- PostgreSQL tenant mode live
- Admin console available
- Assistant squads shareable inside workspace
- OTLP traces visible in advanced mode

**Verification**: Two separate tenants operate with full data isolation.
Admin can manage policies visually. Traces export via OTLP.

## Milestone E: Adaptive Systems Beta

**Target**: End of Phase 3

- Knowledge graph explorer working
- Research center live with tracked topics
- Prompt optimization and caching controls present
- Shadow-mode learning supported

**Verification**: Research briefs are generated with evidence links.
Preference graph updates from user behavior. Learning loop is reversible.

## Canonical Reference

See [specs/04-implementation-plan-v2.md](../specs/04-implementation-plan-v2.md).
