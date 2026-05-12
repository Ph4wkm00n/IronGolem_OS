# Recipes — Claude Design Brief

**Route:** `/recipes` · **Job:** Browseable automation templates with safety summaries — recipes you can activate, pause, or customize.

Paste the system prompt from `docs/design/claude-design-guide.md` section 2 FIRST, then paste the brief below.

## Route brief (paste into Claude Design)

```text
Design the Recipes route for IronGolem OS — `/recipes`. Recipes are
pre-composed automations users can activate (e.g. "Triage inbox while I
sleep", "Draft replies to known senders", "Approve standing-order
purchases under $50"). Each recipe ships with a safety summary that
explains exactly what it can and can't do.

Primary surface:
- Grid of recipe cards, 1 column on mobile, 2 on tablet, 3 on desktop.
- Category tabs at the top: Inbox / Calendar / Purchasing / Research /
  Operations / Drafting. Optionally an "All" tab.
- Each card:
  - Recipe name (1-2 lines)
  - One-sentence purpose ("Triage incoming mail and surface what needs you")
  - Status badge: Active (bg-safe), Paused (bg-neutral), New (bg-accent)
  - Three trust signals visible without click:
    - SafetyCard preview (just 1-2 bullets from each section, "see all" link)
    - RiskBadge (overall risk score)
    - "Last run" relative time (or "Never run")
  - Footer: a primary action button (Activate / Pause / Run once) plus an
    "Inspect" link that opens the detail drawer
- Detail drawer when "Inspect" clicked:
  - Full SafetyCard (can / cannot / needs approval / stops if)
  - Full PolicyCard (five layers, all five lines)
  - Schedule (when it runs — every 10m, daily 9am, on inbound mail)
  - Required permissions list with scope (scoped / broad / restricted)
  - Recent runs timeline (Timeline component, last 10 events)
  - Customize button → opens an editor for safety overrides

Mandatory patterns:
- Safety First Cards ABOVE activation controls on every card.
- Visible Trust — the permissions a recipe needs are visible before the
  activation button.
- Progressive disclosure — advanced settings (cron timing, fallback
  behavior, retry policy) live behind a "Customize" toggle in the drawer,
  never on the card.
- Empty state for a category: "No recipes here yet — you can request one
  in Settings → Recipe Requests."

Tech reminders:
- React 19, TypeScript strict, Tailwind utility classes only.
- Use `bg-safe` for Active, `bg-warning` for "Needs your input", `bg-blocked`
  for Disabled-by-policy, `bg-neutral` for Paused.
- Mock 15 recipes inline, split across the six categories. Mix active /
  paused / new statuses so the cards convey variety.

Output: ONE TSX file with a named export `export function Recipes()`.
```

## Mock data shape

```ts
interface Recipe {
  readonly id: string;
  readonly name: string;
  readonly category: "inbox" | "calendar" | "purchasing" | "research" | "operations" | "drafting";
  readonly purpose: string;
  status: "active" | "paused" | "new" | "needs-input" | "disabled";
  readonly riskLevel: "low" | "medium" | "high";
  readonly lastRun: string | null;     // null when never run
  readonly schedule: string;            // "every 10m", "daily 9am", "on inbound mail"
  readonly safety: {
    readonly can: readonly string[];
    readonly cannot: readonly string[];
    readonly needsApproval: readonly string[];
    readonly stopsIf: readonly string[];
  };
  readonly permissions: ReadonlyArray<{
    readonly name: string;
    readonly scope: "scoped" | "broad" | "restricted";
  }>;
  readonly recentRuns?: ReadonlyArray<{ at: string; outcome: "ok" | "blocked" | "healed" }>;
}
```

## Components to reuse (TODO substitution markers)

- `SafetyCard` — preview on the card, full in the detail drawer
- `PolicyCard` — full five-layer card in the detail drawer
- `RiskBadge` — on each card
- `Timeline` — "Recent runs" inside the detail drawer
- `WorkspaceTopbar` — page chrome

## Page patterns

- **Safety First Cards** mandatory on every recipe card.
- **Policy Explainer Cards** in the detail drawer.
- **Progressive disclosure** for advanced settings.
- **Trust posture** — "Active" tone matches the system's overall posture; if any layer of the safety rules is in "watching", show a small amber dot on the card.

## Route-specific anti-patterns

| Don't | Do |
|---|---|
| Show an activation button without safety bullets first | Safety summary always above Activate |
| Surface advanced cron / retry config on the card | Hide behind "Customize" in the drawer |
| Mix categories in one grid | Tab between categories; show one at a time |
| Default to "All" tab dumping every recipe | Default tab is the most-relevant category for the user (Inbox is a fine default) |
| Generic "Run" button without explaining what runs | Action button text describes the outcome ("Activate", "Run once now") |
