# Health — Claude Design Brief

**Route:** `/health` · **Job:** System heartbeats, self-healing log, and component status across every workspace service.

Paste the system prompt from `docs/design/claude-design-guide.md` section 2 FIRST, then paste the brief below.

## Route brief (paste into Claude Design)

```text
Design the Health route for IronGolem OS — `/health`. Health is where the
operator sees what's running, what self-healed, what's quietly recovering,
and what needs human attention. The tone is calm — Ambient Operations is
the pillar this surface most respects.

Primary surface:
- Top header band: overall workspace health (one of five canonical states
  from design-patterns.md). Big, calm, non-alarming.
  - "Healthy" — safe palette, calm sentence: "Everything's running. Last
    self-heal 23 minutes ago."
  - "Quietly recovering" — recovered palette, "1 component is recovering
    on its own. No action needed."
  - "Needs your attention" — warning palette, "1 component needs you to
    look at it. Open it to see what's blocking."
  - "Paused" — neutral palette
  - "Quarantined" — quarantined palette, the most alarming but still
    calm-toned.
- Heartbeat grid below — one HeartbeatStatus-style card per component.
  ~12-18 components: Gateway, Runtime daemon, Sandbox, Memory store, Event
  store, Telegram connector, Email connector, Webhook connector, Inbox
  team, Calendar team, Research team, Operations team, etc.
  - Each card: name, current state (canonical 5 above), last heartbeat,
    uptime streak, one-line current activity ("Re-embedding 2,140 docs —
    8 min remaining").
  - Card tone matches the component's state palette.
- Self-healing log section (Timeline component, state="healed" entries):
  the last 10 things that auto-healed. Each entry: what failed, the
  recovery action, how long it took, whether the rule will be reviewed.
- "What might fail next" panel — predictive section showing components
  with degrading reliability (e.g. error budget tracking). One card per
  warning, with a "Pause it" action and a "Show graph" toggle.
- Empty state for self-healing log: "Nothing's needed self-heal in the
  last 24 hours. Heartbeat green for 17 days."

Mandatory patterns:
- Ambient Operations — suppress-on-OK aggressively. Components that are
  healthy show up as a compact green chip in the grid, not a full card.
  Cards expand for non-healthy states.
- Calm copy — no exclamation marks, no all-caps "ALERT", no red unless a
  human absolutely needs to act now.
- Recovery story — every healed event has a 4-part story: what was
  checked, what changed, whether recovery succeeded, whether action is
  needed.
- Predictive when possible — surface degrading components BEFORE they
  fail, not after.

Tech reminders:
- React 19, TypeScript strict, Tailwind utility classes only.
- The "All systems normal" pill in WorkspaceTopbar already shows the
  high-level state — keep this page's header consistent with it (don't
  contradict).
- Mock 14 components inline. Mix states so the grid shows variety: 11
  healthy, 1 recovering, 1 needs-attention, 1 quarantined.
- Mock 8 self-heal events inline. Include details on what was checked +
  what was fixed.

Output: ONE TSX file with a named export `export function Health()`.
```

## Mock data shape

```ts
type ComponentState = "healthy" | "recovering" | "needs-attention" | "paused" | "quarantined";

interface HealthComponent {
  readonly id: string;
  readonly name: string;
  readonly category: "runtime" | "data" | "connector" | "team" | "infra";
  state: ComponentState;
  readonly lastHeartbeat: string;          // "37s ago"
  readonly uptimeStreak: string;            // "17 days"
  readonly currentActivity?: string;        // when not just "idle"
  readonly errorBudget?: { used: number; total: number };
}

interface HealEvent {
  readonly id: string;
  readonly component: string;
  readonly whatFailed: string;               // "Token expired"
  readonly recoveryAction: string;           // "Refreshed credentials"
  readonly durationMs: number;
  readonly succeeded: boolean;
  readonly ruleWillBeReviewed: boolean;
  readonly at: string;
}
```

## Components to reuse (TODO substitution markers)

- `HeartbeatStatus` — every component card uses this; real shape is
  `{ state, lastHeartbeat, uptimeStreak, currentActivity }`
- `Timeline` — the self-healing log, filtered to `state="healed"`
- `WorkspaceTopbar` — but pass `showHeartbeatPill={false}` so the topbar
  doesn't redundantly tell you everything's fine while you're literally on
  the Health page.
- `RiskBadge` — repurpose for "error budget remaining" pill

## Page patterns

- **Heartbeat Status Cards** (the full pattern from `design-patterns.md`).
- **Ambient Operations** — calm tones, suppress-on-OK.
- **Recovery story** on every healed event (4-part: checked → changed → recovery → action needed).

## Route-specific anti-patterns

| Don't | Do |
|---|---|
| Show every component as a full-size card when 11 of them are healthy | Compact green chip for healthy; full card for non-healthy |
| Use red/alarming copy for self-healed events | Self-heal is good news — use recovered tone, calm copy |
| Surface error counts as raw numbers | Use error-budget framing ("82% of budget left this week") |
| Hide "Pause this component" behind a menu | When attention is needed, the pause action is visible |
| Duplicate the header heartbeat pill from the topbar | Pass `showHeartbeatPill={false}` to WorkspaceTopbar on this page |
