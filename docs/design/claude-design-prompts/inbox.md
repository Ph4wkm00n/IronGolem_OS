# Inbox — Claude Design Brief

**Route:** `/inbox` · **Job:** Proposals and drafts your assistant teams have queued for your approval, edit, or skip.

Paste the system prompt from `docs/design/claude-design-guide.md` section 2 FIRST, then paste the brief below.

## Route brief (paste into Claude Design)

```text
Design the Inbox route for IronGolem OS — `/inbox`. This is the page where
the user approves, edits, denies, or postpones every assistant-team
proposal. It's the second-most-used route after the Workspace Dashboard.

Primary surface (the page is largely this):
- Two-column layout on desktop: list on left (24rem wide), detail drawer
  on right (flex-1). Single-column on mobile, where selecting a row pushes
  the detail view to a full-screen.
- Top of list: filter chips — All / Awaiting approval / Drafts / Held for
  review / Done today. Live counts per chip. Selected chip is filled,
  others are outlined.
- Each item in the list:
  - Title (5-10 words, plain language)
  - Source pill (email / telegram / webhook / calendar) — small icon + label
  - RiskBadge (low / medium / high) — `<RiskBadge level={...} />` from @irongolem/ui
  - Time ("2m ago" relative)
  - One-line summary
  - "Why this is here" cause sentence in plain language
  - Subtle unread indicator on the row (dot to the left of the title)
- Detail drawer (right column):
  - Item title at page-title scale
  - Origin chips: source · risk · time · routed-by-team
  - Drafted content (for proposals): rendered as a readable block, not
    monospace — emails read like emails, calendar invites like invites
  - "Why this is here" callout in a warning-tinted card with the cause
  - SafetyCard underneath: can / cannot / needs approval / stops if
  - Action row at the bottom: primary Approve button (accent-solid), Edit
    Draft (outline), Deny (text-only blocked), and a tertiary "Snooze 1h"
  - Audit trail at the bottom: timeline of how the item arrived (who
    classified it, what team handed it off, what triggered the draft)

Mandatory patterns:
- Approving an item moves it from "Awaiting approval" → "Done today" with
  optimistic UI (instant move + toast confirms server-acked).
- Denying an item moves it to "Held for review" with a cause sentence.
- Editing an item opens an inline editor — keep the rest of the page
  responsive while editing.
- Empty state (no items): the message is "Your inbox is empty — your
  assistant teams are handling everything inside the rules."
- Suppress-on-OK: items the system handled cleanly never appear here.
  Only things needing the human surface.

Tech reminders:
- React 19, TypeScript strict, Tailwind utility classes only.
- Semantic palette: bg-warning / text-warning for "Awaiting approval";
  bg-blocked for "Denied"; bg-safe for "Done".
- Use TODO(integrator) comments to flag @irongolem/ui swaps.
- Mock 20 inbox items inline at the top of the file. Mix sources, risks,
  and statuses so the filter chips have meaningful counts.

Output: ONE TSX file with a named export `export function Inbox()`.
```

## Mock data shape

```ts
interface InboxItem {
  readonly id: string;
  status: "proposed" | "denied" | "snoozed" | "done";
  readonly source: "email" | "telegram" | "webhook" | "calendar";
  readonly title: string;
  readonly summary: string;
  readonly riskLevel: "low" | "medium" | "high";
  readonly proposedAt: string; // "2m ago"
  readonly cause: string;       // one-sentence "why this is here"
  readonly draftedContent?: string;       // for proposals
  readonly routedByTeam?: string;
  readonly auditTrail?: ReadonlyArray<{ event: string; at: string }>;
}
```

## Components to reuse (TODO substitution markers)

- `RiskBadge` — `<RiskBadge level={item.riskLevel} />` for the risk pill
- `SafetyCard` — `canAccess` / `cannotAccess` / `needsApprovalFor` / `stopsIf` arrays in the detail drawer
- `Timeline` — the audit trail at the bottom of the detail drawer
- `WorkspaceTopbar` — already imported from `pages/v2/_shared/WorkspaceTopbar`; sit the page content below it

## Page patterns

- **Safety First Cards** above action buttons (always).
- **Timeline v2** for the audit trail.
- **Visible Trust** — "Permission used: send external email" inline on each item.
- **Empty state** copy explains what *will* appear, not just that it's empty.

## Route-specific anti-patterns

| Don't | Do |
|---|---|
| Show items without a one-sentence cause | Always include "Why this is here" |
| Default "read" state without explicit action | Items stay in the queue until acted on |
| Hide the safety summary behind a click | Surface it above the action buttons by default |
| Render drafted content in a monospace block | Render it like the medium it'll be sent in |
| Show counts only on the active chip | All chips show their count |
