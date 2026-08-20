#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# go list 不会主动解压只有模块元数据的依赖，先按 go.mod 固定版本准备源码目录
go mod download k8s.io/code-generator
readonly codegen_pkg="$(cd "${repo_root}" && go list -m -f '{{.Dir}}' k8s.io/code-generator)"

source "${codegen_pkg}/kube_codegen.sh"

kube::codegen::gen_helpers \
  --boilerplate "${repo_root}/hack/boilerplate.go.txt" \
  "${repo_root}/pkg/apis"

kube::codegen::gen_client \
  --with-watch \
  --output-dir "${repo_root}/pkg/generated" \
  --output-pkg "github.com/lgc202/ingate/pkg/generated" \
  --boilerplate "${repo_root}/hack/boilerplate.go.txt" \
  "${repo_root}/pkg/apis"

# client-gen 默认生成测试专用 fake client；当前项目不维护单元测试，不保留没有生产调用方的生成包
rm -rf \
  "${repo_root}/pkg/generated/clientset/versioned/fake" \
  "${repo_root}/pkg/generated/clientset/versioned/typed/gateway/v1/fake"

kube::codegen::gen_openapi \
  --output-dir "${repo_root}/pkg/generated/openapi" \
  --output-pkg "github.com/lgc202/ingate/pkg/generated/openapi" \
  --report-filename "${repo_root}/hack/openapi/api-rule-violations.report" \
  --boilerplate "${repo_root}/hack/boilerplate.go.txt" \
  "${repo_root}/pkg/apis"
