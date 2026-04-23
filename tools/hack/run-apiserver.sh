#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ingate::hack::require_root

cd "${ROOT_DIR}"

APISERVER_BIN="${APISERVER_BIN:-}"
ETCD_SERVERS="${ETCD_SERVERS:-http://127.0.0.1:2379}"

if [[ -z "${APISERVER_BIN}" ]]; then
  APISERVER_BIN="$(ingate::hack::build_dir)/ingate-apiserver"
fi

if [[ ! -x "${APISERVER_BIN}" ]]; then
  echo "apiserver binary not found: ${APISERVER_BIN}" >&2
  echo "run: make build-apiserver" >&2
  exit 1
fi

exec "${APISERVER_BIN}" --etcd-servers="${ETCD_SERVERS}" "$@"
