# Component Catalog

Key UI components organized by the screen they appear on. All components live
in `packages/ui/` and are shared across web, desktop, and admin-console apps.

## Home Screen Components

| Component | Description |
|-----------|-------------|
| `HealthWidget` | Compact system health summary with heartbeat indicator |
| `ActiveRecipeList` | List of currently running recipes with status |
| `SquadStatusBar` | Active squads with last-run time and trust level |
| `AlertBanner` | Important alerts requiring attention |

## Inbox Components

| Component | Description |
|-----------|-------------|
| `ApprovalCard` | Pending decision with context and approve/reject actions |
| `DraftCard` | Agent-prepared content for user review and editing |
| `ResearchBriefCard` | New research finding summary with confidence indicator |
| `InboxFilter` | Filter by type (approvals, drafts, briefs) |

## Recipe Components

| Component | Description |
|-----------|-------------|
| `RecipeGalleryGrid` | Browse-able grid of recipe templates |
| `RecipeCard` | Recipe preview with safety summary badge |
| `SafetyBlock` | Can access / Cannot access / Needs approval / Stops if |
| `RecipeActivator` | Configuration and activation flow |
| `RecipeHistory` | Past executions with timeline entries |

## Research Components

| Component | Description |
|-----------|-------------|
| `TopicTracker` | List of tracked topics with freshness indicators |
| `ResearchCard` | Full research finding with evidence and confidence |
| `ContradictionMarker` | Visual indicator for conflicting information |
| `SourceList` | Sources with trust scores |

## Memory Components

| Component | Description |
|-----------|-------------|
| `MemoryListView` | Default list/card view of memory nodes |
| `MemoryGraphView` | Optional graph visualization |
| `MemoryNodeCard` | Single node with evidence, freshness, confidence |
| `EvidenceLink` | Link to source material for a memory claim |

## Health Components

| Component | Description |
|-----------|-------------|
| `HeartbeatCard` | Per-service/connector health with state indicator |
| `RecoveryTimeline` | Recent recovery events with outcomes |
| `IncidentCard` | Incident summary with cause and resolution |

## Security Components

| Component | Description |
|-----------|-------------|
| `BlockedActionCard` | Blocked action with policy explanation |
| `QuarantineCard` | Quarantined item with review actions |
| `PolicyCoverageMap` | Visual coverage of policy layers |
| `AuditExportButton` | Generate and download audit reports |

## Admin Components

| Component | Description |
|-----------|-------------|
| `WorkspaceList` | Workspace management with isolation indicators |
| `MemberRoleTable` | User roles and permissions management |
| `ConnectorPanel` | Connector health, credentials, and assignment |
| `PolicyEditor` | Policy visualization and editing |
| `TraceExplorer` | Execution trace browsing and replay |

## Shared Components

| Component | Description |
|-----------|-------------|
| `Timeline` | Action timeline with v2 states |
| `PolicyCard` | Five-layer policy explainer |
| `RiskBadge` | Semantic risk indicator |
| `CommandPalette` | Power user command interface |
| `StatusIndicator` | Reusable status dot/badge with semantic color |

## Canonical Reference

See [specs/05-ui-ux-design-guide-v2.md](../specs/05-ui-ux-design-guide-v2.md).
