#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

BUILD_DIR="$(ingate::hack::build_dir)"
BINS="${BINS:-ingate-apiserver ingate-admin-api ingate-controller-manager ingate-xds-server ingatectl}"

mkdir -p "${BUILD_DIR}"

GIT_VERSION="${GIT_VERSION:-$(ingate::hack::git_version)}"
GIT_COMMIT="${GIT_COMMIT:-$(ingate::hack::git_commit)}"
BUILD_DATE="${BUILD_DATE:-$(ingate::hack::build_date)}"
LDFLAGS="${LDFLAGS:-} -X github.com/lgc202/ingate/pkg/version.GitVersion=${GIT_VERSION} -X github.com/lgc202/ingate/pkg/version.GitCommit=${GIT_COMMIT} -X github.com/lgc202/ingate/pkg/version.BuildDate=${BUILD_DATE}"

for component in ${BINS}; do
  binary_name=$(ingate::hack::binary_name_for_component "${component}")
  command_path=$(ingate::hack::command_path_for_component "${component}")
  echo "building ${binary_name} -> ${BUILD_DIR}/${binary_name}"
  go build -ldflags "${LDFLAGS}" -o "${BUILD_DIR}/${binary_name}" "${command_path}"
done
