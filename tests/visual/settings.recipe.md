# Recipe — settings (route `/settings`)

Source: `apps/web/src/pages/v2/Settings.tsx`
Mock source: `apps/web/src/_mocks/settings.ts` via `api.v2.settings.getMock()`

## Baseline (`settings.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/settings`.
3. Default section "Account" selected in the sidebar.
4. Capture full-page screenshot.

### Expected on-page state

- Sidebar lists six sections (Account, Connectors, Deployment, Notifications,
  Recipes, Advanced); Account highlighted.
- Operator card shows Mira Okafor (MO avatar), passkey sign-in.
- Workspace card shows Okafor Studio · US-West (Oregon) · Solo · Beta.
- Sessions table shows 5 entries, current device flagged.
