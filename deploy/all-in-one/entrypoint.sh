#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${INGATE_DATA_DIR:-/var/lib/ingate}"
LOG_DIR="${INGATE_LOG_DIR:-/var/log/ingate}"
APISERVER_ADDR="127.0.0.1:18443"
ETCD_ADDR="127.0.0.1:2379"
CONTROLLER_INTERNAL_ADDR="127.0.0.1:18080"
KUBECONFIG_FILE="/etc/ingate/kubeconfig"
APISERVER_CONFIG="/etc/ingate/configs/ingate-apiserver.yaml"
CONTROLLER_CONFIG="/etc/ingate/configs/ingate-controller.yaml"
ADMIN_API_CONFIG="/etc/ingate/configs/ingate-admin-api.yaml"

all_pids=()
critical_pids=()

mkdir -p "$DATA_DIR/etcd" "$DATA_DIR/redis" "$DATA_DIR/certs" "$LOG_DIR"

start_bg() {
	local role="$1"
	local name="$2"
	shift 2
	echo "starting $name"
	"$@" >"$LOG_DIR/$name.process.log" 2>&1 &
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

start_bg critical etcd etcd \
	--data-dir "$DATA_DIR/etcd" \
	--listen-client-urls "http://$ETCD_ADDR" \
	--advertise-client-urls "http://$ETCD_ADDR"

wait_tcp etcd 127.0.0.1 2379

start_bg auxiliary redis redis-server /etc/ingate/redis/redis.conf \
	--dir "$DATA_DIR/redis"

export INGATE_APISERVER_SERVER_CERT_DIRECTORY="${INGATE_APISERVER_SERVER_CERT_DIRECTORY:-$DATA_DIR/certs}"
export INGATE_CONTROLLER_APISERVER_KUBECONFIG="${INGATE_CONTROLLER_APISERVER_KUBECONFIG:-$KUBECONFIG_FILE}"
export INGATE_ADMIN_API_APISERVER_KUBECONFIG="${INGATE_ADMIN_API_APISERVER_KUBECONFIG:-$KUBECONFIG_FILE}"
export INGATE_ADMIN_API_SERVER_LISTEN_ADDRESS="${INGATE_ADMIN_API_SERVER_LISTEN_ADDRESS:-0.0.0.0:8001}"
export INGATE_ADMIN_API_SERVER_CONSOLE_DIR="${INGATE_ADMIN_API_SERVER_CONSOLE_DIR:-/opt/ingate/console}"
export INGATE_APISERVER_LOGGING_STDOUT="${INGATE_APISERVER_LOGGING_STDOUT:-false}"
export INGATE_APISERVER_LOGGING_FILE_PATH="${INGATE_APISERVER_LOGGING_FILE_PATH:-$LOG_DIR/ingate-apiserver.log}"
export INGATE_CONTROLLER_LOGGING_STDOUT="${INGATE_CONTROLLER_LOGGING_STDOUT:-false}"
export INGATE_CONTROLLER_LOGGING_FILE_PATH="${INGATE_CONTROLLER_LOGGING_FILE_PATH:-$LOG_DIR/ingate-controller.log}"
export INGATE_ADMIN_API_LOGGING_STDOUT="${INGATE_ADMIN_API_LOGGING_STDOUT:-false}"
export INGATE_ADMIN_API_LOGGING_FILE_PATH="${INGATE_ADMIN_API_LOGGING_FILE_PATH:-$LOG_DIR/ingate-admin-api.log}"

start_bg critical ingate-apiserver ingate-apiserver \
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

start_bg critical ingate-controller ingate-controller \
	--config "$CONTROLLER_CONFIG"

wait_http ingate-controller "http://$CONTROLLER_INTERNAL_ADDR/readyz"

start_bg critical envoy envoy \
	-c /etc/ingate/envoy/bootstrap.yaml

start_bg critical ingate-admin-api ingate-admin-api \
	--config "$ADMIN_API_CONFIG"

set +e
wait -n "${critical_pids[@]}"
status="$?"
set -e
exit "$status"
