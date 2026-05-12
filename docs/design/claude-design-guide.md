# Claude Design — UI/UX Guide for IronGolem OS

This is the design contract a designer (or `Claude.ai` chat) uses when producing
TSX/React pages for the IronGolem OS frontend.

It pairs with two existing docs:
- **`docs/design/ux-mission-and-pillars.md`** — *why* (mission, pillars, voice).
- **`docs/design/claude-design-handoff.md`** — *what happens after design* (audit, promotion, visual regression).

This file is *what to design*: principles, palette, components, page patterns,
tech constraints, and a copy-pasteable system prompt that loads all of it into
Claude.ai in one block.

---

## 1. Audience and use

There are two audiences for this document, in this order:

1. **The human designer** — needs the mental model, the palette, the components,
   the page patterns, and the "what good looks like" examples.
2. **Claude inside the Claude.ai chat canvas** — needs a precise system prompt
   so its TSX output is shaped to land cleanly through the integration pipeline.

If you only do one thing with this doc, copy section 2 ("The Claude Design
system prompt") into the start of every Claude.ai design conversation.

---

## 2. The Claude Design system prompt

Paste this block at the start of every Claude.ai conversation where you want
to produce IronGolem OS UI. It contains everything below condensed for the
model.

```
You are designing UI for IronGolem OS — a self-hosted autonomous-assistant
platform. Produce React 19 + TypeScript components, strict-mode safe, using
Tailwind CSS utility classes only. No shadcn/ui or other third-party UI libs
unless I explicitly approve. No new icon libraries — use inline SVG from
Heroicons-style strokes (1.5 stroke-width, currentColor) matching the
existing app shell.

Output format per page:
- One TSX file, named export with capitalized component name
  (e.g. `export function Inbox() { ... }`).
- Mock data inline at the top of the file as typed `const`s — the
  integration pipeline swaps these for real API calls later.
- Brief top comment: route path + one-sentence purpose.
- React Router v7 idioms only (Link, NavLink, useNavigate, Outlet).
- No state libraries beyond useState/useReducer/useEffect.

Color palette — use these SEMANTIC Tailwind classes, not raw color names:

  bg-safe / text-safe / border-safe          → success, healthy, approved
  bg-warning / text-warning / border-warning → needs attention
  bg-blocked / text-blocked / border-blocked → denied, error
  bg-recovered / text-recovered / ...        → auto-healed
  bg-quarantined / text-quarantined / ...    → isolated for review
  bg-accent / text-accent / border-accent    → primary brand interaction
  bg-neutral / text-neutral / border-neutral → default informational

  Suffixes: -bg (default), -bg-hover, -border, -text, -solid, -solid-hover
  Example: bg-safe = light-green tint; bg-safe-solid = strong green fill.

  For Tailwind's built-in numeric shades (bg-neutral-200, text-neutral-600,
  bg-white, etc.) — fine to use when no semantic intent applies (e.g.
  borders, chrome). Prefer semantic aliases for status-bearing surfaces.

Typography — match these named scales (rendered via the design system):
- Page title:    text-3xl font-bold tracking-tight text-neutral-900
- Section title: text-xl font-semibold tracking-tight text-neutral-900
- Body:          default size, color text-neutral-700
- Caption:       text-sm text-neutral-500
- Label:         text-xs font-medium uppercase tracking-wide text-neutral-500
- Code:          font-mono text-sm bg-neutral-50 px-1.5 py-0.5 rounded

Spacing — 4px base unit. Use Tailwind's default scale (gap-2, p-4, py-6, etc.).
Card padding: p-4 sm:p-6. Page container: max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6.

Reusable utility classes already in globals.css:
  .page-container, .page-title, .section-title, .card, .card-padded

Existing components in @irongolem/ui — PREFER REUSING these over rebuilding:
- HeartbeatStatus  — health-status card
- PolicyCard       — five-layer permission explainer
- ResearchCard     — research finding with confidence + freshness
- RiskBadge        — small inline risk indicator
- SafetyCard       — "can / cannot / needs approval / stops if" summary
- Timeline         — primary explanation surface (taken/proposed/blocked/healed/quarantined/research-update/squad-handoff)

If your design needs one of these patterns, write a comment pointing at the
component name and use a placeholder JSX shape; the integrator will swap to
the real import.

Voice (text content) — plain language for the default surface. Avoid jargon
("orchestrator", "vector", "tenant", "OTLP"). Use outcome-oriented labels:
"Assistant team" not "agent squad"; "Workspace" not "tenant"; "Safety rules"
not "policy engine". Every blocked or quarantined event needs a one-sentence
human-readable cause.

Mandatory UX patterns when applicable:
- Visible Trust: show permissions BEFORE actions execute, not after.
- Suppress-on-OK: only surface alerts that need attention.
- Explainable Autonomy: every automation has a "why did this happen?" link.
- Progressive disclosure: basic users see summaries, advanced controls hide
  behind explicit toggles.

NEVER produce:
- Emojis (none, anywhere — including inside JSX text).
- Raw hex colors in className strings.
- New icon libraries or component libraries.
- State management beyond useState/useReducer/useEffect/useContext.
- Default exports (always named exports — `export function Inbox`).
- Server-side code, route handlers, or anything outside the React surface.
- Mock data inside child components — keep it at the top of the file.
- More than one route's UI per file.

When in doubt, ask which route I'm designing before producing code.
```

---

## 3. Visual language

### Color palette (semantic)

Every surface that conveys state uses one of seven semantic palettes. All are
wired through `packages/design-tokens` and exposed as Tailwind aliases via
the bridge.

| Palette | Meaning | When to use |
|---|---|---|
| `safe` | Healthy / approved / completed | "Action taken", "All checks passed" |
| `warning` | Needs attention | "Awaiting approval", "Slow response" |
| `blocked` | Denied / error / cannot proceed | "Policy blocked", "Failed" |
| `recovered` | Auto-healed | "Healed itself", "Recovered from failure" |
| `quarantined` | Isolated for review | "Suspended pending audit" |
| `accent` | Primary brand interaction | Primary buttons, active nav state |
| `neutral` | Default / informational | Chrome, body surfaces, dividers |

Each palette has six fields, available as suffixed Tailwind classes:

```
bg-safe          // light tint (DEFAULT — same as bg-safe-bg)
bg-safe-bg-hover // slightly darker — for hover/active states
border-safe      // border line tint
text-safe        // dark readable text on neutral bg
bg-safe-solid    // strong fill — badges, indicators
bg-safe-solid-hover
```

**Rule:** never write raw hex colors (`#10B981`) in className. Use the
semantic alias and let the design system control the actual value.

### Typography

| Token | Use | Tailwind |
|---|---|---|
| Page title | Top-of-page heading | `text-3xl font-bold tracking-tight text-neutral-900` |
| Section title | Sub-heading within a page | `text-xl font-semibold tracking-tight text-neutral-900` |
| Body | Default reading text | base size, `text-neutral-700` |
| Caption | Metadata, timestamps | `text-sm text-neutral-500` |
| Label | Form labels, badges | `text-xs font-medium uppercase tracking-wide text-neutral-500` |
| Code | Inline code, identifiers | `font-mono text-sm bg-neutral-50 px-1.5 py-0.5 rounded` |

The repo's `globals.css` also exposes utility classes — `.page-title`,
`.section-title`, `.page-container`, `.card`, `.card-padded` — for the most
common patterns. Use them when you can.

### Spacing

Base unit is 4px. Use Tailwind's default numeric scale (`p-1` = 4px,
`p-4` = 16px, etc.). House conventions:

- **Card padding:** `p-4 sm:p-6`
- **Page container:** `max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6`
- **Section gap:** `space-y-6` between sections, `space-y-3` within
- **Inline gap:** `gap-2` for tight icon+text rows, `gap-4` for card grids

### Iconography

Inline SVG only. Match the existing app shell's style: 1.5 stroke-width,
`stroke="currentColor"`, `fill="none"`, viewBox `0 0 24 24`. The Heroicons
outline set is the visual reference — copy-paste paths directly rather than
introducing an icon library.

```tsx
<svg className="w-5 h-5" fill="none" viewBox="0 0 24 24"
     stroke="currentColor" strokeWidth={1.5}>
  <path strokeLinecap="round" strokeLinejoin="round" d="..." />
</svg>
```

---

## 4. Component vocabulary

`packages/ui/src/components/` already ships these. **Prefer reusing them over
rebuilding the same shape.** The audit pipeline (Step F3 of the integration
plan) flags duplicates so you can dedupe at promotion time, but it's cleaner
to ask Claude to reuse upfront.

| Component | Props summary | Used by |
|---|---|---|
| `SafetyCard` | `canAccess`, `cannotAccess`, `needsApprovalFor`, `stopsIf`, `riskLevel?` | Recipes page, Squad detail |
| `PolicyCard` | Five-layer policy explainer (who / agent / channel / tools / approval) | Security page, recipe drawers |
| `RiskBadge` | `level` (none / low / medium / high / critical) | Anywhere risk is named |
| `Timeline` | `entries` (state, title, description, timestamp, agentRole?), `maxVisible?` | Home, Inbox, Health detail |
| `HeartbeatStatus` | State (Healthy / Quietly recovering / Needs attention / Paused / Quarantined) | Health page, Home sidebar |
| `ResearchCard` | Title, summary, confidence, freshness, sources, contradiction marker, action | Research page |

If a design needs one of these shapes, **reference it by name** in JSX with a
placeholder and a comment:

```tsx
{/* TODO(integrator): replace with <SafetyCard canAccess={...} ... /> */}
<div className="card-padded">
  <h3 className="section-title">Safety summary</h3>
  ...
</div>
```

The integrator swaps the placeholder at promotion. Don't try to import
`@irongolem/ui` inside a Claude.ai chat — the imports won't resolve until
the file lands in the repo.

---

## 5. Page patterns

Patterns are reusable layouts that span multiple routes. Lifted from
`docs/design/design-patterns.md` for the Claude-Design context.

### 5.1 Safety First Cards

Every recipe, squad, and connector exposes a compact safety block before
showing controls. Four sections:

- **Can access** (list of permitted resources)
- **Cannot access** (list of denied resources)
- **Needs approval for** (actions requiring user OK)
- **Stops automatically if** (kill-switch conditions)

Use `SafetyCard` from `@irongolem/ui`. Default position: above the action
buttons, not below — users read safety before they act.

### 5.2 Timeline v2

The timeline is the **primary explanation surface** of the entire app. It
appears on Home, Inbox detail, Health detail, and within squad cards.

States are fixed (use the `TimelineState` union from `@irongolem/ui`):
`taken | proposed | blocked | healed | quarantined | research-update | squad-handoff`.

Each entry needs:
- `title` — what happened (5-10 words)
- `description` — why and how (one sentence, plain language)
- `timestamp` — relative ("2m ago") preferred over absolute
- `agentRole?` — which agent did this (optional)

### 5.3 Policy Explainer Cards

Layered cards that translate the five-layer permission model into plain
language. Each card answers in order:

1. Who can trigger?
2. Which agent acts?
3. Which channel?
4. Which tools allowed?
5. What needs approval?

Use `PolicyCard`. One card per recipe / squad / connector.

### 5.4 Heartbeat Status Cards

Calm health status indicators. Five canonical states with their tones:

| State | Tone | When |
|---|---|---|
| Healthy | `safe` palette | Everything normal |
| Quietly recovering | `recovered` palette | Self-heal in progress |
| Needs your attention | `warning` palette | User action required |
| Paused | `neutral` palette | Intentionally stopped |
| Quarantined | `quarantined` palette | Isolated for safety |

Each heartbeat shows: *what was checked*, *what changed*, *whether recovery
succeeded*, *whether user action is needed* — in that order.

### 5.5 Research Cards

Display research findings with trust indicators. Fields: title, summary
(2-3 sentences), confidence (high/medium/low), freshness ("verified 2h
ago"), source count, contradiction marker (if sources disagree), action
suggestion ("Apply finding" / "Mark reviewed").

### 5.6 Memory Graph Explorer

- **List and card views are the default.** Graph visualisation is opt-in.
- Every memory node shows its evidence and freshness.
- "Why do you know this?" is always one click away.

### 5.7 Progressive disclosure / Advanced mode

Basic users see summaries. Advanced surfaces hide behind explicit toggles:

- Traces and spans
- Cache metrics
- Provider routing
- Reasoning controls
- Policy detail
- Squad internals

Default users should *never need* advanced mode to accomplish their core
flows. When designing a page, ask: *can a non-technical user complete the
primary task without revealing advanced controls?* If the answer is no,
the layout is wrong.

---

## 6. Routes and their jobs

The current frontend has 8 routes. Each has one job. Design output should
respect that job — a route that tries to do two things needs to be split.

| Route | Job | Primary surface |
|---|---|---|
| `/` (Home) | What's happening right now | Timeline + Heartbeat summary |
| `/inbox` | Approvals, drafts awaiting user | Item list + detail drawer |
| `/recipes` | Browse / configure automations | Recipe cards with SafetyCard |
| `/research` | Findings the agents discovered | ResearchCard grid |
| `/memory` | What the system knows + why | Memory list / graph toggle |
| `/health` | System status, healing log | HeartbeatStatus + Timeline |
| `/security` | Policies, audit log, blocked events | PolicyCard list + filter |
| `/settings` | Account, connectors, deployment mode | Sectioned settings form |

When designing a redesign of an existing route, look at the legacy
`apps/web/src/pages/<Route>.tsx` first to understand what data it currently
shows. The redesign can change everything *visual*, but the *job* should
stay the same.

---

## 7. Tech constraints

All apply to every TSX file Claude Design produces.

| Topic | Rule |
|---|---|
| Language | TypeScript strict mode |
| Framework | React 19 (function components only, no class components) |
| Router | React Router v7 (`Link`, `NavLink`, `useNavigate`, `Outlet`) |
| Styling | Tailwind utility classes only (no styled-components, no Emotion, no CSS modules) |
| State | `useState`, `useReducer`, `useEffect`, `useContext` — nothing else |
| Imports from `@irongolem/ui` | Reference by name in a comment; do NOT write the import (it won't resolve in Claude.ai) |
| Data | Inline typed mocks at the top of the file |
| Export shape | Named export, capitalised — `export function Inbox()` |
| File scope | One route per file. Helpers inside the file are OK; pages are not nested |
| Accessibility | `aria-label` on icon buttons, semantic landmarks (`<main>`, `<nav>`, `<aside>`), `role="list"` on custom lists |
| Responsive | Mobile-first; use `sm:`, `md:`, `lg:` breakpoints — but mobile QA is deferred to v0.2 |

### Mock-data template

```tsx
// route: /inbox
// purpose: queue of agent proposals and drafts awaiting user approval

interface InboxItem {
  readonly id: string;
  readonly title: string;
  readonly source: "email" | "telegram" | "webhook";
  readonly riskLevel: "low" | "medium" | "high";
  readonly proposedAt: string;
  readonly summary: string;
}

const mockItems: readonly InboxItem[] = [
  {
    id: "1",
    title: "Draft reply to Sarah re: Q3 forecast",
    source: "email",
    riskLevel: "low",
    proposedAt: "2m ago",
    summary: "Two-paragraph response confirming the meeting and asking about availability.",
  },
  // ...
];
```

---

## 8. Anti-patterns

What good Claude Design output looks like is partly defined by what it
avoids. Audit script (Step F3) flags these on every export.

| Anti-pattern | Why it's banned | Use this instead |
|---|---|---|
| `bg-emerald-50` for status surfaces | Loses semantic meaning | `bg-safe` |
| `<Card>` from shadcn/ui | Not in repo | `<div className="card-padded">` |
| Lucide / Tabler / FontAwesome icons | Not in repo | Inline SVG matching Heroicons |
| Class components | Outdated | Function components + hooks |
| Default exports | Inconsistent | Named exports |
| Inline mock data inside child components | Hard to swap | Mock constants at top of file |
| Emojis in UI text | Project-wide ban | Plain text or icon SVG |
| Animations beyond `transition-` utilities | Performance, motion sensitivity | Stick to Tailwind transitions |
| Multiple pages per file | Breaks the integration pipeline | One TSX file = one route |
| Hard-coded ARIA labels in English when localisation is needed | Bypasses i18n | Wrap in `t("...")` placeholder — i18n is deferred but the seam should stay |

---

## 9. Writing voice

Per the UX mission: **plain language on every default surface.** The
designer (and Claude) writes the actual user-facing strings, so the voice
is as much a design responsibility as the layout.

- Use outcome-oriented labels, not engineering labels.
- Every blocked / quarantined event needs a one-sentence cause in human language.
- "Why did this happen?" must always be visible or one click away.
- Tooltips are for hints, not for hiding critical information.
- Empty states explain what *will* appear, not just that something is empty.

Examples:

| Don't | Do |
|---|---|
| "Agent squad executed orchestrator policy" | "Inbox team applied your draft-only rule" |
| "OTEL span ingestion failed" | "Couldn't log this action — retrying" |
| "Tenant isolation breach" | "This action would touch another workspace — blocked for safety" |
| "No data" | "No proposals yet — your assistants will queue them here as they arrive" |

---

## 10. Worked example — what good Claude Design output looks like

Below is the shape Claude.ai should produce when asked to design the Inbox
page. (Truncated for brevity — full file would include the rest of the
list rendering and a detail drawer.)

```tsx
// route: /inbox
// purpose: queue of agent proposals and drafts awaiting user approval

import React, { useState } from "react";

// TODO(integrator): import RiskBadge from "@irongolem/ui"
// TODO(integrator): import SafetyCard from "@irongolem/ui"

interface InboxItem {
  readonly id: string;
  readonly title: string;
  readonly source: "email" | "telegram" | "webhook";
  readonly riskLevel: "low" | "medium" | "high";
  readonly proposedAt: string;
  readonly summary: string;
  readonly cause: string;  // human-readable reason this entered the inbox
}

const mockItems: readonly InboxItem[] = [
  {
    id: "1",
    title: "Draft reply to Sarah re: Q3 forecast",
    source: "email",
    riskLevel: "low",
    proposedAt: "2m ago",
    summary: "Two-paragraph response confirming the meeting and asking about availability.",
    cause: "Sarah's email matches your 'auto-draft reply' rule for known senders.",
  },
];

export function Inbox() {
  const [selectedId, setSelectedId] = useState<string | null>(mockItems[0]?.id ?? null);
  const selected = mockItems.find((i) => i.id === selectedId) ?? null;

  return (
    <main className="page-container">
      <header className="mb-6">
        <h1 className="page-title">Inbox</h1>
        <p className="text-neutral-500 mt-1">
          Proposals and drafts waiting for you to approve, edit, or skip.
        </p>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-[24rem_1fr] gap-6">
        <section aria-label="Inbox list" className="card divide-y divide-neutral-100">
          {mockItems.length === 0 ? (
            <p className="p-6 text-neutral-500">
              No proposals yet — your assistants will queue them here as they arrive.
            </p>
          ) : (
            <ul role="list">
              {mockItems.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    onClick={() => setSelectedId(item.id)}
                    className={`w-full text-left px-4 py-3 transition-colors ${
                      selectedId === item.id ? "bg-accent" : "hover:bg-neutral-50"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-medium text-neutral-900 truncate">{item.title}</span>
                      {/* TODO(integrator): <RiskBadge level={item.riskLevel} /> */}
                      <span className="text-xs uppercase tracking-wide text-neutral-500">
                        {item.riskLevel}
                      </span>
                    </div>
                    <p className="text-sm text-neutral-500 mt-1 line-clamp-1">{item.summary}</p>
                    <span className="text-xs text-neutral-400 mt-1 block">{item.proposedAt}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section aria-label="Selected proposal detail" className="card-padded">
          {selected ? (
            <>
              <h2 className="section-title">{selected.title}</h2>
              <p className="mt-2 text-neutral-700">{selected.summary}</p>

              <div className="mt-6 rounded-lg border border-warning bg-warning p-4">
                <p className="text-sm text-warning-text">
                  <strong className="font-medium">Why this is here:</strong>{" "}
                  {selected.cause}
                </p>
              </div>

              {/* TODO(integrator): replace with <SafetyCard ... /> */}
              <div className="mt-6 card-padded">
                <h3 className="text-sm font-medium uppercase tracking-wide text-neutral-500">
                  Safety summary
                </h3>
                <p className="mt-2 text-sm text-neutral-700">
                  Sending replies you approve. Cannot send without your OK.
                </p>
              </div>

              <div className="mt-6 flex gap-3">
                <button className="px-4 py-2 rounded-lg bg-safe-solid hover:bg-safe-solid-hover text-white text-sm font-medium">
                  Approve and send
                </button>
                <button className="px-4 py-2 rounded-lg border border-neutral-border text-neutral-700 text-sm font-medium hover:bg-neutral-bg-hover">
                  Edit draft
                </button>
                <button className="px-4 py-2 rounded-lg text-blocked text-sm font-medium hover:bg-blocked">
                  Skip
                </button>
              </div>
            </>
          ) : (
            <p className="text-neutral-500">Select a proposal to review.</p>
          )}
        </section>
      </div>
    </main>
  );
}
```

Things that make this "good":

- Single route, single named export.
- Mock data + typed shape at top, easy for the integrator to swap.
- TODO comments naming the `@irongolem/ui` components to substitute at promotion.
- Semantic color classes (`bg-warning`, `bg-safe-solid`, `text-blocked`) — no raw hex.
- Plain-language strings everywhere, including the empty state and the
  "Why this is here" callout.
- Uses repo utility classes (`page-container`, `page-title`, `section-title`, `card`, `card-padded`).
- Keyboard / a11y-aware: `<button type="button">`, `role="list"`, `aria-label`s.
- No `useEffect` for data — mocks render synchronously, real fetching is wired by the integrator.

---

## 11. After designing — handoff

When the design is ready in Claude.ai:

1. Copy the final TSX into a file under
   `apps/web/src/_design-inbox/<route>/<route>.tsx` in the repo.
2. (Optional) Add `notes.md` in the same folder for intent, motivations, open questions.
3. Open a PR titled `[design] inbox/<route> draft N`.
4. The integration pipeline (Step F3 of `Plans/integrate-claude-design.md`)
   runs the audit and the integrator promotes the page from there.

Everything beyond that is the integrator's job — see
`docs/design/claude-design-handoff.md` for the post-design pipeline.

---

## Cross-references

- **UX mission and pillars:** `docs/design/ux-mission-and-pillars.md`
- **Design patterns reference:** `docs/design/design-patterns.md`
- **Integration handoff (post-design):** `docs/design/claude-design-handoff.md`
- **Integration plan:** `Plans/integrate-claude-design.md`
- **Tokens source:** `packages/design-tokens/src/`
- **Component catalog:** `packages/ui/src/components/`
- **Tailwind bridge:** `packages/design-tokens/src/tailwind-bridge.ts`
