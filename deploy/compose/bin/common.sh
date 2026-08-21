#!/usr/bin/env bash

set -Eeuo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
COMPOSE=(
  docker compose
  --project-directory "$ROOT"
  --env-file "$ROOT/.env"
  -f "$ROOT/compose.yaml"
)
