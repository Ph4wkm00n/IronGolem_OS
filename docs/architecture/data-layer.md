# Data Layer

IronGolem OS uses a flexible data layer that scales from single-user desktop
to multi-tenant team deployments.

## Storage Backends

### SQLite (Solo and Household Modes)
- Lightweight, zero-configuration
- Single file on disk
- Ideal for desktop and personal server deployments
- One workspace per database

### PostgreSQL (Team Mode)
- Multi-tenant with per-workspace isolation
- Connection pooling for concurrent access
- Supports cross-workspace admin queries
- Per-tenant data boundary enforcement

## Event Sourcing

Event sourcing is the **canonical history model**. Every significant action
produces an immutable event record.

### Why Event Sourcing?

- **Auditability**: Complete history of every action, decision, and change
- **Replay**: Debug issues by replaying event sequences
- **Recovery**: Rebuild state from events after failures
- **Transparency**: Users can inspect exactly what happened and when

### Event Categories

| Category | Examples |
|----------|---------|
| Agent actions | Tool calls, LLM calls, approvals, delegations |
| System events | Heartbeats, recoveries, config changes |
| User actions | Recipe activations, preference changes, approvals/rejections |
| Security events | Blocked actions, quarantine decisions, policy violations |
| Research events | Source fetches, contradiction detections, brief publications |

## Memory and Knowledge Graphs

Four graph structures built on top of the event log:

### Event Log (Source of Truth)
Append-only record of all actions and events. Everything else derives from this.

### Preference Graph
Encodes user and workspace preferences learned from behavior:
- Approval patterns
- Draft edit patterns
- Scheduling preferences
- Communication style preferences

### Relationship Graph
Connects entities across the system:
- People, tasks, sources, and topics
- Cross-references between research and actions
- Contact and organization relationships

### Knowledge Graph
Extracted from research and usage:
- Evidence back-links to source material
- Freshness indicators (when was this last verified?)
- Confidence scores (how reliable is this?)
- Contradiction markers (conflicting information detected)

## Isolation Boundaries

```
Tenant → Workspace → User → Channel → Agent Session
```

Each level provides data isolation. In team mode, cross-boundary access
is explicitly prevented and tested.

## Canonical Reference

See sections on memory and knowledge graph in
[specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md).
