#!/bin/sh
set -eu

APISERVER_ADDRESS="${APISERVER_ADDRESS:-https://apiserver:18443}"
APISERVER_TOKEN="${APISERVER_TOKEN:-ingate-dev-admin-token}"
CONTROLLER_MANAGER_HEALTH_URL="${CONTROLLER_MANAGER_HEALTH_URL:-http://controller-manager:18081/healthz}"
XDS_SERVER_HEALTH_URL="${XDS_SERVER_HEALTH_URL:-http://xds-server:19091/healthz}"
GATEWAY_NAME="${GATEWAY_NAME:-compose-gateway}"
ROUTE_NAME="${ROUTE_NAME:-compose-orders-route}"
BACKEND_NAME="${BACKEND_NAME:-compose-backend}"
GATEWAY_HOST="${GATEWAY_HOST:-api.example.com}"
ROUTE_PATH_PREFIX="${ROUTE_PATH_PREFIX:-/orders}"
BACKEND_ENDPOINT_ADDRESS="${BACKEND_ENDPOINT_ADDRESS:-172.31.250.10}"
BACKEND_ENDPOINT_PORT="${BACKEND_ENDPOINT_PORT:-8080}"
BACKEND_PROTOCOL="${BACKEND_PROTOCOL:-HTTP}"

AUTH_HEADER="Authorization: Bearer ${APISERVER_TOKEN}"
CONTENT_TYPE="Content-Type: application/json"

wait_for_http() {
  url="$1"
  attempts="${2:-90}"
  delay="${3:-2}"
  i=1
  while [ "${i}" -le "${attempts}" ]; do
    if curl --noproxy '*' -kfsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay}"
    i=$((i + 1))
  done
  echo "timed out waiting for ${url}" >&2
  return 1
}

create_if_missing() {
  path="$1"
  payload="$2"
  resource_name="$3"

  status_code="$(
    curl --noproxy '*' -ksS -o /tmp/ingate-seed-response.$$ -w '%{http_code}' \
      -X POST "${APISERVER_ADDRESS}${path}" \
      -H "${AUTH_HEADER}" \
      -H "${CONTENT_TYPE}" \
      -d "${payload}"
  )"

  if [ "${status_code}" = "201" ] || [ "${status_code}" = "409" ]; then
    rm -f /tmp/ingate-seed-response.$$
    return 0
  fi

  echo "failed to create ${resource_name}, status=${status_code}" >&2
  cat /tmp/ingate-seed-response.$$ >&2 || true
  rm -f /tmp/ingate-seed-response.$$
  return 1
}

wait_for_http "${APISERVER_ADDRESS}/healthz"
wait_for_http "${CONTROLLER_MANAGER_HEALTH_URL}"
wait_for_http "${XDS_SERVER_HEALTH_URL}"

backend_payload=$(cat <<EOF
{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Backend","metadata":{"name":"${BACKEND_NAME}"},"spec":{"type":"Static","protocol":"${BACKEND_PROTOCOL}","defaultPort":${BACKEND_ENDPOINT_PORT},"static":{"endpoints":[{"address":"${BACKEND_ENDPOINT_ADDRESS}","port":${BACKEND_ENDPOINT_PORT},"weight":100,"healthy":true}]}}}
EOF
)
gateway_payload=$(cat <<EOF
{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Gateway","metadata":{"name":"${GATEWAY_NAME}"},"spec":{"listeners":[{"name":"web","protocol":"HTTP","port":80,"hostnames":["${GATEWAY_HOST}"]}]}}
EOF
)
route_payload=$(cat <<EOF
{"apiVersion":"gateway.ingate.io/v1alpha1","kind":"Route","metadata":{"name":"${ROUTE_NAME}"},"spec":{"parentRefs":[{"name":"${GATEWAY_NAME}"}],"hostnames":["${GATEWAY_HOST}"],"rules":[{"matches":[{"path":{"type":"PathPrefix","value":"${ROUTE_PATH_PREFIX}"}}],"backendRefs":[{"name":"${BACKEND_NAME}","port":${BACKEND_ENDPOINT_PORT},"weight":100}]}]}}
EOF
)

create_if_missing "/apis/gateway.ingate.io/v1alpha1/backends" "${backend_payload}" "backend ${BACKEND_NAME}"
create_if_missing "/apis/gateway.ingate.io/v1alpha1/gateways" "${gateway_payload}" "gateway ${GATEWAY_NAME}"
create_if_missing "/apis/gateway.ingate.io/v1alpha1/routes" "${route_payload}" "route ${ROUTE_NAME}"

echo "seeded gateway=${GATEWAY_NAME} route=${ROUTE_NAME} backend=${BACKEND_NAME}"
