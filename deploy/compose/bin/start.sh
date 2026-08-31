#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"

require_installation
chmod 600 "$ROOT/.env"
[[ -n "$(env_value INGATE_ADMIN_PASSWORD)" ]] || fail "INGATE_ADMIN_PASSWORD is empty in $ROOT/.env"
SESSION_SECRET="$(env_value INGATE_SESSION_SECRET)"
[[ ${#SESSION_SECRET} -ge 32 ]] || fail "INGATE_SESSION_SECRET must contain at least 32 characters"
APISERVER_BEARER_TOKEN="$(env_value INGATE_APISERVER_BEARER_TOKEN)"
[[ ${#APISERVER_BEARER_TOKEN} -ge 32 && ${#APISERVER_BEARER_TOKEN} -le 256 ]] ||
  fail "INGATE_APISERVER_BEARER_TOKEN must contain 32 to 256 characters"
[[ -n "$(env_value INGATE_MYSQL_PASSWORD)" ]] || fail "INGATE_MYSQL_PASSWORD is empty in $ROOT/.env"
[[ -n "$(env_value INGATE_MYSQL_ROOT_PASSWORD)" ]] || fail "INGATE_MYSQL_ROOT_PASSWORD is empty in $ROOT/.env"
[[ -n "$(env_value INGATE_CLICKHOUSE_PASSWORD)" ]] || fail "INGATE_CLICKHOUSE_PASSWORD is empty in $ROOT/.env"

"${COMPOSE[@]}" up -d --wait --wait-timeout "${INGATE_WAIT_TIMEOUT:-300}"
