# `pages/v2/` — Integrated Claude Design routes

Pages here are the production-promoted, audited counterparts of designs that
started in `apps/web/src/_design-inbox/`. They're the v2 route family selected
by the `VITE_ENABLE_V2_UI` env flag at build time.

**Routing model:**

- Default (`VITE_ENABLE_V2_UI` unset or `false`): `apps/web/src/App.tsx` mounts pages from `apps/web/src/pages/`.
- Opt-in (`VITE_ENABLE_V2_UI=true`): for each route, App.tsx prefers `pages/v2/<Route>.tsx` if it exists; otherwise falls back to the legacy `pages/<Route>.tsx`.

This keeps both route families buildable while v2 is incomplete.

## Adding a v2 page

1. Finish the integration pass in `_design-inbox/<route>/.scratch/`.
2. Move the final `.tsx` here as `pages/v2/<Route>.tsx` with the same default export shape used by the legacy page (named export of the page component).
3. Update `apps/web/src/pages/v2/registry.ts` to list the new route (one line — Route path → component import).
4. Capture an Interceptor screenshot baseline to `tests/visual/<route>.baseline.png`.
5. Build with `VITE_ENABLE_V2_UI=true bun run build` and verify routing.

## Cutover

When every legacy page has a v2 counterpart, the cutover is:
1. Flip the flag default to on.
2. Run one release with both families coexistent for safety.
3. Delete `pages/`, rename `pages/v2/` → `pages/`, drop the flag and the registry.
