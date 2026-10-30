#!/usr/bin/env bash
# 使用真实 tmux 和无网络的假组件验证托管边界；不启动数据库或 Ingate。
set -euo pipefail
umask 077

source_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
test_dir=$(mktemp -d /tmp/ingate-processes.XXXXXX)
repo_dir="$test_dir/repo with spaces"
dev_dir="$test_dir/state"
mkdir -p "$repo_dir/scripts" "$repo_dir/deploy/dev" "$dev_dir"
cp "$source_dir/deploy/dev/tmux.conf" "$repo_dir/deploy/dev/tmux.conf"
source "$source_dir/scripts/dev/processes.sh"

cleanup() {
    dev_tmux kill-server 2>/dev/null || true
    rm -rf "$test_dir"
}
trap cleanup EXIT

cat > "$repo_dir/scripts/dev.sh" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == admin ]]; then
    echo intentional-failure
    exit 7
fi
if [[ "$1" == migrate || "$1" == namespace ]]; then
    exit 0
fi
trap 'echo graceful-stop; exit 0' INT
echo "started-$1"
while :; do sleep 0.1; done
EOF

# 组件不监听网络；端口碰撞分支在最后单独验证。
component_ports() { :; }

assert_not_managed() {
    if dev_tmux has-session -t "=$1" 2>/dev/null; then
        echo "FAIL: unexpected managed session: $1" >&2
        exit 1
    fi
}

if require_component unknown 2>/dev/null; then
    echo 'FAIL: accepted unknown component' >&2
    exit 1
fi

start_component apiserver
start_component controller
sleep 0.2
component_running apiserver
component_running controller
first_pane=$(dev_tmux display-message -p -t '=apiserver:0.0' '#{pane_id}')
start_component apiserver
[[ $(dev_tmux display-message -p -t '=apiserver:0.0' '#{pane_id}') == "$first_pane" ]]

stop_component apiserver
assert_not_managed apiserver
component_running controller
rg -q graceful-stop "$dev_dir/logs/apiserver.log"
stop_component apiserver

start_component admin-api
for ((attempt=0; attempt<50; attempt++)); do
    if [[ $(dev_tmux display-message -p -t '=admin-api:0.0' '#{pane_dead}') == 1 ]]; then
        break
    fi
    sleep 0.1
done
[[ $(dev_tmux display-message -p -t '=admin-api:0.0' '#{pane_dead_status}') == 7 ]]
if wait_component admin-api; then
    echo 'FAIL: exited component reported ready' >&2
    exit 1
fi
rg -q intentional-failure "$dev_dir/logs/admin-api.log"
stop_component admin-api

# 模拟端口被外部监听；拒绝创建会话，也不影响其他托管组件。
component_ports() { echo 8000; }
lsof() { return 0; }
if start_component admin-api 2>/dev/null; then
    echo 'FAIL: occupied port was accepted' >&2
    exit 1
fi
assert_not_managed admin-api
component_running controller
stop_component controller

# 检查一键启动的排除规则，外部依赖与就绪探测使用替身，进程仍由真实 tmux 运行。
unset -f lsof
component_ports() { :; }
require_config() { :; }
compose() { :; }
docker() { :; }
prepare_component() { :; }
component_ready() { return 0; }
EXCLUDE=admin-api,web,data-plane
(cd "$repo_dir" && dev_up)
component_running apiserver
component_running controller
assert_not_managed admin-api
assert_not_managed web
stop_component controller
stop_component apiserver

# 用确定的探测次数模拟超过 30 秒的 API Server 排空；不实际等待长超时。
(
    probes=0
    removed=false
    dev_tmux() {
        if [[ "$1" == kill-session ]]; then
            removed=true
        fi
    }
    component_running() {
        probes=$((probes+1))
        ((probes<180))
    }
    sleep() { :; }

    stop_component apiserver
    [[ "$removed" == true ]]

    probes=0
    removed=false
    if stop_component controller 2>/dev/null; then
        echo 'FAIL: component exceeding its stop deadline reported stopped' >&2
        exit 1
    fi
    [[ "$removed" == false ]]
)

echo 'PASS: exact selection, quoted paths, idempotent start/stop, graceful interrupt, failure status/logs, occupied-port refusal, excluded components'
