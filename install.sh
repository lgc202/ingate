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
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
CONTAINER_CONSOLE_PORT="8001"
CONTAINER_HTTP_PORT="8080"
CONTAINER_HTTPS_PORT="8443"

usage() {
	cat <<EOF
Usage: ./install.sh [start|stop|restart|delete|status|logs] [options]

Options:
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
	mkdir -p \
		"$DATA_DIR/configs" \
		"$DATA_DIR/certificates" \
		"$DATA_DIR/data/admin-api/logs" \
		"$DATA_DIR/data/apiserver/logs" \
		"$DATA_DIR/data/controller/logs" \
		"$DATA_DIR/data/envoy/logs" \
		"$DATA_DIR/data/etcd/data" \
		"$DATA_DIR/data/etcd/logs" \
		"$DATA_DIR/data/redis/data" \
		"$DATA_DIR/data/redis/logs" \
		"$DATA_DIR/data/plugins" \
		"$DATA_DIR/data/backups"
	local config_file
	for config_file in ingate-apiserver.yaml ingate-admin-api.yaml ingate-controller.yaml; do
		if [[ ! -f "$DATA_DIR/configs/$config_file" ]]; then
			cp "$SCRIPT_DIR/configs/$config_file" "$DATA_DIR/configs/$config_file"
		fi
	done
	DATA_DIR="$(cd "$DATA_DIR" && pwd -P)"
}

container_id() {
	docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format "{{.ID}}"
}

container_running() {
	docker ps --filter "name=^/${CONTAINER_NAME}$" --filter "status=running" --format "{{.ID}}"
}

wait_healthy() {
	local i state health
	for i in $(seq 1 90); do
		state="$(docker inspect -f '{{.State.Status}}' "$CONTAINER_NAME")"
		health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$CONTAINER_NAME")"
		if [[ "$state" != "running" ]]; then
			echo "Container $CONTAINER_NAME stopped during startup (status: $state)" >&2
			docker logs --tail 50 "$CONTAINER_NAME" >&2 || true
			return 1
		fi
		case "$health" in
		healthy)
			return 0
			;;
		unhealthy)
			echo "Container $CONTAINER_NAME is unhealthy; component logs: $DATA_DIR/data/<component>/logs" >&2
			docker logs --tail 50 "$CONTAINER_NAME" >&2 || true
			return 1
			;;
		none)
			echo "Image $IMAGE:$TAG does not define a health check" >&2
			return 1
			;;
		esac
		sleep 1
	done

	echo "Timed out waiting for container $CONTAINER_NAME to become healthy; component logs: $DATA_DIR/data/<component>/logs" >&2
	docker logs --tail 50 "$CONTAINER_NAME" >&2 || true
	return 1
}

print_success() {
	cat <<EOF
Ingate is running.

Console:      http://localhost:$CONSOLE_PORT
Gateway HTTP: http://localhost:$HTTP_PORT
Gateway TLS:  https://localhost:$HTTPS_PORT
Ingate dir:   $DATA_DIR
Runtime data: $DATA_DIR/data
Logs:         $DATA_DIR/data/<component>/logs
Configs:      $DATA_DIR/configs
Certificates: $DATA_DIR/certificates
Stop:         ./install.sh stop --container-name $CONTAINER_NAME
EOF
}

start_container() {
	require_docker
	ensure_dirs

	if [[ -n "$(container_running)" ]]; then
		wait_healthy
		print_success
		return
	fi

	if [[ -n "$(container_id)" ]]; then
		docker start "$CONTAINER_NAME" >/dev/null
		wait_healthy
		print_success
		return
	fi

	docker run -d \
		--name "$CONTAINER_NAME" \
		-p "$BIND:$CONSOLE_PORT:$CONTAINER_CONSOLE_PORT" \
		-p "$BIND:$HTTP_PORT:$CONTAINER_HTTP_PORT" \
		-p "$BIND:$HTTPS_PORT:$CONTAINER_HTTPS_PORT" \
		-v "$DATA_DIR/data:/data/ingate" \
		-v "$DATA_DIR/configs/ingate-apiserver.yaml:/opt/ingate/apiserver/configs/config.yaml:ro" \
		-v "$DATA_DIR/configs/ingate-admin-api.yaml:/opt/ingate/admin-api/configs/config.yaml:ro" \
		-v "$DATA_DIR/configs/ingate-controller.yaml:/opt/ingate/controller/configs/config.yaml:ro" \
		-v "$DATA_DIR/certificates:/opt/ingate/apiserver/certificates" \
		"$IMAGE:$TAG" >/dev/null

	wait_healthy
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
	local id status health
	id="$(container_id)"
	if [[ -z "$id" ]]; then
		echo "Container $CONTAINER_NAME does not exist"
		return
	fi
	status="$(docker inspect -f '{{.State.Status}}' "$CONTAINER_NAME")"
	health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$CONTAINER_NAME")"
	echo "Container: $CONTAINER_NAME"
	echo "Status:    $status"
	echo "Health:    $health"
	echo "Console:   http://localhost:$CONSOLE_PORT"
	echo "HTTP:      http://localhost:$HTTP_PORT"
	echo "HTTPS:     https://localhost:$HTTPS_PORT"
	echo "Ingate dir: $DATA_DIR"
	echo "Data:       $DATA_DIR/data"
	echo "Logs:       $DATA_DIR/data/<component>/logs"
	echo "Configs:    $DATA_DIR/configs"
	echo "Certs:      $DATA_DIR/certificates"
}

show_logs() {
	require_docker
	if [[ -z "$(container_id)" ]]; then
		echo "Container $CONTAINER_NAME does not exist" >&2
		exit 1
	fi
	local component log_file
	local log_files=()
	for component in admin-api apiserver controller envoy etcd redis; do
		for log_file in "$DATA_DIR/data/$component/logs/"*.log; do
			if [[ -e "$log_file" ]]; then
				log_files+=("$log_file")
			fi
		done
	done
	if [[ ${#log_files[@]} -eq 0 ]]; then
		echo "No log files found in $DATA_DIR/data/<component>/logs" >&2
		exit 1
	fi
	tail -F "${log_files[@]}"
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
