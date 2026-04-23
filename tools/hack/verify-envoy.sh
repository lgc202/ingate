#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

APISERVER_BIN="${APISERVER_BIN:-$(ingate::hack::build_dir)/ingate-apiserver}"
CONTROLLER_MANAGER_BIN="${CONTROLLER_MANAGER_BIN:-$(ingate::hack::build_dir)/ingate-controller-manager}"
XDS_SERVER_BIN="${XDS_SERVER_BIN:-$(ingate::hack::build_dir)/ingate-xds-server}"
INGATECTL_BIN="${INGATECTL_BIN:-$(ingate::hack::build_dir)/ingatectl}"
ENVOY_IMAGE="${ENVOY_IMAGE:-envoyproxy/envoy:v1.32.4}"
XDS_SERVER_GRPC_BIND_ADDRESS="${XDS_SERVER_GRPC_BIND_ADDRESS:-0.0.0.0:19090}"
XDS_SERVER_CLIENT_ADDRESS="${XDS_SERVER_CLIENT_ADDRESS:-127.0.0.1:19090}"
XDS_SERVER_DOCKER_ADDRESS="${XDS_SERVER_DOCKER_ADDRESS:-host.docker.internal:19090}"
BACKEND_MOCK_ADDRESS="${BACKEND_MOCK_ADDRESS:-0.0.0.0:18081}"
ENVOY_ADMIN_ADDRESS="${ENVOY_ADMIN_ADDRESS:-127.0.0.1:19901}"
ENVOY_PROXY_ADDRESS="${ENVOY_PROXY_ADDRESS:-127.0.0.1:10080}"

if [[ -z "${BACKEND_ENDPOINT_ADDRESS:-}" ]]; then
  docker_host_ip="$(
    docker run --rm --entrypoint sh "${ENVOY_IMAGE}" -c "getent hosts host.docker.internal | sed -n '1s/[[:space:]].*\$//p'"
  )"
  if [[ -z "${docker_host_ip}" ]]; then
    echo "failed to resolve host.docker.internal inside Docker" >&2
    exit 1
  fi
  BACKEND_ENDPOINT_ADDRESS="${docker_host_ip}:18081"
fi

APISERVER_BIN="${APISERVER_BIN}" \
CONTROLLER_MANAGER_BIN="${CONTROLLER_MANAGER_BIN}" \
XDS_SERVER_BIN="${XDS_SERVER_BIN}" \
INGATECTL_BIN="${INGATECTL_BIN}" \
ENVOY_IMAGE="${ENVOY_IMAGE}" \
VERIFY_XDS_ENVOY=yes \
VERIFY_XDS_TRAFFIC_POLICY_TIMEOUT=yes \
VERIFY_XDS_TRAFFIC_POLICY_RETRY=yes \
VERIFY_XDS_TRAFFIC_POLICY_RATELIMIT=yes \
XDS_SERVER_GRPC_BIND_ADDRESS="${XDS_SERVER_GRPC_BIND_ADDRESS}" \
XDS_SERVER_CLIENT_ADDRESS="${XDS_SERVER_CLIENT_ADDRESS}" \
XDS_SERVER_DOCKER_ADDRESS="${XDS_SERVER_DOCKER_ADDRESS}" \
BACKEND_MOCK_ADDRESS="${BACKEND_MOCK_ADDRESS}" \
BACKEND_ENDPOINT_ADDRESS="${BACKEND_ENDPOINT_ADDRESS}" \
ENVOY_ADMIN_ADDRESS="${ENVOY_ADMIN_ADDRESS}" \
ENVOY_PROXY_ADDRESS="${ENVOY_PROXY_ADDRESS}" \
./tools/hack/verify-xds-server.sh
