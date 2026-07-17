# Ingate 架构

## 目标

Ingate 是面向 API 网关和 AI 网关的声明式 Envoy 控制面。

- Envoy 是唯一数据平面
- Higress 只提供带 Redis 扩展 ABI 的 Envoy 二进制
- 一套 Ingate 表示一个环境、一个配置域和一组配置完全相同的 Envoy 实例
- 一套 Ingate 可以包含多个逻辑 Gateway
- 应用、模型、MCP 和 Agent 统一建模为 Upstream

## 组件

```text
Console
   |
ingate-admin-api
   |
ingate-apiserver ------ etcd
   |
ingate-controller
   |  Resource Watch
   |  Envoy Config Compiler
   |  Config Delivery
   |  SotW ADS xDS
   |
Envoy ----------------- Redis
```

- `ingate-admin-api` 提供面向控制台用例的产品 DTO
- `ingate-apiserver` 提供声明式资源 API，是 etcd 的唯一访问者
- `ingate-controller` 监听完整资源集合，编译并发布一份 Envoy 配置
- Envoy 执行路由、代理和内置治理插件
- Redis 保存限流及未来 Token 配额等请求路径共享状态

当前不包含 AI runtime、data-plane agent 和 Kubernetes operator。

## 配置链路

```text
Resource
  -> Envoy Config Compiler
  -> Config Delivery
  -> xDS Snapshot Cache
  -> Envoy
```

Controller 使用唯一全局队列 key。Gateway、Route、Upstream、Policy 和 PolicyBinding 的任意 spec 变化都会触发一次完整配置域编译。

Compiler 直接生成 LDS、RDS、CDS 和 EDS protobuf，不输出公开 IR，不存在 Target、Translator、RuntimeGroup 或 RuntimeSnapshot。

任意 Error diagnostic 都会阻止新配置发布，但不会修改当前进程内 Active。Warning 不阻止发布。

## Delivery

Delivery 是 Snapshot Cache 的唯一写入者，并在单 goroutine 中维护：

- Candidate：已发布、等待 Envoy 响应或 ACK 的配置
- Active：至少被一个 Envoy 实例完整 ACK 的配置
- Baseline：首次 Candidate 被 NACK 且没有 Active 时使用的空配置

Candidate 可以被更新版本替换。旧版本迟到的 ACK、NACK 和 timeout 不得改变当前状态。

NACK 时：

- 有 Active：同步恢复 Active
- 无 Active：同步安装 Baseline
- 配置已成为 Active 后，其他实例的后置 NACK 只进入 Degraded，不回滚整个实例组

Candidate、Active 和 Baseline 只存在于 Controller 进程内。声明式资源是唯一持久化事实；Controller 重启后重新全量编译，不持久化 Last Good，也不创建特殊 apiserver 存储接口。

Controller 启动时不会预先发布空 Baseline，避免重启期间覆盖仍在运行的 Envoy 配置。首次编译完成后才向 Snapshot Cache 发布。

## xDS

Controller 内嵌标准 go-control-plane State-of-the-World ADS：

- 所有 Envoy node 映射到固定 cache key `ingate`
- Node ID 仅用于连接唯一性和 ACK/NACK 观测
- xDS package 只上报 typed event，不依赖 Delivery
- Delta xDS 当前不实现

## 声明式资源

当前资源：

- Gateway
- Route
- Upstream
- RateLimitPolicy
- AccessControlPolicy
- PolicyBinding

资源之间使用不可变 ID 引用。Admin API 创建资源时生成 UUID 并映射为底层 `metadata.name`；用户可编辑名称使用 `spec.displayName`。

RateLimitPolicy 的 Global mode 自动使用系统 Redis，不包含 RedisStore、redisRef 或私有插件 JSON。

## 内置治理插件

限流和访问控制以强类型 Policy 与 PolicyBinding 对外提供。Compiler 把策略和绑定展开成插件执行配置，并在 Listener/HCM 中注入一次内置 Wasm filter。

内置插件：

- 使用标准 Proxy-Wasm SDK
- 通过 Ingate 自己维护的最小 Redis ABI adapter 调用 Higress Envoy 扩展
- 不 import Higress Go package
- 默认安装在 `/opt/ingate/plugins`
- 不向用户暴露 Wasm 路径、版本、phase、priority 或私有 JSON

Envoy bootstrap 中固定存在 `ingate-system-redis`。Redis 是安装级系统组件，不是声明式资源。

## all-in-one

all-in-one 只运行：

- etcd
- Redis
- ingate-apiserver
- ingate-controller
- ingate-admin-api
- Envoy
- Console

不包含独立 ingate-xds、ingate-dataplane 或示例 backend。

Envoy 二进制来自固定 digest 的 Higress gateway 镜像；Redis 来自固定 digest 的官方 bookworm 镜像。Redis 只绑定容器 loopback，重启丢失限流计数是允许的。

## 明确删除

- RuntimeGroup
- RuntimeSnapshot
- RedisStore
- Logical IR
- Target 和 Translator
- 独立 ingate-xds
- ingate-dataplane
- 插件到 dataplane 的 HTTP transport
- Last Good 持久化
- schema marker、自动迁移和存储 reset 协议
