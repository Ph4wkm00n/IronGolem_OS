# IronGolem OS UI/UX Design Guide v2

## UX mission
IronGolem OS v2 must make a more powerful platform feel simpler, safer, and more understandable. The addition of multi-tenancy, assistant squads, heartbeat monitoring, knowledge graphs, deeper security, and broader channels should not make the product feel more technical.

## Core UX promise
Users should feel:
- informed, not overwhelmed
- protected, not restricted
- assisted, not replaced
- in control, even when the system is acting in the background

## New v2 experience pillars

### 1. Visible trust
- show permissions clearly
- show risk before action
- show what was blocked and why
- show what was healed and how

### 2. Managed complexity
- basic users see recipes and summaries
- advanced users can inspect squads, traces, and policies
- admins can manage tenants, channels, and incidents

### 3. Ambient operations
- heartbeats, recoveries, and research updates should feel like calm system maintenance, not noisy alerts

### 4. Explainable autonomy
- every automation needs a human-readable contract
- every squad needs a plain-language role explanation
- every blocked or quarantined event needs a simple cause statement

## Updated information architecture
Primary nav:
- Home
- Inbox
- Recipes
- Research
- Memory
- Health
- Security
- Settings

Admin/team nav additions:
- Workspaces
- Members
- Policies
- Connectors
- Traces

## v2 screen additions

### Assistant Squads
A dedicated screen or section for prebuilt squads.

Each squad card shows:
- purpose
- member roles
- channels and tools used
- autonomy level
- last run
- trust level
- pause / inspect / edit controls

### Workspaces and tenancy
Admin users need:
- workspace list
- member roles
- connector assignment by workspace
- data boundary explanations
- cross-workspace isolation reminders

### Policy explainer
Policies must be visualized as layered cards:
- Who can trigger this?
- Which agent can act?
- Which channel can be used?
- Which tools are allowed?
- What always needs approval?

### Heartbeat status
Heartbeat UX should be calm and informative.

States:
- Healthy
- Quietly recovering
- Needs your attention
- Paused
- Quarantined

Each heartbeat event should explain:
- what was checked
- what changed
- whether recovery succeeded
- whether user action is needed

### Security center v2
The security center must be strong without feeling like an enterprise SIEM.

Primary views:
- Blocked actions
- Suspicious content
- Quarantined items
- Policy coverage
- Security recommendations
- Audit exports

## Design patterns

### Safety first cards
Every recipe, squad, and connector should include a compact safety block:
- Can access
- Cannot access
- Needs approval for
- Stops automatically if

### Timeline v2
The timeline now needs richer states:
- Action taken
- Action proposed
- Action blocked
- Action healed
- Action quarantined
- Research update
- Squad handoff

### Research with evidence
Research cards should show:
- title
- summary
- confidence
- freshness
- source count
- contradiction marker
- action suggestion

### Memory graph explorer
Keep the graph understandable:
- list and card views first
- graph visualization optional, not default
- every node shows evidence and freshness
- “why do you know this?” must always be one click away

### Advanced mode
Advanced mode exposes:
- traces
- cache metrics
- provider routing
- reasoning controls
- policy detail
- squad internals

Default users should never need advanced mode.

## Writing style
- Use plain language.
- Avoid words like “orchestrator,” “vector,” “tenant,” or “OTLP” on default surfaces unless explained.
- Replace technical labels with outcome-oriented labels where possible.

Examples:
- “Assistant team” instead of “agent squad” in beginner mode
- “Workspace” instead of “tenant” on default screens
- “Safety rules” instead of “policy engine” on onboarding screens

## Mobile behavior
- approvals and incident summaries first
- compressed heartbeat summaries
- swipeable cards for inbox actions
- bottom navigation for key areas
- security alerts should use full-screen sheets for clarity

## Visual system guidance
- maintain calm neutral surfaces with one accent color
- use semantic color carefully for Safe, Warning, Blocked, Recovered, Quarantined
- avoid glowing, cyber, or surveillance aesthetics
- emphasize status through text, icons, and layout, not color alone

## Accessibility updates
- all security and heartbeat statuses must be screen-reader friendly
- graph and trace views require list/table alternatives
- policy cards must be readable without hover interactions
- reduced-motion mode should simplify graph and timeline transitions

## v2 UX checklist
- Can a non-technical user understand what an assistant team does before enabling it?
- Can a user tell whether the system is healthy at a glance?
- Can an admin explain the five layers of protection using the UI alone?
- Can a user understand why something was blocked or quarantined?
- Can a user inspect a memory item and see where it came from?
- Can the product feel calmer as it gets more powerful?
