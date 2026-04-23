#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly default_apiserver_port="19448"
readonly default_admin_api_port="18081"
readonly gateway_name="adminapi-verify-gateway"
readonly certificate_name="adminapi-verify-certificate"
readonly certificate_secret_name="adminapi-verify-certificate-tls"
readonly backend_name="adminapi-verify-backend"
readonly backend_http_name="adminapi-verify-backend-http"
readonly route_name="adminapi-verify-route"
readonly auth_policy_name="adminapi-verify-auth-policy"
readonly traffic_policy_name="adminapi-verify-traffic-policy"
readonly apiserver_token="${APISERVER_TOKEN:-ingate-dev-admin-token}"
readonly admin_api_token="${ADMIN_API_TOKEN:-ingate-dev-admin-api-token}"
readonly apiserver_auth_header="Authorization: Bearer ${apiserver_token}"
readonly admin_api_auth_header="Authorization: Bearer ${admin_api_token}"
readonly content_type_json_header="Content-Type: application/json"

APISERVER_BIN="${APISERVER_BIN:-$(ingate::hack::build_dir)/ingate-apiserver}"
ADMIN_API_BIN="${ADMIN_API_BIN:-$(ingate::hack::build_dir)/ingate-admin-api}"
ETCD_SERVERS="${ETCD_SERVERS:-http://127.0.0.1:2379}"
APISERVER_HOST="${APISERVER_HOST:-127.0.0.1}"
APISERVER_PORT="${APISERVER_PORT:-${default_apiserver_port}}"
APISERVER_ADDRESS="${APISERVER_ADDRESS:-https://${APISERVER_HOST}:${APISERVER_PORT}}"
APISERVER_LOG_FILE="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver-admin-api.log}"
ADMIN_API_BIND_ADDRESS="${ADMIN_API_BIND_ADDRESS:-127.0.0.1}"
ADMIN_API_PORT="${ADMIN_API_PORT:-${default_admin_api_port}}"
ADMIN_API_LOG_FILE="${ADMIN_API_LOG_FILE:-$(ingate::hack::build_dir)/ingate-admin-api.log}"
ADMIN_BASE_URL="http://${ADMIN_API_BIND_ADDRESS}:${ADMIN_API_PORT}"

if [[ ! -x "${APISERVER_BIN}" ]]; then
  echo "apiserver binary not found: ${APISERVER_BIN}" >&2
  echo "run: make build-apiserver" >&2
  exit 1
fi

if [[ ! -x "${ADMIN_API_BIN}" ]]; then
  echo "admin-api binary not found: ${ADMIN_API_BIN}" >&2
  echo "run: make build-admin-api" >&2
  exit 1
fi

cleanup() {
  kill "${admin_pid:-}" >/dev/null 2>&1 || true
  wait "${admin_pid:-}" 2>/dev/null || true
  kill "${apiserver_pid:-}" >/dev/null 2>&1 || true
  wait "${apiserver_pid:-}" 2>/dev/null || true
}

assert_code() {
  local actual="$1"
  local expected="$2"
  local label="$3"
  local response_file="${4:-}"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "expected ${label} to return ${expected}, got ${actual}" >&2
    if [[ -n "${response_file}" ]]; then
      sed -n '1,180p' "${response_file}" >&2 || true
    fi
    exit 1
  fi
}

assert_contains() {
  local file="$1"
  local expected="$2"
  local label="$3"
  if ! grep -q "${expected}" "${file}"; then
    echo "expected ${label} to contain ${expected}" >&2
    sed -n '1,220p' "${file}" >&2 || true
    exit 1
  fi
}

admin_json_code() {
  local method="$1"
  local path="$2"
  local data="$3"
  local response_file="$4"
  curl --noproxy '*' -sS -o "${response_file}" -w '%{http_code}' \
    -X "${method}" "${ADMIN_BASE_URL}${path}" \
    -H "${admin_api_auth_header}" \
    -H "${content_type_json_header}" \
    -d "${data}"
}

admin_get_code() {
  local path="$1"
  local response_file="$2"
  curl --noproxy '*' -sS -o "${response_file}" -w '%{http_code}' \
    -H "${admin_api_auth_header}" \
    "${ADMIN_BASE_URL}${path}"
}

admin_delete_code() {
  local path="$1"
  local response_file="$2"
  curl --noproxy '*' -sS -o "${response_file}" -w '%{http_code}' \
    -X DELETE \
    -H "${admin_api_auth_header}" \
    "${ADMIN_BASE_URL}${path}"
}

generate_tls_pair() {
  local cert_file="$1"
  local key_file="$2"
  local common_name="$3"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "${key_file}" \
    -out "${cert_file}" \
    -subj "/CN=${common_name}" \
    -days 90 >/dev/null 2>&1
}

mkdir -p "$(dirname "${APISERVER_LOG_FILE}")" "$(dirname "${ADMIN_API_LOG_FILE}")"
"${APISERVER_BIN}" \
  --etcd-servers="${ETCD_SERVERS}" \
  --bind-address="${APISERVER_HOST}" \
  --secure-port="${APISERVER_PORT}" \
  >"${APISERVER_LOG_FILE}" 2>&1 &
apiserver_pid=$!
trap cleanup EXIT

if ! ingate::hack::wait_for_https_ready "${APISERVER_ADDRESS}/healthz" 30 1; then
  echo "apiserver did not become ready: ${APISERVER_ADDRESS}/healthz" >&2
  sed -n '1,220p' "${APISERVER_LOG_FILE}" >&2 || true
  exit 1
fi

"${ADMIN_API_BIN}" \
  --bind-address="${ADMIN_API_BIND_ADDRESS}" \
  --port="${ADMIN_API_PORT}" \
  --apiserver-address="${APISERVER_ADDRESS}" \
  --apiserver-token="${apiserver_token}" \
  --apiserver-insecure-skip-tls-verify=true \
  --admin-token="${admin_api_token}" \
  >"${ADMIN_API_LOG_FILE}" 2>&1 &
admin_pid=$!

for _attempt in $(seq 1 30); do
  if curl --noproxy '*' -fsS "${ADMIN_BASE_URL}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! curl --noproxy '*' -fsS "${ADMIN_BASE_URL}/healthz" >/dev/null 2>&1; then
  echo "admin-api did not become ready: ${ADMIN_BASE_URL}/healthz" >&2
  sed -n '1,220p' "${ADMIN_API_LOG_FILE}" >&2 || true
  exit 1
fi

curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/trafficpolicies/${traffic_policy_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/authpolicies/${auth_policy_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${route_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/certificates/${certificate_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/secrets/${certificate_secret_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/backends/${backend_http_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/backends/${backend_name}" >/dev/null 2>&1 || true
curl --noproxy '*' -ksS -X DELETE -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}" >/dev/null 2>&1 || true

response_files=()
for _i in $(seq 1 33); do
  response_files+=("$(mktemp)")
done
backend_http_create_response="$(mktemp)"
backend_get_response="$(mktemp)"
backend_http_get_response="$(mktemp)"
certificate_secret_options_response="$(mktemp)"
certificate_cert_file="$(mktemp)"
certificate_key_file="$(mktemp)"
certificate_updated_cert_file="$(mktemp)"
certificate_updated_key_file="$(mktemp)"
trap 'rm -f "${response_files[@]}" "${backend_http_create_response}" "${backend_get_response}" "${backend_http_get_response}" "${certificate_secret_options_response}" "${certificate_cert_file}" "${certificate_key_file}" "${certificate_updated_cert_file}" "${certificate_updated_key_file}"; cleanup' EXIT

generate_tls_pair "${certificate_cert_file}" "${certificate_key_file}" "api.example.com"
generate_tls_pair "${certificate_updated_cert_file}" "${certificate_updated_key_file}" "api-updated.example.com"

unauth_code="$(curl --noproxy '*' -sS -o "${response_files[0]}" -w '%{http_code}' "${ADMIN_BASE_URL}/admin/v1/gateways")"
assert_code "${unauth_code}" "401" "unauthenticated admin-api request" "${response_files[0]}"

gateway_code="$(admin_json_code POST /admin/v1/gateways '{"name":"'"${gateway_name}"'","listeners":[{"name":"web","protocol":"HTTP","port":80,"hostnames":["api.example.com","admin.example.com"]}]}' "${response_files[1]}")"
assert_code "${gateway_code}" "201" "gateway create" "${response_files[1]}"

certificate_create_payload="$(jq -n \
  --arg name "${certificate_name}" \
  --arg certPEM "$(cat "${certificate_cert_file}")" \
  --arg keyPEM "$(cat "${certificate_key_file}")" \
  '{name: $name, source: "Upload", upload: {certPEM: $certPEM, keyPEM: $keyPEM}, domains: ["api.example.com", "*.example.com"]}')"
certificate_code="$(admin_json_code POST /admin/v1/certificates "${certificate_create_payload}" "${response_files[27]}")"
assert_code "${certificate_code}" "201" "certificate create" "${response_files[27]}"
assert_contains "${response_files[27]}" '"source":"Upload"' "certificate create response"
assert_contains "${response_files[27]}" "\"secretRef\":{\"name\":\"${certificate_secret_name}\"}" "certificate create response"

backend_code="$(admin_json_code POST /admin/v1/backends '{"name":"'"${backend_name}"'","type":"Static","protocol":"HTTPS","defaultPort":8080,"static":{"endpoints":[{"address":"127.0.0.1","port":8080,"weight":100,"healthy":true},{"address":"127.0.0.2","port":8080,"weight":80,"healthy":true}]}}' "${response_files[2]}")"
assert_code "${backend_code}" "201" "backend create" "${response_files[2]}"
assert_contains "${response_files[2]}" '"protocol":"HTTPS"' "backend create response"

backend_http_code="$(admin_json_code POST /admin/v1/backends '{"name":"'"${backend_http_name}"'","type":"Static","defaultPort":8081,"static":{"endpoints":[{"address":"127.0.0.3","port":8081,"weight":100,"healthy":true}]}}' "${backend_http_create_response}")"
assert_code "${backend_http_code}" "201" "backend create without protocol" "${backend_http_create_response}"
assert_contains "${backend_http_create_response}" '"protocol":"HTTP"' "backend create without protocol response"

route_code="$(admin_json_code POST /admin/v1/routes '{"name":"'"${route_name}"'","parentRefs":[{"name":"'"${gateway_name}"'"}],"hostnames":["api.example.com"],"rules":[{"matches":[{"path":{"type":"PathPrefix","value":"/orders"},"method":"GET","headers":[{"name":"x-tenant","value":"vip"}]}],"backendRefs":[{"name":"'"${backend_name}"'","port":8080,"weight":90},{"name":"'"${backend_http_name}"'","port":8081,"weight":10}],"filters":[{"type":"URLRewrite","urlRewrite":{"path":{"type":"ReplacePrefixMatch","replacePrefixMatch":"/api"}}},{"type":"RequestHeaderModifier","requestHeaderModifier":{"set":[{"name":"x-user-id","value":"0000123"}],"remove":["x-remove-me"]}},{"type":"ResponseHeaderModifier","responseHeaderModifier":{"add":[{"name":"x-response-source","value":"ingate"}]}}]}]}' "${response_files[3]}")"
assert_code "${route_code}" "201" "route create" "${response_files[3]}"
assert_contains "${response_files[3]}" '"headers"' "route create response"
assert_contains "${response_files[3]}" '"filters"' "route create response"
assert_contains "${response_files[3]}" '"replacePrefixMatch":"/api"' "route create response"
assert_contains "${response_files[3]}" '"requestHeaderModifier"' "route create response"
assert_contains "${response_files[3]}" '"responseHeaderModifier"' "route create response"

auth_policy_code="$(admin_json_code POST /admin/v1/auth-policies '{"name":"'"${auth_policy_name}"'","targetRefs":[{"kind":"Route","name":"'"${route_name}"'"}],"type":"APIKey","apiKey":{"fromHeaders":[{"name":"X-API-Key"}]}}' "${response_files[4]}")"
assert_code "${auth_policy_code}" "201" "auth-policy create" "${response_files[4]}"

traffic_policy_code="$(admin_json_code POST /admin/v1/traffic-policies '{"name":"'"${traffic_policy_name}"'","targetRefs":[{"kind":"Route","name":"'"${route_name}"'"},{"kind":"Backend","name":"'"${backend_name}"'"}],"timeout":{"duration":"2s"},"retry":{"attempts":2,"conditions":["5xx"]},"rateLimit":{"requestsPerUnit":100,"unit":"minute","scope":"route"}}' "${response_files[5]}")"
assert_code "${traffic_policy_code}" "201" "traffic-policy create" "${response_files[5]}"

assert_code "$(admin_get_code /admin/v1/gateways "${response_files[6]}")" "200" "gateway list" "${response_files[6]}"
assert_code "$(admin_get_code /admin/v1/backends "${response_files[7]}")" "200" "backend list" "${response_files[7]}"
assert_code "$(admin_get_code /admin/v1/routes "${response_files[8]}")" "200" "route list" "${response_files[8]}"
assert_code "$(admin_get_code /admin/v1/auth-policies "${response_files[9]}")" "200" "auth-policy list" "${response_files[9]}"
assert_code "$(admin_get_code /admin/v1/traffic-policies "${response_files[10]}")" "200" "traffic-policy list" "${response_files[10]}"
assert_code "$(admin_get_code /admin/v1/certificates "${response_files[28]}")" "200" "certificate list" "${response_files[28]}"
assert_code "$(admin_get_code /admin/v1/certificate-secrets "${certificate_secret_options_response}")" "200" "certificate secret options list" "${certificate_secret_options_response}"
assert_contains "${response_files[28]}" "${certificate_name}" "certificate list response"
assert_contains "${response_files[28]}" '"status":"Healthy"' "certificate list summary"
assert_contains "${certificate_secret_options_response}" "${certificate_secret_name}" "certificate secret options list"
assert_contains "${response_files[7]}" '"protocol":"HTTPS"' "backend list response"
assert_contains "${response_files[7]}" '"protocol":"HTTP"' "backend list response"

assert_code "$(admin_json_code PUT "/admin/v1/gateways/${gateway_name}" '{"listeners":[{"name":"web","protocol":"HTTP","port":80,"hostnames":["api-updated.example.com","admin-updated.example.com"]}]}' "${response_files[11]}")" "200" "gateway update" "${response_files[11]}"
certificate_update_payload="$(jq -n \
  --arg certPEM "$(cat "${certificate_updated_cert_file}")" \
  --arg keyPEM "$(cat "${certificate_updated_key_file}")" \
  '{source: "Upload", upload: {certPEM: $certPEM, keyPEM: $keyPEM}, domains: ["api-updated.example.com"]}')"
assert_code "$(admin_json_code PUT "/admin/v1/certificates/${certificate_name}" "${certificate_update_payload}" "${response_files[29]}")" "200" "certificate update" "${response_files[29]}"
assert_contains "${response_files[29]}" '"source":"Upload"' "certificate update response"
assert_contains "${response_files[29]}" "\"secretRef\":{\"name\":\"${certificate_secret_name}\"}" "certificate update response"
assert_contains "${response_files[29]}" '"status":"Healthy"' "certificate update summary"
assert_code "$(admin_json_code PUT "/admin/v1/backends/${backend_name}" '{"type":"Static","protocol":"gRPC","defaultPort":9090,"static":{"endpoints":[{"address":"127.0.0.1","port":9090,"weight":100,"healthy":true},{"address":"127.0.0.2","port":9090,"weight":80,"healthy":true}]}}' "${response_files[12]}")" "200" "backend update" "${response_files[12]}"
assert_contains "${response_files[12]}" '"protocol":"gRPC"' "backend update response"
assert_code "$(admin_get_code "/admin/v1/backends/${backend_name}" "${backend_get_response}")" "200" "backend get" "${backend_get_response}"
assert_contains "${backend_get_response}" '"protocol":"gRPC"' "backend get response"

backend_update_missing_code="$(admin_json_code PUT "/admin/v1/backends/${backend_http_name}" '{"type":"Static","defaultPort":8082,"static":{"endpoints":[{"address":"127.0.0.3","port":8082,"weight":100,"healthy":true}]}}' "${response_files[13]}")"
assert_code "${backend_update_missing_code}" "400" "backend update without protocol" "${response_files[13]}"

backend_update_empty_code="$(admin_json_code PUT "/admin/v1/backends/${backend_http_name}" '{"type":"Static","protocol":"","defaultPort":8082,"static":{"endpoints":[{"address":"127.0.0.3","port":8082,"weight":100,"healthy":true}]}}' "${response_files[14]}")"
assert_code "${backend_update_empty_code}" "400" "backend update with empty protocol" "${response_files[14]}"

assert_code "$(admin_json_code PUT "/admin/v1/routes/${route_name}" '{"parentRefs":[{"name":"'"${gateway_name}"'"}],"hostnames":["api-updated.example.com"],"rules":[{"matches":[{"path":{"type":"PathPrefix","value":"/orders/v2"},"method":"POST","headers":[{"name":"x-tenant","value":"vip"}]}],"backendRefs":[{"name":"'"${backend_name}"'","port":9090,"weight":100}],"filters":[{"type":"URLRewrite","urlRewrite":{"path":{"type":"ReplacePrefixMatch","replacePrefixMatch":"/internal"}}},{"type":"RequestHeaderModifier","requestHeaderModifier":{"add":[{"name":"x-extra-tenant","value":"gold"}]}}]}]}' "${response_files[15]}")" "200" "route update" "${response_files[15]}"
assert_contains "${response_files[15]}" '"method":"POST"' "route update response"
assert_contains "${response_files[15]}" '"replacePrefixMatch":"/internal"' "route update response"
assert_contains "${response_files[15]}" '"requestHeaderModifier"' "route update response"
assert_code "$(admin_json_code PUT "/admin/v1/auth-policies/${auth_policy_name}" '{"targetRefs":[{"kind":"Route","name":"'"${route_name}"'"}],"type":"APIKey","apiKey":{"fromHeaders":[{"name":"X-API-Key","prefix":"Bearer "}]}}' "${response_files[16]}")" "200" "auth-policy update" "${response_files[16]}"
assert_code "$(admin_json_code PUT "/admin/v1/traffic-policies/${traffic_policy_name}" '{"targetRefs":[{"kind":"Route","name":"'"${route_name}"'"},{"kind":"Backend","name":"'"${backend_name}"'"}],"timeout":{"duration":"3s"},"retry":{"attempts":3,"conditions":["5xx"]},"rateLimit":{"requestsPerUnit":120,"unit":"minute","scope":"route"}}' "${response_files[17]}")" "200" "traffic-policy update" "${response_files[17]}"

assert_code "$(admin_get_code "/admin/v1/topology" "${response_files[18]}")" "200" "topology" "${response_files[18]}"
assert_contains "${response_files[18]}" "${gateway_name}" "topology"
assert_contains "${response_files[18]}" "${route_name}" "topology"
assert_contains "${response_files[18]}" "${backend_name}" "topology"
assert_contains "${response_files[18]}" "${auth_policy_name}" "topology"
assert_contains "${response_files[18]}" "${traffic_policy_name}" "topology"
assert_contains "${response_files[18]}" '"nodes"' "topology"
assert_contains "${response_files[18]}" '"edges"' "topology"

assert_code "$(admin_get_code "/admin/v1/overview" "${response_files[19]}")" "200" "overview" "${response_files[19]}"
assert_contains "${response_files[19]}" '"summary"' "overview"
assert_contains "${response_files[19]}" '"chains"' "overview"
assert_contains "${response_files[19]}" "${gateway_name}" "overview"
assert_contains "${response_files[19]}" "${route_name}" "overview"
assert_contains "${response_files[19]}" "${backend_name}" "overview"

assert_code "$(admin_get_code "/admin/v1/backends/${backend_http_name}" "${backend_http_get_response}")" "200" "backend get defaulted" "${backend_http_get_response}"
assert_contains "${backend_http_get_response}" '"protocol":"HTTP"' "backend get defaulted response"
assert_contains "${response_files[7]}" "${backend_name}" "backend list response"
assert_contains "${response_files[7]}" "${backend_http_name}" "backend list response"
assert_contains "${response_files[7]}" '"protocol":"HTTP"' "backend list response"

curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/gateways/${gateway_name}" >/dev/null
curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/backends/${backend_name}" >/dev/null
curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/backends/${backend_http_name}" >/dev/null
curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/secrets/${certificate_secret_name}" >/dev/null
curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/certificates/${certificate_name}" >/dev/null
curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/routes/${route_name}" >/dev/null
curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/authpolicies/${auth_policy_name}" >/dev/null
curl --noproxy '*' -kfsS -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/policy.ingate.io/v1alpha1/trafficpolicies/${traffic_policy_name}" >/dev/null

assert_code "$(admin_delete_code "/admin/v1/traffic-policies/${traffic_policy_name}" "${response_files[20]}")" "204" "traffic-policy delete" "${response_files[20]}"
assert_code "$(admin_delete_code "/admin/v1/auth-policies/${auth_policy_name}" "${response_files[21]}")" "204" "auth-policy delete" "${response_files[21]}"
assert_code "$(admin_delete_code "/admin/v1/routes/${route_name}" "${response_files[22]}")" "204" "route delete" "${response_files[22]}"
assert_code "$(admin_delete_code "/admin/v1/certificates/${certificate_name}" "${response_files[30]}")" "204" "certificate delete" "${response_files[30]}"
assert_code "$(admin_delete_code "/admin/v1/backends/${backend_http_name}" "${response_files[23]}")" "204" "backend delete defaulted" "${response_files[23]}"
assert_code "$(admin_delete_code "/admin/v1/backends/${backend_name}" "${response_files[24]}")" "204" "backend delete" "${response_files[24]}"
assert_code "$(admin_delete_code "/admin/v1/gateways/${gateway_name}" "${response_files[25]}")" "204" "gateway delete" "${response_files[25]}"

post_delete_code="$(admin_get_code "/admin/v1/gateways/${gateway_name}" "${response_files[26]}")"
assert_code "${post_delete_code}" "404" "gateway get after delete" "${response_files[26]}"

certificate_post_delete_code="$(admin_get_code "/admin/v1/certificates/${certificate_name}" "${response_files[31]}")"
assert_code "${certificate_post_delete_code}" "404" "certificate get after delete" "${response_files[31]}"
secret_post_delete_code="$(curl --noproxy '*' -ksS -o "${response_files[32]}" -w '%{http_code}' -H "${apiserver_auth_header}" "${APISERVER_ADDRESS}/apis/gateway.ingate.io/v1alpha1/secrets/${certificate_secret_name}")"
assert_code "${secret_post_delete_code}" "404" "managed secret get after certificate delete" "${response_files[32]}"

printf 'ADMIN_API_HEALTHZ=ok\n'
printf 'ADMIN_API_UNAUTH_CODE=%s\n' "${unauth_code}"
printf 'ADMIN_API_GATEWAY_CREATE_CODE=%s\n' "${gateway_code}"
printf 'ADMIN_API_CERTIFICATE_CREATE_CODE=%s\n' "${certificate_code}"
printf 'ADMIN_API_BACKEND_CREATE_CODE=%s\n' "${backend_code}"
printf 'ADMIN_API_BACKEND_DEFAULTED_CREATE_CODE=%s\n' "${backend_http_code}"
printf 'ADMIN_API_ROUTE_CREATE_CODE=%s\n' "${route_code}"
printf 'ADMIN_API_AUTH_POLICY_CREATE_CODE=%s\n' "${auth_policy_code}"
printf 'ADMIN_API_TRAFFIC_POLICY_CREATE_CODE=%s\n' "${traffic_policy_code}"
printf 'ADMIN_API_UPDATE_VERIFY=yes\n'
printf 'ADMIN_API_TOPOLOGY_VERIFY=yes\n'
printf 'ADMIN_API_OVERVIEW_VERIFY=yes\n'
printf 'ADMIN_API_DELETE_VERIFY=yes\n'
printf 'ADMIN_API_APISERVER_RESOURCE_VERIFY=yes\n'
