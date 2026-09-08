#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
readonly BACKUP_IMAGE="busybox:1.37.0"
readonly PERSISTENT_VOLUMES=(
  als-data
  apiserver-certificates
  clickhouse-data
  controller-wasm
  etcd-data
  kafka-data
  mysql-data
  redis-data
)

fail() {
  echo "Error: $*" >&2
  exit 1
}

require_installation() {
  [[ -f "$ROOT/VERSION" && -f "$ROOT/compose.yaml" && -f "$ROOT/.env" ]] ||
    fail "invalid Ingate installation: $ROOT"
}

env_value() {
  local key="$1"
  awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "$ROOT/.env"
}

set_env_value() {
  local key="$1"
  local value="$2"
  local temporary
  temporary="$(mktemp "$ROOT/.env.XXXXXX")"
  if ! awk -v key="$key" -v value="$value" '
    BEGIN { found = 0 }
    index($0, key "=") == 1 { print key "=" value; found = 1; next }
    { print }
    END { if (!found) print key "=" value }
  ' "$ROOT/.env" > "$temporary"; then
    rm -f "$temporary"
    return 1
  fi
  chmod 600 "$temporary"
  mv "$temporary" "$ROOT/.env"
}

# COMPOSE 是所有安装脚本共享的部署入口。观测栈一旦在 .env 中启用，
# 日常启停、备份恢复和升级必须继续使用同一组 Compose 文件，避免 ALS 被重建为无遥测配置。
COMPOSE=(
  docker compose
  --project-directory "$ROOT"
  --env-file "$ROOT/.env"
  -f "$ROOT/compose.yaml"
)
OBSERVABILITY_COMPOSE=(
  "${COMPOSE[@]}"
  -f "$ROOT/docker/observability/compose.yaml"
)
if [[ -f "$ROOT/.env" && -f "$ROOT/docker/observability/compose.yaml" &&
  "$(env_value INGATE_OBSERVABILITY_ENABLED)" == "true" ]]; then
  COMPOSE=("${OBSERVABILITY_COMPOSE[@]}")
fi
readonly -a COMPOSE
readonly -a OBSERVABILITY_COMPOSE

random_hex() {
  od -An -N "$1" -tx1 /dev/urandom | tr -d '[:space:]'
}

validate_tar_archive() {
  local archive="$1"
  local entries
  local entry
  local component
  local -a components

  entries="$(tar -tzf "$archive")" || fail "cannot read archive: $archive"
  [[ -n "$entries" ]] || fail "archive is empty: $archive"
  while IFS= read -r entry; do
    [[ "$entry" != /* ]] || fail "archive contains an absolute path: $entry"
    IFS='/' read -r -a components <<< "$entry"
    for component in "${components[@]}"; do
      [[ "$component" != ".." ]] || fail "archive contains a parent path: $entry"
    done
  done <<< "$entries"
}
