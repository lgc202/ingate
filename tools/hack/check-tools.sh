#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

missing=0

check_cmd() {
  local name="$1"
  if ! command -v "${name}" >/dev/null 2>&1; then
    echo "missing required tool: ${name}" >&2
    missing=1
  fi
}

check_cmd go
check_cmd etcd

codegen_pkg=$(ingate::hack::codegen_pkg)
if [[ ! -f "${codegen_pkg}/kube_codegen.sh" ]]; then
  echo "missing code-generator helper: ${codegen_pkg}/kube_codegen.sh" >&2
  echo "run: go run k8s.io/code-generator/cmd/deepcopy-gen@${CODEGEN_VERSION} --help >/dev/null" >&2
  missing=1
fi

check_cmd protoc
check_cmd protoc-gen-go
check_cmd protoc-gen-go-grpc

if [[ ${missing} -ne 0 ]]; then
  exit 1
fi

echo "check-tools: required tools are available"
