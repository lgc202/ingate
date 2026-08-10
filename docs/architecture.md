# Ingate 架构

## 产品边界

一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例。一套 Ingate 可以创建多个逻辑 Gateway，但不承担生产、测试或租户之间的控制面隔离；需要隔离时部署多套 Ingate。

Envoy 是唯一数据平面。控制面直接生成 Envoy 配置，不建立通用数据平面适配层。

## 组件通信

```text
                    management traffic
Browser -> Console ----------------------> Admin API
                                               |
                                               | resource client
                                               v
                                         API Server -> etcd
                                               ^
                                               | watch/status
                                               v
                                          Controller
                                               |
                                               | xDS
                                               v
Client -------------------------------------> Envoy
                                               |
                                               +----> Upstream
                                               |
                                               +----> Redis
                                                     rate-limit state
```

边界规则：

- Console 只访问 Admin API
- Admin API 只通过 API Server 读写声明式资源，不访问 etcd
- Controller 只通过 API Server Watch 资源和回写 Status，不访问 etcd
- API Server 是 etcd 的唯一访问者
- Envoy 通过 xDS 获取配置，通过内置插件访问 Redis
- Redis 不是用户资源，安装时提供统一地址

## 资源模型

```text
Gateway <---- Route ----> Upstream
   |          ^             |
   |          |             +-- endpoints / TLS / health check
   |          |
   +-- Certificate
   |
   +-- RateLimitPolicy
   +-- IPRestrictionPolicy
```

- `Gateway` 定义端口、协议、域名范围和 TLS 证书
- `Route` 把一个或多个 Gateway 的请求匹配到一个或多个 Upstream
- `Upstream` 定义 HTTP 服务端点和连接策略
- `Certificate` 保存可复用的 TLS 证书与私钥
- Policy 通过 `targetRefs[]` 直接作用于 Gateway 或 Route

资源由 `metadata.name` 作为不可变 ID。Admin API 为控制台生成 UUID，并用 `spec.displayName` 保存用户可编辑名称。资源版本、创建时间、更新时间和 Status 由系统维护。

## Controller

Controller 在一个进程中保留清晰的职责边界：

```text
Resource Watch -> Reconcile -> Envoy Compiler -> Delivery -> xDS
                                     |
                                     +-----------> Status
```

- Resource Watch 维护声明式资源缓存并触发收敛
- Compiler 把当前完整资源集合编译为 Envoy Listener、Route、Cluster 和 Secret
- Delivery 只在完整配置通过校验后切换 Active 配置
- xDS 使用 Envoy Snapshot Cache 向数据面下发 Active 配置
- Status 把资源是否成功参与当前配置回写到 API Server

Candidate 和 Active 只存在于进程内。Controller 重启后从声明式资源重新全量编译，不持久化 Last Good。

## 内置治理

`RateLimitPolicy` 和 `IPRestrictionPolicy` 是强类型资源。Controller 将它们编译为内置 Wasm filter 的执行索引，用户不接触插件私有 JSON、Wasm 文件路径、phase 或 priority。

限流使用 Redis 共享计数。IP 访问限制不依赖外部存储。策略目标不存在时，只影响对应目标状态，不阻止其他有效目标继续发布。

## 部署

服务二进制、YAML 配置、环境变量、健康检查和优雅退出与部署方式无关。Docker Compose 只用于开发和联调，每个组件独立容器。Controller 与 Envoy 共享网络命名空间，使未启用 mTLS 的 xDS 只监听 loopback。

安装路径统一使用 `/opt/ingate/<component>`；运行数据由部署方式挂载到明确目录。
