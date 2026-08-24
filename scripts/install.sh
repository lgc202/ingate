#!/usr/bin/env bash

set -Eeuo pipefail

readonly REPOSITORY="lgc202/ingate"
readonly ARCHIVE_NAME="ingate-compose.tar.gz"

VERSION="${INGATE_VERSION:-latest}"
DESTINATION=""
START_AFTER_INSTALL=true

usage() {
  cat <<'EOF'
Usage: install.sh [DIR] [OPTIONS]

Install the Ingate Docker Compose distribution into DIR (./ingate by default).

Options:
  --version VERSION  Install a fixed release tag such as vX.Y.Z
  --no-start         Download and configure Ingate without starting containers
  -h, --help        Show this help
EOF
}

fail() {
  echo "Error: $*" >&2
  exit 1
}

random_hex() {
  od -An -N "$1" -tx1 /dev/urandom | tr -d '[:space:]'
}

set_env() {
  local key="$1"
  local value="$2"
  local file="$3"
  local temporary="${file}.tmp"
  awk -v key="$key" -v value="$value" '
    BEGIN { found = 0 }
    index($0, key "=") == 1 { print key "=" value; found = 1; next }
    { print }
    END { if (!found) print key "=" value }
  ' "$file" > "$temporary"
  mv "$temporary" "$file"
}

if [[ $# -gt 0 && "$1" != -* ]]; then
  DESTINATION="$1"
  shift
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --no-start)
      START_AFTER_INSTALL=false
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

DESTINATION="${DESTINATION:-$PWD/ingate}"

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"
command -v docker >/dev/null 2>&1 || fail "Docker is required"
docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is required"
docker info >/dev/null 2>&1 || fail "Docker daemon is unavailable or inaccessible"
docker compose up --help | grep -q -- "--wait" || fail "Docker Compose with 'up --wait' support is required"

if [[ "$VERSION" != "latest" && ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  fail "invalid release version: $VERSION"
fi

if [[ -d "$DESTINATION" ]]; then
  if [[ -n "$(find "$DESTINATION" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    fail "installation directory is not empty: $DESTINATION"
  fi
fi

mkdir -p "$DESTINATION" || fail "cannot create installation directory: $DESTINATION"
DESTINATION="$(cd -- "$DESTINATION" && pwd -P)"

TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ingate-installer.XXXXXX")"
cleanup() {
  rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

if [[ "$VERSION" == "latest" ]]; then
  DOWNLOAD_ROOT="https://github.com/$REPOSITORY/releases/latest/download"
else
  DOWNLOAD_ROOT="https://github.com/$REPOSITORY/releases/download/$VERSION"
fi

echo "Downloading Ingate $VERSION..."
curl --fail --location --silent --show-error --retry 3 \
  "$DOWNLOAD_ROOT/$ARCHIVE_NAME" \
  --output "$TEMP_DIR/$ARCHIVE_NAME"
curl --fail --location --silent --show-error --retry 3 \
  "$DOWNLOAD_ROOT/$ARCHIVE_NAME.sha256" \
  --output "$TEMP_DIR/$ARCHIVE_NAME.sha256"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$TEMP_DIR" && sha256sum --check "$ARCHIVE_NAME.sha256")
elif command -v shasum >/dev/null 2>&1; then
  (cd "$TEMP_DIR" && shasum -a 256 --check "$ARCHIVE_NAME.sha256")
else
  fail "sha256sum or shasum is required"
fi

tar -xzf "$TEMP_DIR/$ARCHIVE_NAME" -C "$DESTINATION" --strip-components=1

ADMIN_PASSWORD="$(random_hex 12)"
SESSION_SECRET="$(random_hex 32)"
MYSQL_PASSWORD="$(random_hex 24)"
MYSQL_ROOT_PASSWORD="$(random_hex 24)"
set_env INGATE_ADMIN_PASSWORD "$ADMIN_PASSWORD" "$DESTINATION/.env"
set_env INGATE_SESSION_SECRET "$SESSION_SECRET" "$DESTINATION/.env"
set_env INGATE_MYSQL_PASSWORD "$MYSQL_PASSWORD" "$DESTINATION/.env"
set_env INGATE_MYSQL_ROOT_PASSWORD "$MYSQL_ROOT_PASSWORD" "$DESTINATION/.env"

if [[ "$START_AFTER_INSTALL" == true ]]; then
  "$DESTINATION/bin/start.sh"
fi

INSTALLED_VERSION="$(cat "$DESTINATION/VERSION")"
cat <<EOF

Ingate $INSTALLED_VERSION installed successfully

Installation:  $DESTINATION
Console:       http://127.0.0.1:8001
Username:      admin
Password:      $ADMIN_PASSWORD
Gateway HTTP:  http://127.0.0.1:8080
Gateway HTTPS: https://127.0.0.1:8443

Start:  $DESTINATION/bin/start.sh
Stop:   $DESTINATION/bin/stop.sh
Status: $DESTINATION/bin/status.sh
Logs:   $DESTINATION/bin/logs.sh
Remove: $DESTINATION/bin/uninstall.sh

Next: open Console and create Service -> Gateway -> Route
EOF
