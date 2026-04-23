#!/usr/bin/env bash
set -euo pipefail

LOCAL_ARCH=$(uname -m)
if [[ -n "${TARGET_ARCH:-}" ]]; then
  TARGET_ARCH_VALUE="${TARGET_ARCH}"
elif [[ "${LOCAL_ARCH}" == "x86_64" ]]; then
  TARGET_ARCH_VALUE="amd64"
elif [[ "${LOCAL_ARCH}" == "arm64" || "${LOCAL_ARCH}" == "aarch64" || "${LOCAL_ARCH}" == arm64* || "${LOCAL_ARCH}" == armv8* ]]; then
  TARGET_ARCH_VALUE="arm64"
else
  echo "unsupported architecture: ${LOCAL_ARCH}" >&2
  exit 1
fi

LOCAL_OS=$(uname)
if [[ -n "${TARGET_OS:-}" ]]; then
  TARGET_OS_VALUE="${TARGET_OS}"
elif [[ "${LOCAL_OS}" == "Darwin" ]]; then
  TARGET_OS_VALUE="darwin"
elif [[ "${LOCAL_OS}" == "Linux" ]]; then
  TARGET_OS_VALUE="linux"
else
  echo "unsupported operating system: ${LOCAL_OS}" >&2
  exit 1
fi

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BUILD_DIR_VALUE="${BUILD_DIR:-${ROOT_DIR}/_output/${TARGET_OS_VALUE}_${TARGET_ARCH_VALUE}}"

if [[ "${1:-}" == "envfile" ]]; then
  echo "LOCAL_OS=${LOCAL_OS}"
  echo "LOCAL_ARCH=${LOCAL_ARCH}"
  echo "TARGET_OS=${TARGET_OS_VALUE}"
  echo "TARGET_ARCH=${TARGET_ARCH_VALUE}"
  echo "BUILD_DIR=${BUILD_DIR_VALUE}"
  echo "LC_ALL=C"
  echo "LANG=C"
  echo "LC_CTYPE=C"
  exit 0
fi

export LOCAL_OS="${LOCAL_OS}"
export LOCAL_ARCH="${LOCAL_ARCH}"
export TARGET_OS="${TARGET_OS_VALUE}"
export TARGET_ARCH="${TARGET_ARCH_VALUE}"
export BUILD_DIR="${BUILD_DIR_VALUE}"
export LC_ALL=C
export LANG=C
export LC_CTYPE=C
