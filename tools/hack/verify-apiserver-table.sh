#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly default_verify_port="19446"
readonly admin_token="${APISERVER_AUTH_TOKEN:-ingate-dev-admin-token}"
readonly auth_header="Authorization: Bearer ${admin_token}"
readonly table_accept_header='Accept: application/json;as=Table;g=meta.k8s.io;v=v1, application/json'
readonly gateway_name="table-verify-gateway"
readonly backend_name="table-verify-backend"
readonly route_name="table-verify-route"
readonly auth_policy_name="table-verify-authpolicy"
readonly traffic_policy_name="table-verify-trafficpolicy"

api_server_bin="${APISERVER_BIN:-}"
etcd_servers="${ETCD_SERVERS:-http://127.0.0.1:2379}"
host="${APISERVER_HOST:-127.0.0.1}"
port="${APISERVER_PORT:-${default_verify_port}}"
log_file="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver-table.log}"
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

create_or_replace() {
  local url="$1"
  local payload="$2"
  local response_file
  response_file="$(mktemp)"
  trap 'rm -f "${response_file}"; cleanup' EXIT
  local code
  code="$(curl --noproxy '*' -ksS -o "${response_file}" -w '%{http_code}' \
    -X POST "${url}" \
    -H "${auth_header}" \
    -H 'Content-Type: application/json' \
    --data "${payload}")"
  if [[ "${code}" != "201" && "${code}" != "409" ]]; then
    echo "expected create to return 201 or 409, got ${code}" >&2
    sed -n '1,200p' "${response_file}" >&2 || true
    exit 1
  fi
  rm -f "${response_file}"
  trap cleanup EXIT
}

assert_contains() {
  local value="$1"
  local expected="$2"
  local context="$3"
  if [[ "${value}" != *"${expected}"* ]]; then
    echo "expected ${context} to contain: ${expected}" >&2
    printf '%s\n' "${value}" >&2
    exit 1
  fi
}

fetch_table() {
  local url="$1"
  curl --noproxy '*' -kfsS "${url}" \
    -H "${auth_header}" \
    -H "${table_accept_header}"
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

curl --noproxy '*' -ksS -X DELETE -H "${auth_header}" "${base_url}/apis/gateway.ingate.io/v1alpha1/routes/${route_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${auth_header}" "${base_url}/apis/gateway.ingate.io/v1alpha1/backends/${backend_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${auth_header}" "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${auth_header}" "${base_url}/apis/policy.ingate.io/v1alpha1/authpolicies/${auth_policy_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${auth_header}" "${base_url}/apis/policy.ingate.io/v1alpha1/trafficpolicies/${traffic_policy_name}" >/dev/null 2>&1 || true

create_or_replace "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways" '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Gateway","metadata":{"name":"'"${gateway_name}"'"},"spec":{"listeners":[{"name":"web","protocol":"HTTP","port":80,"hostnames":["api.example.com","admin.example.com"]}]}}'
create_or_replace "${base_url}/apis/gateway.ingate.io/v1alpha1/backends" '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Backend","metadata":{"name":"'"${backend_name}"'"},"spec":{"type":"Static","defaultPort":8080,"static":{"endpoints":[{"address":"127.0.0.1","port":8080,"weight":100,"healthy":true}]}}}'
create_or_replace "${base_url}/apis/gateway.ingate.io/v1alpha1/routes" '{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Route","metadata":{"name":"'"${route_name}"'"},"spec":{"parentRefs":[{"name":"'"${gateway_name}"'"}],"hostnames":["api.example.com"],"rules":[{"matches":[{"path":{"type":"PathPrefix","value":"/orders"},"method":"GET"}],"backendRefs":[{"name":"'"${backend_name}"'","port":8080,"weight":100}]}]}}'
create_or_replace "${base_url}/apis/policy.ingate.io/v1alpha1/authpolicies" '{"apiVersion":"policy.ingate.io/v1alpha1","kind":"AuthPolicy","metadata":{"name":"'"${auth_policy_name}"'"},"spec":{"targetRefs":[{"kind":"Route","name":"'"${route_name}"'"}],"type":"APIKey","apiKey":{"fromHeaders":[{"name":"X-API-Key"}]}}}'
create_or_replace "${base_url}/apis/policy.ingate.io/v1alpha1/trafficpolicies" '{"apiVersion":"policy.ingate.io/v1alpha1","kind":"TrafficPolicy","metadata":{"name":"'"${traffic_policy_name}"'"},"spec":{"targetRefs":[{"kind":"Route","name":"'"${route_name}"'"}],"timeout":{"duration":"2s"}}}'

gateway_table="$(fetch_table "${base_url}/apis/gateway.ingate.io/v1alpha1/gateways")"
route_table="$(fetch_table "${base_url}/apis/gateway.ingate.io/v1alpha1/routes")"
backend_table="$(fetch_table "${base_url}/apis/gateway.ingate.io/v1alpha1/backends")"
auth_policy_table="$(fetch_table "${base_url}/apis/policy.ingate.io/v1alpha1/authpolicies")"
traffic_policy_table="$(fetch_table "${base_url}/apis/policy.ingate.io/v1alpha1/trafficpolicies")"

assert_contains "${gateway_table}" '"kind": "Table"' 'gateway table response'
assert_contains "${gateway_table}" '"Listeners"' 'gateway table columns'
assert_contains "${gateway_table}" '"Hostnames"' 'gateway table columns'
assert_contains "${gateway_table}" '"'"${gateway_name}"'"' 'gateway table rows'
assert_contains "${gateway_table}" 'api.example.com' 'gateway table hostnames'
assert_contains "${gateway_table}" 'admin.example.com' 'gateway table hostnames'

assert_contains "${route_table}" '"Parents"' 'route table columns'
assert_contains "${route_table}" '"Rules"' 'route table columns'
assert_contains "${route_table}" '"'"${route_name}"'"' 'route table rows'
assert_contains "${route_table}" '"api.example.com"' 'route table hostnames'

assert_contains "${backend_table}" '"Type"' 'backend table columns'
assert_contains "${backend_table}" '"Endpoints"' 'backend table columns'
assert_contains "${backend_table}" '"'"${backend_name}"'"' 'backend table rows'
assert_contains "${backend_table}" '"Static"' 'backend table type'

assert_contains "${auth_policy_table}" '"Targets"' 'authpolicy table columns'
assert_contains "${auth_policy_table}" '"Type"' 'authpolicy table columns'
assert_contains "${auth_policy_table}" '"'"${auth_policy_name}"'"' 'authpolicy table rows'
assert_contains "${auth_policy_table}" '"APIKey"' 'authpolicy table type'

assert_contains "${traffic_policy_table}" '"Timeout"' 'trafficpolicy table columns'
assert_contains "${traffic_policy_table}" '"RateLimit"' 'trafficpolicy table columns'
assert_contains "${traffic_policy_table}" '"'"${traffic_policy_name}"'"' 'trafficpolicy table rows'
assert_contains "${traffic_policy_table}" '"2s"' 'trafficpolicy timeout'

printf 'GATEWAY_TABLE_OK=yes\n'
printf 'ROUTE_TABLE_OK=yes\n'
printf 'BACKEND_TABLE_OK=yes\n'
printf 'AUTHPOLICY_TABLE_OK=yes\n'
printf 'TRAFFICPOLICY_TABLE_OK=yes\n'
