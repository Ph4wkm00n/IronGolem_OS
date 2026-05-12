#!/usr/bin/env bash
# smoke-e2e.sh — Step 8 Gate 3 of the v0.1 plan.
#
# Boots the actual gateway against the actual runtimed binary with the
# mock LLM provider, mints an HMAC bearer token, posts an inbound
# message, and asserts:
#
#   1. POST /api/v1/messages/inbound returns 200 and a body containing
#      the mock provider's reply ("pong" by default).
#   2. GET /api/v1/events surfaces a `message.inbound` event for the
#      tenant — proof that the audit trail wrote through to SQLite.
#
# Exits 0 only when both pass. Logs the gateway's stderr on failure so
# operators can read the policy / runtime / db trail.
#
# Telegram outbound impersonation (the prose version of Gate 3) is
# deferred until a connector instance is wired into main.go — that work
# is `Plans/create-a-plan-to-glowing-nest.md` v0.2 territory. The
# architectural seam (`MessageInbound` → planner → runtime → reply +
# audit event) is what Gate 3 ultimately validates, and that path is
# exercised end-to-end here.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GATEWAY_PORT="${GATEWAY_PORT:-18099}"
GATEWAY_URL="http://localhost:${GATEWAY_PORT}"
WORK_DIR="$(mktemp -d -t irongolem-smoke-XXXXXX)"
SECRET="${IRONGOLEM_HMAC_SECRET:-smoke-secret-$(date +%s)}"
EXPECTED_REPLY="${IRONGOLEM_LLM_MOCK_RESPONSE:-pong}"

cleanup() {
  local exit_code=$?
  if [[ -n "${GATEWAY_PID:-}" ]] && kill -0 "${GATEWAY_PID}" 2>/dev/null; then
    kill -TERM "${GATEWAY_PID}" 2>/dev/null || true
    wait "${GATEWAY_PID}" 2>/dev/null || true
  fi
  if [[ $exit_code -ne 0 && -f "${WORK_DIR}/gateway.log" ]]; then
    echo "----- gateway.log (tail 80) -----" >&2
    tail -n 80 "${WORK_DIR}/gateway.log" >&2 || true
    echo "---------------------------------" >&2
  fi
  trash "${WORK_DIR}" 2>/dev/null || true
  exit $exit_code
}
trap cleanup EXIT INT TERM

echo "smoke-e2e · work dir ${WORK_DIR}"

echo "smoke-e2e · building runtimed (cargo)"
( cd "${ROOT}" && cargo build -p irongolem-runtimed --quiet )

echo "smoke-e2e · building gateway + mint-token (go)"
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/gateway" ./gateway/cmd )
( cd "${ROOT}/services" && go build -o "${WORK_DIR}/mint-token" ./gateway/cmd/mint-token )

echo "smoke-e2e · starting gateway on port ${GATEWAY_PORT}"
IRONGOLEM_HMAC_SECRET="${SECRET}" \
  IRONGOLEM_GATEWAY_DB="${WORK_DIR}/gateway.db" \
  IRONGOLEM_RUNTIMED_PATH="${ROOT}/target/debug/runtimed" \
  IRONGOLEM_LLM_PROVIDER=mock \
  IRONGOLEM_LLM_MOCK_RESPONSE="${EXPECTED_REPLY}" \
  GATEWAY_ADDR=":${GATEWAY_PORT}" \
  DEPLOYMENT_MODE=solo \
  "${WORK_DIR}/gateway" >"${WORK_DIR}/gateway.log" 2>&1 &
GATEWAY_PID=$!

# Wait up to ~10s for /healthz to come up.
for i in $(seq 1 20); do
  if curl -fsS "${GATEWAY_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "${GATEWAY_PID}" 2>/dev/null; then
    echo "smoke-e2e · gateway exited during startup" >&2
    exit 1
  fi
  sleep 0.5
done

if ! curl -fsS "${GATEWAY_URL}/healthz" >/dev/null 2>&1; then
  echo "smoke-e2e · gateway never became healthy" >&2
  exit 1
fi
echo "smoke-e2e · gateway healthy"

TOKEN=$(IRONGOLEM_HMAC_SECRET="${SECRET}" "${WORK_DIR}/mint-token" --tenant default --user smoke --role executor --channel smoke --ttl 5m)
if [[ -z "${TOKEN}" ]]; then
  echo "smoke-e2e · mint-token produced empty output" >&2
  exit 1
fi
echo "smoke-e2e · token minted (${#TOKEN} chars)"

echo "smoke-e2e · POST /api/v1/messages/inbound"
INBOUND_BODY=$(curl -sS -X POST "${GATEWAY_URL}/api/v1/messages/inbound" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<EOF
{
  "connector_id": "telegram-smoke",
  "tenant_id": "default",
  "channel_id": "chat-smoke",
  "user_id": "smoke",
  "content": "ping"
}
EOF
)
echo "smoke-e2e · inbound response: ${INBOUND_BODY}"

# Gateway requires the connector to be registered before MessageInbound
# will dispatch. Register it now and retry once.
case "${INBOUND_BODY}" in
  *"connector not found"*)
    echo "smoke-e2e · registering telegram-smoke connector"
    REG=$(curl -sS -X POST "${GATEWAY_URL}/api/v1/connectors/telegram-smoke/connect" \
      -H "Authorization: Bearer ${TOKEN}")
    echo "smoke-e2e · connect response: ${REG}"
    INBOUND_BODY=$(curl -sS -X POST "${GATEWAY_URL}/api/v1/messages/inbound" \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "Content-Type: application/json" \
      --data-binary @- <<EOF
{
  "connector_id": "telegram-smoke",
  "tenant_id": "default",
  "channel_id": "chat-smoke",
  "user_id": "smoke",
  "content": "ping"
}
EOF
)
    echo "smoke-e2e · inbound retry response: ${INBOUND_BODY}"
    ;;
esac

# Assert the reply contains the mock response (default "pong"). The body
# is a JSON object with "reply": "<text>" — grep is sufficient here, no
# need for a JSON parser dependency in the smoke script.
if ! echo "${INBOUND_BODY}" | grep -q "\"reply\":\"${EXPECTED_REPLY}\""; then
  echo "smoke-e2e · FAIL: reply did not contain ${EXPECTED_REPLY}" >&2
  echo "smoke-e2e · body=${INBOUND_BODY}" >&2
  exit 1
fi
echo "smoke-e2e · ✅ reply == '${EXPECTED_REPLY}'"

echo "smoke-e2e · GET /api/v1/events"
EVENTS=$(curl -sS "${GATEWAY_URL}/api/v1/events?page=1&page_size=10" \
  -H "Authorization: Bearer ${TOKEN}")
echo "smoke-e2e · events: ${EVENTS}" | head -c 400 ; echo
if ! echo "${EVENTS}" | grep -q '"message.inbound"'; then
  echo "smoke-e2e · FAIL: timeline missing message.inbound event" >&2
  exit 1
fi
echo "smoke-e2e · ✅ message.inbound event landed in timeline"

echo "smoke-e2e · ALL GATES PASSED"
