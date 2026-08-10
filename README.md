# Ingate

Ingate 是以 Envoy 为唯一数据平面的声明式 API 网关控制面。当前仓库只保留可以完整运行的 HTTP/HTTPS 网关能力，不包含 AI 网关、Agent、访问密钥或计费能力。

## 架构

```text
Browser
  |
  v
ingate-console ---> ingate-admin-api ---> ingate-apiserver ---> etcd
                                                 ^
                                                 | watch/status
                                                 v
                                      ingate-controller ---> Envoy
                                                |               |
                                                +---- Redis <---+
                                                     内置限流
```

- `ingate-console`：控制台静态资源和管理 API 反向代理入口
- `ingate-admin-api`：面向控制台的产品 API 和业务校验
- `ingate-apiserver`：声明式资源 CRUD、Watch、版本和 Status
- `ingate-controller`：监听资源、编译 Envoy 配置、提供 xDS 并回写 Status
- `Envoy`：唯一数据平面
- `etcd`：声明式资源持久化，仅由 API Server 访问
- `Redis`：内置限流插件的共享计数存储

当前资源：

- `Gateway`
- `Route`
- `Upstream`
- `Certificate`
- `RateLimitPolicy`
- `IPRestrictionPolicy`

配置链路：

```text
Resource -> Envoy Config Compiler -> Config Delivery -> xDS Snapshot Cache -> Envoy
```

## 已支持能力

- 多个逻辑 Gateway
- HTTP 与 HTTPS Listener
- SNI、TLS 终止和可复用证书
- Host、路径、方法和 Header 路由匹配
- 多 Upstream 加权转发
- 上游 HTTPS、负载均衡和主动健康检查
- 请求与响应 Header 修改
- Route 超时和重试
- 基于 Redis 的共享、客户端 IP 或请求 Header 限流
- IP 允许列表和拒绝列表
- Console OIDC 登录与角色授权

## 本地运行

```bash
make check-tools
make generate
make docker-up
```

默认入口：

- Console：`http://127.0.0.1:8001`
- HTTP Gateway：`http://127.0.0.1:8080`
- HTTPS Gateway：`https://127.0.0.1:8443`

停止环境：

```bash
make docker-down
```

## 开发

```bash
make generate
make verify
```

生成工具安装在 `_output/tools`，构建产物写入 `_output`。资源协议见 [docs/resources](docs/resources)，组件关系见 [docs/architecture.md](docs/architecture.md)。
