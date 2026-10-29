# 与宿主 tmux 进程分开：Envoy 和 AI Proxy 必须共享 Linux 中的 UDS。
start_dataplane() {
    local arch attempt
    if ! docker image inspect envoyproxy/envoy:v1.39.1 >/dev/null 2>&1; then
        compose --profile dataplane pull envoy
    fi
    arch=$(docker image inspect envoyproxy/envoy:v1.39.1 --format '{{.Architecture}}')
    mkdir -p "$dev_dir/image"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -mod=readonly \
        -o "$dev_dir/image/ingate-ai-proxy" ./cmd/ingate-ai-proxy
    compose --profile dataplane build --pull=false ai-proxy
    compose --profile dataplane up -d --wait --wait-timeout 90 ai-proxy envoy
    if ! component_ready controller; then
        echo '数据面容器已启动，等待 IDE/外部 Controller 提供 ADS；尚未确认 Envoy 就绪。'
        return
    fi
    for ((attempt=0; attempt<30; attempt++)); do
        if curl --fail --silent --max-time 2 http://127.0.0.1:19000/ready >/dev/null; then
            echo 'Envoy /ready 检查通过，数据面已启动；入口端口 18088 对应 Gateway Listener 8080。'
            return
        fi
        sleep 1
    done
    echo 'Envoy 尚未就绪，请执行 make dev-logs COMPONENT=data-plane。' >&2
    return 1
}

stop_dataplane() {
    compose --profile dataplane stop envoy ai-proxy
}
