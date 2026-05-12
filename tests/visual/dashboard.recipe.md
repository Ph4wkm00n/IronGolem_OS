# Recipe — dashboard (route `/`)

Source: `apps/web/src/pages/v2/Home.tsx`
Mock source: `apps/web/src/_mocks/home.ts` via `api.v2.home.getMock()`

## Baseline (`dashboard.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/` in the Interceptor browser.
3. Wait until heartbeat pill renders "18/19 systems".
4. No filter chip selected; default sort.
5. Capture full-page screenshot.

### Expected on-page state

- Heartbeat: healthy, 18/19, Research index degraded.
- Trust history rendered for 6 teams.
- Event timeline shows 20 mock events, first row is the Marcus Yi proposal.

## After-approve (`dashboard.after-approve.png`)

1. From baseline state, click "Approve" on event `e01` (Marcus Yi reply).
2. Wait for the toast to settle (~1s).
3. Capture full-page screenshot.

### Expected on-page state

- Event `e01` now shows status `taken` with a green check.
- Toast strip reads "Approved · sent to marcus@riverbend.co" and is mid-fade.
- All other events unchanged.
