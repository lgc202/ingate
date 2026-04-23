#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

readonly default_host="127.0.0.1"
readonly default_port="18443"
readonly default_output_file="${ROOT_DIR}/_output/kubeconfig/ingate-apiserver.kubeconfig"
readonly default_cluster_name="ingate-local"
readonly default_admin_user_name="ingate-admin"
readonly default_viewer_user_name="ingate-viewer"
readonly default_admin_token="ingate-dev-admin-token"
readonly default_viewer_token="ingate-dev-viewer-token"
readonly default_admin_context="ingate-admin"
readonly default_viewer_context="ingate-viewer"

host="${APISERVER_HOST:-${default_host}}"
port="${APISERVER_PORT:-${default_port}}"
server="${APISERVER_SERVER:-https://${host}:${port}}"
output_file="${KUBECONFIG_OUTPUT:-${default_output_file}}"
cluster_name="${KUBECONFIG_CLUSTER_NAME:-${default_cluster_name}}"
admin_user_name="${KUBECONFIG_ADMIN_USER_NAME:-${default_admin_user_name}}"
viewer_user_name="${KUBECONFIG_VIEWER_USER_NAME:-${default_viewer_user_name}}"
admin_token="${APISERVER_AUTH_ADMIN_TOKEN:-${default_admin_token}}"
viewer_token="${APISERVER_AUTH_VIEWER_TOKEN:-${default_viewer_token}}"
current_context="${KUBECONFIG_CURRENT_CONTEXT:-${default_admin_context}}"

mkdir -p "$(dirname "${output_file}")"
cat > "${output_file}" <<KUBECONFIG
apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: ${server}
  name: ${cluster_name}
users:
- name: ${admin_user_name}
  user:
    token: ${admin_token}
- name: ${viewer_user_name}
  user:
    token: ${viewer_token}
contexts:
- context:
    cluster: ${cluster_name}
    user: ${admin_user_name}
  name: ${default_admin_context}
- context:
    cluster: ${cluster_name}
    user: ${viewer_user_name}
  name: ${default_viewer_context}
current-context: ${current_context}
KUBECONFIG

echo "kubeconfig written to ${output_file}"
