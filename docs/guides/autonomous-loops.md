# Autonomous Loops

IronGolem OS operates five governed autonomous loops. Each loop has built-in
safeguards: scope limits, pause/resume, shadow mode, audit logging, and
confidence reporting.

## Loop Overview

| Loop | Purpose | Domain |
|------|---------|--------|
| Self-Healing | Recover from failures automatically | Go (Healing module) |
| Self-Learning | Learn preferences from user behavior | Rust (Memory) + Go |
| Self-Improving | Optimize prompts and providers | Go (Optimizer) |
| Auto-Research | Track topics and synthesize findings | Go (Research) |
| Self-Defending | Detect and contain threats | Go (Defense) |

## Self-Healing Loop

Ensures the system recovers from failures without manual intervention.

**Triggers**:
- Missed connector heartbeat
- Repeated workflow failure
- Dependency health drop
- Policy-safe rollback candidate

**Actions** (in escalation order):
1. Retry the failed operation
2. Restart the connector
3. Rotate to alternative strategy
4. Restore last-known-good configuration
5. Rollback to prior stable step
6. Escalate to user/admin

**UI**: Health Center shows recovery status. Timeline entries marked as "healed."

## Self-Learning Loop

Learns user preferences over time to reduce friction and improve suggestions.

**Triggers**:
- Repeated approvals or rejections (pattern detected)
- User edits to agent-drafted content
- Consistent preferences over time
- Recurring routines in behavior data

**Actions**:
- Update preference graph
- Adjust recommendation defaults
- Propose recipe refinements in shadow mode
- Improve scheduling or summary quality

**Safety**: All learning operates in shadow mode first. Changes are reversible.
Users can inspect and reset learned preferences.

## Self-Improving Loop

Optimizes the system's own performance by experimenting with alternatives.

**Triggers**:
- Low approval rate for a recipe
- High edit distance on drafts
- Cost or latency spikes
- Quality regressions detected

**Actions**:
- Compare prompt variations
- Compare provider performance
- Adjust reasoning depth
- Enable prompt caching where safe
- Promote best-performing candidate
- Rollback on regression

**Safety**: A/B experiments run in shadow mode. Regression triggers automatic
rollback. Metrics tracked via observability module.

## Auto-Research Loop

Keeps knowledge current by monitoring topics and sources.

**Triggers**:
- Tracked topic updates detected
- Source freshness expiry
- User subscription topic changes
- Competitor or API change watches

**Actions**:
- Fetch from approved sources only
- Rank source trust
- Detect contradictions across sources
- Create research briefs
- Propose user actions based on findings
- Write evidence-linked memory nodes

**UI**: Research Center displays briefs with confidence, freshness, and
contradiction markers. Sources always linked.

## Self-Defending Loop

Protects the system from external threats and internal anomalies.

**Triggers**:
- Prompt injection indicators
- Destination allowlist failures
- Unusual send volume
- Privilege escalation attempts
- Suspicious cross-tenant access patterns

**Actions**:
- Deny or sandbox suspicious input
- Quarantine flagged items
- Reduce agent privileges
- Require admin review
- Produce explainable incident summary

**UI**: Security Center shows blocked actions and quarantined items. Every
block includes a plain-language explanation.

## Governance Controls

All five loops share these governance controls:

| Control | Description |
|---------|-------------|
| **Scope limits** | Each loop operates within defined boundaries |
| **Pause/resume** | Any loop can be paused by user or admin |
| **Shadow mode** | Test changes without affecting production |
| **Audit logging** | Every loop action is recorded in the event log |
| **Confidence reporting** | Loops report confidence levels for their actions |

## Canonical Reference

See agent loop definitions in
[specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md).
