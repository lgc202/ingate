#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

COMPOSE_BUILD_DIR="${ROOT_DIR}/_output/compose"
COMPOSE_TARGET_OS="${COMPOSE_TARGET_OS:-linux}"
COMPOSE_TARGET_ARCH="${COMPOSE_TARGET_ARCH:-$(go env GOARCH)}"
CONSOLE_WORKSPACE="${CONSOLE_WORKSPACE:-$(cd "${ROOT_DIR}/.." && pwd)/ingate-console}"
CONSOLE_BUILD_DIR="${COMPOSE_BUILD_DIR}/console"

mkdir -p "${COMPOSE_BUILD_DIR}"

build_binary() {
  local output_name="$1"
  local package_path="$2"

  echo "BUILD_BINARY=${output_name}"
  CGO_ENABLED=0 GOOS="${COMPOSE_TARGET_OS}" GOARCH="${COMPOSE_TARGET_ARCH}" go build -o "${COMPOSE_BUILD_DIR}/${output_name}" "${package_path}"
}

build_image() {
  local dockerfile="$1"
  local image="$2"
  local context="${3:-.}"

  echo "BUILD_IMAGE=${image}"
  docker build -f "${dockerfile}" -t "${image}" "${context}"
}

require_console_dist() {
  if [[ ! -f "${CONSOLE_WORKSPACE}/dist/index.html" ]]; then
    echo "console build artifact missing: ${CONSOLE_WORKSPACE}/dist/index.html" >&2
    echo "run 'npm run build' in ${CONSOLE_WORKSPACE} before compose packaging" >&2
    exit 1
  fi
}

stage_console_dist() {
  mkdir -p "${CONSOLE_BUILD_DIR}/dist"
  cp -R "${CONSOLE_WORKSPACE}/dist/." "${CONSOLE_BUILD_DIR}/dist/"
}

build_binary "ingate-apiserver" "./cmd/apiserver"
build_binary "ingate-controller-manager" "./cmd/controller-manager"
build_binary "ingate-xds-server" "./cmd/xds-server"
build_binary "ingate-admin-api" "./cmd/admin-api"

build_image "build/package/apiserver.Dockerfile" "ingate/apiserver:dev"
build_image "build/package/controller-manager.Dockerfile" "ingate/controller-manager:dev"
build_image "build/package/xds-server.Dockerfile" "ingate/xds-server:dev"
build_image "build/package/admin-api.Dockerfile" "ingate/admin-api:dev"
build_image "build/package/sample-backend.Dockerfile" "ingate/sample-backend:dev"
require_console_dist
stage_console_dist
build_image "build/package/console.Dockerfile" "ingate/console:dev"
