#!/usr/bin/env bash
# smoke-slack.sh — v0.3 Step 8 of Plans/modular-puzzling-blum.md.
#
# Validates the Slack connector's Connect → auth.test → Send round-trip
# without touching a real workspace. Mirrors the smoke-telegram pattern.
#
# Required env:
#   IRONGOLEM_SLACK_BOT_TOKEN     — bot OAuth token (xoxb-...)
#   IRONGOLEM_SLACK_TEST_CHANNEL  — channel ID to send to (e.g. C0123456789)
#
# Optional env:
#   IRONGOLEM_SLACK_SIGNING_SECRET — Slack signing secret (used by v0.4
#                                    Events API receiver; ignored here)
#
# Exits 2 with a clear message when the bot token is unset so CI can
# skip cleanly on forks / first runs.
set -euo pipefail

if [[ -z "${IRONGOLEM_SLACK_BOT_TOKEN:-}" ]]; then
  echo "smoke-slack · IRONGOLEM_SLACK_BOT_TOKEN not set; nothing to validate" >&2
  exit 2
fi
if [[ -z "${IRONGOLEM_SLACK_TEST_CHANNEL:-}" ]]; then
  echo "smoke-slack · IRONGOLEM_SLACK_TEST_CHANNEL not set; nothing to validate" >&2
  exit 2
fi

# 1. auth.test — confirms the token is valid + identifies the bot.
echo "smoke-slack · POST /api/auth.test"
AUTH=$(curl -sS -X GET "https://slack.com/api/auth.test" \
  -H "Authorization: Bearer ${IRONGOLEM_SLACK_BOT_TOKEN}")
if ! echo "${AUTH}" | grep -q '"ok":true'; then
  echo "smoke-slack · FAIL: auth.test rejected token: ${AUTH}" >&2
  exit 1
fi
echo "smoke-slack · auth.test ok"

# 2. chat.postMessage — actually send. Cheap call, but it lands in the
# test channel; arrange whatever cleanup your test workspace requires.
TS=$(date -u +%s)
TEXT="smoke-slack probe @ ${TS}"
echo "smoke-slack · POST /api/chat.postMessage to ${IRONGOLEM_SLACK_TEST_CHANNEL}"
RESP=$(curl -sS -X POST "https://slack.com/api/chat.postMessage" \
  -H "Authorization: Bearer ${IRONGOLEM_SLACK_BOT_TOKEN}" \
  -H "Content-Type: application/json; charset=utf-8" \
  -d "{\"channel\":\"${IRONGOLEM_SLACK_TEST_CHANNEL}\",\"text\":\"${TEXT}\"}")
if ! echo "${RESP}" | grep -q '"ok":true'; then
  echo "smoke-slack · FAIL: chat.postMessage rejected: ${RESP}" >&2
  exit 1
fi
echo "smoke-slack · ✅ message delivered: ${TEXT}"
