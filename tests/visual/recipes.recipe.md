# Recipe — recipes (route `/recipes`)

Source: `apps/web/src/pages/v2/Recipes.tsx`
Mock source: `apps/web/src/_mocks/recipes.ts` via `api.v2.recipes.getMock()`

## Baseline (`recipes.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/recipes`.
3. "All" category tab selected; no recipe drawer open.
4. Capture full-page screenshot.

### Expected on-page state

- 15 recipe cards, grouped by category.
- Status pills: active (green), paused (neutral), new (accent).
- Five-layer safety summary visible above the category tabs.
