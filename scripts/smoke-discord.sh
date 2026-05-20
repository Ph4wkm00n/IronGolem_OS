#!/usr/bin/env bash
# smoke-discord.sh — v0.3 Step 8 of Plans/modular-puzzling-blum.md.
#
# Validates the Discord connector's token → channels/{id}/messages
# round-trip. Mirrors smoke-slack.sh.
#
# Required env:
#   IRONGOLEM_DISCORD_BOT_TOKEN     — bot token from the Discord
#                                     Developer Portal
#   IRONGOLEM_DISCORD_TEST_CHANNEL  — channel ID to send to
#
# Exits 2 with a clear message when either env var is unset.
set -euo pipefail

if [[ -z "${IRONGOLEM_DISCORD_BOT_TOKEN:-}" ]]; then
  echo "smoke-discord · IRONGOLEM_DISCORD_BOT_TOKEN not set" >&2
  exit 2
fi
if [[ -z "${IRONGOLEM_DISCORD_TEST_CHANNEL:-}" ]]; then
  echo "smoke-discord · IRONGOLEM_DISCORD_TEST_CHANNEL not set" >&2
  exit 2
fi

# 1. /users/@me — token validation.
echo "smoke-discord · GET /users/@me"
ME=$(curl -sS -H "Authorization: Bot ${IRONGOLEM_DISCORD_BOT_TOKEN}" \
  "https://discord.com/api/v10/users/@me")
if echo "${ME}" | grep -q '"code":0\|Unauthorized\|"code":40001'; then
  echo "smoke-discord · FAIL: /users/@me rejected: ${ME}" >&2
  exit 1
fi
echo "smoke-discord · token ok"

# 2. POST /channels/{id}/messages — actually send.
TS=$(date -u +%s)
TEXT="smoke-discord probe @ ${TS}"
echo "smoke-discord · POST /channels/${IRONGOLEM_DISCORD_TEST_CHANNEL}/messages"
RESP=$(curl -sS -X POST \
  "https://discord.com/api/v10/channels/${IRONGOLEM_DISCORD_TEST_CHANNEL}/messages" \
  -H "Authorization: Bot ${IRONGOLEM_DISCORD_BOT_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"content\":\"${TEXT}\"}")
if ! echo "${RESP}" | grep -q '"id":'; then
  echo "smoke-discord · FAIL: message create rejected: ${RESP}" >&2
  exit 1
fi
echo "smoke-discord · ✅ message delivered: ${TEXT}"
