# Operations And Troubleshooting

这份文档面向运维和排障，说明这套 compose stack 的日常操作和常见故障处理。

## 常用命令

启动：

```bash
make compose-up
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
```

停止：

```bash
make compose-down
```

查看状态：

```bash
make compose-ps
```

查看日志：

```bash
make compose-logs
```

做一次完整验收：

```bash
make verify-compose
```

## 启动后先看什么

建议按这个顺序确认：

1. 前端是否能打开：

```bash
curl -fsS http://127.0.0.1:8088/
```

2. `admin-api` 是否健康：

```bash
curl -fsS http://127.0.0.1:18080/healthz
```

3. Envoy admin 是否 ready：

```bash
curl -fsS http://127.0.0.1:19901/ready
```

4. 代理流量是否可达：

```bash
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

## 排障分层

### 前端层

看页面是否能打开，是否能读到 `admin-api` 数据。

常见现象：

- 页面打不开：
  - 先看 `console` 容器是否健康
- 页面能打开但资源为空：
  - 看浏览器里保存的 `服务地址` 和 `访问凭据`

### 管理面层

主要看 `admin-api`。

常见现象：

- `401 Unauthorized`
  - Bearer Token 错误
- 前端有请求但读不到资源
  - 看 `admin-api` 日志和 `apiserver` 健康状态

### 控制面层

主要看：

- `apiserver`
- `controller-manager`
- `xds-server`
- `etcd`

### 数据面层

主要看：

- `envoy`
- 上游 backend endpoint

## 典型问题

### 1. 不带 Host 头访问代理返回 404

示例：

```bash
curl http://127.0.0.1:10080/orders
```

这会失败，是预期行为。

原因：

- 当前默认路由要求 `Host=api.example.com`
- 上面这条命令实际发出的 Host 是 `127.0.0.1:10080`

正确方式：

```bash
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

### 2. Envoy 返回 upstream connect error / timeout

优先判断是短暂启动抖动还是持续不可达。

先看：

```bash
curl -fsS http://127.0.0.1:19901/clusters
```

如果能看到类似：

```text
compose-backend::172.31.250.10:8080
```

说明 xDS 已下发到 Envoy。

然后看 `cx_connect_fail`、`rq_error`、`rq_success` 这些统计值。

再用：

```bash
./_output/darwin_arm64/ingatectl xds resolve --backend compose-backend
```

确认控制面解析出的 endpoint 是否正确。

### 3. `compose-up` 时 `apiserver` 起不来

如果你本机已经有一套旧主栈在跑，`compose-up` 可能发生半重建，导致：

- 一部分容器是旧的
- 一部分容器是新的
- `apiserver` 在重新接网时短暂查不到 `etcd`

典型日志会看到：

```text
lookup etcd on 127.0.0.11:53: no such host
```

处理方式：

```bash
make compose-down
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
```

也就是先完整下掉，再全量起一遍，不要在“半旧半新”的栈上继续重试。

### 4. `verify-compose` 失败，但手动 curl 又能通

之前这套部署里出现过这种情况。

原因不是 Envoy 坏了，而是宿主机端口探针时序不稳定。

现在 `verify-compose` 已经改成：

- 用宿主机验证 `apiserver`、`admin-api`、`console`
- 用 compose 内网验证 Envoy admin 和代理流量

如果它再失败，优先看 compose 日志，不要先怀疑前端接入。

### 5. 网络段冲突

如果本机已有 Docker 网络占用了默认子网，启动可能失败。

处理方式：

在 `deploy/compose/.env` 修改：

```dotenv
COMPOSE_SUBNET=172.31.250.0/24
SAMPLE_BACKEND_ADDRESS=172.31.250.10
BACKEND_ENDPOINT_ADDRESS=172.31.250.10
```

如果你改了子网，又还在用内置 `sample-backend`，这三个值必须保持一致。

## 推荐排障命令

查看控制面资源：

```bash
curl -fsS http://127.0.0.1:18080/admin/v1/routes \
  -H 'Authorization: Bearer ingate-dev-admin-api-token'
curl -fsS http://127.0.0.1:18080/admin/v1/backends \
  -H 'Authorization: Bearer ingate-dev-admin-api-token'
```

看 xDS 解析：

```bash
./_output/darwin_arm64/ingatectl xds list --output text
./_output/darwin_arm64/ingatectl xds summary --gateway compose-gateway --output text
./_output/darwin_arm64/ingatectl xds resolve --backend compose-backend --output text
./_output/darwin_arm64/ingatectl xds check --gateway compose-gateway --backend compose-backend --output text
```

看 Envoy：

```bash
curl -fsS http://127.0.0.1:19901/ready
curl -fsS http://127.0.0.1:19901/clusters
curl -fsS http://127.0.0.1:19901/config_dump
docker logs ingate-envoy-1 --tail 200
```

## 重置策略

如果状态明显乱了，最稳妥的恢复方式是：

```bash
make compose-down
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
```

不要一边保留旧主栈，一边局部重建。

## 当前已知限制

- 默认 token 只适合本地开发，不适合生产
- demo 资源由 `init-control-plane` 自动写入，不是多租户模型
- compose 方案当前更偏单机演示和集成验证
- 真实生产化还需要单独补证书、监控、备份、密钥管理和升级策略
