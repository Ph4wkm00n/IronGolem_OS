#!/usr/bin/env bash
# visual-check.sh — re-screenshot every integrated v2 route via Interceptor
# and pixel-diff against the committed baseline in tests/visual/.
#
# Boots a Vite preview server in the background (mock mode, v2 UI enabled),
# walks every route in ROUTES below, captures with `interceptor screenshot
# --save`, pixel-diffs via scripts/png-diff.ts, and exits non-zero if any
# route exceeds the threshold.
#
# Usage:
#   bash scripts/visual-check.sh                # all routes, default threshold
#   bash scripts/visual-check.sh dashboard inbox # subset
#   THRESHOLD=0.02 bash scripts/visual-check.sh # tighter
#   PREVIEW_PORT=4173 ...
#
# Dependencies:
#   - interceptor CLI (PAI Interceptor skill)
#   - bun
#   - apps/web already builds (pnpm --filter @irongolem/web build)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASELINE_DIR="${ROOT}/tests/visual"
SHOT_DIR="${TMPDIR:-/tmp}/irongolem-visual-$$"
THRESHOLD="${THRESHOLD:-0.05}"
PER_CHANNEL="${PER_CHANNEL:-8}"
PREVIEW_PORT="${PREVIEW_PORT:-4173}"
BASE_URL="http://localhost:${PREVIEW_PORT}"
STARTUP_WAIT="${STARTUP_WAIT:-6}"
SETTLE_WAIT="${SETTLE_WAIT:-2}"

# slug → path
declare -a ROUTES=(
  "dashboard|/"
  "inbox|/inbox"
  "recipes|/recipes"
  "research|/research"
  "memory|/memory"
  "health|/health"
  "security|/security"
  "settings|/settings"
)

# Subset filter via positional args.
if [[ $# -gt 0 ]]; then
  declare -a FILTERED=()
  for want in "$@"; do
    for entry in "${ROUTES[@]}"; do
      slug="${entry%%|*}"
      if [[ "$slug" == "$want" ]]; then
        FILTERED+=("$entry")
      fi
    done
  done
  ROUTES=("${FILTERED[@]}")
  if [[ ${#ROUTES[@]} -eq 0 ]]; then
    echo "no routes matched filter: $*" >&2
    exit 2
  fi
fi

mkdir -p "$SHOT_DIR"
echo "visual-check · capturing into $SHOT_DIR"
echo "visual-check · threshold=$THRESHOLD per-channel=$PER_CHANNEL"

cleanup() {
  local exit_code=$?
  if [[ -n "${PREVIEW_PID:-}" ]] && kill -0 "$PREVIEW_PID" 2>/dev/null; then
    kill "$PREVIEW_PID" 2>/dev/null || true
    wait "$PREVIEW_PID" 2>/dev/null || true
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

# Build then serve preview. Each route must render under v2 + mock mode.
echo "visual-check · building apps/web"
(cd "${ROOT}/apps/web" && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock pnpm run build >/dev/null)

echo "visual-check · starting preview on port ${PREVIEW_PORT}"
(cd "${ROOT}/apps/web" && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock pnpm exec vite preview --port "${PREVIEW_PORT}" --strictPort >"$SHOT_DIR/preview.log" 2>&1) &
PREVIEW_PID=$!
sleep "$STARTUP_WAIT"

if ! curl -fsS "${BASE_URL}/" >/dev/null; then
  echo "visual-check · preview server not responding at ${BASE_URL}" >&2
  cat "$SHOT_DIR/preview.log" >&2 || true
  exit 1
fi

declare -a FAILURES=()

for entry in "${ROUTES[@]}"; do
  slug="${entry%%|*}"
  path="${entry##*|}"
  url="${BASE_URL}${path}"
  shot_path="${SHOT_DIR}/${slug}.png"
  baseline_path="${BASELINE_DIR}/${slug}.baseline.png"

  if [[ ! -f "$baseline_path" ]]; then
    echo "visual-check · ${slug} · SKIP (no baseline at $baseline_path)"
    continue
  fi

  echo "visual-check · ${slug} · opening ${url}"
  if ! interceptor open "$url" --no-wait >/dev/null 2>&1; then
    echo "visual-check · ${slug} · FAIL (interceptor open errored)"
    FAILURES+=("$slug")
    continue
  fi
  sleep "$SETTLE_WAIT"

  # `--target-max-long-edge 1568` keeps capture under the Interceptor extension's
  # 15s response budget at our route resolutions; without it dashboard times out.
  shot_result=$(interceptor --json screenshot --save --format png --target-max-long-edge "${SHOT_MAX_EDGE:-1568}" 2>/dev/null || true)
  # Interceptor --json prints a single JSON object on stdout. `--save` populates
  # `data.filePath`. We parse the whole stdin as JSON; the `try/catch` shields
  # against stray log lines from older Interceptor builds.
  shot_src=$(printf '%s' "$shot_result" | bun -e '
    const txt = await Bun.stdin.text();
    const start = txt.indexOf("{");
    const end = txt.lastIndexOf("}");
    if (start < 0 || end <= start) { process.exit(0); }
    try {
      const j = JSON.parse(txt.slice(start, end + 1));
      process.stdout.write(j?.data?.filePath ?? j?.result?.filePath ?? j?.filePath ?? "");
    } catch {
      /* no-op */
    }
  ' 2>/dev/null || true)
  if [[ -z "$shot_src" || ! -f "$shot_src" ]]; then
    echo "visual-check · ${slug} · FAIL (no screenshot path; payload: $shot_result)"
    FAILURES+=("$slug")
    continue
  fi
  cp "$shot_src" "$shot_path"
  rm -f "$shot_src"

  # Capture stdout and the real exit code; `|| true` would mask the failure.
  diff_summary=$(bun "${ROOT}/scripts/png-diff.ts" "$baseline_path" "$shot_path" --threshold "$THRESHOLD" --per-channel "$PER_CHANNEL" 2>&1) && diff_exit=0 || diff_exit=$?
  echo "visual-check · ${slug} · ${diff_summary}"
  if [[ $diff_exit -ne 0 ]]; then
    FAILURES+=("$slug")
  fi
done

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  echo "visual-check · FAILED routes: ${FAILURES[*]}"
  echo "visual-check · candidate screenshots saved under $SHOT_DIR"
  exit 1
fi

echo "visual-check · all routes within ${THRESHOLD} threshold"
rm -rf "$SHOT_DIR"
