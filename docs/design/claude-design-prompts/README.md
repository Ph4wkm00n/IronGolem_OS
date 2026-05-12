# Claude Design Prompts — Per-Route Briefs

This directory contains one pasteable prompt block per IronGolem OS route.
Each file is a complete brief you can paste into a fresh Claude Design
conversation to produce a TSX export that lands cleanly in the integration
pipeline.

## How to use

1. Open a fresh chat at `claude.ai/design`.
2. Open the file in this directory matching the route you want to design.
3. Paste the **System prompt** block from `docs/design/claude-design-guide.md`
   section 2 first (it loads the visual language + tech constraints).
4. Then paste the **Route brief** block from this directory's file (it adds
   route-specific job, data shape, components to reuse, and worked-example
   skeleton).
5. Iterate with Claude until the design lands.
6. Copy the final TSX into `apps/web/src/_design-inbox/<route>/<file>.tsx`.
7. Run the audit, promote to `pages/v2/`, register, capture visual baseline.

## What's here

| File | Route | Job |
|---|---|---|
| [`inbox.md`](inbox.md) | `/inbox` | Proposals + drafts awaiting your approval |
| [`recipes.md`](recipes.md) | `/recipes` | Browse + activate automation templates |
| [`research.md`](research.md) | `/research` | Findings with confidence and freshness |
| [`memory.md`](memory.md) | `/memory` | What the system knows + evidence trail |
| [`health.md`](health.md) | `/health` | System status + healing log |
| [`security.md`](security.md) | `/security` | Policies + audit + blocked events |
| [`settings.md`](settings.md) | `/settings` | Account, connectors, deployment mode |

The `/` (Workspace Dashboard) route is already designed — see
`apps/web/src/pages/v2/Home.tsx`. Its source design is in
`apps/web/src/_design-inbox/dashboard/`.

## Why one file per route

Pasting a single big brief tends to produce diluted output — Claude tries
to balance every concern at once. Per-route briefs keep the prompt scoped
so the output is focused on one job. The shared system prompt handles the
universal constraints (palette, components, tech) once; each route brief
narrows the scope.

## Convention

Each file follows the same shape:

- **Route brief** — pasteable into the chat after the system prompt
- **Mock data shape** — typed example Claude should base its mock on
- **Components to reuse** — the `@irongolem/ui` import list
- **Page patterns** — the relevant patterns from `design-patterns.md`
- **Route-specific anti-patterns** — what NOT to produce for this route
- **Worked example skeleton** — what "good" output looks like at file
  structure level

Brief blocks live in fenced `text` code blocks so the markdown renders
cleanly on GitHub AND the content is one-click copyable.
