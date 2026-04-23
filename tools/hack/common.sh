#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
MODULE=github.com/lgc202/ingate
CODEGEN_VERSION=v0.35.3
BOILERPLATE_FILE="${ROOT_DIR}/tools/hack/boilerplate.go.txt"
HOST_OS=$(cd "${ROOT_DIR}" && go env GOOS)
HOST_ARCH=$(cd "${ROOT_DIR}" && go env GOARCH)
GO_BIN_DIR=$(cd "${ROOT_DIR}" && go env GOBIN)
if [[ -z "${GO_BIN_DIR}" ]]; then
  GO_BIN_DIR="$(cd "${ROOT_DIR}" && go env GOPATH)/bin"
fi
DEFAULT_BUILD_DIR="${ROOT_DIR}/_output/${HOST_OS}_${HOST_ARCH}"
export PATH="${GO_BIN_DIR}:${PATH}"

function ingate::hack::require_root() {
  if [[ ! -f "${ROOT_DIR}/go.mod" ]]; then
    echo "go.mod not found under ${ROOT_DIR}" >&2
    exit 1
  fi
}

function ingate::hack::build_dir() {
  echo "${BUILD_DIR:-${DEFAULT_BUILD_DIR}}"
}

function ingate::hack::binary_name_for_component() {
  case "$1" in
    apiserver|ingate-apiserver) echo "ingate-apiserver" ;;
    admin-api|ingate-admin-api) echo "ingate-admin-api" ;;
    controller-manager|ingate-controller-manager) echo "ingate-controller-manager" ;;
    xds-server|ingate-xds-server) echo "ingate-xds-server" ;;
    ingatectl) echo "ingatectl" ;;
    *)
      echo "unknown component: $1" >&2
      return 1
      ;;
  esac
}

function ingate::hack::command_path_for_component() {
  case "$1" in
    apiserver|ingate-apiserver) echo "./cmd/apiserver" ;;
    admin-api|ingate-admin-api) echo "./cmd/admin-api" ;;
    controller-manager|ingate-controller-manager) echo "./cmd/controller-manager" ;;
    xds-server|ingate-xds-server) echo "./cmd/xds-server" ;;
    ingatectl) echo "./cmd/ingatectl" ;;
    *)
      echo "unknown component: $1" >&2
      return 1
      ;;
  esac
}

function ingate::hack::codegen_pkg() {
  local gomodcache
  gomodcache=$(cd "${ROOT_DIR}" && go env GOMODCACHE)
  echo "${gomodcache}/k8s.io/code-generator@${CODEGEN_VERSION}"
}

function ingate::hack::require_codegen_pkg() {
  local codegen_pkg
  codegen_pkg=$(ingate::hack::codegen_pkg)
  if [[ ! -f "${codegen_pkg}/kube_codegen.sh" ]]; then
    echo "k8s.io/code-generator ${CODEGEN_VERSION} not found in module cache: ${codegen_pkg}" >&2
    echo "run: go run k8s.io/code-generator/cmd/deepcopy-gen@${CODEGEN_VERSION} --help >/dev/null" >&2
    exit 1
  fi
  echo "${codegen_pkg}"
}

function ingate::hack::git_version() {
  if git -C "${ROOT_DIR}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo dev
  else
    echo dev
  fi
}

function ingate::hack::git_commit() {
  if git -C "${ROOT_DIR}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "${ROOT_DIR}" rev-parse HEAD 2>/dev/null || echo unknown
  else
    echo unknown
  fi
}

function ingate::hack::build_date() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

function ingate::hack::wait_for_https_ready() {
  local url="$1"
  local attempts="${2:-30}"
  local delay_seconds="${3:-1}"

  for _attempt in $(seq 1 "${attempts}"); do
    if curl --noproxy '*' -kfsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay_seconds}"
  done

  return 1
}
