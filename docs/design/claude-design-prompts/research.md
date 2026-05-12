# Research — Claude Design Brief

**Route:** `/research` · **Job:** Published findings from the research team with confidence indicators, freshness, and source counts.

Paste the system prompt from `docs/design/claude-design-guide.md` section 2 FIRST, then paste the brief below.

## Route brief (paste into Claude Design)

```text
Design the Research route for IronGolem OS — `/research`. Research is what
the research team has surfaced from external sources (release notes,
indexes, alerts, papers, weekly digests) that's relevant to this
workspace. Every finding has a confidence score (0-100%), freshness, and
source attribution.

Primary surface:
- Grid of ResearchCards, 2 columns on tablet, 3 on desktop, 1 on mobile.
- Filter / sort bar at the top:
  - Sort: Most recent / Highest impact / Highest confidence (default: Most recent)
  - Topic chips: All / Pricing / API changes / Supplier risk / Industry / Internal
  - Toggle: "Hide low-confidence (<70%)" (default off)
- Each ResearchCard:
  - Title (1-2 lines, plain language)
  - 2-3 sentence summary
  - Confidence pill (bg-safe ≥85%, bg-warning 70-84%, bg-blocked <70%) showing the percentage
  - Freshness ("2 hours ago", "yesterday", "5 hours ago") with clock icon
  - Source attribution at the bottom (font-mono)
  - Source count pill ("3 sources") when there's more than one
  - Contradiction marker (AlertTriangle icon + "1 conflicting source")
    when sources disagree
  - Action suggestion at the very bottom, primary tone: "Apply finding",
    "Mark reviewed", "Discuss in standup"
- Featured finding at the top (one card with a larger size, badge "Top
  impact"): the highest impact + freshest finding gets its own row.
- Empty state: "No new findings — the research team is monitoring 47 sources."

Mandatory patterns:
- Visible Trust — confidence is shown BEFORE the title is read.
- Suppress-on-OK — findings that didn't change anything stay archived;
  only meaningful new ones surface.
- Explainable Autonomy — "Why this finding?" link on every card opens a
  drawer showing the sources, snippet excerpts, and the classifier that
  flagged it.
- Contradiction-aware — when sources conflict, surface the conflict, don't
  paper over it. Use AlertTriangle in the warning palette.

Tech reminders:
- React 19, TypeScript strict, Tailwind utility classes only.
- Confidence visual: a thin progress bar under the title that visualizes
  the percentage, colored by tone.
- Mock 12-15 findings inline. Mix confidence levels, freshness, topics,
  and include 2-3 contradictory ones.

Output: ONE TSX file with a named export `export function Research()`.
```

## Mock data shape

```ts
interface Finding {
  readonly id: string;
  readonly title: string;
  readonly summary: string;
  readonly source: string;                // primary source — e.g. "Bloomberg Carbon Index"
  readonly confidence: number;             // 0-1
  readonly freshness: string;               // "2 hours ago"
  readonly topic: "pricing" | "api-changes" | "supplier-risk" | "industry" | "internal";
  readonly sourceCount: number;             // 1+
  readonly hasContradiction: boolean;
  readonly impact: "low" | "medium" | "high";
  readonly actionSuggestion: string;        // "Apply finding", "Mark reviewed", "Discuss in standup"
  readonly evidence?: ReadonlyArray<{
    readonly source: string;
    readonly excerpt: string;
    readonly publishedAt: string;
  }>;
}
```

## Components to reuse (TODO substitution markers)

- `ResearchCard` — `<ResearchCard finding={...} />` from @irongolem/ui (real component takes the shape above)
- `RiskBadge` — small impact pill (low/medium/high)
- `WorkspaceTopbar` — page chrome

## Page patterns

- **Research Cards** (full from `design-patterns.md`).
- **Confidence as primary trust signal** — readable before the title.
- **Contradiction markers** — surfaced explicitly when present.
- **Suppress-on-OK** — only surface findings that actually changed something.

## Route-specific anti-patterns

| Don't | Do |
|---|---|
| Hide the source until clicked | Source attribution is visible on the card |
| Render confidence as a raw decimal | Render as a percentage with a tone |
| Surface findings without freshness | Always show "N hours ago" with a clock icon |
| Bury contradictions under a "view sources" link | Surface conflicts as AlertTriangle on the card |
| Sort by alphabetic title by default | Sort by most-recent (or highest-impact if user picks) |
