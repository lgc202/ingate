#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

go run k8s.io/code-generator/cmd/deepcopy-gen@"${CODEGEN_VERSION}" \
  --go-header-file="${BOILERPLATE_FILE}" \
  --output-file zz_generated.deepcopy.go \
  "${MODULE}/pkg/apis/gateway/v1alpha1" \
  "${MODULE}/pkg/apis/policy/v1alpha1"

go run k8s.io/code-generator/cmd/defaulter-gen@"${CODEGEN_VERSION}" \
  --go-header-file="${BOILERPLATE_FILE}" \
  --output-file zz_generated.defaults.go \
  "${MODULE}/pkg/apis/gateway/v1alpha1" \
  "${MODULE}/pkg/apis/policy/v1alpha1"

OPENAPI_GEN_VERSION=$(cd "${ROOT_DIR}" && go list -m -f '{{.Version}}' k8s.io/kube-openapi)
OPENAPI_PKG="${MODULE}/pkg/generated/openapi"
OPENAPI_MODEL_BUILD_DIR="${BUILD_DIR}/openapi-modelnames"

rm -rf "${OPENAPI_MODEL_BUILD_DIR}"

go run k8s.io/kube-openapi/cmd/openapi-gen@"${OPENAPI_GEN_VERSION}" \
  --go-header-file "${BOILERPLATE_FILE}" \
  --output-file zz_generated.openapi.go \
  --output-model-name-file zz_generated.model_name.go \
  --output-dir "${OPENAPI_MODEL_BUILD_DIR}" \
  --output-pkg "${OPENAPI_PKG}" \
  "./pkg/apis/gateway/v1alpha1" \
  "./pkg/apis/policy/v1alpha1"

# The first pass only needs zz_generated.model_name.go files in API packages.
# openapi-gen also writes a temporary OpenAPI file to --output-dir; keep it out
# of the build output to avoid confusing it with the real schema generated below.
rm -rf "${OPENAPI_MODEL_BUILD_DIR}"

go run k8s.io/kube-openapi/cmd/openapi-gen@"${OPENAPI_GEN_VERSION}" \
  --go-header-file "${BOILERPLATE_FILE}" \
  --output-file zz_generated.openapi.go \
  --output-dir pkg/generated/openapi \
  --report-filename "${BUILD_DIR}/openapi-api-rules.report" \
  --output-pkg "${OPENAPI_PKG}" \
  "./pkg/apis/gateway/v1alpha1" \
  "./pkg/apis/policy/v1alpha1" \
  k8s.io/apimachinery/pkg/apis/meta/v1 \
  k8s.io/apimachinery/pkg/runtime \
  k8s.io/apimachinery/pkg/version
