# Security — Claude Design Brief

**Route:** `/security` · **Job:** Safety rules in five layers, audit log of every blocked or quarantined action, and policy-explainer cards.

Paste the system prompt from `docs/design/claude-design-guide.md` section 2 FIRST, then paste the brief below.

## Route brief (paste into Claude Design)

```text
Design the Security route for IronGolem OS — `/security`. This is where
operators understand the five-layer safety model, see the audit trail of
everything that was blocked or quarantined, and adjust policies. The
audience spans non-technical operators and security-conscious admins —
progressive disclosure is critical here.

Primary surface:
- Top section: "Five layers, all active" PolicyCard (the existing pattern
  from design-patterns.md). Each layer expanded inline:
  - Layer 1 — Identity
  - Layer 2 — Workspace
  - Layer 3 — Team
  - Layer 4 — Action
  - Layer 5 — Outcome
  Each shows its state (ok / watching / paused / failed), an explanation
  in plain language, and the count of actions currently governed by it.
- Audit log section: a Timeline filtered to blocked / quarantined entries
  from the workspace dashboard's vocabulary. Filter chips at the top:
  All / Blocked / Quarantined / Denied by me / Last 24h / Last 7d.
- Each audit entry:
  - Status mark (StatusMark from the dashboard's pattern)
  - Title in plain language
  - Cause sentence (mandatory — from the Workspace Dashboard pattern)
  - Permission that was checked, with scope (scoped / broad / restricted)
  - "Open rule that caught this" link → opens a drawer showing the rule
    text, its history, related blocked events, and the option to relax,
    tighten, or reword the rule.
- Policy library (lower section): the editable policies. List of policy
  cards, each with:
  - Name + one-sentence purpose
  - State badge (active / paused / under-review)
  - The five-layer surface mapping (which layer this rule sits in)
  - Action buttons: Edit / Pause / Test (test runs the rule against the
    recent audit log to show how many events would change)
- Empty state: "No safety rules have triggered in the last 24 hours.
  Heartbeat green for 17 days."

Mandatory patterns:
- Visible Trust — every audit entry has a one-sentence cause.
- Explainable Autonomy — "What rule caught this?" is one click away.
- Progressive disclosure — the policy editor lives in a drawer, not on the
  main surface; rule history and testing live in the drawer.
- Reversibility — pause is preferred over delete; every action has an
  undo within 30 days.

Tech reminders:
- React 19, TypeScript strict, Tailwind utility classes only.
- Use `bg-blocked` for blocked entries, `bg-quarantined` for quarantined,
  `bg-warning` for under-review policies.
- Mock 25 audit entries inline. Mix blocked and quarantined, include
  causes for all of them, vary the scope (scoped / broad / restricted).
- Mock 8 policies inline across the 5 layers.

Output: ONE TSX file with a named export `export function Security()`.
```

## Mock data shape

```ts
type PolicyLayer = 1 | 2 | 3 | 4 | 5;
type LayerState = "ok" | "watching" | "paused" | "failed";

interface PolicyLayerEntry {
  readonly id: PolicyLayer;
  readonly name: string;                         // "Identity", "Workspace", etc.
  state: LayerState;
  readonly governedActionCount: number;
  readonly note: string;
}

interface AuditEntry {
  readonly id: string;
  readonly status: "blocked" | "quarantined";
  readonly title: string;
  readonly cause: string;                         // required
  readonly permission: string;
  readonly permissionScope: "scoped" | "broad" | "restricted";
  readonly ruleId: string;
  readonly ruleName: string;
  readonly at: string;
}

interface Policy {
  readonly id: string;
  readonly name: string;
  readonly purpose: string;
  readonly layer: PolicyLayer;
  state: "active" | "paused" | "under-review";
  readonly text: string;
  readonly history: ReadonlyArray<{ at: string; edit: string; by: string }>;
  readonly triggeredCount: number;
}
```

## Components to reuse (TODO substitution markers)

- `PolicyCard` — the five-layer card at the top
- `SafetyCard` — for the "can / cannot / needs approval / stops if" summary in the policy detail drawer
- `Timeline` — the audit log
- `RiskBadge` — small severity pill on each audit entry
- `WorkspaceTopbar` — page chrome (with "Rules" active)

## Page patterns

- **Policy Explainer Cards** (from `design-patterns.md`).
- **Timeline v2** filtered to blocked/quarantined.
- **Visible Trust** — cause is non-negotiable on every audit entry.
- **Reversibility** — pause preferred over delete.

## Route-specific anti-patterns

| Don't | Do |
|---|---|
| Show audit entries without a cause | Cause is mandatory and visible |
| Bury the rule that caught an event behind a click | "Open rule" link visible on every entry |
| Render policy text in monospace | Render policy text in body type — these are human rules, not code |
| Hide "pause this rule" behind a menu | Pause is a first-class action |
| Make the audit log scroll forever | Page at 50 per page, infinite scroll on demand |
