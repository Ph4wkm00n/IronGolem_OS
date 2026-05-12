# Recipe — health (route `/health`)

Source: `apps/web/src/pages/v2/Health.tsx`
Mock source: `apps/web/src/_mocks/health.ts` via `api.v2.health.getMock()`

## Baseline (`health.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/health`.
3. Default state: overall "attention" (Operations team paused).
4. Capture full-page screenshot.

### Expected on-page state

- Header shows overall state = "Needs your attention" (warning tone).
- 14 health-component tiles grouped Core / Connectors / Teams.
- 8 self-heal events in the log, newest first (Webhook receiver 23m ago).
- 3 predictive warnings: Email IMAP (72%), Telegram (100%), Operations (41%).
