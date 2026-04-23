#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly expected_created_code="201"
readonly expected_forbidden_code="403"
readonly gateway_name="admission-verify-gateway"
readonly reserved_gateway_name="reserved-metadata-gateway"
readonly reserved_annotation_key="internal.ingate.io/managed-by"
readonly reserved_error_snippet="reserved for system use"
readonly default_verify_port="19445"
readonly admin_token="${APISERVER_AUTH_TOKEN:-ingate-dev-admin-token}"
readonly auth_header="Authorization: Bearer ${admin_token}"

api_server_bin="${APISERVER_BIN:-}"
etcd_servers="${ETCD_SERVERS:-http://127.0.0.1:2379}"
host="${APISERVER_HOST:-127.0.0.1}"
port="${APISERVER_PORT:-${default_verify_port}}"
log_file="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver-admission.log}"
base_url="https://${host}:${port}"
health_url="${base_url}/healthz"

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

curl --noproxy '*' -ksS -X DELETE -H "${auth_header}" "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${auth_header}" "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways/${reserved_gateway_name}" >/dev/null 2>&1 || true

create_file="$(mktemp)"
reserved_file="$(mktemp)"
trap 'rm -f "${create_file}" "${reserved_file}"; cleanup' EXIT

create_code="$(curl --noproxy '*' -ksS -o "${create_file}" -w '%{http_code}' \
  -X POST "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways" \
  -H "${auth_header}" \
  -H 'Content-Type: application/json' \
  -d '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Gateway","metadata":{"name":"'"${gateway_name}"'"},"spec":{"listeners":[{"name":"web","protocol":"HTTP","port":80}]}}')"

if [[ "${create_code}" != "${expected_created_code}" ]]; then
  echo "expected normal create to return ${expected_created_code}, got ${create_code}" >&2
  sed -n '1,160p' "${create_file}" >&2
  exit 1
fi

reserved_code="$(curl --noproxy '*' -ksS -o "${reserved_file}" -w '%{http_code}' \
  -X POST "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways" \
  -H "${auth_header}" \
  -H 'Content-Type: application/json' \
  -d '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Gateway","metadata":{"name":"'"${reserved_gateway_name}"'","annotations":{"'"${reserved_annotation_key}"'":"controller"}},"spec":{"listeners":[{"name":"web","protocol":"HTTP","port":80}]}}')"

if [[ "${reserved_code}" != "${expected_forbidden_code}" ]]; then
  echo "expected reserved metadata create to return ${expected_forbidden_code}, got ${reserved_code}" >&2
  sed -n '1,160p' "${reserved_file}" >&2
  exit 1
fi

if ! grep -q "${reserved_error_snippet}" "${reserved_file}"; then
  echo 'expected reserved metadata rejection message in admission response' >&2
  sed -n '1,160p' "${reserved_file}" >&2
  exit 1
fi

printf 'NORMAL_CREATE_CODE=%s\n' "${create_code}"
printf 'RESERVED_METADATA_CODE=%s\n' "${reserved_code}"
