#!/usr/bin/env bash
# smoke-signal.sh — v0.3 Step 8 of Plans/modular-puzzling-blum.md.
#
# Validates the Signal connector's signal-cli bridge by issuing a
# `signal-cli -u <account> send -m "..." <recipient>` invocation.
#
# Required env:
#   IRONGOLEM_SIGNAL_ACCOUNT    — your verified Signal phone number,
#                                 with country code (e.g. +15551234567)
#   IRONGOLEM_SIGNAL_RECIPIENT  — phone number to deliver to
#
# Optional env:
#   IRONGOLEM_SIGNAL_CLI_PATH   — override signal-cli binary location
#
# Exits 2 when env is unset OR when signal-cli isn't on PATH.
set -euo pipefail

if [[ -z "${IRONGOLEM_SIGNAL_ACCOUNT:-}" ]]; then
  echo "smoke-signal · IRONGOLEM_SIGNAL_ACCOUNT not set" >&2
  exit 2
fi
if [[ -z "${IRONGOLEM_SIGNAL_RECIPIENT:-}" ]]; then
  echo "smoke-signal · IRONGOLEM_SIGNAL_RECIPIENT not set" >&2
  exit 2
fi

CLI="${IRONGOLEM_SIGNAL_CLI_PATH:-signal-cli}"
if ! command -v "${CLI}" >/dev/null 2>&1; then
  echo "smoke-signal · ${CLI} not found in PATH; install via brew or apt" >&2
  exit 2
fi

TS=$(date -u +%s)
TEXT="smoke-signal probe @ ${TS}"
echo "smoke-signal · ${CLI} -u ${IRONGOLEM_SIGNAL_ACCOUNT} send"
if ! "${CLI}" -u "${IRONGOLEM_SIGNAL_ACCOUNT}" send -m "${TEXT}" "${IRONGOLEM_SIGNAL_RECIPIENT}"; then
  echo "smoke-signal · FAIL: signal-cli rejected the send" >&2
  exit 1
fi
echo "smoke-signal · ✅ message delivered: ${TEXT}"
