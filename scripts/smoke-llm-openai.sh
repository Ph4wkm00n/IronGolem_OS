#!/usr/bin/env bash
# smoke-llm-openai.sh — v0.3 Step 3 of Plans/modular-puzzling-blum.md.
#
# Sibling of `smoke-llm.sh`. Validates the OpenAI provider end-to-end
# without breaking the default mock path. Boots the gateway with
# IRONGOLEM_LLM_PROVIDER=openai, drives a single inbound message
# through, and asserts the reply is real:
#
#   1. non-empty
#   2. NOT the literal mock default ("pong")
#   3. longer than 5 chars (rules out trivial canned responses)
#
# Cost control: pins gpt-4o-mini and uses the profile-default
# max_tokens (1024). One prompt per run → estimated < $0.001.
#
# Required env:
#   OPENAI_API_KEY   — OpenAI API credential (workflow secret)
#
# Optional env:
#   IRONGOLEM_LLM_MODEL — override the pinned model
#   SMOKE_PORT          — gateway port (default 18102; offset from
#                         smoke-llm.sh so both can run side-by-side)
#   SMOKE_TIMEOUT       — wall-clock cap (default 60s)
#
# Exits 2 with a clear message when OPENAI_API_KEY is unset so the
# CI workflow can skip cleanly on forks / first runs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${SMOKE_PORT:-18102}"
TIMEOUT="${SMOKE_TIMEOUT:-60s}"
MODEL="${IRONGOLEM_LLM_MODEL:-gpt-4o-mini}"

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "smoke-llm-openai · OPENAI_API_KEY not set; nothing to validate" >&2
  echo "smoke-llm-openai · (this is the expected exit on forks / first cron runs)" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d -t irongolem-smoke-llm-openai-XXXXXX)"

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

echo "smoke-llm-openai · building runtimed (cargo, release for CI speed)"
( cd "${ROOT}" && cargo build -p irongolem-runtimed --quiet )

echo "smoke-llm-openai · building gateway + mint-token (go)"
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/gateway" ./gateway/cmd )
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/mint-token" ./gateway/cmd/mint-token )

HMAC_SECRET="smoke-llm-openai-$(date +%s)-${RANDOM}"

echo "smoke-llm-openai · starting gateway with openai provider (model: ${MODEL})"
IRONGOLEM_HMAC_SECRET="${HMAC_SECRET}" \
  IRONGOLEM_GATEWAY_DB="${WORK_DIR}/gateway.db" \
  IRONGOLEM_RUNTIMED_PATH="${ROOT}/target/debug/runtimed" \
  IRONGOLEM_LLM_PROVIDER=openai \
  OPENAI_API_KEY="${OPENAI_API_KEY}" \
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
    echo "smoke-llm-openai · gateway exited during startup" >&2
    exit 1
  fi
  sleep 0.5
done
if ! curl -fsS "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
  echo "smoke-llm-openai · gateway never became healthy" >&2
  exit 1
fi
echo "smoke-llm-openai · gateway healthy"

TOKEN=$(IRONGOLEM_HMAC_SECRET="${HMAC_SECRET}" "${WORK_DIR}/mint-token" \
  --tenant default --user openai-smoke --role executor --channel openai-smoke --ttl 5m)
if [[ -z "${TOKEN}" ]]; then
  echo "smoke-llm-openai · mint-token produced empty output" >&2
  exit 1
fi

# Confirm /api/v1/providers reports openai as active before issuing the inbound.
# This is the v0.3 Step 3 contract: a successful boot exposes the active
# profile via the new IPC verb, not just via env-var inspection.
PROVIDERS=$(curl -fsS "http://localhost:${PORT}/api/v1/providers" \
  -H "Authorization: Bearer ${TOKEN}" || true)
echo "smoke-llm-openai · /api/v1/providers: ${PROVIDERS:0:240}"
if ! echo "${PROVIDERS}" | grep -q '"active":"openai"'; then
  echo "smoke-llm-openai · FAIL: /api/v1/providers did not report openai active" >&2
  exit 1
fi

# Register the connector first so MessageInbound dispatches.
curl -fsS -X POST "http://localhost:${PORT}/api/v1/connectors/openai-smoke/connect" \
  -H "Authorization: Bearer ${TOKEN}" >/dev/null

echo "smoke-llm-openai · POST /api/v1/messages/inbound"
BODY=$(curl -sS -X POST "http://localhost:${PORT}/api/v1/messages/inbound" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --max-time 30 \
  --data-binary @- <<'EOF'
{
  "connector_id": "openai-smoke",
  "channel_id": "openai-smoke",
  "user_id": "openai-smoke",
  "content": "Reply with the single word: galvanize"
}
EOF
)
echo "smoke-llm-openai · response: ${BODY}"

REPLY=$(printf '%s' "${BODY}" | awk -F'"reply":"' '{print $2}' | awk -F'"' '{print $1}')

if [[ -z "${REPLY}" ]]; then
  echo "smoke-llm-openai · FAIL: reply field empty (response: ${BODY})" >&2
  exit 1
fi
if [[ "${REPLY}" == "pong" ]]; then
  echo "smoke-llm-openai · FAIL: reply is the mock default 'pong' — provider didn't engage" >&2
  exit 1
fi
if [[ ${#REPLY} -lt 5 ]]; then
  echo "smoke-llm-openai · FAIL: reply suspiciously short (${#REPLY} chars): ${REPLY}" >&2
  exit 1
fi

echo "smoke-llm-openai · ✅ OpenAI provider returned a real reply (${#REPLY} chars)"
echo "smoke-llm-openai · reply preview: ${REPLY:0:120}"
