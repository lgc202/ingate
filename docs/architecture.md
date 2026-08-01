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
- Redis 保存限流及 Token 配额等请求路径共享状态

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
- TokenQuotaPolicy

资源之间使用不可变 ID 引用。Admin API 创建资源时生成 UUID 并映射为底层 `metadata.name`；用户可编辑名称使用 `spec.displayName`。

`RateLimitPolicy`、`AccessControlPolicy` 和 `TokenQuotaPolicy` 通过 `spec.targetRefs[]` 直接引用 Gateway 或 Route，不再使用独立策略绑定资源。`targetRefs[]` 可以为空，表示策略已保存但当前不应用到流量。

每个资源遵循标准的 `spec/status` 分离：

- `spec` 是用户声明的期望状态，也是唯一业务事实来源
- `status.conditions` 是 Controller 可重新计算的观察结果，只能通过 status 子资源更新
- `Accepted` 表示当前 generation 的资源配置是否被接受
- `ResolvedRefs` 表示 Gateway、Route 和 Policy 的引用是否有效
- `Programmed` 表示当前 UID 与 generation 已进入 Active 配置
- `observedGeneration` 小于 `metadata.generation` 时，调用方必须将状态视为处理中

Policy 除总体 `status.conditions` 外，还通过 `status.targets[]` 记录每个 `targetRef` 的解析和生效结果。缺失目标只产生 Warning，有效目标继续进入配置；任一目标已生效时总体 `Programmed=True`，控制台结合目标状态展示部分生效；启用但 `targetRefs[]` 为空，或所有目标都没有实际展开到流量入口时，使用 `Programmed=False` 和 `NotApplied` 表达未应用。Admin API 删除 Gateway 或 Route 时会拒绝删除仍被 Policy 引用的目标，声明式 API 仍允许删除并由 `ResolvedRefs=False` 反馈。

Admin API 只把 Condition 转换成面向页面的状态摘要，不向控制台泄漏 Kubernetes 资源结构、Envoy、xDS、ACK 或 NACK 等实现细节。

## AI Gateway

AI Gateway 直接复用现有控制面和 Envoy 数据面，不新增 AI runtime、协议转换服务或其他独立组件：

```text
OpenAI Client
  -> Envoy Listener / Route
  -> 内置 tokenquota Wasm
  -> 内置 ai-proxy Wasm
  -> 按请求体 model 选择模型 Upstream
  -> OpenAI / DeepSeek / 通义 / Anthropic / Gemini / 自定义兼容服务
```

资源职责保持单一：

- 模型服务仍是 `Upstream`，使用 `type=model` 表达业务分类，使用 `protocol` 表达 OpenAI、Anthropic 或 Gemini 通信语义，使用 `spec.model.provider` 表达具体厂商
- 用户在 `spec.model.models[]` 中手工维护厂商模型目录，不新增 Provider 或 Model 资源，也不自动同步厂商模型列表
- API Key 直接保存在模型 `Upstream.spec.authentication.apiKey.value` 中，不再创建独立凭据资源或跨资源引用
- Admin API 只返回 `apiKeyConfigured`，不回显密钥内容；更新时省略 API Key 表示保留，显式移除才会清除
- 配置或保留 API Key 时模型 Upstream 必须启用 TLS，避免密钥通过明文 HTTP 发送
- 一条模型 RouteRule 的 `modelRouting.models[]` 中，每个客户端模型别名分别保存 `upstreamRef` 和 `upstreamModel`；同一路由可以按 `model` 跨多个模型 Upstream 选择目标
- Compiler 为模型规则生成公开入口 Route 和只接受内部 Header 的续接 Route；续接 Route 使用标准 Envoy `cluster_header` 选择目标 Cluster，并按目标写入上游 Host
- Compiler 把目标 Cluster、协议、基础路径、认证执行计划和模型映射编译为 Listener 级 `ai-proxy` 私有配置；内部选路 Header 在进入插件前清除客户端值，并在发往模型服务前移除，xDS 配置乱序时保持 fail-closed

声明式资源示例：

```yaml
pathPrefix: /v1/chat/completions
methods:
  - POST
modelRouting:
  models:
    - model: chat-default
      upstreamRef: openai-upstream-id
      upstreamModel: gpt-4o-mini
    - model: claude-sonnet
      upstreamRef: anthropic-upstream-id
      upstreamModel: claude-sonnet-4
```

Admin API 和控制台在每条模型映射中使用 `upstreamID` 表达模型服务引用，不把内部资源引用命名泄漏给页面协议。

数据面只对 `POST /v1/chat/completions` 缓冲并解析请求体。`ai-proxy` 根据公开模型别名选择目标，转换厂商请求体和路径，写入受控的规则版本 Header、Cluster Header 与认证 Header。Proxy-Wasm 修改请求路径和 Header 时，Envoy 会按标准语义清除 Route cache；随后只有携带这两个内部 Header 的续接 Route 能命中，续接 Route 选择目标 Cluster、写入上游 Host，并移除内部 Header。这样不依赖 Higress 的产品路由模型，也不会让修改后的 Anthropic 或 Gemini 路径丢失 Route。客户端提供的内部选路和上游凭据一律不可信并在插件入口清除。

OpenAI-compatible、Anthropic 和 Gemini 上游都只接受当前公开的文本消息字段；Anthropic 和 Gemini 进一步完成厂商协议转换。对外统一返回 OpenAI-compatible 错误、finish reason、Token usage 和公开模型名称。跨 chunk SSE 使用有状态解析，不受 `bufio.Scanner` 64 KiB 限制，但单个未完成事件有显式的 1 MiB 安全上限。

模型 Upstream 可通过 `tls.serverName` 使用 HTTPS。Compiler 配置 SNI、数据面镜像的系统 CA 根证书包、精确 DNS/IP SAN 校验和 HTTP/1.1 ALPN。

纯协议转换放在顶层 `pkg/llm`。该包不依赖 Ingate 资源、Envoy、Proxy-Wasm、Gin 或 Kubernetes，不发送模型 HTTP 请求、不读取环境变量，也不管理密钥持久化；数据面适配层只负责把编译配置和 Proxy-Wasm hostcall 接到纯转换函数。

当前限制：

- 单次请求体上限为 1 MiB，不面向大文件或大体积多模态输入
- 只支持文本 `system`、`user`、`assistant` 消息和 `model/messages/stream/temperature/top_p/max_tokens/stop`
- 不支持 Responses、Embeddings 等其他 OpenAI API
- 不支持 Tools/function calling、图片、音频、文件等多模态内容
- 不支持多 Provider fallback 和模型路由重试
- 不支持 OAuth、IAM、Azure Entra、AWS SigV4 等云认证
- 不包含独立 AI runtime

standalone 默认提供 HTTP `8080` 和 HTTPS `8443` 两个固定数据面入口。相同协议和端口的逻辑 Gateway 会合并为一个 Envoy Listener；HTTP 通过 Host 分流，HTTPS 通过 SNI filter chain 选择 Gateway 引用的 Certificate。证书 PEM 当前随 LDS 内联下发，后续只有在需要独立密钥轮转时才引入 SDS。

RateLimitPolicy 统一使用系统 Redis，用户协议不包含 Local/Global 模式、限流算法、RedisStore、redisRef 或私有插件 JSON。数据面当前使用系统选定的令牌桶实现，`burst` 为 0 时使用 `requests` 作为桶容量，正数表示显式桶容量。

TokenQuotaPolicy 为一个策略定义一个 Token 预算池，仅展开到目标 Gateway 或 Route 下的模型 RouteRule。预算池可以由所有命中请求共享，也可以按网关看到的来源 IP 或指定请求 Header 值区分；Header 和 IP 原始值经过哈希后才进入 Redis key。多个 targetRef 命中同一策略时仍共享同一预算池，需要独立预算时应创建多条策略。

数据面在请求进入模型服务前检查当前固定窗口的已用额度，在 AI Proxy 完成普通响应或 SSE 归一化后读取最后一个 `usage.total_tokens` 并记账。OpenAI-compatible 流式上游在策略生效时由 AI Proxy 内部请求最终 usage，客户端协议不开放 `stream_options`。当前不做字符估算和模型 tokenizer 预扣；并发中的在途请求可能造成有限超额，因此这是 best-effort 的后付费软额度，不是严格硬额度，也不能替代计费系统。

Token 配额的主体划分和流式记账有以下运行约束：

- 流式连接在完成标记到达前中断时无法记账，实际用量可能少计
- Header 维度必须使用可信认证层写入且客户端无法伪造的 Header；缺失该 Header 的请求会共用同一个未标识预算池，允许客户端自由修改或轮换值还会绕过单主体额度并产生高基数 Redis key
- IP 维度使用 Envoy 看到的连接源地址；前置负载均衡或反向代理未保留真实源地址时，多个客户端可能按代理地址共用预算池
- 自定义 OpenAI-compatible 上游只有支持 `stream_options.include_usage` 并在最终 SSE 事件返回 usage 时，才能用于受 TokenQuotaPolicy 保护的流式请求

## 内置治理插件

限流、访问控制和 Token 配额以强类型 Policy 对外提供。Compiler 解析每个 Policy 的 `targetRefs[]`，展开成按 Gateway、Route 和必要 RouteRule 索引的插件执行配置，并在 Listener/HCM 中注入一次内置 Wasm filter。

内置插件：

- 使用标准 Proxy-Wasm SDK
- 通过 Ingate 自己维护的最小 Redis ABI adapter 调用 Higress Envoy 扩展
- 不 import Higress Go package
- 默认安装在 `/opt/ingate/plugins`
- 不向用户暴露 Wasm 路径、版本、phase、priority 或私有 JSON

Envoy bootstrap 中固定存在 `ingate-system-redis`。Redis 是安装级系统组件，不是声明式资源。

## 容器目录

Ingate 镜像使用 `/opt/ingate` 保存随组件发布的文件：

- 各组件使用 `/opt/ingate/<component>/bin` 和 `/opt/ingate/<component>/configs`
- API Server 自身运行证书放在 `/opt/ingate/apiserver/certificates`
- 内置插件放在 `/opt/ingate/plugins`

API Server 自身运行证书是组件运行文件。用户为 Gateway 配置的 Certificate 是声明式资源，其 PEM 内容由 API Server 持久化到 etcd，再由 Controller 编译并下发给 Envoy，不写入 `/opt/ingate/apiserver/certificates`。

这些路径只约束当前容器镜像。源码开发仍使用仓库中的 `configs/` 和 `_output/`；运行数据位置由具体部署方式决定。

## 部署边界

Ingate 不把部署方式写入业务代码。每个 Go 服务都通过 YAML、环境变量和启动参数配置，提供健康检查并响应进程退出信号；日志可以写入标准输出或文件。systemd、Docker Compose 和 Kubernetes 可以使用同一套二进制和领域协议。

当前仓库只落地 Docker Compose 开发联调环境，不把它描述为生产拓扑，也不提供把所有进程放进同一容器的镜像。未来出现明确交付需求时，再分别设计 systemd Unit 或 Kubernetes Workload。

## Docker Compose 开发环境

Compose 使用独立容器运行：

- etcd
- Redis
- ingate-apiserver
- ingate-controller
- ingate-admin-api 和 Console
- Envoy

Controller 与 Envoy 是独立容器，但共享网络命名空间。当前 xDS 没有启用 mTLS，因此继续只监听 `127.0.0.1:18000`；不能为了容器互联把敏感 xDS 直接开放到 Bridge Network。

Compose 直接使用固定版本的 `lgc202/ingate-envoy:v0.1.0`。该镜像从 Higress Gateway `v2.2.3` 提取带 Redis ABI 的 Envoy 二进制，并加入 Ingate 内置 Wasm 插件和 Bootstrap。Redis 在 Compose 内不暴露宿主机端口，使用 AOF 保存开发联调期间的限流和 Token 配额状态。

Compose 只发布 Console `8001`、Gateway HTTP `8080` 和 Gateway HTTPS `8443`。etcd、Redis、API Server、xDS 和 Envoy Admin 都保持内部可见。
