#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"

require_installation
[[ -n "$(env_value INGATE_ADMIN_PASSWORD)" ]] || fail "INGATE_ADMIN_PASSWORD is empty in $ROOT/.env"
SESSION_SECRET="$(env_value INGATE_SESSION_SECRET)"
[[ ${#SESSION_SECRET} -ge 32 ]] || fail "INGATE_SESSION_SECRET must contain at least 32 characters"

"${COMPOSE[@]}" up -d --wait --wait-timeout "${INGATE_WAIT_TIMEOUT:-300}"
