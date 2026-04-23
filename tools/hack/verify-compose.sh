#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

COMPOSE_FILE="${COMPOSE_FILE:-${ROOT_DIR}/deploy/compose/compose.yaml}"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-${ROOT_DIR}/deploy/compose/.env.example}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-ingate-verify}"
COMPOSE_PARALLEL_LIMIT="${COMPOSE_PARALLEL_LIMIT:-1}"
COMPOSE_SUBNET="${COMPOSE_SUBNET:-172.31.251.0/24}"
SAMPLE_BACKEND_ADDRESS="${SAMPLE_BACKEND_ADDRESS:-172.31.251.10}"
BACKEND_ENDPOINT_ADDRESS="${BACKEND_ENDPOINT_ADDRESS:-172.31.251.10}"
APISERVER_PORT="${APISERVER_PORT:-28443}"
ADMIN_API_PORT="${ADMIN_API_PORT:-28080}"
XDS_SERVER_GRPC_PORT="${XDS_SERVER_GRPC_PORT:-29090}"
CONSOLE_PORT="${CONSOLE_PORT:-28088}"
ENVOY_PROXY_PORT="${ENVOY_PROXY_PORT:-20080}"
ENVOY_ADMIN_PORT="${ENVOY_ADMIN_PORT:-29901}"
APISERVER_HEALTH_URL="${APISERVER_HEALTH_URL:-https://127.0.0.1:${APISERVER_PORT}/healthz}"
ADMIN_API_HEALTH_URL="${ADMIN_API_HEALTH_URL:-http://127.0.0.1:${ADMIN_API_PORT}/healthz}"
CONSOLE_HEALTH_URL="${CONSOLE_HEALTH_URL:-http://127.0.0.1:${CONSOLE_PORT}/}"
ENVOY_ADMIN_READY_URL="${ENVOY_ADMIN_READY_URL:-http://envoy:9901/ready}"
ENVOY_PROXY_URL="${ENVOY_PROXY_URL:-http://envoy/orders}"
GATEWAY_HOST="${GATEWAY_HOST:-api.example.com}"
EXPECTED_BODY="${EXPECTED_BODY:-sample-backend-ok}"

export COMPOSE_PROJECT_NAME
export COMPOSE_SUBNET
export SAMPLE_BACKEND_ADDRESS
export BACKEND_ENDPOINT_ADDRESS
export APISERVER_PORT
export ADMIN_API_PORT
export XDS_SERVER_GRPC_PORT
export CONSOLE_PORT
export ENVOY_PROXY_PORT
export ENVOY_ADMIN_PORT

wait_for_http_ready() {
  local url="$1"
  local attempts="${2:-90}"
  local delay_seconds="${3:-2}"

  for _attempt in $(seq 1 "${attempts}"); do
    if curl --noproxy '*' -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay_seconds}"
  done

  return 1
}

wait_for_compose_http_ready() {
  local url="$1"
  local attempts="${2:-90}"
  local delay_seconds="${3:-2}"

  for _attempt in $(seq 1 "${attempts}"); do
    if docker run --rm --network "${COMPOSE_PROJECT_NAME}-network" curlimages/curl:8.16.0 -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay_seconds}"
  done

  return 1
}

wait_for_proxy_response() {
  local attempts="${1:-90}"
  local delay_seconds="${2:-2}"
  local body

  for _attempt in $(seq 1 "${attempts}"); do
    body="$(
      docker run --rm --network "${COMPOSE_PROJECT_NAME}-network" curlimages/curl:8.16.0 \
        -fsS -H "Host: ${GATEWAY_HOST}" "${ENVOY_PROXY_URL}" 2>/dev/null || true
    )"
    if [[ "${body}" == *"${EXPECTED_BODY}"* ]]; then
      printf '%s' "${body}"
      return 0
    fi
    sleep "${delay_seconds}"
  done

  return 1
}

compose_cmd() {
  COMPOSE_PARALLEL_LIMIT="${COMPOSE_PARALLEL_LIMIT}" docker compose \
    -f "${COMPOSE_FILE}" \
    --env-file "${COMPOSE_ENV_FILE}" \
    --project-name "${COMPOSE_PROJECT_NAME}" \
    "$@"
}

cleanup() {
  compose_cmd down -v --remove-orphans >/dev/null 2>&1 || true
}

dump_logs() {
  printf '\n--- compose ps ---\n' >&2
  compose_cmd ps >&2 || true
  printf '\n--- compose logs ---\n' >&2
  compose_cmd logs --no-color --tail=200 >&2 || true
}

trap 'status=$?; if [[ ${status} -ne 0 ]]; then dump_logs; fi; cleanup; exit ${status}' EXIT

cleanup
compose_cmd config >/dev/null
"${ROOT_DIR}/tools/hack/build-compose-images.sh"
compose_cmd up -d --no-build

if ! ingate::hack::wait_for_https_ready "${APISERVER_HEALTH_URL}" 90 2; then
  echo "apiserver healthz never became ready" >&2
  exit 1
fi
echo "COMPOSE_APISERVER_HEALTHZ=ok"

if ! wait_for_http_ready "${ADMIN_API_HEALTH_URL}" 90 2; then
  echo "admin-api healthz never became ready" >&2
  exit 1
fi
echo "COMPOSE_ADMIN_API_HEALTHZ=ok"

if ! wait_for_http_ready "${CONSOLE_HEALTH_URL}" 90 2; then
  echo "console never became ready" >&2
  exit 1
fi
echo "COMPOSE_CONSOLE_READY=yes"

if ! wait_for_compose_http_ready "${ENVOY_ADMIN_READY_URL}" 90 2; then
  echo "envoy admin never became ready" >&2
  exit 1
fi
echo "COMPOSE_ENVOY_READY=yes"

response_body="$(wait_for_proxy_response 90 2)" || {
  echo "envoy never returned the expected backend response" >&2
  exit 1
}
echo "COMPOSE_PROXY_VERIFY=yes"
echo "COMPOSE_PROXY_BODY=${response_body}"
