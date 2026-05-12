# `_mocks/` — Typed mock data per route

Every integrated page renders against typed mock data first; the swap to the
real gateway is a single change in `apps/web/src/lib/api.ts`. Mocks here are
the source for `mock` mode of that seam.

## Convention

- One file per route: `inbox.ts`, `recipes.ts`, `research.ts`, etc.
- Each file exports a single `mock<RouteName>: <Schema>` constant plus any
  helper functions the page needs (e.g. `mockInboxFiltered(query: string)`).
- Mock shapes derive from `@irongolem/schema` when the schema exists.
  Where it doesn't, declare a local type with a `// TODO: align with schema.X`
  comment so the audit pipeline can flag it later.

## Wiring

Pages never import `_mocks/*` directly. They call into `apps/web/src/lib/api.ts`,
which decides between mock and real based on `VITE_API_MODE=mock|real`.

```ts
// in lib/api.ts
import { mockInbox } from "../_mocks/inbox";

export async function getInbox(): Promise<InboxItem[]> {
  if (import.meta.env.VITE_API_MODE === "mock") return mockInbox;
  const res = await fetch("/api/v1/inbox");
  return res.json();
}
```

## When real API lands

Per v0.1 backend plan, real endpoints arrive in waves. To swap a single
route's mock for real, edit the matching `lib/api.ts` function and re-run
the Interceptor visual baseline to confirm no regression.
