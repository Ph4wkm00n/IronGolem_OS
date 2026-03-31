# Agent Roles and Squads

IronGolem OS uses specialized agent roles that can be composed into squads
(multi-agent teams) for complex tasks.

## Agent Roles

| Role | Responsibility |
|------|---------------|
| **Planner** | Breaks goals into steps, creates execution plans |
| **Executor** | Runs plan steps, calls tools, produces outputs |
| **Verifier** | Checks output quality, catches errors and hallucinations |
| **Researcher** | Fetches information, evaluates sources, synthesizes findings |
| **Defender** | Monitors for threats, enforces security boundaries |
| **Healer** | Detects failures, orchestrates recovery actions |
| **Optimizer** | Experiments with prompts and providers, improves performance |
| **Narrator** | Generates summaries, briefs, and explanations for users |
| **Router** | Directs tasks to appropriate agents or squads |

## Pre-Composed Squads

Squads are user-facing multi-agent teams presented as "assistant teams" in
the UI.

### Inbox Squad
**Purpose**: Email triage and response drafting

| Role | Agent |
|------|-------|
| Classifier | Categorizes incoming messages |
| Drafter | Prepares response drafts |
| Verifier | Checks draft quality and policy compliance |

### Research Squad
**Purpose**: Topic tracking and knowledge synthesis

| Role | Agent |
|------|-------|
| Scout | Fetches sources and monitors changes |
| Synthesizer | Combines findings into coherent briefs |
| Verifier | Checks source credibility and contradictions |

### Ops Squad
**Purpose**: Operational task execution

| Role | Agent |
|------|-------|
| Planner | Creates execution plans for operational tasks |
| Executor | Runs plan steps and tool calls |
| Reporter | Summarizes outcomes and status |

### Security Squad
**Purpose**: Threat monitoring and response

| Role | Agent |
|------|-------|
| Monitor | Watches for anomalies and injection attempts |
| Defender | Executes containment and quarantine |
| Explainer | Produces incident summaries in plain language |

### Executive Assistant Squad
**Purpose**: Personal productivity support

| Role | Agent |
|------|-------|
| Planner | Organizes tasks and priorities |
| Scheduler | Manages calendar and appointments |
| Briefer | Creates daily/weekly summaries |

## Agent Coordination

### Delegation
Agents can delegate subtasks to other agents or squads:
- Planner delegates execution steps to Executor
- Executor requests verification from Verifier
- Router directs tasks to the appropriate squad

### Handoffs
Two types of handoffs between agents:
- **Sync**: Agent waits for the result before continuing
- **Async**: Agent continues while the delegated task runs

### Quality Gates
Verifier agents act as quality gates:
- Check outputs before they reach users
- Validate policy compliance
- Detect hallucinations and format errors
- Flag low-confidence results for human review

## Example Flow: Team Research Brief

1. User activates Research Squad on a tracked vendor
2. Scheduler creates recurring research jobs
3. Scout (Researcher) fetches from trusted sources
4. Knowledge graph updates relationships and freshness
5. Synthesizer (Verifier) checks contradictions and confidence
6. Briefer (Narrator) creates brief for Inbox and Research Center
7. Defender checks source hygiene and isolation boundaries

## Example Flow: Connector Failure

1. Heartbeat detects calendar connector failure
2. Healer retries and validates token freshness
3. If retry fails, Healer restores last-known-good connector config
4. Health Center shows "Recovered" or "Needs attention"
5. Narrator logs the incident and action taken in timeline

## Canonical Reference

See agent roles and squad definitions in
[specs/02-features-modules-and-agent-loops-v2.md](../specs/02-features-modules-and-agent-loops-v2.md).
