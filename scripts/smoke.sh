#!/usr/bin/env bash
# Smoke-test a built apifox-api binary without Node/Bun.
set -euo pipefail
BINARY="${1:-./apifox-api}"
if [[ ! -x "${BINARY}" ]]; then
  echo "binary not executable: ${BINARY}" >&2
  exit 1
fi
"${BINARY}" --version
"${BINARY}" version
"${BINARY}" --help >/dev/null
echo "smoke ok: ${BINARY}"
