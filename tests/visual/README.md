# Visual baselines for v2 routes

Each `.baseline.png` here is the canonical look of a v2 route in its mock-mode
default state. Each `.recipe.md` describes the exact state the screenshot was
captured in — `scripts/visual-check.sh` re-creates that state, captures a fresh
shot, and pixel-diffs against the baseline (5% threshold by default; tighten
per-route as design stabilizes).

## File naming

- `<route>.baseline.png` — canonical mock-mode shot.
- `<route>.after-approve.png` (optional) — post-interaction shot used for flows
  that need a second-state assertion.
- `<route>.recipe.md` — operator-readable script for reproducing the state.

The route slug matches `apps/web/src/pages/v2/<Route>.tsx`. `home` lives at
`/`; everything else at `/<slug>`.

## Updating a baseline

When a design change is intentional, regenerate with `scripts/visual-capture.sh`
so the new baseline uses the exact same Interceptor capture parameters that
`scripts/visual-check.sh` will compare against:

```
bash scripts/visual-capture.sh           # regenerate all
bash scripts/visual-capture.sh inbox     # regenerate one
```

Then review the git diff and commit the new baselines alongside the code change.
Capturing by hand with `interceptor screenshot` will produce sizes that the
checker rejects as "size-mismatch" because the default DPR + viewport doesn't
match what the script enforces (`--target-max-long-edge 1568`).

**First-time setup:** the baselines committed in this directory were captured
against Claude Design's HTML preview, not the production Vite build. Run
`scripts/visual-capture.sh` once before relying on `make test-visual` as a
regression gate.

## Threshold tuning

`scripts/visual-check.sh` defaults to a 5% pixel-diff threshold. Anti-aliasing
plus font hinting drifts by a few percent across machines, so don't tighten
below ~2% until we render in a deterministic CI environment (headless Chrome
in Docker).
