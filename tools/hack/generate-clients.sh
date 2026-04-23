#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

CODEGEN_PKG=$(ingate::hack::require_codegen_pkg)

# shellcheck source=/dev/null
source "${CODEGEN_PKG}/kube_codegen.sh"

rm -rf \
  "${ROOT_DIR}/pkg/generated/clientset" \
  "${ROOT_DIR}/pkg/generated/informers" \
  "${ROOT_DIR}/pkg/generated/listers"
mkdir -p "${ROOT_DIR}/pkg/generated"

kube::codegen::gen_client \
  --output-dir "${ROOT_DIR}/pkg/generated" \
  --output-pkg "${MODULE}/pkg/generated" \
  --with-watch \
  --boilerplate "${BOILERPLATE_FILE}" \
  "${ROOT_DIR}/pkg/apis"
