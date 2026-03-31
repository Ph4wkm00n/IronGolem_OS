# Information Architecture

## Primary Navigation

| Screen | Purpose | Key Content |
|--------|---------|-------------|
| **Home** | Dashboard | Health status, active recipes, squad status, important alerts |
| **Inbox** | Decisions | Approvals, pending decisions, drafts, research briefs |
| **Recipes** | Automation | Gallery, safety summaries, activation flows |
| **Research** | Knowledge | Tracked topics, briefs, contradictions, evidence |
| **Memory** | Data | List/graph views with evidence and freshness indicators |
| **Health** | Status | Heartbeats, recoveries, system status |
| **Security** | Safety | Blocked actions, quarantined items, policy coverage |
| **Settings** | Config | Personal, workspace, and advanced controls |

## Admin/Team Navigation (Additions)

| Screen | Purpose |
|--------|---------|
| **Workspaces** | Workspace list, data boundaries, isolation status |
| **Members** | User roles, permissions, team management |
| **Policies** | Policy visualization and management |
| **Connectors** | Connector health, assignment, credential management |
| **Traces** | Execution traces and audit exploration |

## Screen Hierarchy

```
Home
├── Health widget (summary)
├── Active recipes (compact list)
├── Squad status (compact list)
└── Important alerts

Inbox
├── Approvals (pending decisions)
├── Drafts (agent-prepared content)
└── Research briefs (new findings)

Recipes
├── Gallery (browse all recipes)
├── Active recipes (running automations)
└── Recipe detail (safety card, history, controls)

Research
├── Tracked topics
├── Recent briefs
├── Contradictions
└── Source list

Memory
├── List view (default)
├── Graph view (optional)
└── Node detail (evidence, freshness, confidence)

Health
├── Heartbeat status (all services/connectors)
├── Recent recoveries
└── Incident history

Security
├── Blocked actions
├── Suspicious content
├── Quarantined items
├── Policy coverage map
└── Audit exports
```

## User Flows

### First-Run Onboarding
1. Welcome screen with persona selection
2. Connector setup (guided)
3. First recipe activation
4. Safety summary review and approval
5. Success confirmation with next steps

### Recipe Activation
1. Browse gallery
2. Read safety card (can access, cannot access, needs approval, stops if)
3. Configure parameters
4. Review and activate
5. Monitor in timeline

### Approval Flow
1. Notification in Inbox
2. View proposed action with context
3. Review safety information
4. Approve, reject, or modify
5. Confirmation and timeline update

## Canonical Reference

See [specs/05-ui-ux-design-guide-v2.md](../specs/05-ui-ux-design-guide-v2.md).
