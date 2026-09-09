#!/usr/bin/env bash
set -eu -o pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
go_command=${GO:-go}
run_id="$(date -u +%Y%m%dt%H%M%S)-$$-$RANDOM"
run_root=$(mktemp -d "${TMPDIR:-/tmp}/ingate-als-e2e.XXXXXX")
project="ingate-als-e2e-$run_id"
compose_file="$source_root/hack/als-e2e/compose.yaml"
diagnostic_file="$source_root/_output/als-e2e/$run_id.log"
logs_pid=""

export ALS_E2E_ROOT="$run_root"
export ALS_E2E_SOURCE="$source_root"

compose() {
	docker compose --project-name "$project" --file "$compose_file" "$@"
}

cleanup() {
	status=$?
	set +e

	mkdir -p "$(dirname "$diagnostic_file")"
	{
		printf 'ALS E2E run: %s\n' "$run_id"
		printf 'Exit status: %s\n\n' "$status"
		compose ps --all
		printf '\nContainer logs\n'
		compose logs --no-color --no-log-prefix
	} >"$diagnostic_file" 2>&1

	if [[ -n "$logs_pid" ]]; then
		kill "$logs_pid" 2>/dev/null
		wait "$logs_pid" 2>/dev/null
	fi
	compose down --volumes --remove-orphans >/dev/null 2>&1
	rm -rf -- "$run_root"

	if [[ $status -eq 0 ]]; then
		printf 'ALS E2E passed; diagnostics: %s\n' "$diagnostic_file"
	else
		printf 'ALS E2E failed; diagnostics: %s\n' "$diagnostic_file" >&2
	fi
	exit "$status"
}

trap cleanup EXIT
trap 'exit 130' INT TERM

step() {
	printf '\n==> %s\n' "$1"
}

probe() {
	compose exec -T probe /work/bin/als-e2e "$@"
}

follow_als_logs() {
	if [[ -n "$logs_pid" ]]; then
		kill "$logs_pid" 2>/dev/null || true
		wait "$logs_pid" 2>/dev/null || true
	fi
	# logs --follow 随容器停止而退出；每次启动后重新附着，才能覆盖崩溃恢复后的实例。
	compose logs --since 2s --no-color --no-log-prefix --follow als >>"$run_root/logs/als.log" 2>&1 &
	logs_pid=$!
}

wait_kafka() {
	local attempt
	for attempt in {1..60}; do
		if compose exec -T kafka /opt/kafka/bin/kafka-topics.sh \
			--bootstrap-server 127.0.0.1:9092 --list >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	return 1
}

step "build isolated Linux binaries"
case "$(docker info --format '{{.Architecture}}')" in
	arm64 | aarch64) goarch=arm64 ;;
	amd64 | x86_64) goarch=amd64 ;;
	*)
		printf 'unsupported Docker architecture\n' >&2
		exit 1
		;;
esac
mkdir -p "$run_root/bin" "$run_root/config" "$run_root/data" "$run_root/logs"
touch "$run_root/logs/als.log"
cp "$source_root/hack/als-e2e/config/als.yaml" "$run_root/config/als.yaml"
(
	cd "$source_root"
	CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" "$go_command" build -trimpath -o "$run_root/bin/ingate-als" ./cmd/ingate-als
	CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" "$go_command" build -trimpath -o "$run_root/bin/als-e2e" ./hack/als-e2e
)

step "start Kafka and create the request-record topic"
compose up --detach kafka
compose run --rm kafka-init

step "start the isolated observability stack and ALS"
compose up --detach loki tempo otel-collector prometheus probe als
follow_als_logs
probe http --url http://otel-collector:13133/
probe http --url http://prometheus:9090/-/ready
probe ready --target kafka --pending 0
compose up --detach envoy
probe http --url http://envoy:9901/ready

step "verify direct Kafka delivery leaves the WAL empty"
probe send --node "$run_id" --request healthy
probe kafka --request healthy
probe ready --target kafka --pending 0

step "verify Kafka failure, crash recovery, and automatic replay"
compose stop kafka
# Kafka 不可达时重新启动 ALS，Topic 状态保持未知；写入路径因此直接选择 WAL，
# 不依赖 TCP 连接何时宣告失效，也不会为演练修改生产超时。
compose stop als
compose start als
follow_als_logs
probe ready --target disk_queue --pending 0
probe send --node "$run_id" --request queued-one
probe send --node "$run_id" --request queued-two
probe ready --target disk_queue --min-pending 2
compose kill --signal SIGKILL als
compose start als
follow_als_logs
probe ready --target disk_queue --min-pending 2
compose start kafka
wait_kafka
# Topic 契约按生产周期刷新；等待真实后台回放，不通过重启或测试配置绕过恢复路径。
probe ready --target kafka --pending 0
probe kafka --request queued-one
probe kafka --request queued-two

step "verify uncertain publish replay preserves the stable record ID"
probe duplicate --queue /work/data/duplicate --marker "$run_id"

step "fill the WAL and prove traffic remains available"
compose stop kafka
compose stop als
compose start als
follow_als_logs
probe ready --target disk_queue --pending 0

# 无效记录同时产生丢弃指标、带 Trace 上下文的日志和对应 Span，
# 后续查询用本次运行的唯一标记把三种遥测信号关联起来。
log_marker="$run_id-invalid"
probe send --node "$log_marker" --request invalid --invalid

successful_writes=0
for attempt in {1..20}; do
	if probe send \
		--node "$run_id" \
		--request "full-$attempt" \
		--stream "full-$attempt" \
		--count 3 \
		--payload-bytes 18432 >/dev/null; then
		((successful_writes += 1))
		continue
	fi
	break
done
if ((successful_writes < 3)); then
	printf 'WAL accepted only %d entries before becoming full\n' "$successful_writes" >&2
	exit 1
fi
probe ready --reason wal_unavailable --target none --min-pending 3
probe http --url http://envoy:8080/

step "query this run in Prometheus, Loki, and Tempo"
probe observe --marker "$log_marker"

step "verify middle-entry corruption prevents unsafe startup"
compose stop als
probe corrupt \
	--source /work/data/queue \
	--destination /work/data/corrupt \
	--segment-bytes 65536
compose up --detach als-diagnostic
diagnostic_container=$(compose ps --all --quiet als-diagnostic)
for attempt in {1..30}; do
	state=$(docker inspect --format '{{.State.Status}} {{.State.ExitCode}}' "$diagnostic_container")
	if [[ $state == exited\ * ]]; then
		break
	fi
	sleep 1
done
if [[ $state != exited\ * || $state == "exited 0" ]]; then
	printf 'corrupted WAL process state is %q, want a non-zero exit\n' "$state" >&2
	exit 1
fi
if ! compose logs --no-color als-diagnostic 2>&1 | grep -Eq 'decode disk queue sequence|queue entry checksum|unmarshal queue entry'; then
	printf 'corrupted WAL failure did not contain a queue diagnostic\n' >&2
	exit 1
fi

step "all ALS fault scenarios passed"
