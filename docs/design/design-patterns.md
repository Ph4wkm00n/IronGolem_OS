# Design Patterns

Reusable UI patterns used across IronGolem OS screens.

## Safety First Cards

Every recipe, squad, and connector includes a compact safety block:

```
┌─────────────────────────────┐
│ Safety Summary              │
│                             │
│ ✓ Can access:               │
│   Email, Calendar           │
│                             │
│ ✗ Cannot access:            │
│   Filesystem, Browser       │
│                             │
│ ⚠ Needs approval for:      │
│   Sending emails            │
│                             │
│ ◼ Stops automatically if:   │
│   3 consecutive failures    │
└─────────────────────────────┘
```

## Timeline v2

The timeline is the **primary explanation surface**. It displays richer states
than a simple activity log:

| State | Meaning | Visual Indicator |
|-------|---------|-----------------|
| Action taken | Completed successfully | Solid indicator |
| Action proposed | Awaiting approval | Outlined indicator |
| Action blocked | Denied by policy | Warning indicator |
| Action healed | Recovered from failure | Recovery indicator |
| Action quarantined | Isolated for review | Quarantine indicator |
| Research update | New finding published | Info indicator |
| Squad handoff | Delegated to another agent | Delegation indicator |

## Policy Explainer Cards

Layered cards that translate the five-layer permission model:

```
┌─────────────────────────────┐
│ Policy: Email Drafting      │
│                             │
│ Who can trigger?            │
│   Any workspace member      │
│                             │
│ Which agent acts?           │
│   Inbox Squad - Drafter     │
│                             │
│ Which channel?              │
│   Email only                │
│                             │
│ Which tools allowed?        │
│   Read email, Draft reply   │
│                             │
│ What needs approval?        │
│   Sending any email         │
└─────────────────────────────┘
```

## Research Cards

Display research findings with trust indicators:

| Field | Purpose |
|-------|---------|
| Title | Brief topic description |
| Summary | Key findings (2-3 sentences) |
| Confidence | How reliable is this information? |
| Freshness | When was this last verified? |
| Source count | How many sources support this? |
| Contradiction marker | Are there conflicting sources? |
| Action suggestion | What can the user do with this? |

## Heartbeat Status Cards

Calm, informative health status display:

| State | Description | Tone |
|-------|-------------|------|
| Healthy | Everything working normally | Calm, green |
| Quietly recovering | Self-healing in progress | Neutral, amber |
| Needs your attention | User action required | Attention, amber |
| Paused | Intentionally stopped | Neutral, gray |
| Quarantined | Isolated for safety | Alert, red |

Each heartbeat event explains: what was checked, what changed, whether recovery
succeeded, and whether user action is needed.

## Squad Cards

Display assistant squad information:

| Field | Purpose |
|-------|---------|
| Purpose | What this squad does |
| Member roles | Which agents are in the squad |
| Channels and tools | What the squad can access |
| Autonomy level | How independently it operates |
| Last run | When it last executed |
| Trust level | Current trust rating |
| Controls | Pause, inspect, edit buttons |

## Memory Graph Explorer

- List and card views are the **default** (not graph visualization)
- Graph visualization is optional and accessible from list view
- Every node shows evidence and freshness
- "Why do you know this?" is always one click away

## Advanced Mode

Progressive disclosure exposes advanced features:
- Traces and spans
- Cache metrics
- Provider routing
- Reasoning controls
- Policy detail
- Squad internals

Default users should **never need** advanced mode.

## Canonical Reference

See [specs/05-ui-ux-design-guide-v2.md](../specs/05-ui-ux-design-guide-v2.md).
