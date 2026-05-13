#!/usr/bin/env bash
# smoke-telegram.sh — Gate 4 of Plans/v0.2-foundation.md.
#
# Builds runtimed + gateway + the smoke-telegram harness, then invokes the
# harness which stands up an httptest Telegram impersonator, boots the
# gateway pointed at it, drives a fake update, and asserts that the gateway
# pushed `sendMessage` back to the impersonator with the right chat_id +
# text. End-to-end proof that the real Telegram connector is wired into
# the gateway's connector pump.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK_DIR="$(mktemp -d -t irongolem-smoke-telegram-XXXXXX)"
PORT="${SMOKE_PORT:-18100}"
TIMEOUT="${SMOKE_TIMEOUT:-30s}"

cleanup() {
  local code=$?
  trash "${WORK_DIR}" 2>/dev/null || true
  exit $code
}
trap cleanup EXIT INT TERM

echo "smoke-telegram · building runtimed (cargo)"
( cd "${ROOT}" && cargo build -p irongolem-runtimed --quiet )

echo "smoke-telegram · building gateway + harness (go)"
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/gateway" ./gateway/cmd )
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/smoke-telegram" ./gateway/cmd/smoke-telegram )

echo "smoke-telegram · running harness on port ${PORT} (timeout ${TIMEOUT})"
"${WORK_DIR}/smoke-telegram" \
  --gateway-bin="${WORK_DIR}/gateway" \
  --runtimed-bin="${ROOT}/target/debug/runtimed" \
  --port="${PORT}" \
  --timeout="${TIMEOUT}"
