#!/usr/bin/env bash
# visual-capture.sh — re-capture baselines using the exact same Interceptor
# parameters that visual-check.sh uses for comparison. Run this once before
# running `make test-visual`, and any time a v2 design changes intentionally.
#
# Boots the preview server, walks every (or selected) route, and writes
# tests/visual/<route>.baseline.png. Overwrites existing baselines — review
# the git diff before committing.
#
# Usage:
#   bash scripts/visual-capture.sh                 # all routes
#   bash scripts/visual-capture.sh dashboard inbox # subset
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASELINE_DIR="${ROOT}/tests/visual"
PREVIEW_PORT="${PREVIEW_PORT:-4173}"
BASE_URL="http://localhost:${PREVIEW_PORT}"
STARTUP_WAIT="${STARTUP_WAIT:-6}"
SETTLE_WAIT="${SETTLE_WAIT:-2}"
SHOT_MAX_EDGE="${SHOT_MAX_EDGE:-1568}"

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
fi

cleanup() {
  local code=$?
  if [[ -n "${PREVIEW_PID:-}" ]] && kill -0 "$PREVIEW_PID" 2>/dev/null; then
    kill "$PREVIEW_PID" 2>/dev/null || true
    wait "$PREVIEW_PID" 2>/dev/null || true
  fi
  exit "$code"
}
trap cleanup EXIT INT TERM

echo "visual-capture · building apps/web"
(cd "${ROOT}/apps/web" && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock pnpm run build >/dev/null)

echo "visual-capture · starting preview on port ${PREVIEW_PORT}"
(cd "${ROOT}/apps/web" && VITE_ENABLE_V2_UI=true VITE_API_MODE=mock pnpm exec vite preview --port "${PREVIEW_PORT}" --strictPort >/dev/null 2>&1) &
PREVIEW_PID=$!
sleep "$STARTUP_WAIT"

if ! curl -fsS "${BASE_URL}/" >/dev/null; then
  echo "visual-capture · preview server not responding at ${BASE_URL}" >&2
  exit 1
fi

for entry in "${ROUTES[@]}"; do
  slug="${entry%%|*}"
  path="${entry##*|}"
  url="${BASE_URL}${path}"
  target="${BASELINE_DIR}/${slug}.baseline.png"

  echo "visual-capture · ${slug} · opening ${url}"
  interceptor open "$url" --no-wait >/dev/null 2>&1
  sleep "$SETTLE_WAIT"

  shot_result=$(interceptor --json screenshot --save --format png --target-max-long-edge "$SHOT_MAX_EDGE" 2>/dev/null || true)
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
    echo "visual-capture · ${slug} · FAIL (no screenshot path)"
    exit 1
  fi
  cp "$shot_src" "$target"
  rm -f "$shot_src"
  echo "visual-capture · ${slug} · wrote $target"
done

echo "visual-capture · done"
