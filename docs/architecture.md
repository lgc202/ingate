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
Console --------> ingate-admin-api
                         |
CLI / SDK --------------+----> ingate-apiserver ----> etcd
                                  ^          |
                                  | status   | watch spec
                                  |          v
                             ingate-controller
                                  |
                                  | Envoy Config Compiler
                                  | Config Delivery
                                  | SotW ADS xDS
                                  v
                                Envoy ----------------> Redis
```

- `ingate-admin-api` 提供面向控制台用例的产品 DTO
- `ingate-apiserver` 提供声明式资源 API，是 etcd 的唯一访问者
- `ingate-controller` 监听完整资源集合，编译并发布一份 Envoy 配置，并通过 status 子资源回写观察结果
- Envoy 执行路由、代理和内置治理插件
- Redis 保存限流及未来 Token 配额等请求路径共享状态

Admin API 只访问 API Server，不直接访问 Controller。Controller 的内部 HTTP 服务只提供健康检查，不作为产品状态查询协议。

当前不包含 AI runtime、data-plane agent 和 Kubernetes operator。

## 配置链路

```text
Resource
  -> Envoy Config Compiler
  -> Config Delivery
  -> xDS Snapshot Cache
  -> Envoy
```

Controller 使用唯一全局队列 key。Gateway、Certificate、Route、Upstream 和强类型 Policy 的任意 spec 变化都会触发一次完整配置域编译。

Compiler 直接生成 Envoy protobuf，不输出公开 IR，不存在 Target、Translator、RuntimeGroup 或 RuntimeSnapshot。IP Upstream 生成 EDS，包含 hostname 的 Upstream 生成带内联端点的 `STRICT_DNS` cluster。

任意 Error diagnostic 都会阻止新配置发布，但不会修改当前进程内 Active。Warning 不阻止发布。

## Delivery

Delivery 是 Snapshot Cache 的唯一写入者，并在单 goroutine 中维护：

- Candidate：已发布、等待 Envoy 响应或 ACK 的配置
- Active：至少被一个 Envoy 实例完整 ACK 的配置
- Baseline：首次 Candidate 被 NACK 且没有 Active 时使用的空配置

Candidate 可以被更新版本替换。旧版本迟到的 ACK、NACK 和 timeout 不得改变当前配置。

最新资源无法编译时，Controller 会保留 Active，但取消仍在飞行的 Candidate 并恢复 Active 或 Baseline。Candidate 等待 ACK 超时后，对应资源会标记为发布失败，但 Candidate 仍可接收迟到的完整 ACK 并提升为 Active。

NACK 时：

- 有 Active：同步恢复 Active
- 无 Active：同步安装 Baseline
- 配置已成为 Active 后，其他实例的后置 NACK 不回滚整个实例组，也不改变资源 `Programmed` 状态；实例连接状态属于监控能力

Candidate、Active 和 Baseline 只存在于 Controller 进程内。声明式资源是唯一持久化事实；Controller 重启后重新全量编译，不持久化 Last Good，也不创建特殊 apiserver 存储接口。

Candidate 和 Active 会在进程内携带参与编译的资源 UID 与 generation，以及实际展开进配置的 Policy/Target 身份，用于在配置确认后更新对应资源的 `Programmed` Condition。这些来源信息不参与 xDS version 计算，也不持久化。

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
- Certificate
- Route
- Upstream
- RateLimitPolicy
- AccessControlPolicy

资源之间使用不可变 ID 引用。Admin API 创建资源时生成 UUID 并映射为底层 `metadata.name`；用户可编辑名称使用 `spec.displayName`。

`RateLimitPolicy` 和 `AccessControlPolicy` 通过 `spec.targetRefs[]` 直接引用 Gateway 或 Route，不再使用独立策略绑定资源。`targetRefs[]` 可以为空，表示策略已保存但当前不应用到流量。

每个资源遵循标准的 `spec/status` 分离：

- `spec` 是用户声明的期望状态，也是唯一业务事实来源
- `status.conditions` 是 Controller 可重新计算的观察结果，只能通过 status 子资源更新
- `Accepted` 表示当前 generation 的资源配置是否被接受
- `ResolvedRefs` 表示 Gateway、Route 和 Policy 的引用是否有效
- `Programmed` 表示当前 UID 与 generation 已进入 Active 配置
- `observedGeneration` 小于 `metadata.generation` 时，调用方必须将状态视为处理中

Policy 除总体 `status.conditions` 外，还通过 `status.targets[]` 记录每个 `targetRef` 的解析和生效结果。缺失目标只产生 Warning，有效目标继续进入配置；任一目标已生效时总体 `Programmed=True`，控制台结合目标状态展示部分生效；启用但 `targetRefs[]` 为空，或所有目标都没有实际展开到流量入口时，使用 `Programmed=False` 和 `NotApplied` 表达未应用。Admin API 删除 Gateway 或 Route 时会拒绝删除仍被 Policy 引用的目标，声明式 API 仍允许删除并由 `ResolvedRefs=False` 反馈。

Admin API 只把 Condition 转换成面向页面的状态摘要，不向控制台泄漏 Kubernetes 资源结构、Envoy、xDS、ACK 或 NACK 等实现细节。

standalone 默认提供 HTTP `8080` 和 HTTPS `8443` 两个固定数据面入口。相同协议和端口的逻辑 Gateway 会合并为一个 Envoy Listener；HTTP 通过 Host 分流，HTTPS 通过 SNI filter chain 选择 Gateway 引用的 Certificate。证书 PEM 当前随 LDS 内联下发，后续只有在需要独立密钥轮转时才引入 SDS。

RateLimitPolicy 统一使用系统 Redis，用户协议不包含 Local/Global 模式、限流算法、RedisStore、redisRef 或私有插件 JSON。数据面当前使用系统选定的令牌桶实现，`burst` 为 0 时使用 `requests` 作为桶容量，正数表示显式桶容量。

## 内置治理插件

限流和访问控制以强类型 Policy 对外提供。Compiler 解析每个 Policy 的 `targetRefs[]`，展开成按 Gateway 和 Route 索引的插件执行配置，并在 Listener/HCM 中注入一次内置 Wasm filter。

内置插件：

- 使用标准 Proxy-Wasm SDK
- 通过 Ingate 自己维护的最小 Redis ABI adapter 调用 Higress Envoy 扩展
- 不 import Higress Go package
- 默认安装在 `/opt/ingate/plugins`
- 不向用户暴露 Wasm 路径、版本、phase、priority 或私有 JSON

Envoy bootstrap 中固定存在 `ingate-system-redis`。Redis 是安装级系统组件，不是声明式资源。

## 运行目录

安装包和容器内使用统一目录语义，不因 systemd、Docker 或 Kubernetes 等部署方式改变：

- `/opt/ingate` 保存组件二进制、配置、静态资源、脚本和其他随组件发布的文件
- `/data/ingate` 保存日志、etcd、Redis、外部插件和备份等运行产生或需要持久化的数据
- 各组件使用 `/opt/ingate/<component>/bin` 和 `/opt/ingate/<component>/configs`
- API Server 自身运行证书放在 `/opt/ingate/apiserver/certificates`
- 内置插件放在 `/opt/ingate/plugins`，外部安装或动态缓存的插件放在 `/data/ingate/plugins`

API Server 自身运行证书是组件运行文件。用户为 Gateway 配置的 Certificate 是声明式资源，其 PEM 内容由 API Server 持久化到 etcd，再由 Controller 编译并下发给 Envoy，不写入 `/opt/ingate/apiserver/certificates`。

绝对路径只约束安装包和容器内布局。源码开发仍可使用仓库中的 `configs/`、`_output/` 和 `ingate-dev/` 等相对路径。

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
