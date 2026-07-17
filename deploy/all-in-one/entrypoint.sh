#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${INGATE_DATA_DIR:-/var/lib/ingate}"
LOG_DIR="${INGATE_LOG_DIR:-/var/log/ingate}"
APISERVER_ADDR="127.0.0.1:18443"
ETCD_ADDR="127.0.0.1:2379"
CONTROLLER_XDS_ADDR="127.0.0.1:18000"
CONTROLLER_INTERNAL_ADDR="127.0.0.1:18080"
CONTROLLER_ACK_TIMEOUT="${INGATE_CANDIDATE_ACK_TIMEOUT:-30s}"
CONTROLLER_NACK_ROLLBACK_TIMEOUT="${INGATE_NACK_ROLLBACK_TIMEOUT:-3s}"
CONTROLLER_RESYNC_PERIOD="${INGATE_RESYNC_PERIOD:-0s}"
CONTROLLER_STATUS_TIMEOUT="500ms"
KUBECONFIG_FILE="/etc/ingate/kubeconfig"

all_pids=()
critical_pids=()

mkdir -p "$DATA_DIR/etcd" "$DATA_DIR/redis" "$DATA_DIR/certs" "$LOG_DIR"

start_bg() {
	local role="$1"
	local name="$2"
	shift 2
	echo "starting $name"
	"$@" >"$LOG_DIR/$name.log" 2>&1 &
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

start_bg critical ingate-apiserver ingate-apiserver \
	--bind-address 127.0.0.1 \
	--secure-port 18443 \
	--cert-dir "$DATA_DIR/certs" \
	--etcd-servers "http://$ETCD_ADDR"

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
	--kubeconfig "$KUBECONFIG_FILE" \
	--xds-listen-address "$CONTROLLER_XDS_ADDR" \
	--internal-listen-address "$CONTROLLER_INTERNAL_ADDR" \
	--candidate-ack-timeout "$CONTROLLER_ACK_TIMEOUT" \
	--nack-rollback-timeout "$CONTROLLER_NACK_ROLLBACK_TIMEOUT" \
	--resync-period "$CONTROLLER_RESYNC_PERIOD"

wait_http ingate-controller "http://$CONTROLLER_INTERNAL_ADDR/readyz"

start_bg critical envoy envoy \
	-c /etc/ingate/envoy/bootstrap.yaml

start_bg critical ingate-admin-api ingate-admin-api \
	--listen-address 0.0.0.0:8001 \
	--kubeconfig "$KUBECONFIG_FILE" \
	--controller-status-url "http://$CONTROLLER_INTERNAL_ADDR" \
	--controller-status-timeout "$CONTROLLER_STATUS_TIMEOUT" \
	--console-dir /opt/ingate/console

set +e
wait -n "${critical_pids[@]}"
status="$?"
set -e
exit "$status"
