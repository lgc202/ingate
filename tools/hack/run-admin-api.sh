#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

ADMIN_API_BIN="${ADMIN_API_BIN:-}"
APISERVER_ADDRESS="${APISERVER_ADDRESS:-https://127.0.0.1:18443}"
APISERVER_TOKEN="${APISERVER_TOKEN:-ingate-dev-admin-token}"
ADMIN_API_TOKEN="${ADMIN_API_TOKEN:-ingate-dev-admin-api-token}"
ADMIN_API_BIND_ADDRESS="${ADMIN_API_BIND_ADDRESS:-127.0.0.1}"
ADMIN_API_PORT="${ADMIN_API_PORT:-18080}"
APISERVER_INSECURE_SKIP_TLS_VERIFY="${APISERVER_INSECURE_SKIP_TLS_VERIFY:-true}"

if [[ -z "${ADMIN_API_BIN}" ]]; then
  ADMIN_API_BIN="$(ingate::hack::build_dir)/ingate-admin-api"
fi

if [[ ! -x "${ADMIN_API_BIN}" ]]; then
  echo "admin-api binary not found: ${ADMIN_API_BIN}" >&2
  echo "run: make build-admin-api" >&2
  exit 1
fi

exec "${ADMIN_API_BIN}" \
  --bind-address="${ADMIN_API_BIND_ADDRESS}" \
  --port="${ADMIN_API_PORT}" \
  --apiserver-address="${APISERVER_ADDRESS}" \
  --apiserver-token="${APISERVER_TOKEN}" \
  --apiserver-insecure-skip-tls-verify="${APISERVER_INSECURE_SKIP_TLS_VERIFY}" \
  --admin-token="${ADMIN_API_TOKEN}" \
  "$@"
