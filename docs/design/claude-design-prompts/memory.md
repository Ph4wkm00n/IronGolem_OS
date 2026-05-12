# Memory — Claude Design Brief

**Route:** `/memory` · **Job:** What the system knows about your workspace — facts, sources, freshness, and "why do you know this?" for every claim.

Paste the system prompt from `docs/design/claude-design-guide.md` section 2 FIRST, then paste the brief below.

## Route brief (paste into Claude Design)

```text
Design the Memory route for IronGolem OS — `/memory`. Memory is what the
system has learned about this workspace: people, accounts, preferences,
recurring patterns. Every fact has evidence, freshness, and a
"why-do-you-know-this" trail. The design must make it trivially easy to
correct or forget a fact.

Primary surface:
- Search bar at the top with placeholder "Search memory…" — full-width on
  mobile, comfortable max-w-2xl on desktop.
- Below the search: facet pills — Subject (People / Accounts / Preferences
  / Patterns) and Freshness (Hours / Days / Weeks / Months).
- Default view: LIST of memory items (per design-patterns.md, list and
  card views are the default; graph is an optional toggle).
- Each memory item is a card:
  - Subject + relationship ("Sarah Lopez — calendar")
  - The fact in plain language ("Prefers Thursday afternoons for status meetings")
  - Confidence pill (bg-safe ≥85%, bg-warning 70-84%, bg-blocked <70%)
  - Freshness ("Verified 2h ago" / "Last seen 17 days ago")
  - Source count + a small "Why do you know this?" link that expands the
    evidence trail inline (or opens a drawer — your call, but it must be
    one click away)
  - Footer actions: "Correct this", "Forget this", "Tag" (text-only)
- Toggle in the top-right: "Switch to graph view" (per design-patterns,
  this is an opt-in, not default).
- Empty state when no memory: "The system hasn't built up much yet — keep
  using your assistant teams and memory will grow."
- Empty state when search returns nothing: "No memory matches 'X' — try a
  broader query or check the recent activity timeline on /."

Mandatory patterns:
- Explainable Autonomy — "Why do you know this?" is one click away on every item.
- Forget-easily — "Forget this" is always visible (don't hide behind a
  menu). Forgetting is non-destructive: it moves the fact to a Forgotten
  bucket with a 30-day undo.
- Freshness-first — stale facts (>30 days untouched) get a subtle warning
  pill encouraging re-verification.
- Progressive disclosure — the graph view is opt-in. The default is the
  list because it's easier to skim, search, and edit.

Tech reminders:
- React 19, TypeScript strict, Tailwind utility classes only.
- For the optional graph view, sketch the structure (force-directed
  network, color-coded by subject) but DO NOT pull in d3 or vis-network in
  the TSX — leave a placeholder `<div className="card-padded">Graph
  rendering goes here</div>` for the integrator to wire later.
- Mock 20-25 memory items inline. Mix subjects (people, accounts,
  preferences, patterns) and confidence levels.

Output: ONE TSX file with a named export `export function Memory()`.
```

## Mock data shape

```ts
interface MemoryItem {
  readonly id: string;
  readonly subject: string;                    // "Sarah Lopez — calendar"
  readonly subjectKind: "person" | "account" | "preference" | "pattern";
  readonly fact: string;
  readonly confidence: number;                 // 0-1
  readonly freshness: string;                  // "Verified 2h ago", "Last seen 17 days ago"
  readonly verifiedAt: string;                 // ISO timestamp
  readonly sourceCount: number;
  readonly evidence: ReadonlyArray<{
    readonly type: "email" | "calendar" | "message" | "observation";
    readonly excerpt: string;
    readonly at: string;
  }>;
  readonly isStale?: boolean;                  // computed: verifiedAt > 30d ago
  readonly tags?: readonly string[];
}
```

## Components to reuse (TODO substitution markers)

- `RiskBadge` — repurposed for confidence indicator (with a different color mapping)
- `WorkspaceTopbar` — page chrome
- A custom `MemoryItem` card — does not exist yet in `@irongolem/ui`; mark with `TODO(integrator): graduate <MemoryItem /> to @irongolem/ui after the second route adopts the pattern`

## Page patterns

- **Memory Graph Explorer** (from `design-patterns.md`) — list/card default, graph opt-in.
- **Explainable Autonomy** — "Why do you know this?" mandatory on every item.
- **Suppress-on-OK** — facts the user already corrected don't re-surface uncorrected.

## Route-specific anti-patterns

| Don't | Do |
|---|---|
| Default to graph view | Default to list view; graph is opt-in |
| Hide "Forget this" behind a menu | Surface forget action visibly |
| Surface confidence as a raw number | Render with semantic tone (safe / warning / blocked) |
| Render evidence as wall-of-text | Each evidence excerpt is a 1-2 sentence card, sourced and timestamped |
| Make corrections require typing the original | "Correct this" opens an inline editor pre-filled with the current value |
