# 由 scripts/dev.sh 加载，复用其 repo_dir、dev_dir、compose 和 require_config。
# 生命周期交给 tmux；此处只做固定四个组件的开发接线，不存储 PID 或扫描进程名。

dev_tmux() {
    tmux -S "$dev_dir/tmux.sock" -f "$repo_dir/deploy/dev/tmux.conf" "$@"
}

require_component() {
    case "$1" in
        apiserver|controller|admin-api|web|data-plane) ;;
        *)
            echo 'COMPONENT/EXCLUDE 只支持 apiserver、controller、admin-api、web、data-plane。' >&2
            return 1
            ;;
    esac
}

component_running() {
    [[ $(dev_tmux display-message -p -t "=$1:0.0" '#{pane_dead}' 2>/dev/null) == 0 ]]
}

component_ports() {
    case "$1" in
        apiserver) echo 18443 ;;
        controller) echo '18000 18080 18081' ;;
        admin-api) echo '8000 9000' ;;
        web) echo 5173 ;;
    esac
}

require_web_tools() {
    local tool npm_version
    for tool in node npm shasum; do
        if ! command -v "$tool" >/dev/null; then
            echo "前端缺少开发工具：$tool" >&2
            return 1
        fi
    done
    node --eval 'if (Number(process.versions.node.split(".")[0]) !== 24) {
        console.error("前端需要 Node.js 24，请先调整 PATH；当前为 " + process.versions.node);
        process.exit(1);
    }' || return
    npm_version=$(npm --version)
    case "$npm_version" in
        11.*|12.*) ;;
        *) echo '前端需要 npm 11 或 12。' >&2; return 1 ;;
    esac
}

prepare_component() {
    if component_running "$1"; then
        return
    fi

    case "$1" in
        apiserver) make build BINS=ingate-apiserver ;;
        controller) make build BINS=ingate-controller-manager ;;
        admin-api) make build BINS=ingate-admin-api ;;
        web)
            local lock_hash
            require_web_tools
            lock_hash=$(shasum -a 256 web/console/package-lock.json)
            if [[ ! -d web/console/node_modules || ! -f "$dev_dir/npm-lock.sha256" ]] || \
                [[ $(cat "$dev_dir/npm-lock.sha256") != "$lock_hash" ]]; then
                (cd web/console && npm ci --ignore-scripts)
                printf '%s\n' "$lock_hash" > "$dev_dir/npm-lock.sha256"
            fi
            ;;
    esac
}

start_component() {
    local component=$1 command port launch
    if component_running "$component"; then
        echo "$component 已托管运行，未重启。"
        return
    fi

    # 不接管外部进程；IDE 停止前不能恢复同端口的托管实例。
    for port in $(component_ports "$component"); do
        if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
            echo "$component 的端口 $port 已被占用；请先停止 IDE/外部进程。" >&2
            return 1
        fi
    done

    case "$component" in
        admin-api) command=admin ;;
        *) command=$component ;;
    esac
    mkdir -p "$dev_dir/logs"
    # 日志直接重定向，不经过终端截屏或 shell xtrace；每次启动追加分界，保留失败诊断。
    printf '\n--- %s start ---\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" >> "$dev_dir/logs/$component.log"
    printf -v launch 'exec /bin/bash %q %q >> %q 2>&1' \
        "$repo_dir/scripts/dev.sh" "$command" "$dev_dir/logs/$component.log"
    if dev_tmux has-session -t "=$component" 2>/dev/null; then
        dev_tmux kill-session -t "=$component"
    fi

    dev_tmux new-session -d -s "$component" -c "$repo_dir" -e "PATH=$PATH" "$launch" || return
    if ! dev_tmux has-session -t "=$component" 2>/dev/null; then
        echo 'tmux 未能创建组件会话，请检查 Socket 和终端权限。' >&2
        return 1
    fi
    echo "$component 已启动；日志：.dev/logs/$component.log"
}

component_ready() {
    case "$1" in
        apiserver)
            curl --fail --silent --max-time 2 --cacert "$dev_dir/ca.crt" \
                --cert "$dev_dir/client.crt" --key "$dev_dir/client.key" \
                https://127.0.0.1:18443/readyz >/dev/null
            ;;
        controller)
            curl --fail --silent --max-time 2 http://127.0.0.1:18081/readyz >/dev/null
            ;;
        admin-api)
            # 只验证认证入口已启动，401 不等于浏览器登录可用。
            [[ $(curl --silent --max-time 2 -o /dev/null -w '%{http_code}' \
                http://127.0.0.1:8000/api/admin/v1/accounts/me) == 401 ]]
            ;;
        web)
            curl --fail --silent --max-time 2 http://127.0.0.1:5173/ >/dev/null
            ;;
    esac
}

wait_component() {
    local component=$1 attempt
    for ((attempt=0; attempt<60; attempt++)); do
        if ! component_running "$component"; then
            echo "$component 已退出，请查看 make dev-logs COMPONENT=${component}。" >&2
            return 1
        fi
        if component_ready "$component"; then
            echo "$component 入口检查通过。"
            return
        fi
        sleep 1
    done

    echo "$component 未在等待期限内就绪，进程和日志已保留；用 dev-status/dev-logs 检查。" >&2
    return 1
}

stop_component() {
    local component=$1 attempt
    if ! dev_tmux has-session -t "=$component" 2>/dev/null; then
        echo "$component 没有托管会话，未操作 IDE 或其他进程。"
        return
    fi

    if component_running "$component"; then
        # 相当于前台 Ctrl+C，由组件自行排空、关闭连接；不根据端口或历史 PID 发信号。
        dev_tmux send-keys -t "=$component:0.0" C-c
        for ((attempt=0; attempt<150; attempt++)); do
            if ! component_running "$component"; then
                break
            fi
            sleep 0.2
        done
        if component_running "$component"; then
            echo "$component 在 30 秒内未停止，保留会话和依赖，请先排查。" >&2
            return 1
        fi
    fi

    dev_tmux kill-session -t "=$component"
    echo "$component 已停止。"
}

dev_up() {
    local component excluded excluded_component tool
    local exclusions=${EXCLUDE:-}
    local web_excluded=false dataplane_excluded=false
    for component in ${exclusions//,/ }; do
        require_component "$component"
        if [[ "$component" == web ]]; then
            web_excluded=true
        fi
        if [[ "$component" == data-plane ]]; then
            dataplane_excluded=true
        fi
    done
    if ! "$web_excluded"; then
        require_web_tools
    fi
    for tool in tmux docker make openssl curl kubectl lsof; do
        if ! command -v "$tool" >/dev/null; then
            echo "缺少开发工具：$tool" >&2
            return 1
        fi
    done
    # 在下载、构建和创建配置之前确认 daemon 可用；不替用户启动或切换容器运行时。
    docker info >/dev/null
    if [[ ! -d "$dev_dir" ]]; then
        init_config
    fi
    require_config
    compose up -d --wait --wait-timeout 180 mysql redis etcd

    # 迁移入口即使被排除调试也仍需构建；不重复启动、不自动重启已托管的组件。
    prepare_component admin-api
    bash scripts/dev.sh migrate
    for component in apiserver controller admin-api web; do
        excluded=false
        for excluded_component in ${exclusions//,/ }; do
            if [[ "$component" == "$excluded_component" ]]; then
                excluded=true
            fi
        done
        if "$excluded"; then
            echo "$component 已排除；由 IDE/外部进程提供，现有托管会话不受影响。"
            continue
        fi

        # Admin API 已为迁移准备过，不在同一次启动中重复构建。
        if [[ "$component" != admin-api ]]; then
            prepare_component "$component"
        fi
        start_component "$component"
        # API Server 被排除且尚未由 IDE 启动时，允许 Controller 等待其恢复。
        if [[ "$component" == controller ]] && ! component_ready apiserver; then
            echo 'Controller 已运行，等待 IDE 中的 API Server；之后请执行 make dev-namespace。'
            continue
        fi
        wait_component "$component"
        if [[ "$component" == apiserver ]]; then
            bash scripts/dev.sh namespace
        fi
    done

    if ! "$dataplane_excluded"; then
        start_dataplane
    fi
    echo '启动流程完成；用 make dev-status 查看进程。前端：http://127.0.0.1:5173/gateways?namespace=dev'
}

dev_down() {
    require_config
    stop_dataplane
    local component
    for component in web admin-api controller apiserver; do
        stop_component "$component"
    done
    compose down
}

dev_status() {
    require_config
    local component state exit_code
    printf '%-14s %s\n' COMPONENT PROCESS
    for component in apiserver controller admin-api web; do
        state='未托管（IDE 进程不在此列）'
        if component_running "$component"; then
            state='运行中'
        elif dev_tmux has-session -t "=$component" 2>/dev/null; then
            exit_code=$(dev_tmux display-message -p -t "=$component:0.0" '#{pane_dead_status}')
            state="已退出（${exit_code}），请查看日志"
        fi
        printf '%-14s %s\n' "$component" "$state"
    done
    echo '以上是进程状态，不代表配置已生效或请求链路可用。'
    compose ps
}

dev_logs() {
    require_config
    if [[ "${COMPONENT:-}" == data-plane ]]; then
        compose --profile dataplane logs --follow --tail=100 envoy ai-proxy
        return
    fi
    local component
    local logs=()
    if [[ -n "${COMPONENT:-}" ]]; then
        require_component "$COMPONENT"
        logs=("$dev_dir/logs/$COMPONENT.log")
    else
        for component in apiserver controller admin-api web; do
            if [[ -f "$dev_dir/logs/$component.log" ]]; then
                logs+=("$dev_dir/logs/$component.log")
            fi
        done
    fi
    if [[ ${#logs[@]} == 0 ]]; then
        echo '尚无组件日志，请先启动。' >&2
        return 1
    fi

    exec tail -n 100 -F "${logs[@]}"
}
