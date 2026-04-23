#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

PROTO_FILES=$(find proto -type f -name '*.proto' | sort)
if [[ -z "${PROTO_FILES}" ]]; then
  echo "generate-proto: no proto source files found"
  exit 0
fi

if ! command -v protoc >/dev/null 2>&1; then
  echo "generate-proto: missing required tool: protoc" >&2
  exit 1
fi

if ! command -v protoc-gen-go >/dev/null 2>&1; then
  echo "generate-proto: missing required tool: protoc-gen-go" >&2
  exit 1
fi

if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo "generate-proto: missing required tool: protoc-gen-go-grpc" >&2
  exit 1
fi

rm -rf pkg/generated/proto
mkdir -p pkg/generated/proto

protoc \
  -I proto \
  --go_out=. \
  --go_opt=module=github.com/lgc202/ingate \
  --go-grpc_out=. \
  --go-grpc_opt=module=github.com/lgc202/ingate \
  ${PROTO_FILES}

echo "generate-proto: generated Go proto code"
