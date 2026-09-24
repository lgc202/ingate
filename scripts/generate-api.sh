#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "${repository_root}"

go tool deepcopy-gen \
    --output-file zz_generated.deepcopy.go \
    ./pkg/apis/gateway \
    ./pkg/apis/gateway/v1alpha1

go tool conversion-gen \
    --output-file zz_generated.conversion.go \
    ./pkg/apis/gateway/v1alpha1

openapi_output="${repository_root}/pkg/generated/openapi"
openapi_package="github.com/lgc202/ingate/pkg/generated/openapi"
openapi_violations="${repository_root}/api/api-rules/violation_exceptions.list"
openapi_report="$(mktemp "${TMPDIR:-/tmp}/ingate-openapi-violations.XXXXXX")"
trap 'rm -f "${openapi_report}"' EXIT

go tool openapi-gen \
    --output-dir "${openapi_output}" \
    --output-pkg "${openapi_package}" \
    --output-file zz_generated.openapi.go \
    --output-model-name-file zz_generated.model_name.go \
    --report-filename "${openapi_report}" \
    --readonly-pkg k8s.io/apimachinery/pkg/api/resource \
    --readonly-pkg k8s.io/apimachinery/pkg/apis/meta/v1 \
    --readonly-pkg k8s.io/apimachinery/pkg/runtime \
    --readonly-pkg k8s.io/apimachinery/pkg/types \
    --readonly-pkg k8s.io/apimachinery/pkg/util/intstr \
    --readonly-pkg k8s.io/apimachinery/pkg/version \
    --readonly-pkg time \
    k8s.io/apimachinery/pkg/api/resource \
    k8s.io/apimachinery/pkg/apis/meta/v1 \
    k8s.io/apimachinery/pkg/runtime \
    k8s.io/apimachinery/pkg/version \
    ./pkg/apis/gateway/v1alpha1

if ! diff -u "${openapi_violations}" "${openapi_report}"; then
    echo "OpenAPI rule violations changed; review them before updating the baseline" >&2
    exit 1
fi
