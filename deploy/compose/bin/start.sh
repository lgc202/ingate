#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"

"${COMPOSE[@]}" up -d --wait --wait-timeout "${INGATE_WAIT_TIMEOUT:-300}"
