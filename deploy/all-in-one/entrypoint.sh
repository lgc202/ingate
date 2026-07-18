#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${INGATE_DATA_DIR:-/data/ingate}"
APISERVER_ADDR="127.0.0.1:18443"
ETCD_ADDR="127.0.0.1:2379"
CONTROLLER_INTERNAL_ADDR="127.0.0.1:18080"
KUBECONFIG_FILE="/opt/ingate/configs/kubeconfig"
APISERVER_CONFIG="/opt/ingate/apiserver/configs/config.yaml"
APISERVER_CERT_DIR="/opt/ingate/apiserver/certificates"
CONTROLLER_CONFIG="/opt/ingate/controller/configs/config.yaml"
ADMIN_API_CONFIG="/opt/ingate/admin-api/configs/config.yaml"
ENVOY_CONFIG="/opt/ingate/envoy/configs/bootstrap.yaml"
REDIS_CONFIG="/opt/ingate/redis/configs/redis.conf"

APISERVER_LOG_DIR="$DATA_DIR/apiserver/logs"
ADMIN_API_LOG_DIR="$DATA_DIR/admin-api/logs"
CONTROLLER_LOG_DIR="$DATA_DIR/controller/logs"
ENVOY_LOG_DIR="$DATA_DIR/envoy/logs"
ETCD_DATA_DIR="$DATA_DIR/etcd/data"
ETCD_LOG_DIR="$DATA_DIR/etcd/logs"
REDIS_DATA_DIR="$DATA_DIR/redis/data"
REDIS_LOG_DIR="$DATA_DIR/redis/logs"

all_pids=()
critical_pids=()

mkdir -p \
	"$APISERVER_CERT_DIR" \
	"$APISERVER_LOG_DIR" \
	"$ADMIN_API_LOG_DIR" \
	"$CONTROLLER_LOG_DIR" \
	"$ENVOY_LOG_DIR" \
	"$ETCD_DATA_DIR" \
	"$ETCD_LOG_DIR" \
	"$REDIS_DATA_DIR" \
	"$REDIS_LOG_DIR" \
	"$DATA_DIR/plugins" \
	"$DATA_DIR/backups"

start_bg() {
	local role="$1"
	local name="$2"
	local log_dir="$3"
	shift 3
	echo "starting $name"
	"$@" >"$log_dir/$name.process.log" 2>&1 &
	local pid="$!"
	all_pids+=("$pid")
	if [[ "$role" == "critical" ]]; then
		critical_pids+=("$pid")
	fi
}

stop_all() {
	local pid
	for pid in "${all_pids[@]}"; do
		kill "$pid" 2>/dev/null || true
	done
	wait || true
}

wait_tcp() {
	local name="$1"
	local host="$2"
	local port="$3"
	local i
	for i in $(seq 1 60); do
		if timeout 1 bash -c "cat < /dev/null > /dev/tcp/$host/$port" 2>/dev/null; then
			return 0
		fi
		sleep 1
	done
	echo "timeout waiting for $name at $host:$port" >&2
	return 1
}

wait_http() {
	local name="$1"
	local url="$2"
	local i
	for i in $(seq 1 60); do
		if curl -fsS "$url" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	echo "timeout waiting for $name at $url" >&2
	return 1
}

handle_signal() {
	exit 0
}

trap stop_all EXIT
trap handle_signal INT TERM

start_bg critical etcd "$ETCD_LOG_DIR" /opt/ingate/etcd/bin/etcd \
	--data-dir "$ETCD_DATA_DIR" \
	--listen-client-urls "http://$ETCD_ADDR" \
	--advertise-client-urls "http://$ETCD_ADDR"

wait_tcp etcd 127.0.0.1 2379

start_bg auxiliary redis "$REDIS_LOG_DIR" /opt/ingate/redis/bin/redis-server "$REDIS_CONFIG" \
	--dir "$REDIS_DATA_DIR"

export INGATE_APISERVER_SERVER_CERT_DIRECTORY="${INGATE_APISERVER_SERVER_CERT_DIRECTORY:-$APISERVER_CERT_DIR}"
export INGATE_CONTROLLER_APISERVER_KUBECONFIG="${INGATE_CONTROLLER_APISERVER_KUBECONFIG:-$KUBECONFIG_FILE}"
export INGATE_ADMIN_API_APISERVER_KUBECONFIG="${INGATE_ADMIN_API_APISERVER_KUBECONFIG:-$KUBECONFIG_FILE}"
export INGATE_ADMIN_API_SERVER_LISTEN_ADDRESS="${INGATE_ADMIN_API_SERVER_LISTEN_ADDRESS:-0.0.0.0:8001}"
export INGATE_ADMIN_API_SERVER_CONSOLE_DIR="${INGATE_ADMIN_API_SERVER_CONSOLE_DIR:-/opt/ingate/admin-api/console}"
export INGATE_APISERVER_LOGGING_STDOUT="${INGATE_APISERVER_LOGGING_STDOUT:-false}"
export INGATE_APISERVER_LOGGING_FILE_PATH="${INGATE_APISERVER_LOGGING_FILE_PATH:-$APISERVER_LOG_DIR/ingate-apiserver.log}"
export INGATE_CONTROLLER_LOGGING_STDOUT="${INGATE_CONTROLLER_LOGGING_STDOUT:-false}"
export INGATE_CONTROLLER_LOGGING_FILE_PATH="${INGATE_CONTROLLER_LOGGING_FILE_PATH:-$CONTROLLER_LOG_DIR/ingate-controller.log}"
export INGATE_ADMIN_API_LOGGING_STDOUT="${INGATE_ADMIN_API_LOGGING_STDOUT:-false}"
export INGATE_ADMIN_API_LOGGING_FILE_PATH="${INGATE_ADMIN_API_LOGGING_FILE_PATH:-$ADMIN_API_LOG_DIR/ingate-admin-api.log}"

start_bg critical ingate-apiserver "$APISERVER_LOG_DIR" /opt/ingate/apiserver/bin/ingate-apiserver \
	--config "$APISERVER_CONFIG"

wait_tcp ingate-apiserver 127.0.0.1 18443

MASTER="https://$APISERVER_ADDR"
cat >"$KUBECONFIG_FILE" <<EOF
apiVersion: v1
kind: Config
clusters:
- name: ingate
  cluster:
    server: $MASTER
    insecure-skip-tls-verify: true
contexts:
- name: ingate
  context:
    cluster: ingate
    user: ingate
current-context: ingate
users:
- name: ingate
  user: {}
EOF

start_bg critical ingate-controller "$CONTROLLER_LOG_DIR" /opt/ingate/controller/bin/ingate-controller \
	--config "$CONTROLLER_CONFIG"

wait_http ingate-controller "http://$CONTROLLER_INTERNAL_ADDR/readyz"

start_bg critical envoy "$ENVOY_LOG_DIR" /opt/ingate/envoy/bin/envoy \
	-c "$ENVOY_CONFIG"

start_bg critical ingate-admin-api "$ADMIN_API_LOG_DIR" /opt/ingate/admin-api/bin/ingate-admin-api \
	--config "$ADMIN_API_CONFIG"

set +e
wait -n "${critical_pids[@]}"
status="$?"
set -e
exit "$status"
