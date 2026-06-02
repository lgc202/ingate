#!/usr/bin/env bash
set -euo pipefail

DEFAULT_CONTAINER_NAME="ingate"
DEFAULT_IMAGE="ingate/all-in-one"
DEFAULT_TAG="latest"
DEFAULT_DATA_DIR="./ingate"
DEFAULT_BIND="127.0.0.1"
DEFAULT_CONSOLE_PORT="8001"
DEFAULT_HTTP_PORT="8080"
DEFAULT_HTTPS_PORT="8443"
CONTAINER_CONSOLE_PORT="8001"
CONTAINER_HTTP_PORT="8080"
CONTAINER_HTTPS_PORT="8443"

usage() {
	cat <<EOF
Usage: ./install.sh [start|stop|restart|delete|status|logs] [options]

Options:
  --non-interactive          Accepted for script compatibility; start is non-interactive by default
  --container-name NAME      Container name, default: $DEFAULT_CONTAINER_NAME
  --image IMAGE              Image name, default: $DEFAULT_IMAGE
  --tag TAG                  Image tag, default: $DEFAULT_TAG
  --data-dir DIR             Local data directory, default: $DEFAULT_DATA_DIR
  --bind ADDRESS             Host bind address, default: $DEFAULT_BIND
  --console-port PORT        Console host port, default: $DEFAULT_CONSOLE_PORT
  --http-port PORT           Gateway HTTP host port, default: $DEFAULT_HTTP_PORT
  --https-port PORT          Gateway HTTPS host port, default: $DEFAULT_HTTPS_PORT
  --purge-data               Delete local data directory with delete command
  -h, --help                 Show help
EOF
}

COMMAND="start"
if [[ $# -gt 0 && "$1" != -* ]]; then
	COMMAND="$1"
	shift
fi

CONTAINER_NAME="$DEFAULT_CONTAINER_NAME"
IMAGE="$DEFAULT_IMAGE"
TAG="$DEFAULT_TAG"
DATA_DIR="$DEFAULT_DATA_DIR"
BIND="$DEFAULT_BIND"
CONSOLE_PORT="$DEFAULT_CONSOLE_PORT"
HTTP_PORT="$DEFAULT_HTTP_PORT"
HTTPS_PORT="$DEFAULT_HTTPS_PORT"
PURGE_DATA="false"

while [[ $# -gt 0 ]]; do
	case "$1" in
	--non-interactive)
		shift
		;;
	--container-name)
		CONTAINER_NAME="$2"
		shift 2
		;;
	--image)
		IMAGE="$2"
		shift 2
		;;
	--tag)
		TAG="$2"
		shift 2
		;;
	--data-dir)
		DATA_DIR="$2"
		shift 2
		;;
	--bind)
		BIND="$2"
		shift 2
		;;
	--console-port)
		CONSOLE_PORT="$2"
		shift 2
		;;
	--http-port)
		HTTP_PORT="$2"
		shift 2
		;;
	--https-port)
		HTTPS_PORT="$2"
		shift 2
		;;
	--purge-data)
		PURGE_DATA="true"
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "Unknown option: $1" >&2
		usage
		exit 1
		;;
	esac
done

require_docker() {
	if ! command -v docker >/dev/null 2>&1; then
		echo "Docker is required" >&2
		exit 1
	fi
}

ensure_dirs() {
	mkdir -p "$DATA_DIR/data" "$DATA_DIR/logs"
	DATA_DIR="$(cd "$DATA_DIR" && pwd -P)"
	if [[ ! -f "$DATA_DIR/default.env" ]]; then
		write_default_env
	fi
}

write_default_env() {
	cat >"$DATA_DIR/default.env" <<EOF
INGATE_MODE=all-in-one
INGATE_CONSOLE_ADDR=0.0.0.0:$CONTAINER_CONSOLE_PORT
INGATE_GATEWAY_HTTP_ADDR=0.0.0.0:$CONTAINER_HTTP_PORT
INGATE_GATEWAY_HTTPS_ADDR=0.0.0.0:$CONTAINER_HTTPS_PORT
INGATE_APISERVER_ADDR=127.0.0.1:18443
INGATE_ETCD_ADDR=127.0.0.1:2379
INGATE_XDS_ADDR=127.0.0.1:18000
INGATE_ENVOY_ADMIN_ADDR=127.0.0.1:15000
INGATE_DATA_DIR=/var/lib/ingate
INGATE_LOG_DIR=/var/log/ingate
EOF
}

container_id() {
	docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format "{{.ID}}"
}

container_running() {
	docker ps --filter "name=^/${CONTAINER_NAME}$" --filter "status=running" --format "{{.ID}}"
}

print_success() {
	cat <<EOF
Ingate is running.

Console:      http://localhost:$CONSOLE_PORT
Gateway HTTP: http://localhost:$HTTP_PORT
Data dir:     $DATA_DIR
Logs:         $DATA_DIR/logs
Stop:         ./install.sh stop --container-name $CONTAINER_NAME
EOF
}

start_container() {
	require_docker
	ensure_dirs

	if [[ -n "$(container_running)" ]]; then
		print_success
		return
	fi

	if [[ -n "$(container_id)" ]]; then
		docker start "$CONTAINER_NAME" >/dev/null
		print_success
		return
	fi

	docker run -d \
		--name "$CONTAINER_NAME" \
		--env-file "$DATA_DIR/default.env" \
		-e "INGATE_CONSOLE_ADDR=0.0.0.0:$CONTAINER_CONSOLE_PORT" \
		-e "INGATE_GATEWAY_HTTP_ADDR=0.0.0.0:$CONTAINER_HTTP_PORT" \
		-e "INGATE_GATEWAY_HTTPS_ADDR=0.0.0.0:$CONTAINER_HTTPS_PORT" \
		-p "$BIND:$CONSOLE_PORT:$CONTAINER_CONSOLE_PORT" \
		-p "$BIND:$HTTP_PORT:$CONTAINER_HTTP_PORT" \
		-p "$BIND:$HTTPS_PORT:$CONTAINER_HTTPS_PORT" \
		-v "$DATA_DIR/data:/var/lib/ingate" \
		-v "$DATA_DIR/logs:/var/log/ingate" \
		"$IMAGE:$TAG" >/dev/null

	print_success
}

stop_container() {
	require_docker
	if [[ -z "$(container_id)" ]]; then
		echo "Container $CONTAINER_NAME does not exist"
		return
	fi
	docker stop "$CONTAINER_NAME" >/dev/null
	echo "Container $CONTAINER_NAME stopped"
}

delete_container() {
	require_docker
	if [[ -n "$(container_id)" ]]; then
		docker rm -f "$CONTAINER_NAME" >/dev/null
		echo "Container $CONTAINER_NAME deleted"
	else
		echo "Container $CONTAINER_NAME does not exist"
	fi

	if [[ "$PURGE_DATA" == "true" ]]; then
		rm -rf "$DATA_DIR"
		echo "Data dir $DATA_DIR deleted"
	else
		echo "Data dir kept: $DATA_DIR"
	fi
}

show_status() {
	require_docker
	local id status
	id="$(container_id)"
	if [[ -z "$id" ]]; then
		echo "Container $CONTAINER_NAME does not exist"
		return
	fi
	status="$(docker inspect -f '{{.State.Status}}' "$CONTAINER_NAME")"
	echo "Container: $CONTAINER_NAME"
	echo "Status:    $status"
	echo "Console:   http://localhost:$CONSOLE_PORT"
	echo "Gateway:   http://localhost:$HTTP_PORT"
	echo "Data dir:  $DATA_DIR"
}

show_logs() {
	require_docker
	if [[ -z "$(container_id)" ]]; then
		echo "Container $CONTAINER_NAME does not exist" >&2
		exit 1
	fi
	docker logs -f "$CONTAINER_NAME"
}

restart_container() {
	require_docker
	if [[ -n "$(container_id)" ]]; then
		docker rm -f "$CONTAINER_NAME" >/dev/null
		echo "Container $CONTAINER_NAME recreated"
	fi
	start_container
}

case "$COMMAND" in
start)
	start_container
	;;
stop)
	stop_container
	;;
restart)
	restart_container
	;;
delete)
	delete_container
	;;
status)
	show_status
	;;
logs)
	show_logs
	;;
*)
	echo "Unknown command: $COMMAND" >&2
	usage
	exit 1
	;;
esac
