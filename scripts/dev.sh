#!/usr/bin/env bash
# 本地开发接线；组件可前台调试，也可由独立 tmux 服务托管。
set -euo pipefail
umask 077

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
cd "$repo_dir"
dev_dir="$repo_dir/.dev"
source "$repo_dir/scripts/dev/processes.sh"
source "$repo_dir/scripts/dev/dataplane.sh"

require_config() {
    if [[ ! -f "$dev_dir/compose.env" || ! -f "$dev_dir/ingate.kubeconfig" ]]; then
        echo '开发配置未就绪，请先执行 make dev-init。' >&2
        exit 1
    fi
}

compose() {
    require_config
    # 同名仓库的不同检出目录不复用网络和数据卷；固定宿主端口只支持同时运行一套。
    local checkout_id
    checkout_id=$(printf '%s' "$repo_dir" | cksum | awk '{print $1}')
    docker compose --project-name "ingate-dev-$checkout_id" \
        --env-file "$dev_dir/compose.env" -f deploy/dev/compose.yaml "$@"
}

init_config() {
    if [[ -e "$dev_dir" ]]; then
        echo '.dev 已存在，未覆盖证书或口令。请复用现有配置；重建前先阅读开发文档。' >&2
        exit 1
    fi

    # 整组配置生成成功后才安装；失败只清理本次创建的临时目录。
    umask 077
    config_dir=$(mktemp -d "$repo_dir/.ingate-dev.XXXXXX")
    trap 'rm -rf "$config_dir"' EXIT
    local mysql_password root_password
    mysql_password=$(openssl rand -hex 24)
    root_password=$(openssl rand -hex 24)

    cat > "$config_dir/ca.cnf" <<'EOF'
[req]
distinguished_name = subject
x509_extensions = ca
prompt = no
[subject]
CN = Ingate local development CA
[ca]
basicConstraints = critical,CA:TRUE
keyUsage = critical,keyCertSign,cRLSign
subjectKeyIdentifier = hash
EOF
    openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
        -config "$config_dir/ca.cnf" -keyout "$config_dir/ca.key" \
        -out "$config_dir/ca.crt" >/dev/null 2>&1

    cat > "$config_dir/server.ext" <<'EOF'
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = DNS:localhost,IP:127.0.0.1
EOF
    openssl req -new -newkey rsa:2048 -nodes -subj /CN=localhost \
        -keyout "$config_dir/server.key" -out "$config_dir/server.csr" >/dev/null 2>&1
    openssl x509 -req -in "$config_dir/server.csr" -CA "$config_dir/ca.crt" \
        -CAkey "$config_dir/ca.key" -set_serial 2 -days 365 \
        -extfile "$config_dir/server.ext" -out "$config_dir/server.crt" >/dev/null 2>&1

    # 官方 x509 身份规则：CN 为用户名，O 为组；仅本地开发使用高权限证书。
    cat > "$config_dir/client.ext" <<'EOF'
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature
extendedKeyUsage = clientAuth
EOF
    openssl req -new -newkey rsa:2048 -nodes -subj /CN=ingate-dev/O=system:masters \
        -keyout "$config_dir/client.key" -out "$config_dir/client.csr" >/dev/null 2>&1
    openssl x509 -req -in "$config_dir/client.csr" -CA "$config_dir/ca.crt" \
        -CAkey "$config_dir/ca.key" -set_serial 3 -days 365 \
        -extfile "$config_dir/client.ext" -out "$config_dir/client.crt" >/dev/null 2>&1

    cat > "$config_dir/ingate.kubeconfig" <<'EOF'
apiVersion: v1
kind: Config
clusters:
  - name: ingate-dev
    cluster:
      server: https://127.0.0.1:18443
      certificate-authority: ca.crt
users:
  - name: ingate-dev
    user:
      client-certificate: client.crt
      client-key: client.key
contexts:
  - name: ingate-dev
    context:
      cluster: ingate-dev
      user: ingate-dev
      namespace: dev
current-context: ingate-dev
EOF
    cat > "$config_dir/compose.env" <<EOF
MYSQL_ROOT_PASSWORD=$root_password
MYSQL_PASSWORD=$mysql_password
EOF
    cat > "$config_dir/admin-api.yaml" <<EOF
server:
  http: {network: tcp, addr: '127.0.0.1:8000', timeout: 5s}
  grpc: {network: tcp, addr: '127.0.0.1:9000', timeout: 5s}
api_server:
  kubeconfig: .dev/ingate.kubeconfig
mysql:
  dsn: 'ingate:$mysql_password@tcp(127.0.0.1:13306)/ingate'
  max_open_connections: 20
  max_idle_connections: 10
  connection_max_lifetime: 1800s
  connection_max_idle_time: 300s
  ping_timeout: 3s
redis:
  addrs: ['127.0.0.1:16379']
  db: 0
  ping_timeout: 3s
log: {level: debug, format: text, outputs: [stdout]}
EOF
    cat > "$config_dir/controller-manager.yaml" <<EOF
mysql:
  dsn: 'ingate:$mysql_password@tcp(127.0.0.1:13306)/ingate'
  max_open_connections: 20
  max_idle_connections: 10
  connection_max_lifetime: 1800s
  connection_max_idle_time: 300s
  ping_timeout: 3s
EOF
    rm "$config_dir/ca.cnf" "$config_dir/server.ext" "$config_dir/client.ext" \
        "$config_dir/server.csr" "$config_dir/client.csr"
    mv "$config_dir" "$dev_dir"
    trap - EXIT
    echo '已生成 .dev/ 开发配置（未启动服务），请勿提交或共享其中的密钥。'
}

case "${1:-help}" in
    up)
        dev_up
        ;;
    down)
        dev_down
        ;;
    start)
        require_config
        require_component "${COMPONENT:-}"
        if [[ "$COMPONENT" == data-plane ]]; then
            start_dataplane
            exit
        fi
        prepare_component "$COMPONENT"
        start_component "$COMPONENT"
        wait_component "$COMPONENT"
        if [[ "$COMPONENT" == apiserver ]]; then
            bash scripts/dev.sh namespace
        fi
        ;;
    stop)
        require_config
        require_component "${COMPONENT:-}"
        if [[ "$COMPONENT" == data-plane ]]; then
            stop_dataplane
            exit
        fi
        stop_component "$COMPONENT"
        ;;
    status)
        dev_status
        ;;
    logs)
        dev_logs
        ;;
    init)
        init_config
        ;;
    deps-up)
        compose up -d --wait --wait-timeout 180
        ;;
    deps-down)
        compose down
        ;;
    deps-status)
        compose ps
        ;;
    migrate)
        require_config
        exec ./bin/ingate-admin-api --migrate --conf .dev/admin-api.yaml
        ;;
    apiserver)
        require_config
        # 不依赖宿主集群：只保留本地 NamespaceLifecycle，不启动 Webhook/Policy/APF。
        # 委托客户端指向显式的 Ingate 配置；高权限客户端通过上游组授权，无宿主查询。
        exec ./bin/ingate-apiserver \
            --bind-address=127.0.0.1 --secure-port=18443 \
            --etcd-servers=http://127.0.0.1:12379 \
            --cert-dir="$dev_dir" \
            --tls-cert-file="$dev_dir/server.crt" --tls-private-key-file="$dev_dir/server.key" \
            --client-ca-file="$dev_dir/ca.crt" --authentication-skip-lookup=true \
            --authentication-kubeconfig="$dev_dir/ingate.kubeconfig" \
            --authorization-kubeconfig="$dev_dir/ingate.kubeconfig" \
            --kubeconfig="$dev_dir/ingate.kubeconfig" \
            --enable-priority-and-fairness=false \
            --disable-admission-plugins=MutatingAdmissionPolicy,MutatingAdmissionWebhook,ValidatingAdmissionPolicy,ValidatingAdmissionWebhook
        ;;
    namespace)
        require_config
        curl --fail --silent --show-error --retry 30 --retry-connrefused --retry-delay 1 \
            --max-time 3 --retry-max-time 90 --cacert "$dev_dir/ca.crt" \
            --cert "$dev_dir/client.crt" --key "$dev_dir/client.key" \
            https://127.0.0.1:18443/readyz
        printf '\n'
        exec kubectl --kubeconfig="$dev_dir/ingate.kubeconfig" --request-timeout=15s \
            apply -f deploy/dev/namespace.yaml
        ;;
    controller)
        require_config
        exec ./bin/ingate-controller-manager --kubeconfig "$dev_dir/ingate.kubeconfig" \
            --config .dev/controller-manager.yaml --xds-bind-address 127.0.0.1:18000 \
            --ai-ingress-socket /run/ingate/ingress.sock --ai-egress-socket /run/ingate/egress.sock \
            --metrics-bind-address 127.0.0.1:18080 --health-probe-bind-address 127.0.0.1:18081
        ;;
    admin)
        require_config
        exec ./bin/ingate-admin-api --conf .dev/admin-api.yaml
        ;;
    web)
        cd web/console
        exec npm run dev
        ;;
    *)
        echo '用法：bash scripts/dev.sh {up|down|start|stop|status|logs|init|deps-up|deps-down|deps-status|migrate|apiserver|namespace|controller|admin|web}' >&2
        exit 1
        ;;
esac
