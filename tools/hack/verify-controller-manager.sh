#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly default_apiserver_port="19458"
readonly default_controller_manager_health_addr="127.0.0.1:18091"
readonly gateway_name="controller-verify-gateway"
readonly backend_name="controller-verify-backend"
readonly route_name="controller-verify-route"
readonly certificate_name="controller-verify-certificate"
readonly certificate_secret_name="controller-verify-secret"
readonly auth_policy_name="controller-verify-auth-policy"
readonly traffic_policy_name="controller-verify-traffic-policy"
readonly apiserver_token="${APISERVER_TOKEN:-ingate-dev-admin-token}"
readonly apiserver_auth_header="Authorization: Bearer ${apiserver_token}"
readonly content_type_json_header="Content-Type: application/json"

APISERVER_BIN="${APISERVER_BIN:-$(ingate::hack::build_dir)/ingate-apiserver}"
CONTROLLER_MANAGER_BIN="${CONTROLLER_MANAGER_BIN:-$(ingate::hack::build_dir)/ingate-controller-manager}"
ETCD_SERVERS="${ETCD_SERVERS:-http://127.0.0.1:2379}"
APISERVER_HOST="${APISERVER_HOST:-127.0.0.1}"
APISERVER_PORT="${APISERVER_PORT:-${default_apiserver_port}}"
APISERVER_ADDRESS="${APISERVER_ADDRESS:-https://${APISERVER_HOST}:${APISERVER_PORT}}"
APISERVER_LOG_FILE="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver-controller-manager.log}"
CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS="${CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS:-${default_controller_manager_health_addr}}"
CONTROLLER_MANAGER_LOG_FILE="${CONTROLLER_MANAGER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-controller-manager.log}"
KUBECONFIG_FILE="${KUBECONFIG_FILE:-$(mktemp)}"

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

cleanup() {
  set +e
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/trafficpolicies/${traffic_policy_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/authpolicies/${auth_policy_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${route_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/certificates/${certificate_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/secrets/${certificate_secret_name}" >/dev/null 2>&1 || true
  curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/backends/${backend_name}" >/dev/null 2>&1 || true
  kill "${controller_manager_pid:-}" >/dev/null 2>&1 || true
  wait "${controller_manager_pid:-}" 2>/dev/null || true
  kill "${apiserver_pid:-}" >/dev/null 2>&1 || true
  wait "${apiserver_pid:-}" 2>/dev/null || true
  rm -f "${KUBECONFIG_FILE}" "${response_file:-}"
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

mkdir -p "$(dirname "${APISERVER_LOG_FILE}")" "$(dirname "${CONTROLLER_MANAGER_LOG_FILE}")"
trap cleanup EXIT

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
  --metrics-bind-address="127.0.0.1:18092" \
  --workers=2 \
  >"${CONTROLLER_MANAGER_LOG_FILE}" 2>&1 &
controller_manager_pid=$!

if ! wait_for_http_ready "http://${CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS}/healthz" 30 1; then
  echo "controller-manager did not become ready: http://${CONTROLLER_MANAGER_HEALTHZ_BIND_ADDRESS}/healthz" >&2
  sed -n '1,220p' "${CONTROLLER_MANAGER_LOG_FILE}" >&2 || true
  exit 1
fi

response_file="$(mktemp)"

secret_payload="$(jq -n \
  --arg name "${certificate_secret_name}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Secret",metadata:{name:$name},spec:{type:"kubernetes.io/tls",stringData:{"tls.crt":"dummy-cert","tls.key":"dummy-key"}}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/secrets "${secret_payload}" "${response_file}")" "201" "secret create" "${response_file}"

certificate_payload="$(jq -n \
  --arg name "${certificate_name}" \
  --arg secret_name "${certificate_secret_name}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Certificate",metadata:{name:$name},spec:{secretRef:{name:$secret_name},domains:["api.example.com","*.example.com"]}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/certificates "${certificate_payload}" "${response_file}")" "201" "certificate create" "${response_file}"

gateway_payload="$(jq -n \
  --arg name "${gateway_name}" \
  --arg certificate_name "${certificate_name}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Gateway",metadata:{name:$name},spec:{listeners:[{name:"https",protocol:"HTTPS",port:443,hostnames:["api.example.com"],tls:{mode:"Terminate",certificateRef:{name:$certificate_name}}}]}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/gateways "${gateway_payload}" "${response_file}")" "201" "gateway create" "${response_file}"

backend_payload="$(jq -n \
  --arg name "${backend_name}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Backend",metadata:{name:$name},spec:{type:"Static",protocol:"HTTP",defaultPort:8080,static:{endpoints:[{address:"127.0.0.1",port:8080,weight:100,healthy:true}]}}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/backends "${backend_payload}" "${response_file}")" "201" "backend create" "${response_file}"

route_payload="$(jq -n \
  --arg name "${route_name}" \
  --arg gateway_name "${gateway_name}" \
  --arg backend_name "${backend_name}" \
  '{apiVersion:"gateway.ingate.io/v1alpha1",kind:"Route",metadata:{name:$name},spec:{parentRefs:[{name:$gateway_name}],hostnames:["api.example.com"],rules:[{matches:[{path:{type:"PathPrefix",value:"/orders"},method:"GET"}],backendRefs:[{name:$backend_name,port:8080,weight:100}]}]}}')"
assert_code "$(api_json_code POST /apis/gateway.ingate.io/v1alpha1/routes "${route_payload}" "${response_file}")" "201" "route create" "${response_file}"

auth_policy_payload="$(jq -n \
  --arg name "${auth_policy_name}" \
  --arg route_name "${route_name}" \
  '{apiVersion:"policy.ingate.io/v1alpha1",kind:"AuthPolicy",metadata:{name:$name},spec:{targetRefs:[{kind:"Route",name:$route_name}],type:"APIKey",apiKey:{fromHeaders:[{name:"X-API-Key"}]}}}')"
assert_code "$(api_json_code POST /apis/policy.ingate.io/v1alpha1/authpolicies "${auth_policy_payload}" "${response_file}")" "201" "authpolicy create" "${response_file}"

traffic_policy_payload="$(jq -n \
  --arg name "${traffic_policy_name}" \
  --arg route_name "${route_name}" \
  --arg backend_name "${backend_name}" \
  '{apiVersion:"policy.ingate.io/v1alpha1",kind:"TrafficPolicy",metadata:{name:$name},spec:{targetRefs:[{kind:"Route",name:$route_name},{kind:"Backend",name:$backend_name}],timeout:{duration:"2s"},retry:{attempts:2,conditions:["5xx"]},rateLimit:{requestsPerUnit:100,unit:"minute",scope:"route"}}}')"
assert_code "$(api_json_code POST /apis/policy.ingate.io/v1alpha1/trafficpolicies "${traffic_policy_payload}" "${response_file}")" "201" "trafficpolicy create" "${response_file}"

resolvedgateway_url="${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/resolvedgateways/${gateway_name}"
gateway_url="${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}"
route_url="${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${route_name}"
backend_url="${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/backends/${backend_name}"

resolvedgateway_json="$(wait_for_json_condition "${resolvedgateway_url}" '
  .spec.gatewayRef.name == "'"${gateway_name}"'" and
  (.spec.listeners | length) > 0 and
  (.spec.routes | length) > 0 and
  (.spec.backends | length) > 0 and
  ((.status.conditions // []) | any(.type == "Accepted" and .status == "True")) and
  ((.status.conditions // []) | any(.type == "Resolved" and .status == "True"))
')"

wait_for_json_condition "${gateway_url}" '((.status.conditions // []) | any(.type == "Accepted" and .status == "True")) and ((.status.conditions // []) | any(.type == "Resolved" and .status == "True"))' >/dev/null
wait_for_json_condition "${route_url}" '((.status.conditions // []) | any(.type == "Accepted" and .status == "True")) and ((.status.conditions // []) | any(.type == "Resolved" and .status == "True"))' >/dev/null
wait_for_json_condition "${backend_url}" '((.status.conditions // []) | any(.type == "Accepted" and .status == "True")) and ((.status.conditions // []) | any(.type == "Resolved" and .status == "True"))' >/dev/null

printf 'CONTROLLER_MANAGER_HEALTHZ=ok\n'
printf 'CONTROLLER_MANAGER_GATEWAY_CREATE_CODE=201\n'
printf 'CONTROLLER_MANAGER_BACKEND_CREATE_CODE=201\n'
printf 'CONTROLLER_MANAGER_ROUTE_CREATE_CODE=201\n'
printf 'CONTROLLER_MANAGER_AUTH_POLICY_CREATE_CODE=201\n'
printf 'CONTROLLER_MANAGER_TRAFFIC_POLICY_CREATE_CODE=201\n'
printf 'CONTROLLER_MANAGER_RESOLVEDGATEWAY_READY=yes\n'
printf 'CONTROLLER_MANAGER_RESOLVEDGATEWAY_VERSION=%s\n' "$(printf '%s' "${resolvedgateway_json}" | jq -r '.spec.version')"
printf 'CONTROLLER_MANAGER_STATUS_VERIFY=yes\n'
