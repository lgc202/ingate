#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
readonly codegen_pkg="$(cd "${repo_root}" && go list -m -f '{{.Dir}}' k8s.io/code-generator)"

source "${codegen_pkg}/kube_codegen.sh"

kube::codegen::gen_helpers \
  --boilerplate "${repo_root}/hack/boilerplate.go.txt" \
  "${repo_root}/internal/core/resource"
