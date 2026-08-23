#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"

usage() {
  echo "Usage: restore.sh BACKUP [--yes]"
}

[[ $# -ge 1 ]] || { usage >&2; exit 2; }
ARCHIVE="$1"
shift
ASSUME_YES=false
if [[ "${1:-}" == "--yes" ]]; then
  ASSUME_YES=true
  shift
fi
[[ $# -eq 0 ]] || fail "unknown restore argument: $1"
[[ -f "$ARCHIVE" ]] || fail "backup file does not exist: $ARCHIVE"
require_installation
validate_tar_archive "$ARCHIVE"

if [[ "$ASSUME_YES" != true ]]; then
  [[ -t 0 ]] || fail "interactive confirmation is required; rerun with --yes"
  echo "Restore will replace the current Ingate configuration and all persisted data."
  read -r -p 'Type "restore" to continue: ' confirmation
  [[ "$confirmation" == "restore" ]] || fail "restore cancelled"
fi

STAGING="$(mktemp -d "${TMPDIR:-/tmp}/ingate-restore.XXXXXX")"
cleanup() {
  rm -rf "$STAGING"
}
trap cleanup EXIT

tar -xzf "$ARCHIVE" -C "$STAGING" --no-same-owner
CONTENT="$STAGING/ingate-backup"
[[ -f "$CONTENT/install/VERSION" && -f "$CONTENT/install/.env" &&
  -f "$CONTENT/install/compose.yaml" && -f "$CONTENT/install/README.md" &&
  -d "$CONTENT/install/bin" && -d "$CONTENT/install/docker" &&
  -d "$CONTENT/volumes" ]] || fail "invalid Ingate backup"

for logical_name in "${PERSISTENT_VOLUMES[@]}"; do
  archive="$CONTENT/volumes/$logical_name.tar.gz"
  [[ ! -f "$archive" ]] || validate_tar_archive "$archive"
done

"${COMPOSE[@]}" down --remove-orphans

for logical_name in "${PERSISTENT_VOLUMES[@]}"; do
  archive="$CONTENT/volumes/$logical_name.tar.gz"
  [[ -f "$archive" ]] || continue
  volume_name="ingate_${logical_name}"
  docker volume create "$volume_name" >/dev/null
  docker run --rm \
    -v "$volume_name:/target" \
    -v "$CONTENT/volumes:/backup:ro" \
    "$BACKUP_IMAGE" \
    sh -ec "find /target -mindepth 1 -maxdepth 1 -exec rm -rf {} \\; && tar -xzf /backup/$logical_name.tar.gz -C /target"
done

rm -rf "$ROOT/bin" "$ROOT/docker"
cp "$CONTENT/install/VERSION" "$CONTENT/install/.env" "$CONTENT/install/compose.yaml" "$CONTENT/install/README.md" "$ROOT/"
cp -R "$CONTENT/install/bin" "$CONTENT/install/docker" "$ROOT/"

"${COMPOSE[@]}" up -d --wait --wait-timeout "${INGATE_WAIT_TIMEOUT:-300}"
echo "Ingate restored from $ARCHIVE"
