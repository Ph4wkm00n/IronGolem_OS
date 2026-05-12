# Recipe — inbox (route `/inbox`)

Source: `apps/web/src/pages/v2/Inbox.tsx`
Mock source: `apps/web/src/_mocks/inbox.ts` via `api.v2.inbox.getMock()`

## Baseline (`inbox.baseline.png`)

1. `cd apps/web && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock bun run dev`
2. Open `http://localhost:3000/inbox`.
3. Default chip "Awaiting approval" pre-selected.
4. First row `i01` (Marcus Yi PO reply) auto-selected, draft renders in the
   right pane.
5. Capture full-page screenshot.

### Expected on-page state

- 8 awaiting items in the list, `i01` highlighted.
- Right pane shows email draft, tone-check pill = passed.
- Audit trail under draft has 4 entries.

## After-approve (`inbox.after-approve.png`)

1. From baseline state, click "Approve & send" on `i01`.
2. Wait for the toast.
3. Capture full-page screenshot.

### Expected on-page state

- `i01` is gone from the awaiting list (moved to done).
- `i02` (Sandra Lopez reschedule) is now the auto-selected row.
- Chip counts updated: Awaiting -1, Done +1.
