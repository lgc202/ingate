#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly default_verify_port="19447"
readonly gateway_name="kubectl-verify-gateway"
readonly viewer_create_gateway_name="kubectl-viewer-create-gateway"
readonly admin_context="ingate-admin"
readonly viewer_context="ingate-viewer"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required for verify-apiserver-kubectl" >&2
  exit 1
fi

api_server_bin="${APISERVER_BIN:-}"
etcd_servers="${ETCD_SERVERS:-http://127.0.0.1:2379}"
host="${APISERVER_HOST:-127.0.0.1}"
port="${APISERVER_PORT:-${default_verify_port}}"
log_file="${APISERVER_LOG_FILE:-$(ingate::hack::build_dir)/ingate-apiserver-kubectl.log}"
kubeconfig_file="${KUBECONFIG_OUTPUT:-$(ingate::hack::build_dir)/ingate-apiserver.kubeconfig}"
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

APISERVER_HOST="${host}" APISERVER_PORT="${port}" KUBECONFIG_OUTPUT="${kubeconfig_file}" ./tools/hack/write-apiserver-kubeconfig.sh >/dev/null

kubectl --kubeconfig="${kubeconfig_file}" --context="${admin_context}" delete gateway "${gateway_name}" --ignore-not-found >/dev/null 2>&1 || true
kubectl --kubeconfig="${kubeconfig_file}" --context="${admin_context}" delete gateway "${viewer_create_gateway_name}" --ignore-not-found >/dev/null 2>&1 || true

cat <<YAML | kubectl --validate=false --kubeconfig="${kubeconfig_file}" --context="${admin_context}" create -f - >/dev/null
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
metadata:
  name: ${gateway_name}
spec:
  listeners:
  - name: web
    protocol: HTTP
    port: 80
    hostnames:
    - api.example.com
    - admin.example.com
YAML

viewer_get_output="$(kubectl --kubeconfig="${kubeconfig_file}" --context="${viewer_context}" get gateways 2>&1)"
if [[ "${viewer_get_output}" != *"LISTENERS"* ]] || [[ "${viewer_get_output}" != *"HOSTNAMES"* ]] || [[ "${viewer_get_output}" != *"${gateway_name}"* ]]; then
  echo "expected kubectl get output to include custom table headers and gateway row" >&2
  printf '%s\n' "${viewer_get_output}" >&2
  exit 1
fi

viewer_create_output_file="$(mktemp)"
trap 'rm -f "${viewer_create_output_file}"; cleanup' EXIT
if cat <<YAML | kubectl --validate=false --kubeconfig="${kubeconfig_file}" --context="${viewer_context}" create -f - >"${viewer_create_output_file}" 2>&1
apiVersion: gateway.ingate.io/v1alpha1
kind: Gateway
metadata:
  name: ${viewer_create_gateway_name}
spec:
  listeners:
  - name: web
    protocol: HTTP
    port: 80
YAML
then
  echo "expected viewer kubectl create to fail" >&2
  sed -n '1,160p' "${viewer_create_output_file}" >&2 || true
  exit 1
fi

if ! grep -q 'Forbidden' "${viewer_create_output_file}"; then
  echo "expected viewer kubectl create error to mention Forbidden" >&2
  sed -n '1,160p' "${viewer_create_output_file}" >&2 || true
  exit 1
fi

printf 'KUBECONFIG_WRITE_OK=yes\n'
printf 'KUBECTL_GET_TABLE_OK=yes\n'
printf 'KUBECTL_VIEWER_CREATE_FORBIDDEN=yes\n'
