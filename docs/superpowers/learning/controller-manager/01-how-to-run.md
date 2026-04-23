# 01. 怎么启动和验证 controller-manager / xds-server

这一篇只讲本地 bring-up，不展开源码。

目标是证明三件事：

1. `ingate-controller-manager` 能连上 `ingate-apiserver`
2. 它能把 `Gateway/Route/Backend/...` 收敛成 `ResolvedGateway`
3. `ingate-xds-server` 能 watch `ResolvedGateway`，把 `Programmed` 置为 `True`，并通过 discovery gRPC 返回后端 endpoint

## 依赖关系

本地链路是：

```text
Gateway / Route / Backend / Policy
  -> ingate-apiserver
  -> ingate-controller-manager
  -> ResolvedGateway
  -> ingate-xds-server
  -> Programmed=True
```

运行前至少要有：

- `etcd`
- `ingate-apiserver`
- `ingate-controller-manager`
- `ingate-xds-server`（如果要验证发布链路）

## 构建

```bash
make build-controller-manager
make build-xds-server
make build-ingatectl
```

产物默认在：

```text
_output/<os>_<arch>/ingate-controller-manager
_output/<os>_<arch>/ingate-xds-server
_output/<os>_<arch>/ingatectl
```

## 为什么这里用 kubeconfig

`ingate-apiserver` 默认启用了 bearer token 认证。

`controller-manager` 和 `xds-server` 本地开发时最省事的方式是：

1. 启动本地 apiserver
2. 用 `tools/hack/write-apiserver-kubeconfig.sh` 生成一个 kubeconfig
3. 让两个进程通过 `--kubeconfig` 访问 apiserver

这个 kubeconfig 默认：

- 使用 admin token
- `insecure-skip-tls-verify: true`
- 指向本地 `https://127.0.0.1:18443`

## 自动验证

验证 controller-manager：

```bash
make verify-controller-manager
```

这个脚本会自动做：

1. 启动本地 apiserver
2. 生成临时 kubeconfig
3. 启动 controller-manager
4. 直接向 apiserver 创建 `Secret/Certificate/Gateway/Backend/Route/AuthPolicy/TrafficPolicy`
5. 等待 `ResolvedGateway` 出现
6. 验证 `ResolvedGateway.status.Accepted=True`
7. 验证 `ResolvedGateway.status.Resolved=True`
8. 验证原始 `Gateway/Route/Backend` 也已写回 Accepted/Resolved

预期输出类似：

```text
CONTROLLER_MANAGER_HEALTHZ=ok
CONTROLLER_MANAGER_RESOLVEDGATEWAY_READY=yes
CONTROLLER_MANAGER_STATUS_VERIFY=yes
```

验证 xds-server：

```bash
make verify-xds-server
```

这个脚本会自动做：

1. 启动本地 apiserver
2. 启动 controller-manager
3. 启动 xds-server
4. 创建 `Gateway/Backend/Route`
5. 等待 `ResolvedGateway.status.Programmed=True`
6. 验证 xds-server 的 healthz 和 gRPC 端口都可用
7. 用 `ingatectl xds resolve` 调 discovery gRPC，确认 backend endpoint 可被解析
8. 用 `ingatectl xds resolve --output text` 验证终端友好的 endpoint 视图
9. 用 `ingatectl xds config` 读取当前 `EffectiveConfig`
10. 用 `ingatectl xds config --output text` 验证终端友好的配置摘要
11. 用 `ingatectl xds list` 列出当前已发布的 gateway
12. 用 `ingatectl xds list --output text` 验证终端友好的发布列表
13. 用 `ingatectl xds summary` 读取更适合排障的聚合摘要
14. 用 `ingatectl xds summary --output text` 验证终端友好的值班视图
15. 用 `ingatectl xds check` 做一站式 readiness 检查
16. 用 `ingatectl xds check --output text` 验证终端友好的检查视图
17. 用 `ingatectl xds ads --gateway <gateway> --type lds|rds|cds|eds` 拉取标准 ADS/xDS 资源
18. 校验 LDS 返回 listener
19. 校验 RDS/CDS/EDS 分别返回 route、cluster、endpoint 资源

预期输出类似：

```text
XDS_SERVER_HEALTHZ=ok
XDS_SERVER_GRPC_READY=yes
XDS_SERVER_PROGRAMMED_VERIFY=yes
XDS_SERVER_DISCOVERY_VERIFY=yes
XDS_SERVER_DISCOVERY_TEXT_VERIFY=yes
XDS_SERVER_CONFIGSYNC_VERIFY=yes
XDS_SERVER_CONFIGSYNC_TEXT_VERIFY=yes
XDS_SERVER_LIST_VERIFY=yes
XDS_SERVER_LIST_TEXT_VERIFY=yes
XDS_SERVER_SUMMARY_VERIFY=yes
XDS_SERVER_SUMMARY_TEXT_VERIFY=yes
XDS_SERVER_CHECK_VERIFY=yes
XDS_SERVER_CHECK_TEXT_VERIFY=yes
XDS_SERVER_ADS_LDS_VERIFY=yes
XDS_SERVER_ADS_RDS_VERIFY=yes
XDS_SERVER_ADS_CDS_VERIFY=yes
XDS_SERVER_ADS_EDS_VERIFY=yes
```

验证真实 Envoy 对接：

```bash
make verify-envoy
```

这个脚本会在 `verify-xds-server` 的基础上额外做三件事：

1. 启动一个本地 mock backend
2. 启动 Docker 里的 Envoy，使用 `xds-server` 作为 ADS 服务端，最后通过 `curl http://127.0.0.1:10080/orders -H 'Host: api.example.com'` 验证真实转发
3. 创建一个只绑定到 `slow.example.com` 的慢路由和 `TrafficPolicy.timeout`，验证 `curl http://127.0.0.1:10080/slow-orders -H 'Host: slow.example.com'` 会被 Envoy 切成超时
4. 创建一个只绑定到 `retry.example.com` 的重试路由和 `TrafficPolicy.retry`，验证 `curl http://127.0.0.1:10080/retry-orders -H 'Host: retry.example.com'` 会在前两次上游返回 `503` 后由 Envoy 重试并最终返回 `200`
5. 创建一个只绑定到 `limited.example.com` 的限流路由和 `TrafficPolicy.rateLimit`，验证第一次 `curl http://127.0.0.1:10080/limited-orders -H 'Host: limited.example.com'` 返回 `200`，第二次立刻返回 `429`，并且后端只收到一次请求

预期会额外看到：

```text
XDS_SERVER_ENVOY_VERIFY=yes
XDS_SERVER_TRAFFIC_POLICY_TIMEOUT_VERIFY=yes
XDS_SERVER_TRAFFIC_POLICY_RETRY_VERIFY=yes
XDS_SERVER_TRAFFIC_POLICY_RATELIMIT_VERIFY=yes
```

## Docker Compose 一键体验

如果你想直接起完整控制面、Envoy 和一个 sample backend，不分终端手动拉进程，可以直接用 compose：

```bash
make compose-up
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

预期响应：

```text
sample-backend-ok
```

如果你只想做一次自动化验证：

```bash
make verify-compose
```

预期输出类似：

```text
COMPOSE_APISERVER_HEALTHZ=ok
COMPOSE_ADMIN_API_HEALTHZ=ok
COMPOSE_ENVOY_READY=yes
COMPOSE_PROXY_VERIFY=yes
COMPOSE_PROXY_BODY=sample-backend-ok
```

compose 版运行说明在：

- [`deploy/compose/README.md`](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/README.md)

## 手动运行

如果你想分终端手动跑，顺序建议是：

1. apiserver
2. 生成 kubeconfig
3. controller-manager
4. xds-server

示例：

```bash
make run-apiserver

KUBECONFIG_OUTPUT=/tmp/ingate.kubeconfig ./tools/hack/write-apiserver-kubeconfig.sh

_output/$(go env GOOS)_$(go env GOARCH)/ingate-controller-manager \
  --apiserver-address=https://127.0.0.1:18443 \
  --kubeconfig=/tmp/ingate.kubeconfig

_output/$(go env GOOS)_$(go env GOARCH)/ingate-xds-server \
  --apiserver-address=https://127.0.0.1:18443 \
  --kubeconfig=/tmp/ingate.kubeconfig
```

默认端口：

```text
controller-manager healthz: http://127.0.0.1:18081/healthz
xds-server healthz:         http://127.0.0.1:19091/healthz
xds-server grpc:            127.0.0.1:19090
```

## 排查顺序

如果验证失败，先按这个顺序看：

1. `etcd` 是否可用
2. apiserver `healthz` 是否正常
3. kubeconfig 是否指向了正确地址和 token
4. controller-manager 日志里是否已经注册 trigger controller 和 `resolvedgateway`
5. apiserver 上是否已经出现 `ResolvedGateway`
6. `ResolvedGateway.status.conditions` 是否只有 `Accepted/Resolved` 没有 `Programmed`

如果只有最后一步没成功，问题通常就在 `xds-server` 消费链路，而不是收敛链路。
