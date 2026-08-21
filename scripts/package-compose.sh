#!/usr/bin/env bash

set -Eeuo pipefail

if [[ $# -ne 2 ]]; then
  echo "Usage: $(basename -- "$0") VERSION OUTPUT_DIR" >&2
  exit 2
fi

VERSION="$1"
OUTPUT_DIR="$2"

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Invalid release version: $VERSION" >&2
  exit 2
fi

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
STAGING_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ingate-compose.XXXXXX")"
PACKAGE_ROOT="$STAGING_ROOT/ingate"

cleanup() {
  rm -rf "$STAGING_ROOT"
}
trap cleanup EXIT

mkdir -p "$OUTPUT_DIR" "$PACKAGE_ROOT/bin" "$PACKAGE_ROOT/docker"

cp "$ROOT/deploy/docker-compose.yaml" "$PACKAGE_ROOT/compose.yaml"
cp "$ROOT/deploy/compose/README.md" "$PACKAGE_ROOT/README.md"
cp "$ROOT/LICENSE" "$PACKAGE_ROOT/LICENSE"
cp -R "$ROOT/deploy/compose/bin/." "$PACKAGE_ROOT/bin/"
cp -R "$ROOT/deploy/docker/configs" "$PACKAGE_ROOT/docker/configs"
cp "$ROOT/deploy/docker/envoy-bootstrap.yaml" "$PACKAGE_ROOT/docker/envoy-bootstrap.yaml"
cp "$ROOT/deploy/docker/kubeconfig.yaml" "$PACKAGE_ROOT/docker/kubeconfig.yaml"
cp "$ROOT/deploy/docker/redis.conf" "$PACKAGE_ROOT/docker/redis.conf"

sed "s/__INGATE_VERSION__/$VERSION/" "$ROOT/deploy/compose/.env.example" > "$PACKAGE_ROOT/.env"
printf '%s\n' "$VERSION" > "$PACKAGE_ROOT/VERSION"

ARCHIVE="$OUTPUT_DIR/ingate-compose.tar.gz"
tar -czf "$ARCHIVE" -C "$STAGING_ROOT" ingate

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$OUTPUT_DIR" && sha256sum "$(basename -- "$ARCHIVE")" > "$(basename -- "$ARCHIVE").sha256")
else
  (cd "$OUTPUT_DIR" && shasum -a 256 "$(basename -- "$ARCHIVE")" > "$(basename -- "$ARCHIVE").sha256")
fi
