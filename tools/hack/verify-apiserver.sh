#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly openapi_probe_bytes="40"
readonly api_probe_bytes="200"
readonly default_verify_port="19443"

api_server_bin="${APISERVER_BIN:-}"
etcd_servers="${ETCD_SERVERS:-http://127.0.0.1:2379}"
host="${APISERVER_HOST:-127.0.0.1}"
port="${APISERVER_PORT:-${default_verify_port}}"
log_file="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver.log}"
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

healthz_response="$(curl --noproxy '*' -kfsS "${base_url}/healthz")"
readyz_response="$(curl --noproxy '*' -kfsS "${base_url}/readyz")"
apis_full_response="$(curl --noproxy '*' -kfsS "${base_url}/apis")"
apis_response="${apis_full_response:0:${api_probe_bytes}}"
gateway_discovery_response="$(curl --noproxy '*' -kfsS "${base_url}/apis/gateway.ingate.io/v1alpha1")"
policy_discovery_response="$(curl --noproxy '*' -kfsS "${base_url}/apis/policy.ingate.io/v1alpha1")"
openapi_v2_full_response="$(curl --noproxy '*' -kfsS "${base_url}/openapi/v2")"
openapi_v2_response="${openapi_v2_full_response:0:${openapi_probe_bytes}}"
openapi_v3_full_response="$(curl --noproxy '*' -kfsS "${base_url}/openapi/v3")"
openapi_v3_response="${openapi_v3_full_response:0:${openapi_probe_bytes}}"

if [[ "${gateway_discovery_response}" != *'"shortNames": ['* ]] || [[ "${gateway_discovery_response}" != *'"gw"'* ]] || [[ "${gateway_discovery_response}" != *'"categories": ['* ]] || [[ "${gateway_discovery_response}" != *'"ingate"'* ]]; then
  echo 'gateway discovery metadata is missing expected shortNames/categories' >&2
  printf '%s
' "${gateway_discovery_response}" >&2
  exit 1
fi

if [[ "${policy_discovery_response}" != *'"authp"'* ]] || [[ "${policy_discovery_response}" != *'"tp"'* ]] || [[ "${policy_discovery_response}" != *'"ingate"'* ]]; then
  echo 'policy discovery metadata is missing expected shortNames/categories' >&2
  printf '%s
' "${policy_discovery_response}" >&2
  exit 1
fi

echo "HEALTHZ=${healthz_response}"
echo "READYZ=${readyz_response}"
echo "APIS=${apis_response}"
echo "GATEWAY_DISCOVERY_OK=yes"
echo "POLICY_DISCOVERY_OK=yes"
echo "OPENAPI_V2=${openapi_v2_response}"
echo "OPENAPI_V3=${openapi_v3_response}"
