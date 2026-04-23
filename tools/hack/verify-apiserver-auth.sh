#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly expected_forbidden_code="403"
readonly expected_created_code="201"
readonly expected_ok_code="200"
readonly gateway_name="auth-verify-gateway"
readonly viewer_create_gateway_name="auth-verify-viewer-create-gateway"
readonly public_probe_bytes="40"
readonly default_verify_port="19444"

api_server_bin="${APISERVER_BIN:-}"
etcd_servers="${ETCD_SERVERS:-http://127.0.0.1:2379}"
host="${APISERVER_HOST:-127.0.0.1}"
port="${APISERVER_PORT:-${default_verify_port}}"
log_file="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver-auth.log}"
base_url="https://${host}:${port}"
health_url="${base_url}/healthz"
admin_token="${APISERVER_AUTH_ADMIN_TOKEN:-ingate-dev-admin-token}"
viewer_token="${APISERVER_AUTH_VIEWER_TOKEN:-ingate-dev-viewer-token}"
admin_auth_header="Authorization: Bearer ${admin_token}"
viewer_auth_header="Authorization: Bearer ${viewer_token}"

if [[ -z "${api_server_bin}" ]]; then
  api_server_bin="$(ingate::hack::build_dir)/ingate-apiserver"
fi

if [[ ! -x "${api_server_bin}" ]]; then
  echo "apiserver binary not found: ${api_server_bin}" >&2
  echo "run: make build-apiserver" >&2
  exit 1
fi

cleanup() {
  kill "${pid}" >/dev/null 2>&1 || true
  wait "${pid}" 2>/dev/null || true
}

mkdir -p "$(dirname "${log_file}")"
"${api_server_bin}" --etcd-servers="${etcd_servers}" --bind-address="${host}" --secure-port="${port}" >"${log_file}" 2>&1 &
pid=$!
trap cleanup EXIT

if ! ingate::hack::wait_for_https_ready "${health_url}" 30 1; then
  echo "apiserver did not become ready: ${health_url}" >&2
  sed -n '1,220p' "${log_file}" >&2 || true
  exit 1
fi

public_apis_full="$(curl --noproxy '*' -kfsS 2>/dev/null "${base_url}/apis")"
public_openapi_full="$(curl --noproxy '*' -kfsS 2>/dev/null "${base_url}/openapi/v2")"
printf 'PUBLIC_HEALTHZ=%s\n' "$(curl --noproxy '*' -kfsS 2>/dev/null "${base_url}/healthz")"
printf 'PUBLIC_APIS=%s\n' "${public_apis_full:0:80}"
printf 'PUBLIC_OPENAPI=%s\n' "${public_openapi_full:0:${public_probe_bytes}}"

curl --noproxy '*' -kfsS 2>/dev/null -X DELETE \
  -H "${admin_auth_header}" \
  "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -kfsS 2>/dev/null -X DELETE \
  -H "${admin_auth_header}" \
  "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways/${viewer_create_gateway_name}" >/dev/null 2>&1 || true

anon_create_file="$(mktemp)"
auth_create_file="$(mktemp)"
auth_get_file="$(mktemp)"
viewer_get_file="$(mktemp)"
viewer_create_file="$(mktemp)"
trap 'rm -f "${anon_create_file}" "${auth_create_file}" "${auth_get_file}" "${viewer_get_file}" "${viewer_create_file}"; cleanup' EXIT

anon_create_code="$(curl --noproxy '*' -ksS -o "${anon_create_file}" -w '%{http_code}' \
  -X POST "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways" \
  -H 'Content-Type: application/json' \
  -d '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Gateway","metadata":{"name":"'"${gateway_name}"'"},"spec":{"listeners":[{"name":"web","protocol":"HTTP","port":80}]}}')"

if [[ "${anon_create_code}" != "${expected_forbidden_code}" ]]; then
  echo "expected anonymous create to return ${expected_forbidden_code}, got ${anon_create_code}" >&2
  sed -n '1,160p' "${anon_create_file}" >&2
  exit 1
fi
printf 'ANON_CREATE_CODE=%s\n' "${anon_create_code}"

if ! grep -q 'Forbidden' "${anon_create_file}"; then
  echo 'expected anonymous create response to mention Forbidden' >&2
  sed -n '1,160p' "${anon_create_file}" >&2
  exit 1
fi

auth_create_code="$(curl --noproxy '*' -ksS -o "${auth_create_file}" -w '%{http_code}' \
  -X POST "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways" \
  -H "${admin_auth_header}" \
  -H 'Content-Type: application/json' \
  -d '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Gateway","metadata":{"name":"'"${gateway_name}"'"},"spec":{"listeners":[{"name":"web","protocol":"HTTP","port":80}]}}')"

if [[ "${auth_create_code}" != "${expected_created_code}" ]]; then
  echo "expected authenticated create to return ${expected_created_code}, got ${auth_create_code}" >&2
  sed -n '1,160p' "${auth_create_file}" >&2
  exit 1
fi
printf 'AUTH_CREATE_CODE=%s\n' "${auth_create_code}"

auth_get_code="$(curl --noproxy '*' -ksS -o "${auth_get_file}" -w '%{http_code}' \
  -H "${admin_auth_header}" \
  "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}")"

if [[ "${auth_get_code}" != "${expected_ok_code}" ]]; then
  echo "expected authenticated get to return ${expected_ok_code}, got ${auth_get_code}" >&2
  sed -n '1,160p' "${auth_get_file}" >&2
  exit 1
fi
printf 'AUTH_GET_CODE=%s\n' "${auth_get_code}"

viewer_get_code="$(curl --noproxy '*' -ksS -o "${viewer_get_file}" -w '%{http_code}' \
  -H "${viewer_auth_header}" \
  "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}")"

if [[ "${viewer_get_code}" != "${expected_ok_code}" ]]; then
  echo "expected viewer get to return ${expected_ok_code}, got ${viewer_get_code}" >&2
  sed -n '1,160p' "${viewer_get_file}" >&2
  exit 1
fi
printf 'VIEWER_GET_CODE=%s\n' "${viewer_get_code}"

viewer_create_code="$(curl --noproxy '*' -ksS -o "${viewer_create_file}" -w '%{http_code}' \
  -X POST "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways" \
  -H "${viewer_auth_header}" \
  -H 'Content-Type: application/json' \
  -d '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Gateway","metadata":{"name":"'"${viewer_create_gateway_name}"'"},"spec":{"listeners":[{"name":"web","protocol":"HTTP","port":80}]}}')"

if [[ "${viewer_create_code}" != "${expected_forbidden_code}" ]]; then
  echo "expected viewer create to return ${expected_forbidden_code}, got ${viewer_create_code}" >&2
  sed -n '1,160p' "${viewer_create_file}" >&2
  exit 1
fi
printf 'VIEWER_CREATE_CODE=%s\n' "${viewer_create_code}"

if ! grep -q 'Forbidden' "${viewer_create_file}"; then
  echo 'expected viewer create response to mention Forbidden' >&2
  sed -n '1,160p' "${viewer_create_file}" >&2
  exit 1
fi
