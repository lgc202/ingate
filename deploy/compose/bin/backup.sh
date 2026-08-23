#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"

require_installation

BACKUP_DIR="${1:-$ROOT/backups}"
mkdir -p "$BACKUP_DIR"
BACKUP_DIR="$(cd -- "$BACKUP_DIR" && pwd -P)"
TIMESTAMP="$(date '+%Y%m%d-%H%M%S')"
ARCHIVE="$BACKUP_DIR/ingate-$TIMESTAMP.tar.gz"
STAGING="$(mktemp -d "${TMPDIR:-/tmp}/ingate-backup.XXXXXX")"
CONTENT="$STAGING/ingate-backup"
WAS_RUNNING=false

cleanup() {
  rm -rf "$STAGING"
  if [[ "$WAS_RUNNING" == true ]]; then
    "${COMPOSE[@]}" up -d --wait --wait-timeout "${INGATE_WAIT_TIMEOUT:-300}" >/dev/null
  fi
}
trap cleanup EXIT

if [[ -n "$("${COMPOSE[@]}" ps --status running -q)" ]]; then
  WAS_RUNNING=true
fi

echo "Stopping Ingate writes for a consistent backup..."
"${COMPOSE[@]}" stop >/dev/null

mkdir -p "$CONTENT/install" "$CONTENT/volumes"
cp "$ROOT/VERSION" "$ROOT/.env" "$ROOT/compose.yaml" "$ROOT/README.md" "$CONTENT/install/"
cp -R "$ROOT/bin" "$ROOT/docker" "$CONTENT/install/"

for logical_name in "${PERSISTENT_VOLUMES[@]}"; do
  volume_name="ingate_${logical_name}"
  if ! docker volume inspect "$volume_name" >/dev/null 2>&1; then
    continue
  fi
  echo "Backing up $logical_name..."
  docker run --rm \
    -v "$volume_name:/source:ro" \
    -v "$CONTENT/volumes:/backup" \
    "$BACKUP_IMAGE" \
    tar -czf "/backup/$logical_name.tar.gz" -C /source .
done

tar -czf "$ARCHIVE" -C "$STAGING" ingate-backup
echo "Backup created: $ARCHIVE"
