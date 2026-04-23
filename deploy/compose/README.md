# Docker Compose Deployment

这一套部署面把 `etcd`、`ingate-apiserver`、`ingate-controller-manager`、`ingate-xds-server`、`ingate-admin-api`、前端 console、Envoy 和一个 sample backend 打成一个可直接启动的 `docker compose` stack。

目标是两件事：

- 一条命令起完整控制面、前端控制台和数据面
- 直接通过 Envoy 真实代理一个 HTTP 后端

## 文档导航

- [架构总览](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/architecture.md)
- [真实后端接入手册](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/real-backend-runbook.md)
- [运维与排障手册](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/operations.md)
- [`ingatectl` 使用说明](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/ingatectl.md)

## 前置条件

- Docker Engine
- Docker Compose v2

## 快速启动

先按需复制环境模板：

```bash
cp deploy/compose/.env.example deploy/compose/.env
```

然后启动整套 stack：

```bash
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
```

如果你只想用原生 `docker compose` 命令，先构建本地运行镜像，再启动：

```bash
make compose-build
docker compose -f deploy/compose/compose.yaml --env-file deploy/compose/.env up -d --no-build
```

默认会创建：

- `Gateway`: `compose-gateway`
- `Route`: `compose-orders-route`
- `Backend`: `compose-backend`

默认流量入口：

- Console: `http://127.0.0.1:8088`
- Envoy proxy: `http://127.0.0.1:10080`
- Envoy admin: `http://127.0.0.1:19901`
- admin-api: `http://127.0.0.1:18080`
- apiserver: `https://127.0.0.1:18443`

前端控制台默认会直接访问：

- `服务地址`: `http://127.0.0.1:18080`
- `访问凭据`: `ingate-dev-admin-api-token`

启动后可以直接验证代理：

```bash
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

预期响应：

```text
sample-backend-ok
```

## 服务组成

compose stack 包含：

- `etcd`
- `apiserver`
- `controller-manager`
- `xds-server`
- `admin-api`
- `console`
- `sample-backend`
- `init-control-plane`
- `envoy`

其中 `init-control-plane` 是一次性初始化容器，职责是：

- 等待 apiserver / controller-manager / xds-server 就绪
- 自动写入 `Gateway` / `Route` / `Backend`

## 前端控制台构建

compose 打包 console 镜像时会直接复用相邻仓库 `../ingate-console` 里的静态产物 `dist/`。

如果你更新了前端源码，先在前端仓库执行：

```bash
cd ../ingate-console
npm run build
```

然后回到本仓库重新执行：

```bash
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
```

## 指向真实业务后端

默认 demo 使用内置 sample backend，地址是固定容器 IP `172.31.250.10:8080`。

如果本机已有 Docker 网络占用了默认网段，可以在 `deploy/compose/.env` 里一起调整：

```dotenv
COMPOSE_SUBNET=172.31.250.0/24
SAMPLE_BACKEND_ADDRESS=172.31.250.10
BACKEND_ENDPOINT_ADDRESS=172.31.250.10
```

这三个值在使用内置 sample backend 时需要保持一致。

如果你要代理真实业务，把 [`deploy/compose/.env.example`](/Users/guangcaili/workplace/code/lgc202/ingate/deploy/compose/.env.example) 复制成 `deploy/compose/.env` 后修改：

```dotenv
BACKEND_ENDPOINT_ADDRESS=10.0.0.25
BACKEND_ENDPOINT_PORT=8080
BACKEND_PROTOCOL=HTTP
GATEWAY_HOST=api.example.com
ROUTE_PATH_PREFIX=/orders
```

注意：

- 这里建议使用可达的数值 IP
- 不建议直接把 `host.docker.internal` 写进 `Backend` endpoint
- 如果你的业务服务不是 HTTP，需要让 `BACKEND_PROTOCOL` 与后端协议一致
- 如果你改了 `COMPOSE_SUBNET` 但仍使用内置 sample backend，要同步调整 `SAMPLE_BACKEND_ADDRESS` 和 `BACKEND_ENDPOINT_ADDRESS`

修改后重新启动 stack：

```bash
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
```

## 常用命令

```bash
make compose-build
make compose-up
make compose-up COMPOSE_ENV_FILE=deploy/compose/.env
make compose-ps
make compose-logs
make compose-down
make verify-compose
```

`make verify-compose` 会：

- 构建镜像
- 启动 compose stack
- 等待前端控制台、控制面和 Envoy 就绪
- 验证通过 Envoy 的真实转发
- 自动清理 stack

## 手动清理

```bash
docker compose -f deploy/compose/compose.yaml --env-file deploy/compose/.env down -v --remove-orphans
```
