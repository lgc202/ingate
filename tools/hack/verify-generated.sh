#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

./tools/hack/generate-all.sh

required_files=(
  pkg/apis/gateway/v1alpha1/zz_generated.deepcopy.go
  pkg/apis/gateway/v1alpha1/zz_generated.defaults.go
  pkg/apis/gateway/v1alpha1/zz_generated.model_name.go
  pkg/apis/policy/v1alpha1/zz_generated.deepcopy.go
  pkg/apis/policy/v1alpha1/zz_generated.defaults.go
  pkg/apis/policy/v1alpha1/zz_generated.model_name.go
  pkg/generated/clientset/versioned/clientset.go
  pkg/generated/clientset/versioned/typed/gateway/v1alpha1/gateway_client.go
  pkg/generated/clientset/versioned/typed/policy/v1alpha1/policy_client.go
  pkg/generated/informers/externalversions/factory.go
  pkg/generated/listers/gateway/v1alpha1/gateway.go
  pkg/generated/listers/policy/v1alpha1/authpolicy.go
  pkg/generated/proto/ingate/configsync/v1/configsync.pb.go
  pkg/generated/proto/ingate/configsync/v1/configsync_grpc.pb.go
  pkg/generated/proto/ingate/discovery/v1/discovery.pb.go
  pkg/generated/proto/ingate/discovery/v1/discovery_grpc.pb.go
  pkg/generated/openapi/zz_generated.openapi.go
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "missing generated file: $file" >&2
    exit 1
  fi
done

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  if ! git diff --quiet -- pkg/apis pkg/generated; then
    echo "generated files are out of date; run make generate" >&2
    git --no-pager diff -- pkg/apis pkg/generated
    exit 1
  fi
fi

echo "verify-generated: generated files are up to date"
