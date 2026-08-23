#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"

readonly REPOSITORY="lgc202/ingate"
readonly ARCHIVE_NAME="ingate-compose.tar.gz"

[[ $# -eq 1 ]] || fail "usage: upgrade.sh vX.Y.Z"
TARGET_VERSION="$1"
[[ "$TARGET_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || fail "invalid release version: $TARGET_VERSION"
require_installation

CURRENT_VERSION="$(cat "$ROOT/VERSION")"
[[ "$TARGET_VERSION" != "$CURRENT_VERSION" ]] || fail "Ingate $TARGET_VERSION is already installed"

TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ingate-upgrade.XXXXXX")"
cleanup() {
  rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

DOWNLOAD_ROOT="https://github.com/$REPOSITORY/releases/download/$TARGET_VERSION"
curl --fail --location --silent --show-error --retry 3 "$DOWNLOAD_ROOT/$ARCHIVE_NAME" --output "$TEMP_DIR/$ARCHIVE_NAME"
curl --fail --location --silent --show-error --retry 3 "$DOWNLOAD_ROOT/$ARCHIVE_NAME.sha256" --output "$TEMP_DIR/$ARCHIVE_NAME.sha256"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$TEMP_DIR" && sha256sum --check "$ARCHIVE_NAME.sha256")
else
  (cd "$TEMP_DIR" && shasum -a 256 --check "$ARCHIVE_NAME.sha256")
fi

validate_tar_archive "$TEMP_DIR/$ARCHIVE_NAME"
tar -xzf "$TEMP_DIR/$ARCHIVE_NAME" -C "$TEMP_DIR" --no-same-owner
PACKAGE="$TEMP_DIR/ingate"
[[ -f "$PACKAGE/VERSION" && -f "$PACKAGE/compose.yaml" && -f "$PACKAGE/README.md" &&
  -d "$PACKAGE/bin" && -d "$PACKAGE/docker" ]] || fail "invalid Ingate release package"
[[ "$(cat "$PACKAGE/VERSION")" == "$TARGET_VERSION" ]] || fail "release package version does not match $TARGET_VERSION"

BACKUP_OUTPUT="$($ROOT/bin/backup.sh | awk -F': ' '/^Backup created:/ { print $2 }')"
[[ -n "$BACKUP_OUTPUT" ]] || fail "upgrade backup was not created"

rm -rf "$ROOT/bin" "$ROOT/docker"
cp "$PACKAGE/VERSION" "$PACKAGE/compose.yaml" "$PACKAGE/README.md" "$ROOT/"
cp -R "$PACKAGE/bin" "$PACKAGE/docker" "$ROOT/"
set_env_value INGATE_VERSION "$TARGET_VERSION"

if ! "${COMPOSE[@]}" pull || ! "${COMPOSE[@]}" up -d --wait --wait-timeout "${INGATE_WAIT_TIMEOUT:-300}"; then
  echo "Upgrade failed. Restore the previous version with:" >&2
  echo "  $ROOT/bin/restore.sh $BACKUP_OUTPUT" >&2
  exit 1
fi

echo "Ingate upgraded from $CURRENT_VERSION to $TARGET_VERSION"
echo "Pre-upgrade backup: $BACKUP_OUTPUT"
