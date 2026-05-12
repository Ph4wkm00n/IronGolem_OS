# Chat

_Started 2026-05-11 15:19 UTC_

---

## User



## Assistant

_[tool: list_files]_

_[tool: read_file]_

_[tool: read_file]_

_[tool: read_file]_

_[tool: read_file]_

_[tool: read_file]_

_[tool: read_file]_

_[tool: read_file]_

_[tool: snip]_

_[tool: write_file]_

Got it. Reading the rest of the system shape, I have what I need. I'll match the existing IronGolem visual language (semantic palette from `styles.css`, Inter + JetBrains Mono, the SafetyCard/PolicyCard/Timeline patterns from `ui-primitives.jsx`) and produce **one** TSX file with `export function Recipes()`, plus a host HTML to preview it.

Let me write it.

_[tool: write_file]_

_[tool: snip]_

_[tool: done]_

_[tool: fork_verifier_agent]_

Built `/recipes` as a single TSX file (`Recipes.tsx`) with `export function Recipes()`, plus `Recipes.html` to host it. 15 mock recipes across all six categories, mixed statuses (active / paused / new), responsive 1/2/3 column grid, category tabs with counts, search, and full Inspect drawer.

Pattern compliance:
- **Safety First** — every card renders `SafetyCardPreview` (Can / Needs approval / Cannot) directly above the activation row.
- **Visible Trust** — `TrustStrip` shows risk + permission count (broad/scoped) + last run before the action button.
- **Progressive disclosure** — cron, fallback, retry policy live inside the drawer's `CustomizePanel`, never on the card.
- **Empty state** — copy points to Settings → Recipe Requests; also handles "no search matches".

Drawer contains: full 4-section SafetyCard, 5-layer PolicyCard, scoped permissions list with approval counts, schedule + advanced (cron / fallback / retry), and the last-10 runs timeline.

