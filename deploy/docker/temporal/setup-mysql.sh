#!/bin/sh
set -eu

: "${MYSQL_SEEDS:?MYSQL_SEEDS is required}"
: "${MYSQL_USER:?MYSQL_USER is required}"

nc -z -w 10 "$MYSQL_SEEDS" "${DB_PORT:-3306}"
temporal-sql-tool --plugin mysql8 --ep "$MYSQL_SEEDS" -u "$MYSQL_USER" -p "${DB_PORT:-3306}" --db temporal create
temporal-sql-tool --plugin mysql8 --ep "$MYSQL_SEEDS" -u "$MYSQL_USER" -p "${DB_PORT:-3306}" --db temporal setup-schema -v 0.0
temporal-sql-tool --plugin mysql8 --ep "$MYSQL_SEEDS" -u "$MYSQL_USER" -p "${DB_PORT:-3306}" --db temporal update-schema -d /etc/temporal/schema/mysql/v8/temporal/versioned
temporal-sql-tool --plugin mysql8 --ep "$MYSQL_SEEDS" -u "$MYSQL_USER" -p "${DB_PORT:-3306}" --db temporal_visibility create
temporal-sql-tool --plugin mysql8 --ep "$MYSQL_SEEDS" -u "$MYSQL_USER" -p "${DB_PORT:-3306}" --db temporal_visibility setup-schema -v 0.0
temporal-sql-tool --plugin mysql8 --ep "$MYSQL_SEEDS" -u "$MYSQL_USER" -p "${DB_PORT:-3306}" --db temporal_visibility update-schema -d /etc/temporal/schema/mysql/v8/visibility/versioned
