# DataPlane Capability 服务设计

## 背景

Ingate 的数据面需要承载一类 Wasm 插件不适合直接完成的能力：Redis 全局限流、共享配额、缓存、Secret 解析、Kafka/OTLP 事件投递等。这些能力通常需要连接池、超时、重试、熔断、批量、凭据处理和观测。它们不应该散落在每个插件里，也不应该用某个具体策略命名成临时执行器。

当前阶段不维护 Envoy fork，因此不实现自定义 hostcall。`ingate-dataplane` 使用标准 Proxy-Wasm `DispatchHttpCall` 作为第一种传输方式。长期如果引入 Ingate 专用 Envoy hostcall，插件只替换 runtime client 的传输实现，控制面模型和 capability 协议保持稳定。

## 目标

- 建立 `ingate-dataplane`，作为 Envoy 同生命周期的数据面服务
- 用 capability API 表达数据面能力，不再暴露 `executor` 语义
- 让插件只依赖 runtime client，不直接感知 HTTP path、cluster 和 wire protocol 细节
- 让限流能力成为第一个 capability，后续可扩展 Event、Secret、Cache、Quota
- 保留未来迁移 hostcall 的空间

## 非目标

- 当前不实现 Envoy fork 或自定义 hostcall
- 当前不把 Kafka、Secret、Cache 一次性实现
- 当前不把 RedisStore 配置改成 runtime 独立 watch 控制面，先保持现有 xDS 分发闭环

## 架构

```text
Wasm Plugin
  -> DataPlane Client
      -> 当前：DispatchHttpCall 到 ingate-dataplane
      -> 未来：Ingate hostcall
  -> ingate-dataplane
      -> server/router.go
      -> service/ratelimit
      -> service/event
      -> service/secret
```

`ingate-dataplane` 是数据面服务，不是控制面业务服务。all-in-one 中由 entrypoint 一起启动；生产形态可以和 Envoy sidecar 同 Pod 或同机部署。

## 包组织

```text
cmd/ingate-dataplane

internal/dataplane/app
  app.go
  options.go

internal/dataplane/server
  server.go
  router.go

internal/dataplane/service
  service.go

internal/dataplane/service/ratelimit
  service.go
  algorithm.go
  redis.go

pkg/xredis
  doc.go
  types.go
  client.go

pkg/dataplane/ratelimit
  doc.go
  types.go

plugins/ratelimit/internal/app
plugins/ratelimit/internal/wasm
plugins/ratelimit/internal/policy
plugins/ratelimit/internal/dataplane

pkg/plugin/ratelimit

plugins/internal/wasm
```

`pkg/dataplane/ratelimit` 是限流插件和 `ingate-dataplane` 共用的数据契约包。这里放稳定 DTO 和请求自身校验，不放 Redis client、限流算法或 HTTP 处理逻辑。

`pkg/plugin/ratelimit` 表达 xDS 下发的可执行限流插件配置，由 xDS target、xDS server 和插件共享。`plugins/ratelimit` 按插件运行阶段组织：`app` 只负责插件装配注册，`wasm` 只负责 Proxy-Wasm 生命周期、Pause/Resume/SendResponse 等适配动作，`policy` 承载限流策略判断、key 生成、本地计数和 global check 生成，`dataplane` 封装插件到 `ingate-dataplane` 的调用。多个插件共享的 Proxy-Wasm 辅助能力放在 `plugins/internal/wasm`。

`internal/dataplane/server/router.go` 使用 Gin 统一注册 HTTP 路由，承载协议绑定、JSON 编解码和状态码返回，不把路由注册散落到具体能力包里。

`pkg/xredis` 承载 Redis 连接配置、连接池复用和 standalone/sentinel/cluster 差异。它是 Ingate 内部服务共享基础能力，不依赖限流协议。

`internal/dataplane/service/ratelimit` 承载限流用例和算法执行，不直接写 HTTP 响应，也不直接维护 Redis 连接池；它只把限流协议里的 RedisStore 转换成 `xredis.Config`。

## Capability API

当前限流 API：

```text
POST /v1/capabilities/rate-limit/check
```

请求和响应使用稳定 JSON 数据契约，不在 payload 内额外携带版本字段。当前 API 版本由路径 `/v1` 表达；后续如果出现不兼容升级，再通过新路径或新 Go 包拆分。插件主流程只依赖 dataplane client，不直接依赖传输细节。

## 插件边界

插件主流程负责：

- 根据 Envoy route name 匹配 Route/Rule
- 从请求上下文生成限流 key
- 执行本地限流
- 对 Global 限流调用 dataplane client
- 根据 capability 返回结果放行或拒绝请求

插件不负责：

- Redis 连接池
- Redis TLS/Sentinel/Cluster 细节
- 算法 Lua 执行
- ingate-dataplane HTTP path 和请求体拼装细节

## 故障策略

限流 capability 返回错误时，插件按策略的 `failurePolicy` 处理：

- `FailOpen`：放行请求
- `FailClose`：返回超限响应

`ingate-dataplane` 自身需要记录错误日志，插件也记录调用失败原因。用户输入错误不进入数据面服务。

## 后续扩展

后续 capability 按独立协议扩展：

- `event`: 异步投递 Kafka/OTLP/审计事件，插件只提交事件，`ingate-dataplane` 批量、重试、压缩和背压
- `secret`: Secret 解析和本地缓存
- `cache`: Redis 或其它缓存读写
- `quota`: AI token quota、租户配额和用量扣减

这些能力共享 `ingate-dataplane` 进程、观测、健康检查和部署生命周期，但协议互相独立，避免形成万能代理。
