# Claude Design → IronGolem OS Frontend Integration Plan

## Context

The v0.1 critical-moves plan (`Plans/create-a-plan-to-glowing-nest.md`) is mid-flight: Steps 1-3 verified, Steps 4-8 pending. That plan is **backend-only** — it produces one verified flow (Telegram → Gateway → Rust runtime → response) but never touches the web app.

Meanwhile the user is producing UI/UX in **Claude Design** (Claude.ai's chat-canvas / artifact workflow). Output shape assumed: React/TSX components per route, mock-data-driven, default Tailwind utility classes (sometimes shadcn-style imports).

This plan defines how those design artifacts land in the repo **without blocking the v0.1 backend track and without coupling designer cadence to developer cadence**.

## Confirmed Decisions

1. **Landing zone over direct overwrite.** Claude Design exports drop into `apps/web/src/_design-inbox/` first, never directly into `pages/`. The inbox is reviewed → audited → translated → promoted.
2. **Parallel route family during cutover.** Integrated routes live under `apps/web/src/pages/v2/`; existing pages under `apps/web/src/pages/` stay until v2 reaches parity. A `VITE_ENABLE_V2_UI` flag picks the route family at build time.
3. **Mock-first, real-API-second.** Each integrated page wires to typed mocks first; the swap to real API is a separate PR gated on the matching backend endpoint coming online (per the v0.1 plan).
4. **Semantic tokens are non-negotiable.** Raw hex / non-token Tailwind colors get audited and rewritten against `packages/design-tokens`. Visual intent preserved; semantic meaning enforced.
5. **Visual regression via Interceptor.** Every promoted page has an Interceptor-captured baseline screenshot. CI compares against it. This is the only frontend verification gate before merge.

## Sequenced Integration

### Step F1 — Landing zone scaffolding (S, no deps)

Create the directories and the workflow doc that the rest of the plan references.

- **New directory** `apps/web/src/_design-inbox/` — raw Claude Design exports; one folder per route (`_design-inbox/inbox/`, etc.). README explains "everything in here is verbatim from Claude Design and not directly imported by App.tsx."
- **New directory** `apps/web/src/pages/v2/` — integrated, audited pages. Same filenames as `pages/` so the router can swap families with one flag.
- **New directory** `apps/web/src/_mocks/` — typed mock data per route, derived from `@irongolem/schema` where schema exists.
- **New file** `docs/design/claude-design-handoff.md` — designer-facing handoff doc (see Step F7).
- **Modify** `apps/web/src/App.tsx` — read `import.meta.env.VITE_ENABLE_V2_UI` and conditionally route into `pages/v2/`.
- **Modify** `apps/web/.gitignore` — keep `_design-inbox/` tracked (provenance) but allow optional `_design-inbox/.scratch/` for unfinished imports.

### Step F2 — Tailwind ↔ design-tokens bridge (M, parallel)

Claude Design tends to emit raw Tailwind classes (`bg-emerald-50`, `text-amber-800`). The repo wants `bg-safe`, `text-warning`. Bridge solves this **without rewriting every class** by extending Tailwind's theme.

- **New file** `packages/design-tokens/src/tailwind-bridge.ts` — exports a function that flattens semantic tokens (`safe`, `warning`, `blocked`, `recovered`, …) into Tailwind v4 `@theme` block values.
- **Modify** `apps/web/tailwind.config.ts` — `theme.extend.colors` reads the bridge output. After this:
  - `bg-emerald-50` still works (default Tailwind palette intact).
  - `bg-safe` becomes a semantic alias.
  - Audit pipeline (Step F3) suggests semantic swaps but doesn't force them at import time.
- **Modify** `packages/design-tokens/src/index.ts` — export the bridge alongside existing `colors`/`spacing`/`typography` modules.
- **Unit test** verifying bridge output shape (TypeScript only — no runtime test framework needed).

### Step F3 — Component-dedup audit pipeline (M, deps: F1)

Many Claude Design exports rebuild components the repo already has (`Timeline`, `SafetyCard`, `PolicyCard`, etc.). Suppressing rebuilds is what keeps the design system from fragmenting.

- **New file** `scripts/design-component-audit.ts` — Bun script. Inputs: path to `_design-inbox/<route>/`. Behavior:
  1. Parse each `.tsx` file (lightweight regex / `ts-morph` if needed).
  2. Pattern-match against known components in `packages/ui/src/components/` (signature: roughly the JSX shape and prop names).
  3. Pattern-match raw hex / non-semantic Tailwind colors against the design-tokens palette and suggest semantic swaps.
  4. Emit a human-readable report (`<route>/AUDIT.md`) with:
     - Suggested `@irongolem/ui` substitutions (NOT auto-applied)
     - Suggested semantic-token swaps
     - Components that look new (legit candidates for `packages/ui`)
- **CLI**: `bun run scripts/design-component-audit.ts <route>` produces the report.
- The report is reviewed by a human before promotion to `pages/v2/`. Auto-applying would silently kill design intent.

### Step F4 — Mock-data contracts (M, parallel with F2)

Every integrated page needs typed mock data so it renders without backend.

- **New directory** `apps/web/src/_mocks/` — `inbox.ts`, `recipes.ts`, `research.ts`, `memory.ts`, `health.ts`, `security.ts`, `settings.ts`, `home.ts`.
- Each file exports a typed mock derived from `@irongolem/schema` types where the schema is ready, or a pre-declared local type (with `// TODO: align with schema.X` comment) where it isn't.
- **Modify** `apps/web/src/lib/api.ts` — introduce a `mockMode` toggle driven by `VITE_API_MODE=mock|real`. In `mock` mode every API call returns a mock; in `real` mode it hits the gateway.
- The seam is `lib/api.ts` — pages do not import mocks directly; they call `api.getInbox()` etc. and the lib decides where to source from.

### Step F5 — Per-route integration pipeline (M, repeated per route)

The repeatable unit of frontend work. One pass = one route integrated.

For each Claude Design export:

1. Drop verbatim into `_design-inbox/<route>/`.
2. Run `bun run scripts/design-component-audit.ts <route>` → review `AUDIT.md`.
3. Apply mechanical changes in a working file under `_design-inbox/<route>/.scratch/<route>.tsx`:
   - Replace duplicate components with `@irongolem/ui` imports per audit suggestions accepted.
   - Replace data with `import { mock } from "_mocks/<route>"` or `api.<route>()` call.
   - Replace router patterns with `react-router-dom` v7 idioms (`useNavigate`, `Link`).
   - Replace any shadcn/ui imports the repo doesn't have with raw Tailwind + design-tokens equivalents (or accept the new dependency only if audit flags it).
4. Promote to `apps/web/src/pages/v2/<Route>.tsx`.
5. Add route entry in `apps/web/src/App.tsx` under the `VITE_ENABLE_V2_UI=true` branch.
6. Run `bun run dev`, hit `/` for the route, capture an Interceptor screenshot baseline → `tests/visual/<route>.baseline.png`.
7. PR: title `[design] integrate v2 <route>`. Reviewer compares baseline against Claude Design source-of-truth artifact.

### Step F6 — Real-API wiring (S per route, deps: matching v0.1 backend step)

Per-route swap from mock to real data. **This is what couples the frontend track back to the v0.1 backend track.**

| v2 route | Real-API dep | When unblocked |
|---|---|---|
| Inbox | v0.1 Step 5 (Telegram inbound + plan synth) | After Step 5 lands |
| Health | v0.1 Step 6 (persistent stores) + later health service | Partial after Step 6; full after health service ships |
| Home | Pulls from Inbox + Health summaries | After both above |
| Recipes / Research / Memory / Security / Settings | Out of v0.1 backend scope | Stays on mocks until v0.2 |

Per-route swap is a single change in `lib/api.ts` (flip the mock/real switch for that endpoint) plus a re-screenshot to confirm visual unchanged.

### Step F7 — Designer handoff documentation (S, deps: F1)

The contract with the designer (or future-self designing in Claude).

- **New file** `docs/design/claude-design-handoff.md` — covers:
  - How to export from Claude.ai (Copy code → `.tsx` file in a folder).
  - File-naming convention inside `_design-inbox/<route>/`.
  - What to include vs omit (mock data: omit; component code: include; CSS: prefer Tailwind classes inline).
  - How to indicate "this is a redesign of route X" vs "this is a new route Y" (folder name conveys this).
  - The audit pipeline (Step F3) — what its output means and how to respond to it.
- Optional convenience: `bun run design:ingest <route>` CLI that combines `mkdir`, audit run, and a `code .` open. Defer to F5 stability.

### Step F8 — Visual regression CI gate (S, deps: F5 + Interceptor)

The verification harness. Frontend doesn't merge without this passing.

- **New directory** `tests/visual/` — one `.baseline.png` per integrated route + the route's `.recipe.md` describing what state it should be in for the screenshot (e.g. `inbox.recipe.md` says "mock has 3 messages, 2nd is unread").
- **New file** `scripts/visual-check.sh` — uses Interceptor skill via CLI to:
  1. `bun run build && bun run preview` (or dev server).
  2. For each route in `tests/visual/`, `interceptor open` the URL.
  3. Capture screenshot, diff against baseline.
  4. Fail if pixel difference exceeds threshold (start lenient, e.g. 5% — tighten as the design stabilizes).
- **CI wiring** — `Makefile` target `make test-visual`. Wire into `make test` if you want it gating PRs.

## How This Composes With the v0.1 Backend Plan

```
v0.1 backend (existing plan)                    Frontend integration (this plan)
─────────────────────────────                   ─────────────────────────────
[done] Step 1: IPC contract                     Step F1: Landing zone        (parallel, no deps)
[done] Step 2: runtimed binary                  Step F2: Tailwind bridge     (parallel, no deps)
[done] Step 3: Sandbox registry                 Step F3: Audit pipeline      (parallel, deps F1)
       Step 4: Gateway runtime client           Step F4: Mock contracts      (parallel, deps F1)
       Step 5: Telegram inbound + planner ───→  Step F6: Inbox real-API
       Step 6: SQLite stores ──────────────→    Step F6: Health real-API (partial)
       Step 7: Scope cuts                       Step F5: Per-route pass      (repeats per route)
       Step 8: Verification harness ──────→     Step F8: Visual regression CI
```

**Two independent tracks share only**:
- The mock/real seam in `lib/api.ts` (F4).
- The CI verification harness (F8 piggy-backs on Step 8's harness).

This means frontend work can advance immediately, and backend work can continue without waiting for design.

## Verification Gates

All four must pass before any frontend PR merges.

**Gate F1 — Build**
```
cd apps/web && VITE_ENABLE_V2_UI=true bun run build
```
Must succeed with zero TypeScript errors and at least one integrated v2 page.

**Gate F2 — Lint / Typecheck**
```
cd apps/web && bun run lint && bun run typecheck
```

**Gate F3 — Visual regression**
```
make test-visual
```
Diff each integrated route against its baseline; fail above pixel threshold.

**Gate F4 — Mock-to-real swap smoke test**
For routes whose backend dep has landed, set `VITE_API_MODE=real` and verify the route still renders against the live gateway (one curl-of-route + Interceptor screenshot).

## Critical Files

**To create:**
- `apps/web/src/_design-inbox/` (directory + README)
- `apps/web/src/pages/v2/` (directory)
- `apps/web/src/_mocks/` (directory)
- `packages/design-tokens/src/tailwind-bridge.ts`
- `scripts/design-component-audit.ts`
- `scripts/visual-check.sh`
- `docs/design/claude-design-handoff.md`
- `tests/visual/` (directory)

**To modify:**
- `apps/web/src/App.tsx` — feature-flag router branch
- `apps/web/src/lib/api.ts` — mock/real seam
- `apps/web/tailwind.config.ts` — wire bridge
- `packages/design-tokens/src/index.ts` — export bridge
- `apps/web/.gitignore` — `.scratch/` carve-out
- `Makefile` — `test-visual` target

## Reuse Inventory

- `packages/ui/src/components/{HeartbeatStatus, PolicyCard, ResearchCard, RiskBadge, SafetyCard, Timeline}` — re-use first; audit pipeline (F3) makes this systematic.
- `packages/design-tokens/src/{colors, spacing, typography}` — semantic tokens stay the source of truth.
- `packages/schema` — mock data shapes derive from here.
- `apps/web/src/lib/api.ts` — single seam for mock/real swap.
- **Interceptor skill** (PAI) — visual baselines + diff via real Chrome. Per CLAUDE.md this is mandatory for any web verification.

## Out of Scope (this plan)

- Storybook setup — defer to a v0.2 design-system push.
- State management library (Redux/Zustand) — current `useState` + `lib/api.ts` is fine for v0.1.
- Dark mode — design tokens accommodate but no UI surface yet.
- Mobile responsive QA — defer unless designer explicitly flags a route.
- Internationalization — `packages/i18n` is wired but content stays English-only in v0.1.
- Accessibility audit — separate workstream, not blocked on this plan.

## Risk Register

| ID | Risk | Resolution |
|---|---|---|
| R1 | Claude Design imports shadcn/ui (or other lib) the repo doesn't have | Audit (F3) flags imports; integrator decides: vendor the dep, add to package.json, or rewrite |
| R2 | Designer iterates faster than dev integrates | `_design-inbox/` is the buffer — multiple iterations can land before any is promoted |
| R3 | Backend contract changes invalidate frontend mocks | Mocks derive from `@irongolem/schema` (single source of truth); schema changes force a typed mock update |
| R4 | Visual regression false-positives from anti-aliasing / fonts | Start with lenient threshold (5%); tighten per-route as design stabilizes |
| R5 | Two route families (`pages/` and `pages/v2/`) drift | Plan a v0.1.x cutover milestone where v2 becomes default and old `pages/` deletes |
| R6 | Audit script misses a duplicate component → design system fragments anyway | Pattern catalog (the audit's pattern set) is reviewed and expanded each integration pass; this is acceptable cost of preventing R6 entirely |

## Migration / Cutover

- **v2 reaches parity** when every route in `pages/` has a counterpart in `pages/v2/` and visual baselines exist.
- **Default flip**: `VITE_ENABLE_V2_UI=true` becomes default-on. Old `pages/` stay one release for rollback.
- **Cleanup**: delete `pages/` directory; rename `pages/v2/` → `pages/`; delete the feature flag.
- `_design-inbox/` stays under git permanently as design provenance.

## Open Questions for Calibration

These don't block starting the plan but sharpen Step F5 once answered.

1. **Claude Design output shape** — TSX with default Tailwind, or shadcn-style component imports, or HTML-only? Steps F2 and F3 absorb either, but F3 audit patterns calibrate better with one example.
2. **Iteration cadence expectation** — does the designer push whole-app dumps periodically, or per-route updates as they refine? Affects `_design-inbox/` folder shape only.
3. **Visual fidelity tolerance** — pixel-perfect vs "spirit of the design"? Determines starting threshold for Gate F3.
