# Recipe — security (route `/security`)

Source: `apps/web/src/pages/v2/Security.tsx`
Mock source: `apps/web/src/_mocks/security.ts` via `api.v2.security.getMock()`

## Baseline (`security.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/security`.
3. Audit filter "all"; no policy drawer open.
4. Capture full-page screenshot.

### Expected on-page state

- Five-layer card row: layers 1/2/4/5 = ok, layer 3 = watching.
- Policy library shows 8 policies; pol-05 marked "under review", pol-08 paused.
- Audit log lists 25 entries newest-first; a01 (Telegram quarantine) at top.
