#!/usr/bin/env bash
set -euo pipefail

if [[ -f /etc/ingate/default.env ]]; then
	set -a
	# shellcheck disable=SC1091
	source /etc/ingate/default.env
	set +a
fi

DATA_DIR="${INGATE_DATA_DIR:-/var/lib/ingate}"
LOG_DIR="${INGATE_LOG_DIR:-/var/log/ingate}"
APISERVER_ADDR="${INGATE_APISERVER_ADDR:-127.0.0.1:18443}"
ETCD_ADDR="${INGATE_ETCD_ADDR:-127.0.0.1:2379}"
XDS_ADDR="${INGATE_XDS_ADDR:-127.0.0.1:18000}"
CONSOLE_ADDR="${INGATE_CONSOLE_ADDR:-0.0.0.0:8001}"
HTTPBIN_ADDR="${INGATE_HTTPBIN_ADDR:-127.0.0.1:19090}"
DATAPLANE_ADDR="${INGATE_DATAPLANE_ADDR:-127.0.0.1:18081}"
KUBECONFIG_FILE="/etc/ingate/kubeconfig"

pids=()

mkdir -p "$DATA_DIR/etcd" "$DATA_DIR/runtime" "$DATA_DIR/certs" "$LOG_DIR"

start_bg() {
	local name="$1"
	shift
	echo "starting $name"
	"$@" >"$LOG_DIR/$name.log" 2>&1 &
	pids+=("$!")
}

stop_all() {
	local pid
	for pid in "${pids[@]}"; do
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

trap stop_all INT TERM

start_bg etcd etcd \
	--data-dir "$DATA_DIR/etcd" \
	--listen-client-urls "http://$ETCD_ADDR" \
	--advertise-client-urls "http://$ETCD_ADDR"

wait_tcp etcd 127.0.0.1 2379

start_bg ingate-apiserver ingate-apiserver \
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

start_bg ingate-controller ingate-controller \
	--kubeconfig "$KUBECONFIG_FILE" \
	--target xds

start_bg ingate-xds ingate-xds \
	--listen-address "$XDS_ADDR" \
	--kubeconfig "$KUBECONFIG_FILE" \
	--target xds

wait_tcp ingate-xds 127.0.0.1 18000

HTTPBIN_HOST="${HTTPBIN_ADDR%:*}"
HTTPBIN_PORT="${HTTPBIN_ADDR##*:}"
start_bg ingate-httpbin ingate-httpbin \
	-host "$HTTPBIN_HOST" \
	-port "$HTTPBIN_PORT" \
	-log-format json

wait_tcp ingate-httpbin "$HTTPBIN_HOST" "$HTTPBIN_PORT"

start_bg ingate-dataplane ingate-dataplane \
	--listen-address "$DATAPLANE_ADDR"

DATAPLANE_HOST="${DATAPLANE_ADDR%:*}"
DATAPLANE_PORT="${DATAPLANE_ADDR##*:}"
wait_tcp ingate-dataplane "$DATAPLANE_HOST" "$DATAPLANE_PORT"

start_bg envoy envoy \
	-c /etc/ingate/envoy/bootstrap.yaml

start_bg ingate-admin-api ingate-admin-api \
	--listen-address "$CONSOLE_ADDR" \
	--kubeconfig "$KUBECONFIG_FILE" \
	--console-dir /opt/ingate/console

wait -n
stop_all
