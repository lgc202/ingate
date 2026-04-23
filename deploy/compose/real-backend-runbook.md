# Real Backend Integration Runbook

这份文档说明如何把 compose demo 从内置 `sample-backend` 切换到真实业务服务。

## 适用场景

当你已经确认本地 demo 可用，并希望让 Envoy 代理真实业务时，使用这份 runbook。

## 需要准备的参数

至少需要这几个值：

- 后端地址：
  - `BACKEND_ENDPOINT_ADDRESS`
- 后端端口：
  - `BACKEND_ENDPOINT_PORT`
- 后端协议：
  - `BACKEND_PROTOCOL`
- 路由 Host：
  - `GATEWAY_HOST`
- 路由路径前缀：
  - `ROUTE_PATH_PREFIX`

## 推荐做法

先复制环境模板：

```bash
cp deploy/compose/.env.example deploy/compose/.env
```

然后按真实业务修改：

```dotenv
BACKEND_ENDPOINT_ADDRESS=10.0.0.25
BACKEND_ENDPOINT_PORT=8080
BACKEND_PROTOCOL=HTTP
GATEWAY_HOST=api.example.com
ROUTE_PATH_PREFIX=/orders
```

## 参数解释

### `BACKEND_ENDPOINT_ADDRESS`

这是 Envoy 最终要连接的真实上游地址。

推荐：

- 可达的数值 IP

不推荐：

- `host.docker.internal`
- 依赖本机 DNS 的临时主机名

原因是这套 demo 的 backend endpoint 目前按“静态 endpoint”模型下发，数值 IP 最稳定。

### `BACKEND_ENDPOINT_PORT`

这是上游服务监听端口。

例如：

- HTTP 服务在 `8080`
- 内部网关服务在 `8000`

### `BACKEND_PROTOCOL`

当前最稳妥的是：

- `HTTP`

如果你的后端协议不是 HTTP，要先确认 Ingate 当前控制面和 Envoy 配置是否已经支持该协议。

### `GATEWAY_HOST`

这是 Envoy 路由匹配用的主机名。

例如：

- `api.example.com`
- `orders.internal.example.com`

只有请求里的 `Host` 命中这里，才会匹配当前路由。

### `ROUTE_PATH_PREFIX`

这是路径前缀匹配条件。

例如：

- `/orders`
- `/api`
- `/`

`/` 代表基本匹配所有路径，但要注意这会让路由范围更大。

## 启动或更新部署

修改完 `.env` 后执行：

```bash
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
```

如果只是更新配置，也建议直接重新执行上面的命令，让 stack 重新收敛。

## 验证步骤

### 1. 验证控制面资源

确认 backend 已经写成你的真实地址：

```bash
curl -fsS http://127.0.0.1:18080/admin/v1/backends \
  -H 'Authorization: Bearer ingate-dev-admin-api-token'
```

### 2. 验证 xDS 解析结果

确认 `xds-server` 最终解析出的 endpoint 就是你配置的目标：

```bash
./_output/darwin_arm64/ingatectl xds resolve --backend compose-backend --output text
```

### 3. 验证代理流量

```bash
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

把 `api.example.com` 和 `/orders` 替换成你的真实配置。

## 常见接入方式

### 方式一：代理已有内网 HTTP 服务

适合：

- 你的业务服务已经跑在某台机器或某个内网 IP 上
- 你只想先验证 Ingate 的流量接入能力

示例：

```dotenv
BACKEND_ENDPOINT_ADDRESS=10.10.20.15
BACKEND_ENDPOINT_PORT=8080
BACKEND_PROTOCOL=HTTP
GATEWAY_HOST=api.example.com
ROUTE_PATH_PREFIX=/orders
```

### 方式二：本机另一个服务

如果你的业务服务跑在本机，建议不要直接把 compose backend 指向模糊的主机名。

更稳妥的方式通常是：

- 让业务服务也进入同一个 Docker 网络
- 或者给它一个稳定、明确、可达的 IP

## 失败时优先检查什么

### 返回 404

优先看 Host 和路径是否匹配：

- `GATEWAY_HOST`
- `ROUTE_PATH_PREFIX`

### 返回 upstream timeout / reset

优先看上游地址是否真的可达：

- `BACKEND_ENDPOINT_ADDRESS`
- `BACKEND_ENDPOINT_PORT`

并检查：

```bash
curl -fsS http://127.0.0.1:19901/clusters
```

### 前端能看到资源，但代理不通

这通常说明：

- 管理面和控制面是通的
- 数据面到真实后端这最后一跳不通

这时优先用：

```bash
./_output/darwin_arm64/ingatectl xds resolve --backend compose-backend
curl -fsS http://127.0.0.1:19901/clusters
```

## 当前局限

这套 compose 目前更适合：

- 单个静态后端
- 明确的 Host/Path 规则
- 快速验证 HTTP 代理链路

它还不是完整生产交付方案，尤其还缺：

- 证书和 TLS 管理
- 正式鉴权策略
- 监控告警
- 配置和密钥托管
- 更完善的故障恢复策略
