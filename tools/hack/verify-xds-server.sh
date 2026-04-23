#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly default_apiserver_port="19459"
readonly default_controller_manager_health_addr="127.0.0.1:18093"
readonly default_xds_health_addr="127.0.0.1:19091"
readonly default_xds_grpc_addr="127.0.0.1:19090"
readonly default_xds_grpc_bind_addr="127.0.0.1:19090"
readonly default_backend_mock_addr="127.0.0.1:18081"
readonly default_envoy_admin_addr="127.0.0.1:19901"
readonly default_envoy_proxy_addr="127.0.0.1:10080"
readonly gateway_name="xds-verify-gateway"
readonly backend_name="xds-verify-backend"
readonly route_name="xds-verify-route"
readonly timeout_route_name="xds-timeout-route"
readonly traffic_policy_name="xds-timeout-policy"
readonly retry_route_name="xds-retry-route"
readonly retry_traffic_policy_name="xds-retry-policy"
readonly ratelimit_route_name="xds-ratelimit-route"
readonly ratelimit_traffic_policy_name="xds-ratelimit-policy"
readonly apiserver_token="${APISERVER_TOKEN:-ingate-dev-admin-token}"
readonly apiserver_auth_header="Authorization: Bearer ${apiserver_token}"
readonly content_type_json_header="Content-Type: application/json"

APISERVER_BIN="${APISERVER_BIN:-$(ingate::hack::build_dir)/ingate-apiserver}"
CONTROLLER_MANAGER_BIN="${CONTROLLER_MANAGER_BIN:-$(ingate::hack::build_dir)/ingate-controller-manager}"
XDS_SERVER_BIN="${XDS_SERVER_BIN:-$(ingate::hack::build_dir)/ingate-xds-server}"
INGATECTL_BIN="${INGATECTL_BIN:-$(ingate::hack::build_dir)/ingatectl}"
ETCD_SERVERS="${ETCD_SERVERS:-http://127.0.0.1:2379}"
APISERVER_HOST="${APISERVER_HOST:-127.0.0.1}"
APISERVER_PORT="${APISERVER_PORT:-${default_apiserver_port}}"
APISERVER_ADDRESS="${APISERVER_ADDRESS:-https://${APISERVER_HOST}:${APISERVER_PORT}}"
APISERVER_LOG_FILE="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver-xds.log}"
CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS="${CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS:-${default_controller_manager_health_addr}}"
CONTROLLER_MANAGER_LOG_FILE="${CONTROLLER_MANAGER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-controller-manager-xds.log}"
XDS_SERVER_HEALTHZ_BIND_ADDRESS="${XDS_SERVER_HEALTHZ_BIND_ADDRESS:-${default_xds_health_addr}}"
XDS_SERVER_GRPC_BIND_ADDRESS="${XDS_SERVER_GRPC_BIND_ADDRESS:-${default_xds_grpc_bind_addr}}"
XDS_SERVER_CLIENT_ADDRESS="${XDS_SERVER_CLIENT_ADDRESS:-${default_xds_grpc_addr}}"
XDS_SERVER_DOCKER_ADDRESS="${XDS_SERVER_DOCKER_ADDRESS:-host.docker.internal:19090}"
XDS_SERVER_LOG_FILE="${XDS_SERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-xds-server.log}"
KUBECONFIG_FILE="${KUBECONFIG_FILE:-$(mktemp)}"
VERIFY_XDS_ENVOY="${VERIFY_XDS_ENVOY:-no}"
VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT="${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT:-no}"
VERIFY_XDS_TRAFFIC_POLICY_RETRY="${VERIFY_XDS_TRAFFIC_POLICY_RETRY:-no}"
VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT="${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT:-no}"
BACKEND_MOCK_ADDRESS="${BACKEND_MOCK_ADDRESS:-${default_backend_mock_addr}}"
BACKEND_ENDPOINT_ADDRESS="${BACKEND_ENDPOINT_ADDRESS:-${BACKEND_MOCK_ADDRESS}}"
BACKEND_PROBE_ADDRESS="${BACKEND_PROBE_ADDRESS:-127.0.0.1:${BACKEND_MOCK_ADDRESS##*:}}"
ENVOY_IMAGE="${ENVOY_IMAGE:-envoyproxy/envoy:v1.32.4}"
ENVOY_ADMIN_ADDRESS="${ENVOY_ADMIN_ADDRESS:-${default_envoy_admin_addr}}"
ENVOY_PROXY_ADDRESS="${ENVOY_PROXY_ADDRESS:-${default_envoy_proxy_addr}}"
ENVOY_LOG_FILE="${ENVOY_LOG_FILE:-$(ingate::hack::build_dir)/ingate-envoy.log}"
TRAFFIC_POLICY_TIMEOUT_DURATION="${TRAFFIC_POLICY_TIMEOUT_DURATION:-1s}"
BACKEND_DELAY_SECONDS="${BACKEND_DELAY_SECONDS:-2.5}"
TRAFFIC_POLICY_RETRY_ATTEMPTS="${TRAFFIC_POLICY_RETRY_ATTEMPTS:-2}"
BACKEND_RETRY_FAIL_COUNT="${BACKEND_RETRY_FAIL_COUNT:-2}"
TRAFFIC_POLICY_RATELIMIT_REQUESTS="${TRAFFIC_POLICY_RATELIMIT_REQUESTS:-1}"
TRAFFIC_POLICY_RATELIMIT_UNIT="${TRAFFIC_POLICY_RATELIMIT_UNIT:-minute}"

if [[ ! -x "${APISERVER_BIN}" ]]; then
  echo "apiserver binary not found: ${APISERVER_BIN}" >&2
  echo "run: make build-apiserver" >&2
  exit 1
fi

if [[ ! -x "${CONTROLLER_MANAGER_BIN}" ]]; then
  echo "controller-manager binary not found: ${CONTROLLER_MANAGER_BIN}" >&2
  echo "run: make build-controller-manager" >&2
  exit 1
fi

if [[ ! -x "${XDS_SERVER_BIN}" ]]; then
  echo "xds-server binary not found: ${XDS_SERVER_BIN}" >&2
  echo "run: make build-xds-server" >&2
  exit 1
fi

if [[ ! -x "${INGATECTL_BIN}" ]]; then
  echo "ingatectl binary not found: ${INGATECTL_BIN}" >&2
  echo "run: make build-ingatectl" >&2
  exit 1
fi

cleanup() {
  set +e
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/trafficpolicies/${ratelimit_traffic_policy_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/trafficpolicies/${retry_traffic_policy_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/trafficpolicies/${traffic_policy_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${ratelimit_route_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${retry_route_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${timeout_route_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${route_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/backends/${backend_name}" >/dev/null 2>&1 || true
  docker stop "${envoy_container_name:-}" >/dev/null 2>&1 || true
  wait "${envoy_pid:-}" 2>/dev/null || true
  kill "${backend_mock_pid:-}" >/dev/null 2>&1 || true
  wait "${backend_mock_pid:-}" 2>/dev/null || true
  kill "${xds_server_pid:-}" >/dev/null 2>&1 || true
  wait "${xds_server_pid:-}" 2>/dev/null || true
  kill "${controller_manager_pid:-}" >/dev/null 2>&1 || true
  wait "${controller_manager_pid:-}" 2>/dev/null || true
  kill "${apiserver_pid:-}" >/dev/null 2>&1 || true
  wait "${apiserver_pid:-}" 2>/dev/null || true
  rm -f \
    "${KUBECONFIG_FILE}" \
    "${response_file:-}" \
    "${resolve_output_file:-}" \
    "${resolve_text_output_file:-}" \
    "${config_output_file:-}" \
    "${config_text_output_file:-}" \
    "${list_output_file:-}" \
    "${list_text_output_file:-}" \
    "${summary_output_file:-}" \
    "${summary_text_output_file:-}" \
    "${check_output_file:-}" \
    "${check_text_output_file:-}" \
    "${ads_lds_output_file:-}" \
    "${ads_rds_output_file:-}" \
    "${ads_cds_output_file:-}" \
    "${ads_eds_output_file:-}" \
    "${timeout_ads_rds_output_file:-}" \
    "${retry_ads_rds_output_file:-}" \
    "${ratelimit_ads_rds_output_file:-}" \
    "${envoy_bootstrap_file:-}" \
    "${backend_mock_log_file:-}" \
    "${backend_mock_script:-}"
  rm -rf "${backend_mock_dir:-}"
}

assert_code() {
  local actual="$1"
  local expected="$2"
  local label="$3"
  local response_file_path="${4:-}"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "expected ${label} to return ${expected}, got ${actual}" >&2
    if [[ -n "${response_file_path}" && -f "${response_file_path}" ]]; then
      sed -n '1,220p' "${response_file_path}" >&2 || true
    fi
    sed -n '1,220p' "${APISERVER_LOG_FILE}" >&2 || true
    sed -n '1,220p' "${CONTROLLER_MANAGER_LOG_FILE}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
}

api_json_code() {
  local method="$1"
  local path="$2"
  local data="$3"
  local response_file_path="$4"
  curl --noproxy '*' -ksS -o "${response_file_path}" -w '%{http_code}' \
    -X "${method}" "${APISERVER_ADDRESS}${path}" \
    -H "${apiserver_auth_header}" \
    -H "${content_type_json_header}" \
    -d "${data}"
}

wait_for_http_ready() {
  local url="$1"
  local attempts="${2:-30}"
  local delay_seconds="${3:-1}"
  for _attempt in $(seq 1 "${attempts}"); do
    if curl --noproxy '*' -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay_seconds}"
  done
  return 1
}

wait_for_tcp_ready() {
  local host="$1"
  local port="$2"
  local attempts="${3:-30}"
  local delay_seconds="${4:-1}"
  for _attempt in $(seq 1 "${attempts}"); do
    if (echo >/dev/tcp/"${host}"/"${port}") >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay_seconds}"
  done
  return 1
}

wait_for_json_condition() {
  local url="$1"
  local jq_expr="$2"
  local attempts="${3:-60}"
  local delay_seconds="${4:-1}"
  local tmp_file
  tmp_file="$(mktemp)"
  for _attempt in $(seq 1 "${attempts}"); do
    if curl --noproxy '*' -ksS -H "${apiserver_auth_header}" "${url}" >"${tmp_file}" 2>/dev/null; then
      if jq -e "${jq_expr}" "${tmp_file}" >/dev/null 2>&1; then
        cat "${tmp_file}"
        rm -f "${tmp_file}"
        return 0
      fi
    fi
    sleep "${delay_seconds}"
  done
  sed -n '1,220p' "${tmp_file}" >&2 || true
  rm -f "${tmp_file}"
  return 1
}

wait_for_command_json_condition() {
  local output_file_path="$1"
  local jq_expr="$2"
  local attempts="${3:-30}"
  local delay_seconds="${4:-1}"
  shift 4

  for _attempt in $(seq 1 "${attempts}"); do
    if "$@" >"${output_file_path}" 2>/dev/null; then
      if jq -e "${jq_expr}" "${output_file_path}" >/dev/null 2>&1; then
        return 0
      fi
    fi
    sleep "${delay_seconds}"
  done

  sed -n '1,260p' "${output_file_path}" >&2 || true
  return 1
}

create_envoy_bootstrap() {
  local output_file_path="$1"
  local xds_address="$2"

  cat >"${output_file_path}" <<EOF
node:
  id: ${gateway_name}
  cluster: ingate-envoy-verify
dynamic_resources:
  ads_config:
    api_type: GRPC
    transport_api_version: V3
    grpc_services:
      - envoy_grpc:
          cluster_name: xds_cluster
  lds_config:
    ads: {}
    resource_api_version: V3
  cds_config:
    ads: {}
    resource_api_version: V3
static_resources:
  clusters:
    - name: xds_cluster
      type: STRICT_DNS
      connect_timeout: 2s
      typed_extension_protocol_options:
        envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
          "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
          explicit_http_config:
            http2_protocol_options: {}
      load_assignment:
        cluster_name: xds_cluster
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: ${xds_address%:*}
                      port_value: ${xds_address##*:}
admin:
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 9901
EOF
}

verify_envoy_response() {
  local proxy_url="$1"
  local admin_url="$2"
  local host_header="$3"
  local request_path="$4"
  local expected_status="$5"
  local expected_body="$6"
  local attempts="${7:-30}"
  local delay_seconds="${8:-1}"

  if ! wait_for_http_ready "${admin_url}/ready" "${attempts}" "${delay_seconds}"; then
    echo "envoy admin did not become ready: ${admin_url}/ready" >&2
    sed -n '1,220p' "${ENVOY_LOG_FILE}" >&2 || true
    return 1
  fi

  local response_file_path
  local headers_file_path
  local status_code
  response_file_path="$(mktemp)"
  headers_file_path="$(mktemp)"
  for _attempt in $(seq 1 "${attempts}"); do
    status_code="$(curl --noproxy '*' -sS -D "${headers_file_path}" -o "${response_file_path}" -w '%{http_code}' -H "Host: ${host_header}" "${proxy_url}${request_path}" 2>/dev/null || true)"
    if [[ "${status_code}" == "${expected_status}" ]]; then
      if [[ -z "${expected_body}" ]] || grep -Fq "${expected_body}" "${response_file_path}"; then
        rm -f "${response_file_path}"
        rm -f "${headers_file_path}"
        return 0
      fi
    fi
    sleep "${delay_seconds}"
  done

  echo "envoy proxy did not return expected response via ${proxy_url}${request_path}" >&2
  echo "envoy proxy status: ${status_code:-unknown}" >&2
  sed -n '1,80p' "${headers_file_path}" >&2 || true
  sed -n '1,220p' "${response_file_path}" >&2 || true
  curl --noproxy '*' -fsS "${admin_url}/clusters?format=json" | sed -n '1,220p' >&2 || true
  curl --noproxy '*' -fsS "${admin_url}/config_dump" | sed -n '1,220p' >&2 || true
  sed -n '1,220p' "${ENVOY_LOG_FILE}" >&2 || true
  rm -f "${response_file_path}"
  rm -f "${headers_file_path}"
  return 1
}

verify_backend_request_count() {
  local counter_url="$1"
  local expected_count="$2"
  local attempts="${3:-30}"
  local delay_seconds="${4:-1}"
  local actual_count=""

  for _attempt in $(seq 1 "${attempts}"); do
    actual_count="$(curl --noproxy '*' -fsS "${counter_url}" 2>/dev/null || true)"
    if [[ "${actual_count}" =~ ^[0-9]+$ ]] && [[ "${actual_count}" == "${expected_count}" ]]; then
      return 0
    fi
    sleep "${delay_seconds}"
  done

  echo "backend request count did not reach expected value via ${counter_url}" >&2
  echo "expected: ${expected_count}, actual: ${actual_count:-unknown}" >&2
  sed -n '1,220p' "${backend_mock_log_file}" >&2 || true
  return 1
}

mkdir -p "$(dirname "${APISERVER_LOG_FILE}")" "$(dirname "${CONTROLLER_MANAGER_LOG_FILE}")" "$(dirname "${XDS_SERVER_LOG_FILE}")"
trap cleanup EXIT

backend_bind_address="${BACKEND_MOCK_ADDRESS%:*}"
backend_bind_port="${BACKEND_MOCK_ADDRESS##*:}"
backend_endpoint_address="${BACKEND_ENDPOINT_ADDRESS%:*}"
backend_endpoint_port="${BACKEND_ENDPOINT_ADDRESS##*:}"

if [[ "${VERIFY_XDS_ENVOY}" == "yes" ]]; then
  mkdir -p "$(dirname "${ENVOY_LOG_FILE}")"
  backend_mock_dir="$(mktemp -d)"
  backend_mock_log_file="$(mktemp)"
  backend_mock_script="${backend_mock_dir}/server.py"
  cat >"${backend_mock_script}" <<'PY'
import http.server
import os
import socketserver
import time

HOST = os.environ["BACKEND_BIND_ADDRESS"]
PORT = int(os.environ["BACKEND_BIND_PORT"])
DELAY_PATH = os.environ.get("BACKEND_DELAY_PATH", "/slow-orders")
DELAY_SECONDS = float(os.environ.get("BACKEND_DELAY_SECONDS", "2.5"))
RETRY_PATH = os.environ.get("BACKEND_RETRY_PATH", "/retry-orders")
RETRY_COUNT_PATH = os.environ.get("BACKEND_RETRY_COUNT_PATH", "/retry-count")
RETRY_FAIL_COUNT = int(os.environ.get("BACKEND_RETRY_FAIL_COUNT", "2"))
LIMITED_PATH = os.environ.get("BACKEND_LIMITED_PATH", "/limited-orders")
LIMITED_COUNT_PATH = os.environ.get("BACKEND_LIMITED_COUNT_PATH", "/limited-count")
FAST_BODY = os.environ.get("BACKEND_FAST_BODY", "ingate-envoy-ok\n").encode()
SLOW_BODY = os.environ.get("BACKEND_SLOW_BODY", "upstream timeout body\n").encode()
RETRY_FAIL_BODY = os.environ.get("BACKEND_RETRY_FAIL_BODY", "retry attempt failed\n").encode()
RETRY_SUCCESS_BODY = os.environ.get("BACKEND_RETRY_SUCCESS_BODY", "retry success\n").encode()
LIMITED_BODY = os.environ.get("BACKEND_LIMITED_BODY", "limited upstream ok\n").encode()

retry_requests = {"count": 0}
limited_requests = {"count": 0}


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/orders":
            self._write_response(200, FAST_BODY)
            return
        if self.path == DELAY_PATH:
            time.sleep(DELAY_SECONDS)
            self._write_response(200, SLOW_BODY)
            return
        if self.path == RETRY_PATH:
            retry_requests["count"] += 1
            if retry_requests["count"] <= RETRY_FAIL_COUNT:
                self._write_response(503, RETRY_FAIL_BODY)
                return
            self._write_response(200, RETRY_SUCCESS_BODY)
            return
        if self.path == RETRY_COUNT_PATH:
            self._write_response(200, f"{retry_requests['count']}\n".encode())
            return
        if self.path == LIMITED_PATH:
            limited_requests["count"] += 1
            self._write_response(200, LIMITED_BODY)
            return
        if self.path == LIMITED_COUNT_PATH:
            self._write_response(200, f"{limited_requests['count']}\n".encode())
            return
        self._write_response(404, b"not found\n")

    def log_message(self, fmt, *args):
        return

    def _write_response(self, status, body):
        self.send_response(status)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


class ReuseTCPServer(socketserver.TCPServer):
    allow_reuse_address = True


with ReuseTCPServer((HOST, PORT), Handler) as server:
    server.serve_forever()
PY
  BACKEND_BIND_ADDRESS="${backend_bind_address}" \
  BACKEND_BIND_PORT="${backend_bind_port}" \
  BACKEND_DELAY_PATH="/slow-orders" \
  BACKEND_DELAY_SECONDS="${BACKEND_DELAY_SECONDS}" \
  BACKEND_RETRY_PATH="/retry-orders" \
  BACKEND_RETRY_COUNT_PATH="/retry-count" \
  BACKEND_RETRY_FAIL_COUNT="${BACKEND_RETRY_FAIL_COUNT}" \
  BACKEND_LIMITED_PATH="/limited-orders" \
  BACKEND_LIMITED_COUNT_PATH="/limited-count" \
  python3 "${backend_mock_script}" >"${backend_mock_log_file}" 2>&1 &
  backend_mock_pid=$!

  if ! wait_for_http_ready "http://${BACKEND_PROBE_ADDRESS}/orders" 30 1; then
    echo "mock backend did not become ready: http://${BACKEND_PROBE_ADDRESS}/orders" >&2
    sed -n '1,220p' "${backend_mock_log_file}" >&2 || true
    exit 1
  fi
fi

"${APISERVER_BIN}" \
  --etcd-servers="${ETCD_SERVERS}" \
  --bind-address="${APISERVER_HOST}" \
  --secure-port="${APISERVER_PORT}" \
  >"${APISERVER_LOG_FILE}" 2>&1 &
apiserver_pid=$!

if ! ingate::hack::wait_for_https_ready "${APISERVER_ADDRESS}/healthz" 30 1; then
  echo "apiserver did not become ready: ${APISERVER_ADDRESS}/healthz" >&2
  sed -n '1,220p' "${APISERVER_LOG_FILE}" >&2 || true
  exit 1
fi

APISERVER_HOST="${APISERVER_HOST}" APISERVER_PORT="${APISERVER_PORT}" KUBECONFIG_OUTPUT="${KUBECONFIG_FILE}" ./tools/hack/write-apiserver-kubeconfig.sh >/dev/null

"${CONTROLLER_MANAGER_BIN}" \
  --apiserver-address="${APISERVER_ADDRESS}" \
  --kubeconfig="${KUBECONFIG_FILE}" \
  --healthz-bind-address="${CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS}" \
  --metrics-bind-address="127.0.0.1:18094" \
  --workers=2 \
  >"${CONTROLLER_MANAGER_LOG_FILE}" 2>&1 &
controller_manager_pid=$!

if ! wait_for_http_ready "http://${CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS}/healthz" 30 1; then
  echo "controller-manager did not become ready: http://${CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS}/healthz" >&2
  sed -n '1,220p' "${CONTROLLER_MANAGER_LOG_FILE}" >&2 || true
  exit 1
fi

"${XDS_SERVER_BIN}" \
  --apiserver-address="${APISERVER_ADDRESS}" \
  --kubeconfig="${KUBECONFIG_FILE}" \
  --healthz-bind-address="${XDS_SERVER_HEALTHZ_BIND_ADDRESS}" \
  --grpc-bind-address="${XDS_SERVER_GRPC_BIND_ADDRESS}" \
  >"${XDS_SERVER_LOG_FILE}" 2>&1 &
xds_server_pid=$!

if ! wait_for_http_ready "http://${XDS_SERVER_HEALTHZ_BIND_ADDRESS}/healthz" 30 1; then
  echo "xds-server did not become ready: http://${XDS_SERVER_HEALTHZ_BIND_ADDRESS}/healthz" >&2
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

xds_host="${XDS_SERVER_CLIENT_ADDRESS%:*}"
xds_port="${XDS_SERVER_CLIENT_ADDRESS##*:}"
if ! wait_for_tcp_ready "${xds_host}" "${xds_port}" 30 1; then
  echo "xds-server gRPC port did not become ready: ${XDS_SERVER_CLIENT_ADDRESS}" >&2
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

response_file="$(mktemp)"
resolve_output_file="$(mktemp)"
resolve_text_output_file="$(mktemp)"
config_output_file="$(mktemp)"
config_text_output_file="$(mktemp)"
list_output_file="$(mktemp)"
list_text_output_file="$(mktemp)"
summary_output_file="$(mktemp)"
summary_text_output_file="$(mktemp)"
check_output_file="$(mktemp)"
check_text_output_file="$(mktemp)"
ads_lds_output_file="$(mktemp)"
ads_rds_output_file="$(mktemp)"
ads_cds_output_file="$(mktemp)"
ads_eds_output_file="$(mktemp)"
timeout_ads_rds_output_file="$(mktemp)"
retry_ads_rds_output_file="$(mktemp)"
ratelimit_ads_rds_output_file="$(mktemp)"

gateway_hosts=("api.example.com")
expected_route_count=1
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
  gateway_hosts+=("slow.example.com")
  expected_route_count=$((expected_route_count + 1))
fi
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
  gateway_hosts+=("retry.example.com")
  expected_route_count=$((expected_route_count + 1))
fi
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
  gateway_hosts+=("limited.example.com")
  expected_route_count=$((expected_route_count + 1))
fi
expected_retry_request_count=$((BACKEND_RETRY_FAIL_COUNT + 1))
gateway_hostnames="$(
  printf '%s\n' "${gateway_hosts[@]}" | jq -R . | jq -s .
)"

gateway_payload="$(jq -n \
  --arg name "${gateway_name}" \
  --argjson hostnames "${gateway_hostnames}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Gateway",metadata:{name:$name},spec:{listeners:[{name:"web",protocol:"HTTP",port:80,hostnames:$hostnames}]}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/gateways "${gateway_payload}" "${response_file}")" "201" "gateway create" "${response_file}"

backend_payload="$(jq -n \
  --arg name "${backend_name}" \
  --arg address "${backend_endpoint_address}" \
  --argjson port "${backend_endpoint_port}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Backend",metadata:{name:$name},spec:{type:"Static",protocol:"HTTP",defaultPort:$port,static:{endpoints:[{address:$address,port:$port,weight:100,healthy:true}]}}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/backends "${backend_payload}" "${response_file}")" "201" "backend create" "${response_file}"

route_payload="$(jq -n \
  --arg name "${route_name}" \
  --arg gateway_name "${gateway_name}" \
  --arg backend_name "${backend_name}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Route",metadata:{name:$name},spec:{parentRefs:[{name:$gateway_name}],hostnames:["api.example.com"],rules:[{matches:[{path:{type:"PathPrefix",value:"/orders"}}],backendRefs:[{name:$backend_name,port:8080,weight:100}]}]}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/routes "${route_payload}" "${response_file}")" "201" "route create" "${response_file}"

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
  timeout_route_payload="$(jq -n \
    --arg name "${timeout_route_name}" \
    --arg gateway_name "${gateway_name}" \
    --arg backend_name "${backend_name}" \
    '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Route",metadata:{name:$name},spec:{parentRefs:[{name:$gateway_name}],hostnames:["slow.example.com"],rules:[{matches:[{path:{type:"PathPrefix",value:"/slow-orders"}}],backendRefs:[{name:$backend_name,port:8080,weight:100}]}]}}')"
  assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/routes "${timeout_route_payload}" "${response_file}")" "201" "timeout route create" "${response_file}"

  traffic_policy_payload="$(jq -n \
    --arg name "${traffic_policy_name}" \
    --arg route_name "${timeout_route_name}" \
    --arg duration "${TRAFFIC_POLICY_TIMEOUT_DURATION}" \
    '{apiVersion:"policy.ingate.io/v1alpha1",kind:"TrafficPolicy",metadata:{name:$name},spec:{targetRefs:[{kind:"Route",name:$route_name}],timeout:{duration:$duration}}}')"
  assert_code "$(api_json_code POST /apis/policy.ingate.io/v1alpha1/trafficpolicies "${traffic_policy_payload}" "${response_file}")" "201" "trafficpolicy create" "${response_file}"
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
  retry_route_payload="$(jq -n \
    --arg name "${retry_route_name}" \
    --arg gateway_name "${gateway_name}" \
    --arg backend_name "${backend_name}" \
    '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Route",metadata:{name:$name},spec:{parentRefs:[{name:$gateway_name}],hostnames:["retry.example.com"],rules:[{matches:[{path:{type:"PathPrefix",value:"/retry-orders"}}],backendRefs:[{name:$backend_name,port:8080,weight:100}]}]}}')"
  assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/routes "${retry_route_payload}" "${response_file}")" "201" "retry route create" "${response_file}"

  retry_traffic_policy_payload="$(jq -n \
    --arg name "${retry_traffic_policy_name}" \
    --arg route_name "${retry_route_name}" \
    --argjson attempts "${TRAFFIC_POLICY_RETRY_ATTEMPTS}" \
    '{apiVersion:"policy.ingate.io/v1alpha1",kind:"TrafficPolicy",metadata:{name:$name},spec:{targetRefs:[{kind:"Route",name:$route_name}],retry:{attempts:$attempts,conditions:["5xx"]}}}')"
  assert_code "$(api_json_code POST /apis/policy.ingate.io/v1alpha1/trafficpolicies "${retry_traffic_policy_payload}" "${response_file}")" "201" "retry trafficpolicy create" "${response_file}"
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
  ratelimit_route_payload="$(jq -n \
    --arg name "${ratelimit_route_name}" \
    --arg gateway_name "${gateway_name}" \
    --arg backend_name "${backend_name}" \
    '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Route",metadata:{name:$name},spec:{parentRefs:[{name:$gateway_name}],hostnames:["limited.example.com"],rules:[{matches:[{path:{type:"PathPrefix",value:"/limited-orders"}}],backendRefs:[{name:$backend_name,port:8080,weight:100}]}]}}')"
  assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/routes "${ratelimit_route_payload}" "${response_file}")" "201" "ratelimit route create" "${response_file}"

  ratelimit_traffic_policy_payload="$(jq -n \
    --arg name "${ratelimit_traffic_policy_name}" \
    --arg route_name "${ratelimit_route_name}" \
    --argjson requests "${TRAFFIC_POLICY_RATELIMIT_REQUESTS}" \
    --arg unit "${TRAFFIC_POLICY_RATELIMIT_UNIT}" \
    '{apiVersion:"policy.ingate.io/v1alpha1",kind:"TrafficPolicy",metadata:{name:$name},spec:{targetRefs:[{kind:"Route",name:$route_name}],rateLimit:{requestsPerUnit:$requests,unit:$unit,scope:"route"}}}')"
  assert_code "$(api_json_code POST /apis/policy.ingate.io/v1alpha1/trafficpolicies "${ratelimit_traffic_policy_payload}" "${response_file}")" "201" "ratelimit trafficpolicy create" "${response_file}"
fi

resolvedgateway_url="${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/resolvedgateways/${gateway_name}"
resolvedgateway_condition='
  .spec.gatewayRef.name == "'"${gateway_name}"'" and
  ((.spec.routes // []) | any(.name == "'"${route_name}"'")) and
  ((.spec.backends // []) | any(.name == "'"${backend_name}"'")) and
  ((.status.conditions // []) | any(.type == "Accepted" and .status == "True")) and
  ((.status.conditions // []) | any(.type == "Resolved" and .status == "True")) and
  ((.status.conditions // []) | any(.type == "Programmed" and .status == "True"))
'
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
  resolvedgateway_condition='
    '"${resolvedgateway_condition}"' and
    ((.spec.routes // []) | any(.name == "'"${timeout_route_name}"'"))
  '
fi
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
  resolvedgateway_condition='
    '"${resolvedgateway_condition}"' and
    ((.spec.routes // []) | any(.name == "'"${retry_route_name}"'"))
  '
fi
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
  resolvedgateway_condition='
    '"${resolvedgateway_condition}"' and
    ((.spec.routes // []) | any(.name == "'"${ratelimit_route_name}"'"))
  '
fi
resolvedgateway_json="$(wait_for_json_condition "${resolvedgateway_url}" "${resolvedgateway_condition}")"

if ! wait_for_command_json_condition \
  "${resolve_output_file}" \
  '
  .backendName == "'"${backend_name}"'" and
  ((.endpoints // []) | length) == 1 and
  .endpoints[0].address == "'"${backend_endpoint_address}"'" and
  .endpoints[0].port == '"${backend_endpoint_port}"' and
  .endpoints[0].weight == 100 and
  .endpoints[0].healthy == true
' \
  30 \
  1 \
  "${INGATECTL_BIN}" xds resolve \
    --server="${XDS_SERVER_CLIENT_ADDRESS}" \
    --backend="${backend_name}" \
    --backend-type="Static"
then
  echo "discovery resolve output did not match expected backend endpoints" >&2
  sed -n '1,220p' "${resolve_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

"${INGATECTL_BIN}" xds resolve \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --backend="${backend_name}" \
  --backend-type="Static" \
  --output=text \
  >"${resolve_text_output_file}"

for expected_line in \
  "server: ${XDS_SERVER_CLIENT_ADDRESS}" \
  "backend: ${backend_name}" \
  "backendType: Static" \
  "endpoints: 1" \
  "${backend_endpoint_address}:${backend_endpoint_port} weight=100 healthy=true"
do
  if ! grep -Fq "${expected_line}" "${resolve_text_output_file}"; then
    echo "resolve text output did not contain expected line: ${expected_line}" >&2
    sed -n '1,260p' "${resolve_text_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
done

"${INGATECTL_BIN}" xds config \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  >"${config_output_file}"

if ! jq -e '
  .gatewayKey == "'"${gateway_name}"'" and
  .sourceVersion != "" and
  .publishVersion != "" and
  .updatedAt != "" and
  .config.version != "" and
  ((.config.listeners // []) | length) == 1 and
  .config.listeners[0].name == "web" and
  .config.listeners[0].port == 80 and
  ((.config.routes // []) | length) == '"${expected_route_count}"' and
  ((.config.routes // []) | any(.name == "'"${route_name}"'")) and
  ((.config.backends // []) | length) == 1 and
  ((.config.backends // []) | any(.name == "'"${backend_name}"'"))
' "${config_output_file}" >/dev/null 2>&1; then
  echo "configsync get-config output did not match expected effective config" >&2
  sed -n '1,260p' "${config_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

"${INGATECTL_BIN}" xds config \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --output=text \
  >"${config_text_output_file}"

for expected_line in \
  "server: ${XDS_SERVER_CLIENT_ADDRESS}" \
  "gateway: ${gateway_name}" \
  "sourceVersion: " \
  "publishVersion: " \
  "updatedAt: " \
  "configVersion: " \
  "listeners: 1" \
  "routes: ${expected_route_count}" \
  "backends: 1"
do
  if ! grep -Fq "${expected_line}" "${config_text_output_file}"; then
    echo "config text output did not contain expected line: ${expected_line}" >&2
    sed -n '1,260p' "${config_text_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
done

"${INGATECTL_BIN}" xds list \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  >"${list_output_file}"

if ! jq -e '
  ((.items // []) | length) >= 1 and
  ((.items // []) | any(
    .gatewayKey == "'"${gateway_name}"'" and
    .sourceVersion != "" and
    .publishVersion != "" and
    .updatedAt != ""
  ))
' "${list_output_file}" >/dev/null 2>&1; then
  echo "configsync list-configs output did not contain expected published gateway" >&2
  sed -n '1,260p' "${list_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

"${INGATECTL_BIN}" xds list \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --output=text \
  >"${list_text_output_file}"

for expected_line in \
  "server: ${XDS_SERVER_CLIENT_ADDRESS}" \
  "gatewayKey | sourceVersion | publishVersion | updatedAt" \
  "${gateway_name} | "
do
  if ! grep -Fq "${expected_line}" "${list_text_output_file}"; then
    echo "config list text output did not contain expected line: ${expected_line}" >&2
    sed -n '1,260p' "${list_text_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
done

"${INGATECTL_BIN}" xds summary \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  >"${summary_output_file}"

if ! jq -e '
  .gatewayKey == "'"${gateway_name}"'" and
  .listenerCount == 1 and
  .routeCount == '"${expected_route_count}"' and
  .backendCount == 1 and
  .endpointCount == 1 and
  ((.listenerNames // []) | any(. == "web")) and
  ((.routeNames // []) | any(. == "'"${route_name}"'")) and
  ((.backendNames // []) | any(. == "'"${backend_name}"'")) and
  ((.listenerHosts // []) | any(. == "api.example.com")) and
  ((.routeHostnames // []) | any(. == "api.example.com")) and
  ((.routePrefixes // []) | any(. == "/orders"))
' "${summary_output_file}" >/dev/null 2>&1; then
  echo "config summary output did not match expected published gateway summary" >&2
  sed -n '1,260p' "${summary_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
  if ! jq -e '
    ((.routeNames // []) | any(. == "'"${timeout_route_name}"'")) and
    ((.routeHostnames // []) | any(. == "slow.example.com")) and
    ((.routePrefixes // []) | any(. == "/slow-orders"))
  ' "${summary_output_file}" >/dev/null 2>&1; then
    echo "config summary output did not include expected timeout route details" >&2
    sed -n '1,260p' "${summary_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
  if ! jq -e '
    ((.routeNames // []) | any(. == "'"${retry_route_name}"'")) and
    ((.routeHostnames // []) | any(. == "retry.example.com")) and
    ((.routePrefixes // []) | any(. == "/retry-orders"))
  ' "${summary_output_file}" >/dev/null 2>&1; then
    echo "config summary output did not include expected retry route details" >&2
    sed -n '1,260p' "${summary_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
  if ! jq -e '
    ((.routeNames // []) | any(. == "'"${ratelimit_route_name}"'")) and
    ((.routeHostnames // []) | any(. == "limited.example.com")) and
    ((.routePrefixes // []) | any(. == "/limited-orders"))
  ' "${summary_output_file}" >/dev/null 2>&1; then
    echo "config summary output did not include expected ratelimit route details" >&2
    sed -n '1,260p' "${summary_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
fi

"${INGATECTL_BIN}" xds summary \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --output=text \
  >"${summary_text_output_file}"

for expected_line in \
  "gateway: ${gateway_name}" \
  "listeners: 1" \
  "routes: ${expected_route_count}" \
  "backends: 1" \
  "endpoints: 1" \
  "listenerNames: web" \
  "backendNames: ${backend_name}" \
  "listenerHosts: api.example.com" \
  "routeNames: " \
  "routePrefixes: "
do
  if ! grep -Fq "${expected_line}" "${summary_text_output_file}"; then
    echo "config summary text output did not contain expected line: ${expected_line}" >&2
    sed -n '1,260p' "${summary_text_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
done

for expected_fragment in \
  "${route_name}" \
  "/orders"
do
  if ! grep -Fq "${expected_fragment}" "${summary_text_output_file}"; then
    echo "config summary text output did not contain expected fragment: ${expected_fragment}" >&2
    sed -n '1,260p' "${summary_text_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
done

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
  for expected_fragment in \
    "${timeout_route_name}" \
    "slow.example.com" \
    "/slow-orders"
  do
    if ! grep -Fq "${expected_fragment}" "${summary_text_output_file}"; then
      echo "config summary text output did not contain expected timeout fragment: ${expected_fragment}" >&2
      sed -n '1,260p' "${summary_text_output_file}" >&2 || true
      sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
      exit 1
    fi
  done
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
  for expected_fragment in \
    "${retry_route_name}" \
    "retry.example.com" \
    "/retry-orders"
  do
    if ! grep -Fq "${expected_fragment}" "${summary_text_output_file}"; then
      echo "config summary text output did not contain expected retry fragment: ${expected_fragment}" >&2
      sed -n '1,260p' "${summary_text_output_file}" >&2 || true
      sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
      exit 1
    fi
  done
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
  for expected_fragment in \
    "${ratelimit_route_name}" \
    "limited.example.com" \
    "/limited-orders"
  do
    if ! grep -Fq "${expected_fragment}" "${summary_text_output_file}"; then
      echo "config summary text output did not contain expected ratelimit fragment: ${expected_fragment}" >&2
      sed -n '1,260p' "${summary_text_output_file}" >&2 || true
      sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
      exit 1
    fi
  done
fi

"${INGATECTL_BIN}" xds check \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --backend="${backend_name}" \
  --backend-type="Static" \
  >"${check_output_file}"

if ! jq -e '
  .gatewayKey == "'"${gateway_name}"'" and
  .backendName == "'"${backend_name}"'" and
  .gatewayPublished == true and
  .configReadable == true and
  .summaryReady == true and
  .backendResolved == true and
  .publishedGatewaySeen == true and
  .listenerCount == 1 and
  .routeCount == '"${expected_route_count}"' and
  .backendCount == 1 and
  .endpointCount == 1 and
  .publishedCount >= 1
' "${check_output_file}" >/dev/null 2>&1; then
  echo "check output did not match expected xds readiness state" >&2
  sed -n '1,260p' "${check_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

"${INGATECTL_BIN}" xds check \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --backend="${backend_name}" \
  --backend-type="Static" \
  --output=text \
  >"${check_text_output_file}"

for expected_line in \
  "server: ${XDS_SERVER_CLIENT_ADDRESS}" \
  "gateway: ${gateway_name}" \
  "backend: ${backend_name}" \
  "gatewayPublished: true" \
  "configReadable: true" \
  "summaryReady: true" \
  "backendResolved: true" \
  "publishedGatewaySeen: true"
do
  if ! grep -Fq "${expected_line}" "${check_text_output_file}"; then
    echo "check text output did not contain expected line: ${expected_line}" >&2
    sed -n '1,260p' "${check_text_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
done

"${INGATECTL_BIN}" xds ads \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --type=lds \
  --resource=web \
  >"${ads_lds_output_file}"

if ! jq -e '
  .gatewayKey == "'"${gateway_name}"'" and
  .resourceType == "lds" and
  .typeUrl == "type.googleapis.com/envoy.config.listener.v3.Listener" and
  .resourceCount == 1 and
  ((.resourceNames // []) | any(. == "web")) and
  .resources[0].name == "web" and
  .resources[0].filterChains[0].filters[0].name == "envoy.filters.network.http_connection_manager"
' "${ads_lds_output_file}" >/dev/null 2>&1; then
  echo "ads lds output did not match expected listener resource" >&2
  sed -n '1,260p' "${ads_lds_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

"${INGATECTL_BIN}" xds ads \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --type=rds \
  --resource="${gateway_name}/routes" \
  >"${ads_rds_output_file}"

if ! jq -e '
  .gatewayKey == "'"${gateway_name}"'" and
  .resourceType == "rds" and
  .typeUrl == "type.googleapis.com/envoy.config.route.v3.RouteConfiguration" and
  .resourceCount == 1 and
  ((.resourceNames // []) | any(. == "'"${gateway_name}/routes"'")) and
  .resources[0].name == "'"${gateway_name}/routes"'" and
  ((.resources[0].virtualHosts // []) | any(
    .name == "'"${route_name}"'" and
    ((.domains // []) | any(. == "api.example.com")) and
    ((.routes // []) | any(.match.prefix == "/orders"))
  ))
' "${ads_rds_output_file}" >/dev/null 2>&1; then
  echo "ads rds output did not match expected route resource" >&2
  sed -n '1,260p' "${ads_rds_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
  "${INGATECTL_BIN}" xds ads \
    --server="${XDS_SERVER_CLIENT_ADDRESS}" \
    --gateway="${gateway_name}" \
    --type=rds \
    --resource="${gateway_name}/routes" \
    >"${timeout_ads_rds_output_file}"

  if ! jq -e '
    ((.resources[0].virtualHosts // []) | any(
      .name == "'"${timeout_route_name}"'" and
      ((.domains // []) | any(. == "slow.example.com")) and
      ((.routes // []) | any(
        .match.prefix == "/slow-orders" and
        .route.timeout == "'"${TRAFFIC_POLICY_TIMEOUT_DURATION}"'"
      ))
    ))
  ' "${timeout_ads_rds_output_file}" >/dev/null 2>&1; then
    echo "ads rds output did not include expected timeout policy route action" >&2
    sed -n '1,260p' "${timeout_ads_rds_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
  "${INGATECTL_BIN}" xds ads \
    --server="${XDS_SERVER_CLIENT_ADDRESS}" \
    --gateway="${gateway_name}" \
    --type=rds \
    --resource="${gateway_name}/routes" \
    >"${retry_ads_rds_output_file}"

  if ! jq -e '
    ((.resources[0].virtualHosts // []) | any(
      .name == "'"${retry_route_name}"'" and
      ((.domains // []) | any(. == "retry.example.com")) and
      ((.routes // []) | any(
        .match.prefix == "/retry-orders" and
        .route.retryPolicy.retryOn == "5xx" and
        .route.retryPolicy.numRetries == '"${TRAFFIC_POLICY_RETRY_ATTEMPTS}"'
      ))
    ))
  ' "${retry_ads_rds_output_file}" >/dev/null 2>&1; then
    echo "ads rds output did not include expected retry policy route action" >&2
    sed -n '1,260p' "${retry_ads_rds_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
fi

if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
  "${INGATECTL_BIN}" xds ads \
    --server="${XDS_SERVER_CLIENT_ADDRESS}" \
    --gateway="${gateway_name}" \
    --type=rds \
    --resource="${gateway_name}/routes" \
    >"${ratelimit_ads_rds_output_file}"

  if ! jq -e '
    ((.resources[0].virtualHosts // []) | any(
      .name == "'"${ratelimit_route_name}"'" and
      ((.domains // []) | any(. == "limited.example.com")) and
      ((.routes // []) | any(
        .match.prefix == "/limited-orders" and
        .typedPerFilterConfig["envoy.filters.http.local_ratelimit"].tokenBucket.maxTokens == '"${TRAFFIC_POLICY_RATELIMIT_REQUESTS}"' and
        .typedPerFilterConfig["envoy.filters.http.local_ratelimit"].tokenBucket.tokensPerFill == '"${TRAFFIC_POLICY_RATELIMIT_REQUESTS}"'
      ))
    ))
  ' "${ratelimit_ads_rds_output_file}" >/dev/null 2>&1; then
    echo "ads rds output did not include expected ratelimit route action" >&2
    sed -n '1,260p' "${ratelimit_ads_rds_output_file}" >&2 || true
    sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
    exit 1
  fi
fi

"${INGATECTL_BIN}" xds ads \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --type=cds \
  --resource="${backend_name}" \
  >"${ads_cds_output_file}"

if ! jq -e '
  .gatewayKey == "'"${gateway_name}"'" and
  .resourceType == "cds" and
  .typeUrl == "type.googleapis.com/envoy.config.cluster.v3.Cluster" and
  .resourceCount == 1 and
  ((.resourceNames // []) | any(. == "'"${backend_name}"'")) and
  .resources[0].name == "'"${backend_name}"'" and
  .resources[0].type == "EDS"
' "${ads_cds_output_file}" >/dev/null 2>&1; then
  echo "ads cds output did not match expected cluster resource" >&2
  sed -n '1,260p' "${ads_cds_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

"${INGATECTL_BIN}" xds ads \
  --server="${XDS_SERVER_CLIENT_ADDRESS}" \
  --gateway="${gateway_name}" \
  --type=eds \
  --resource="${backend_name}" \
  >"${ads_eds_output_file}"

if ! jq -e '
  .gatewayKey == "'"${gateway_name}"'" and
  .resourceType == "eds" and
  .typeUrl == "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment" and
  .resourceCount == 1 and
  ((.resourceNames // []) | any(. == "'"${backend_name}"'")) and
  .resources[0].clusterName == "'"${backend_name}"'" and
  .resources[0].endpoints[0].lbEndpoints[0].endpoint.address.socketAddress.address == "'"${backend_endpoint_address}"'" and
  .resources[0].endpoints[0].lbEndpoints[0].endpoint.address.socketAddress.portValue == '"${backend_endpoint_port}"'
' "${ads_eds_output_file}" >/dev/null 2>&1; then
  echo "ads eds output did not match expected endpoint resource" >&2
  sed -n '1,260p' "${ads_eds_output_file}" >&2 || true
  sed -n '1,220p' "${XDS_SERVER_LOG_FILE}" >&2 || true
  exit 1
fi

if [[ "${VERIFY_XDS_ENVOY}" == "yes" ]]; then
  envoy_bootstrap_file="$(mktemp)"
  envoy_container_name="ingate-envoy-verify-$$"
  create_envoy_bootstrap "${envoy_bootstrap_file}" "${XDS_SERVER_DOCKER_ADDRESS}"

  docker run --rm \
    --name "${envoy_container_name}" \
    -p "${ENVOY_PROXY_ADDRESS%:*}:${ENVOY_PROXY_ADDRESS##*:}:80" \
    -p "${ENVOY_ADMIN_ADDRESS%:*}:${ENVOY_ADMIN_ADDRESS##*:}:9901" \
    -v "${envoy_bootstrap_file}:/etc/envoy/envoy.yaml:ro" \
    "${ENVOY_IMAGE}" \
    -c /etc/envoy/envoy.yaml \
    --log-level info \
    >"${ENVOY_LOG_FILE}" 2>&1 &
  envoy_pid=$!

  if ! verify_envoy_response "http://${ENVOY_PROXY_ADDRESS}" "http://${ENVOY_ADMIN_ADDRESS}" "api.example.com" "/orders" "200" "ingate-envoy-ok"; then
    exit 1
  fi
  if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
    if ! verify_envoy_response "http://${ENVOY_PROXY_ADDRESS}" "http://${ENVOY_ADMIN_ADDRESS}" "slow.example.com" "/slow-orders" "504" "upstream request timeout"; then
      exit 1
    fi
  fi
  if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
    if ! verify_envoy_response "http://${ENVOY_PROXY_ADDRESS}" "http://${ENVOY_ADMIN_ADDRESS}" "retry.example.com" "/retry-orders" "200" "retry success"; then
      exit 1
    fi
    if ! verify_backend_request_count "http://${BACKEND_PROBE_ADDRESS}/retry-count" "${expected_retry_request_count}" 30 1; then
      exit 1
    fi
  fi
  if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
    if ! verify_envoy_response "http://${ENVOY_PROXY_ADDRESS}" "http://${ENVOY_ADMIN_ADDRESS}" "limited.example.com" "/limited-orders" "200" "limited upstream ok"; then
      exit 1
    fi
    if ! verify_envoy_response "http://${ENVOY_PROXY_ADDRESS}" "http://${ENVOY_ADMIN_ADDRESS}" "limited.example.com" "/limited-orders" "429" ""; then
      exit 1
    fi
    if ! verify_backend_request_count "http://${BACKEND_PROBE_ADDRESS}/limited-count" "${TRAFFIC_POLICY_RATELIMIT_REQUESTS}" 30 1; then
      exit 1
    fi
  fi
fi

printf 'XDS_SERVER_HEALTHZ=ok\n'
printf 'XDS_SERVER_GRPC_READY=yes\n'
printf 'XDS_SERVER_GATEWAY_CREATE_CODE=201\n'
printf 'XDS_SERVER_BACKEND_CREATE_CODE=201\n'
printf 'XDS_SERVER_ROUTE_CREATE_CODE=201\n'
printf 'XDS_SERVER_PROGRAMMED_VERIFY=yes\n'
printf 'XDS_SERVER_DISCOVERY_VERIFY=yes\n'
printf 'XDS_SERVER_DISCOVERY_TEXT_VERIFY=yes\n'
printf 'XDS_SERVER_CONFIGSYNC_VERIFY=yes\n'
printf 'XDS_SERVER_CONFIGSYNC_TEXT_VERIFY=yes\n'
printf 'XDS_SERVER_LIST_VERIFY=yes\n'
printf 'XDS_SERVER_LIST_TEXT_VERIFY=yes\n'
printf 'XDS_SERVER_SUMMARY_VERIFY=yes\n'
printf 'XDS_SERVER_SUMMARY_TEXT_VERIFY=yes\n'
printf 'XDS_SERVER_CHECK_VERIFY=yes\n'
printf 'XDS_SERVER_CHECK_TEXT_VERIFY=yes\n'
printf 'XDS_SERVER_ADS_LDS_VERIFY=yes\n'
printf 'XDS_SERVER_ADS_RDS_VERIFY=yes\n'
printf 'XDS_SERVER_ADS_CDS_VERIFY=yes\n'
printf 'XDS_SERVER_ADS_EDS_VERIFY=yes\n'
if [[ "${VERIFY_XDS_ENVOY}" == "yes" ]]; then
  printf 'XDS_SERVER_ENVOY_VERIFY=yes\n'
fi
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT}" == "yes" ]]; then
  printf 'XDS_SERVER_TRAFFIC_POLICY_TIMEOUT_VERIFY=yes\n'
fi
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RETRY}" == "yes" ]]; then
  printf 'XDS_SERVER_TRAFFIC_POLICY_RETRY_VERIFY=yes\n'
fi
if [[ "${VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT}" == "yes" ]]; then
  printf 'XDS_SERVER_TRAFFIC_POLICY_RATELIMIT_VERIFY=yes\n'
fi
printf 'XDS_SERVER_PUBLISHED_VERSION=%s\n' "$(printf '%s' "${resolvedgateway_json}" | jq -r '.spec.version')"
