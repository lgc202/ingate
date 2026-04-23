# Compose Deployment Architecture

这份文档解释 `deploy/compose` 这一套部署面的组件关系、配置流向和流量路径。

## 组件分层

compose stack 可以分成三层：

- 管理面：
  - `console`
  - `admin-api`
- 控制面：
  - `apiserver`
  - `controller-manager`
  - `xds-server`
  - `etcd`
- 数据面：
  - `envoy`
  - 真实业务后端，或者内置 `sample-backend`

## 组件职责

### `console`

- 提供前端管理界面
- 默认访问 `http://127.0.0.1:18080`
- 不直接访问 `apiserver` 或 `etcd`

### `admin-api`

- 给前端提供资源读写接口
- 负责把前端操作转成对 `apiserver` 的调用
- 默认需要 Bearer Token：`ingate-dev-admin-api-token`

### `apiserver`

- 保存和提供 Ingate 资源对象
- 当前资源最终持久化在 `etcd`
- 是控制面的资源权威入口

### `controller-manager`

- 监听资源变化并做控制循环
- 把 `Gateway`、`Route`、`Backend` 等对象整理成可发布状态
- 产出更接近数据面的聚合结果

### `xds-server`

- 读取控制面已解析出的网关配置
- 向 Envoy 提供 xDS/ADS 资源
- 也是 `ingatectl` 默认连接的服务

### `envoy`

- 接收真实流量
- 向 `xds-server` 拉取 Listener、Route、Cluster、Endpoint
- 根据 Host 和 Path 匹配请求，再转发到上游

### `sample-backend`

- 本地 demo 用的内置 HTTP 后端
- 默认地址是 `172.31.250.10:8080`
- 用于验证控制面到数据面的整条链路

### `init-control-plane`

- 一次性初始化容器
- 等待 `apiserver`、`controller-manager`、`xds-server` 和 `sample-backend` 就绪
- 自动写入 demo 资源：
  - `compose-gateway`
  - `compose-orders-route`
  - `compose-backend`

## 配置流向

配置流大致是：

1. `init-control-plane` 或前端通过 `admin-api` 创建/更新 `Gateway`、`Route`、`Backend`
2. `apiserver` 持久化资源
3. `controller-manager` 做状态收敛
4. `xds-server` 读取已发布配置并生成 xDS 响应
5. `envoy` 拉取并应用 xDS 资源
6. 外部请求命中 Envoy，再被转发到 backend endpoint

## 请求流向

### 管理请求

```text
Browser -> console -> admin-api -> apiserver -> etcd
```

说明：

- `console` 本身是静态站点
- 浏览器里的 API 请求直接打到 `admin-api`
- `admin-api` 再调用 `apiserver`

### 数据请求

```text
Client -> Envoy -> backend endpoint
```

当前 demo 下的完整链路是：

```text
Client
  -> http://127.0.0.1:10080
  -> Envoy listener
  -> route(host=api.example.com, prefix=/orders)
  -> cluster compose-backend
  -> 172.31.250.10:8080
  -> sample-backend
```

## 当前默认 demo 资源

默认资源来自 `deploy/compose/.env.example` 和 `deploy/compose/init/seed.sh`。

- Gateway:
  - `compose-gateway`
- Route:
  - `compose-orders-route`
- Backend:
  - `compose-backend`
- Host:
  - `api.example.com`
- PathPrefix:
  - `/orders`

所以当前这条命令会命中路由：

```bash
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

而下面这条不会命中：

```bash
curl http://127.0.0.1:10080/orders
```

原因是它的 `Host` 头会变成 `127.0.0.1:10080`，不匹配当前路由里的 `api.example.com`。

## 关键端口

- `8088`:
  - 前端控制台
- `18080`:
  - `admin-api`
- `18443`:
  - `apiserver`
- `19090`:
  - `xds-server` gRPC
- `19901`:
  - Envoy admin
- `10080`:
  - Envoy proxy

## `ingatectl` 在这套架构里的位置

`ingatectl` 不是资源写入工具，目前主要是控制面排障工具。

它默认连接：

```text
127.0.0.1:19090
```

也就是 compose 暴露出来的 `xds-server` gRPC 地址。

因此：

- `admin-api` 看的是“资源对象是什么”
- `ingatectl` 看的是“xds-server 最终解析成了什么”
- `curl http://127.0.0.1:10080/...` 看的是“Envoy 最终有没有真的转发成功”
