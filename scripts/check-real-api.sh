#!/usr/bin/env bash
# check-real-api.sh — ping each v2 route's gateway endpoint and report
# which are real-backed vs still mock-only. Useful before flipping a
# VITE_API_MODE_<ROUTE>=real flag.
#
# Reads $GATEWAY_URL (default http://localhost:8080) and probes
# /api/v1/v2/<route>. A 200 means the endpoint is wired; 404 / connection
# refused mean it's still mock-only.
#
# Usage:
#   GATEWAY_URL=http://localhost:8080 bash scripts/check-real-api.sh
#   bash scripts/check-real-api.sh inbox health      # subset
set -uo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"

declare -a ROUTES=(home inbox recipes research memory health security settings)

if [[ $# -gt 0 ]]; then
  ROUTES=("$@")
fi

printf '%-12s %-8s %s\n' "ROUTE" "STATUS" "URL"
printf '%-12s %-8s %s\n' "-----" "------" "---"

ready=0
total=0

for route in "${ROUTES[@]}"; do
  total=$((total + 1))
  url="${GATEWAY_URL}/api/v1/v2/${route}"
  # `-o /dev/null -w '%{http_code}'` returns just the status. On connect
  # failure curl writes "000" and exits non-zero; we keep the "000" and
  # swallow the exit so the script doesn't stop on a missing gateway.
  status=$(curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 2 "$url" 2>/dev/null) || true

  case "$status" in
    200|204)
      verdict="real"
      ready=$((ready + 1))
      ;;
    404)
      verdict="mock"
      ;;
    000|"")
      verdict="down"
      ;;
    *)
      verdict="$status"
      ;;
  esac
  printf '%-12s %-8s %s\n' "$route" "$verdict" "$url"
done

echo "---"
echo "real-backed: ${ready}/${total} (gateway: ${GATEWAY_URL})"

# Exit cleanly even when nothing is real yet — this script is a status probe,
# not a gate. Non-zero would let CI break the build for "missing backend",
# which is the wrong signal during the mock-first phase of v0.1.
exit 0
