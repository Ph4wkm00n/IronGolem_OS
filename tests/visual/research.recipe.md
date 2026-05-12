# Recipe — research (route `/research`)

Source: `apps/web/src/pages/v2/Research.tsx`
Mock source: `apps/web/src/_mocks/research.ts` via `api.v2.research.getMock()`

## Baseline (`research.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/research`.
3. Default sort = "recent", no topic filter, low-impact items visible.
4. Capture full-page screenshot.

### Expected on-page state

- Header shows "47 sources monitored" and "1,284 quietly archived today".
- 14 finding cards, `f01` (carbon credit) ringed as featured/top-impact.
- 3 cards display "conflicting source" chips (f01, f03, f10, f13).
