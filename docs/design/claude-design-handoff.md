# Claude Design Handoff Guide

This is the contract between a designer working in Claude Design (Claude.ai
chat artifacts) and the IronGolem OS frontend codebase.

The goal: designs move from chat → repo with predictable mechanics, no
back-pressure on either side, and a documented audit step that protects the
shared design system from fragmenting under iteration.

## Quick start for designers

1. Generate the page in Claude Design as a React/TSX component.
2. Click "Copy code" — copy the entire component.
3. In the repo, create or open `apps/web/src/_design-inbox/<route>/<page>.tsx`
   where `<route>` matches the URL fragment (`inbox`, `recipes`, `health`, …).
4. Paste verbatim. **Do not edit further.** That file is the source-of-truth
   design artifact.
5. (Optional) Drop a sibling `notes.md` with intent, motivations, open questions.
6. Open a PR titled `[design] inbox/<route> draft N` and tag the integrator.

That's it from the designer side. Everything below this line is the
integrator's job.

## The audit pipeline (integrator side)

A Claude Design export typically includes:

- Components the repo already has (`Timeline`, `SafetyCard`, `RiskBadge`, etc.).
- Raw Tailwind utility classes with hex-named colors (`bg-emerald-50`, `text-amber-800`).
- Sometimes `shadcn/ui` or other third-party imports the repo doesn't have.
- Inline mock data the designer used to illustrate state.

The audit pipeline surfaces all of these so promotion to `pages/v2/` doesn't
silently fragment the design system.

```bash
bun run scripts/design-component-audit.ts <route>
```

This writes `_design-inbox/<route>/AUDIT.md` containing:

- **Component substitutions** — JSX patterns that match existing `@irongolem/ui`
  components. Listed with suggested import + replacement, but never auto-applied.
- **Color swaps** — non-semantic Tailwind colors (`bg-emerald-50`, `text-red-700`)
  mapped to semantic aliases (`bg-safe`, `text-blocked`). The Tailwind ↔
  design-tokens bridge (see `packages/design-tokens/src/tailwind-bridge.ts`)
  makes both shapes work, so deferring the swap is safe.
- **Missing imports** — third-party components the repo doesn't have. Integrator
  decides: vendor the dep, add to `package.json`, or rewrite against
  `@irongolem/ui`.
- **New components** — JSX that looks generic and doesn't match an existing
  pattern. Candidate for graduation to `packages/ui/src/components/`.

## Integration steps

For each route:

1. Drop verbatim export under `_design-inbox/<route>/`.
2. Run the audit. Read `AUDIT.md`.
3. Work in `_design-inbox/<route>/.scratch/<route>.tsx`. Apply:
   - Accepted component substitutions (swap inline JSX for `@irongolem/ui` imports).
   - Accepted semantic-color swaps.
   - Mock-data wiring: replace inline data with `import { mock } from "@/_mocks/<route>"` or `await api.<route>()`.
   - Routing patterns: `react-router-dom` v7 (`useNavigate`, `Link`, `Outlet`).
4. Promote to `apps/web/src/pages/v2/<Route>.tsx`. Export name must match what
   `App.tsx` imports (named export, capitalised route name).
5. Register the route in `apps/web/src/pages/v2/registry.ts`.
6. Capture visual baseline via Interceptor:
   ```bash
   VITE_ENABLE_V2_UI=true bun run dev &
   bun run scripts/visual-check.sh <route> --update
   ```
7. PR: title `[design] integrate v2 <route>`. Reviewer compares the baseline
   to the Claude Design artifact.

## Conventions

| Topic | Rule |
|---|---|
| Filename in `_design-inbox/<route>/` | Designer's choice; audit discovers all `.tsx` |
| Filename in `pages/v2/` | `<RouteName>.tsx` matching the legacy page filename |
| Component export | Named export, capitalised: `export function Inbox() {...}` |
| New components in JSX | Audit decides whether to graduate to `packages/ui/` |
| Raw Tailwind colors | OK in scratch; semantic swaps preferred at promotion |
| Inline mock data | Replaced with `_mocks/<route>.ts` at promotion |
| Whole-app redesigns | Split per-route in `_design-inbox/` before audit |
| Component duplication | Audit flags; integrator deduplicates before promotion |

## What stays out of scope (this guide)

- State management beyond `useState` — defer until a route actually needs it.
- Storybook setup — design system tooling is a v0.2 push.
- Dark mode — palettes accommodate but no surface yet.
- Mobile-responsive QA — case-by-case until a responsive sweep is scheduled.
- a11y — separate audit workstream.

## Why these rules

- **Landing zone, not direct overwrite.** Designer cadence is faster than
  integration cadence. The inbox absorbs the throughput mismatch.
- **Parallel `pages/v2/` family.** Lets both families build, ensures cutover
  is reversible per route.
- **Audit, never auto-apply.** Auto-replacement of components silently kills
  design intent. Human review is the audit's reason for being.
- **Mock-first.** Decouples frontend velocity from backend completion. The
  v0.1 backend track ships endpoints in waves; frontend never waits.
- **Visual baseline mandatory.** Tailwind class drift and component swaps
  break visual intent in ways type-checks don't catch. Interceptor
  screenshots are the only verification that survives.
