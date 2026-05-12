# Recipe — memory (route `/memory`)

Source: `apps/web/src/pages/v2/Memory.tsx`
Mock source: `apps/web/src/_mocks/memory.ts` via `api.v2.memory.getMock()`

## Baseline (`memory.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/memory`.
3. List view, "All" subject pill, "All" freshness pill, empty search.
4. Capture full-page screenshot.

### Expected on-page state

- 22 memory cards in the list, oldest 41d stale, newest 2h.
- Re-verify pills appear on 7 items past the 30-day staleness threshold.
- No card expanded (all collapsed by default).
