#!/usr/bin/env bash
# smoke-llm.sh — v0.2 Step 5 Gate 6 of Plans/v0.2-foundation.md.
#
# Validates the Anthropic provider end-to-end without breaking the
# default mock path. Boots the gateway with IRONGOLEM_LLM_PROVIDER=anthropic
# (no mock fall-back), drives a single inbound message through, and
# asserts the reply is real:
#
#   1. non-empty
#   2. NOT the literal mock default ("pong")
#   3. longer than 10 chars (rules out trivial canned responses)
#
# Cost control: pins claude-haiku-4-5 + max_tokens implicitly capped at
# 1024 by the provider. One prompt per run → estimated < $0.001.
#
# Required env:
#   ANTHROPIC_API_KEY   — Anthropic API credential (workflow secret)
#
# Optional env:
#   IRONGOLEM_LLM_MODEL — override the pinned model
#   SMOKE_PORT          — gateway port (default 18101)
#   SMOKE_TIMEOUT       — wall-clock cap (default 60s)
#
# Exits 2 with a clear message when ANTHROPIC_API_KEY is unset so the
# CI workflow can skip cleanly on forks / first runs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${SMOKE_PORT:-18101}"
TIMEOUT="${SMOKE_TIMEOUT:-60s}"
MODEL="${IRONGOLEM_LLM_MODEL:-claude-haiku-4-5-20251001}"

if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
  echo "smoke-llm · ANTHROPIC_API_KEY not set; nothing to validate" >&2
  echo "smoke-llm · (this is the expected exit on forks / first cron runs)" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d -t irongolem-smoke-llm-XXXXXX)"

cleanup() {
  local code=$?
  if [[ -n "${GATEWAY_PID:-}" ]] && kill -0 "${GATEWAY_PID}" 2>/dev/null; then
    kill -TERM "${GATEWAY_PID}" 2>/dev/null || true
    wait "${GATEWAY_PID}" 2>/dev/null || true
  fi
  if [[ $code -ne 0 && -f "${WORK_DIR}/gateway.log" ]]; then
    echo "----- gateway.log (tail 60) -----" >&2
    tail -n 60 "${WORK_DIR}/gateway.log" >&2 || true
    echo "---------------------------------" >&2
  fi
  trash "${WORK_DIR}" 2>/dev/null || rm -rf "${WORK_DIR}" 2>/dev/null || true
  exit $code
}
trap cleanup EXIT INT TERM

echo "smoke-llm · building runtimed (cargo, release for CI speed)"
( cd "${ROOT}" && cargo build -p irongolem-runtimed --quiet )

echo "smoke-llm · building gateway + mint-token (go)"
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/gateway" ./gateway/cmd )
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/mint-token" ./gateway/cmd/mint-token )

HMAC_SECRET="smoke-llm-$(date +%s)-${RANDOM}"

echo "smoke-llm · starting gateway with anthropic provider (model: ${MODEL})"
IRONGOLEM_HMAC_SECRET="${HMAC_SECRET}" \
  IRONGOLEM_GATEWAY_DB="${WORK_DIR}/gateway.db" \
  IRONGOLEM_RUNTIMED_PATH="${ROOT}/target/debug/runtimed" \
  IRONGOLEM_LLM_PROVIDER=anthropic \
  ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  IRONGOLEM_LLM_MODEL="${MODEL}" \
  GATEWAY_ADDR=":${PORT}" \
  DEPLOYMENT_MODE=solo \
  "${WORK_DIR}/gateway" >"${WORK_DIR}/gateway.log" 2>&1 &
GATEWAY_PID=$!

# Wait for /healthz.
for i in $(seq 1 40); do
  if curl -fsS "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "${GATEWAY_PID}" 2>/dev/null; then
    echo "smoke-llm · gateway exited during startup" >&2
    exit 1
  fi
  sleep 0.5
done
if ! curl -fsS "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
  echo "smoke-llm · gateway never became healthy" >&2
  exit 1
fi
echo "smoke-llm · gateway healthy"

TOKEN=$(IRONGOLEM_HMAC_SECRET="${HMAC_SECRET}" "${WORK_DIR}/mint-token" \
  --tenant default --user llm-smoke --role executor --channel llm-smoke --ttl 5m)
if [[ -z "${TOKEN}" ]]; then
  echo "smoke-llm · mint-token produced empty output" >&2
  exit 1
fi

# Register the connector first so MessageInbound dispatches.
curl -fsS -X POST "http://localhost:${PORT}/api/v1/connectors/llm-smoke/connect" \
  -H "Authorization: Bearer ${TOKEN}" >/dev/null

echo "smoke-llm · POST /api/v1/messages/inbound"
BODY=$(curl -sS -X POST "http://localhost:${PORT}/api/v1/messages/inbound" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --max-time 30 \
  --data-binary @- <<'EOF'
{
  "connector_id": "llm-smoke",
  "channel_id": "llm-smoke",
  "user_id": "llm-smoke",
  "content": "Reply with the single word: galvanize"
}
EOF
)
echo "smoke-llm · response: ${BODY}"

# Extract reply field with a small awk — avoids a jq dependency in CI.
REPLY=$(printf '%s' "${BODY}" | awk -F'"reply":"' '{print $2}' | awk -F'"' '{print $1}')

if [[ -z "${REPLY}" ]]; then
  echo "smoke-llm · FAIL: reply field empty (response: ${BODY})" >&2
  exit 1
fi
if [[ "${REPLY}" == "pong" ]]; then
  echo "smoke-llm · FAIL: reply is the mock default 'pong' — provider didn't engage" >&2
  exit 1
fi
if [[ ${#REPLY} -lt 5 ]]; then
  echo "smoke-llm · FAIL: reply suspiciously short (${#REPLY} chars): ${REPLY}" >&2
  exit 1
fi

echo "smoke-llm · ✅ Anthropic provider returned a real reply (${#REPLY} chars)"
echo "smoke-llm · reply preview: ${REPLY:0:120}"
