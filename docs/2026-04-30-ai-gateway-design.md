# Ingate AI Gateway 设计

本文档描述 Ingate 向 AI Gateway 演进的设计方向。目标不是复刻 Higress，而是在现有声明式控制面、RuntimeSnapshot 和 Envoy xDS 链路基础上，形成一套面向产品的 Plugin-first AI Gateway 模型。

## 背景

当前 Ingate 已经打通第一条控制面链路：

```text
Gateway / Route / Upstream
        |
        v
ingate-apiserver
        |
        v
ingate-controller
        |
        v
RuntimeSnapshot
        |
        v
ingate-xds
        |
        v
Envoy
```

下一步要支持 AI Gateway，但不能把 AI 能力简单塞进普通 Route 或插件配置里。AI Gateway 有自己的领域语义，例如模型供应商、模型路由、流式响应、Token 用量、Fallback、内容安全和工具调用。它们需要成为可声明、可观察、可审计的控制面资源。

## 设计目标

- 支持 OpenAI-compatible API 作为第一阶段入口
- 支持多 AI Provider 和多模型配置
- 支持 streaming 请求和响应
- 支持 Provider 路由、Fallback、重试和用量统计
- 保持 Gateway / Route / Upstream 作为通用 API Gateway 能力
- 将 AI 核心语义资源化，将 AI 执行能力插件化
- 支持 Wasm、External Processor、Go Runtime 等多种 AI 执行目标
- 产品形态优先表现为网关插件能力，而不是网关后面挂一个固定 AI 中间层
- 保持 K8s、VM、裸机部署都能使用同一套内部模型

## 非目标

第一阶段不做以下能力：

- 不实现完整 Agent Gateway
- 不实现完整 MCP Server 托管
- 不实现 RAG 编排
- 不实现语义缓存
- 不实现复杂计费系统
- 不实现所有模型供应商
- 不实现完整 Wasm 插件生态

这些能力后续可以基于核心链路继续扩展。

## 核心判断

AI Gateway 能力分为两类。

第一类是核心链路能力：

```text
AIRoute -> AIProvider -> AIModel -> AIPolicy -> AI Execution Target -> LLM Provider
```

这些能力决定请求如何进入模型服务，必须是一等资源。它们需要被 Admin API 展示、被 controller reconcile、被 RuntimeSnapshot 分发，并且能够被审计和回滚。

第二类是增强能力：

```text
Prompt 模板
Token 限流
内容安全
PII 脱敏
语义缓存
审计打点
AI 统计
RAG
工具调用治理
```

这些能力变化快、组合方式多，不同用户需求差异大，适合通过插件扩展。

因此 Ingate 的核心判断是：

```text
控制面资源表达产品语义
插件和执行目标承载运行时能力
```

## 总体架构

```text
Client
  |
  v
Envoy
  |
  | xDS route / cluster / filter / plugin config
  |
  +-- Wasm AI Proxy
  |     |
  |     v
  |   LLM Provider
  |
  +-- External AI Processor
  |     |
  |     v
  |   LLM Provider
  |
  +-- Go AI Runtime
        |
        v
      LLM Provider
```

控制面链路：

```text
AIProvider / AIModel / AIRoute / AIPolicy / PluginBinding
        |
        v
ingate-apiserver
        |
        v
ingate-controller
        |
        v
RuntimeSnapshot
        |
        v
ingate-xds
        |
        v
Envoy
```

用户在产品侧看到的是 `AIProvider`、`AIModel`、`AIRoute`、`AIPolicy` 和插件绑定。底层可以根据能力复杂度选择不同执行目标：

- `Wasm AI Proxy`：产品主形态，适合轻量协议转换、Provider 调用、统计、限流等网关内能力
- `External AI Processor`：适合需要进程隔离、调用外部服务、较复杂策略处理的能力
- `Go AI Runtime`：适合 Provider 适配复杂、需要完整 Go 生态、MCP / Agent / Tool 编排等重逻辑

第一阶段不要求一次实现所有执行目标，但控制面和 RuntimeSnapshot 需要按多执行目标设计，避免把产品模型绑定死到某一种运行方式。

## 组件边界

### ingate-apiserver

负责声明式 AI 资源的 CRUD、watch、status 和校验。

第一阶段新增资源建议：

- `AIProvider`
- `AIModel`
- `AIRoute`
- `AIPolicy`

插件相关资源先保留设计，不急于实现完整运行时：

- `Plugin`
- `PluginBinding`

### ingate-controller

负责监听 AI 资源变化，并将它们编译为 RuntimeSnapshot。

controller 不直接调用模型供应商，只做期望状态到运行时配置的转换。它需要根据 `AIPolicy` 或后续 `AIExecutionPolicy` 决定某个 AIRoute 使用哪种执行目标。

### ingate-xds

负责把 RuntimeSnapshot 中的 AI 路由转换为 Envoy 配置。

它不只生成普通 route，还需要生成对应的 filter / plugin 配置：

- Wasm 插件配置
- ext_proc 配置
- 指向 Go AI Runtime 的 cluster 配置

### AI Execution Target

AI Execution Target 表示 AI 请求在数据路径上的实际执行方式。

建议先定义三类：

```text
Wasm
ExternalProcessor
GoRuntime
```

`Wasm` 是更像 Higress 的产品形态。Envoy 在请求链路中直接加载 AI Proxy 插件，由插件完成 Provider 调用、协议转换、流式处理和基础统计。

`ExternalProcessor` 通过 Envoy ext_proc 将请求处理委托给外部 gRPC 服务，适合内容安全、审计、复杂策略和需要隔离的能力。

`GoRuntime` 是一个可选运行时服务，适合承载更重的 AI 编排能力。它不是数据面本体，也不应该成为唯一产品形态。

Go Runtime 职责可以包括：

- 提供 OpenAI-compatible API
- 根据 RuntimeSnapshot 或 apiserver watch 获取 AI 配置
- 选择 AIProvider / AIModel
- 转发 streaming 请求和响应
- 处理 Provider 鉴权
- 处理 Fallback 和重试
- 采集 Token usage
- 输出调用事件和审计事件

### ingate-ai-runtime

`ingate-ai-runtime` 作为 `GoRuntime` 执行目标存在，而不是 AI Gateway 唯一主链路。

它可以先用于快速验证 Provider 适配、streaming、fallback、MCP / Tool 等复杂能力；当能力稳定、逻辑足够轻，再下沉到 Wasm 或 External Processor。

## 为什么采用 Plugin-first 形态

从产品视角看，AI Gateway 更自然的形态是：

```text
Envoy 数据面
  + AI Proxy 插件
  + AI 缓存插件
  + AI 限流插件
  + AI 安全插件
  + AI 统计插件
```

用户更容易理解为“在某个 Gateway / AIRoute 上启用 AI 能力”，而不是“把流量转发到另一个 AI 中间层”。

但 Plugin-first 不等于把产品模型退化成一坨插件 JSON。反例是：

```yaml
pluginConfig:
  provider:
    type: openai
    apiKey: xxx
    modelMapping:
      "*": qwen-turbo
```

这种方式短期能跑，但长期会让 `AIProvider`、`AIRoute`、`AIPolicy` 这些平台资源缺位。Admin API、状态展示、权限、审计、灰度和回滚都会更难做。

因此 Ingate 的策略是：

```text
AI 核心语义保留为控制面资源
AI 执行能力优先编译为插件配置
复杂能力允许落到 External Processor 或 Go Runtime
```

## Higress 借鉴点

Higress 的实现证明了 AI Gateway 可以大量插件化。它的 `ai-proxy`、`ai-cache`、`ai-token-ratelimit`、`ai-security-guard`、`ai-statistics`、`mcp-server` 等插件都很有参考价值。

Ingate 借鉴以下设计：

- 插件资源和插件绑定分开
- 插件需要 phase、priority、failure policy
- AI Proxy、缓存、统计、安全、限流适合做插件
- MCP / Tool 能力可以后续作为独立 AI 扩展层

Ingate 不直接照搬以下设计：

- 不依赖 Istio 作为核心控制面
- 不把产品核心语义藏进插件 JSON 配置
- 不要求所有 AI 能力只能用 Wasm 实现

## 资源模型

### AIProvider

表示一个模型供应商或模型服务入口。

示例：

```yaml
apiVersion: gateway.ingate.io/v1
kind: AIProvider
metadata:
  name: openai-main
spec:
  type: OpenAI
  endpoint: https://api.openai.com
  credentialRef:
    name: openai-key
  timeout: 60s
```

第一阶段字段建议：

- `type`
- `endpoint`
- `credentialRef`
- `timeout`
- `headers`

### AIModel

表示一个对外可路由的模型。

示例：

```yaml
apiVersion: gateway.ingate.io/v1
kind: AIModel
metadata:
  name: gpt-4o-mini
spec:
  providerRef:
    name: openai-main
  providerModel: gpt-4o-mini
```

第一阶段字段建议：

- `providerRef`
- `providerModel`
- `capabilities`

### AIRoute

表示 AI API 的入口路由。

示例：

```yaml
apiVersion: gateway.ingate.io/v1
kind: AIRoute
metadata:
  name: chat-completions
spec:
  parentRefs:
    - name: public
  path: /v1/chat/completions
  models:
    - name: gpt-4o-mini
      weight: 100
  policyRefs:
    - name: default-ai-policy
```

第一阶段字段建议：

- `parentRefs`
- `hostnames`
- `path`
- `models`
- `policyRefs`

### AIPolicy

表示 AI 请求策略。

示例：

```yaml
apiVersion: gateway.ingate.io/v1
kind: AIPolicy
metadata:
  name: default-ai-policy
spec:
  timeout: 60s
  retry:
    attempts: 1
  fallback:
    enabled: true
    models:
      - qwen-plus
  usage:
    enabled: true
```

第一阶段字段建议：

- `timeout`
- `retry`
- `fallback`
- `usage`

Quota、内容安全、缓存、RAG 暂时不放进 AIPolicy 第一阶段主线，后续通过插件或专门策略资源补充。

## 插件模型

插件模型是 AI Gateway 产品形态的核心，不只是后续增强点。

建议资源：

```text
Plugin
PluginBinding
```

`Plugin` 描述插件本体：

- 插件名
- 运行时类型：`Builtin`、`Wasm`、`External`
- 版本
- 配置 schema
- 默认 failure policy
- 支持的执行阶段
- 支持的目标资源类型

`PluginBinding` 描述插件挂载位置：

- 目标资源：`Gateway`、`Route`、`AIRoute`、`AIProvider`、`Consumer`
- 执行阶段
- 优先级
- 插件配置
- 失败策略

建议插件阶段：

```text
RequestHeaders
RequestBody
BeforeAIRoute
BeforeProviderCall
ResponseHeaders
ResponseBody
StreamChunk
Usage
Error
```

插件失败策略：

```text
FailClose
FailOpen
SkipAndLog
```

AI Proxy 本身也可以被建模为一种插件，但它的配置不应该直接由用户手写大块 JSON。用户声明 `AIProvider / AIModel / AIRoute / AIPolicy`，controller 将这些资源编译成 AI Proxy 插件配置。

## RuntimeSnapshot 扩展

当前 RuntimeSnapshot 已经保存 target-specific config。AI Gateway 需要在 snapshot 中增加 AI 路由、插件绑定和执行目标配置。

建议逻辑结构：

```text
RuntimeSnapshot
  gateways
  routes
  upstreams
  aiRoutes
  aiProviders
  aiModels
  aiPolicies
  pluginBindings
  aiExecutionTargets
```

`aiExecutionTargets` 表示某组 AI 配置最终使用哪种运行方式：

```text
Wasm
ExternalProcessor
GoRuntime
```

xDS target 根据执行目标生成不同 Envoy 配置：

- `Wasm`：生成 route、cluster、wasm filter 和 plugin config
- `ExternalProcessor`：生成 route、cluster、ext_proc filter 和 processor config
- `GoRuntime`：生成 route 和指向 `ingate-ai-runtime` 的 cluster

## 第一阶段开发范围

第一阶段目标不是做完所有插件运行时，而是把 Plugin-first AI Gateway 的主干打通：

```text
AIProvider / AIModel / AIRoute / AIPolicy
        |
        v
apiserver storage
        |
        v
generated client / informer / lister
        |
        v
controller reconcile
        |
        v
RuntimeSnapshot
        |
        v
xDS route + AI execution target config
        |
        v
Envoy AI execution target -> OpenAI-compatible provider
```

第一阶段支持能力：

- OpenAI-compatible `/v1/chat/completions`
- 非流式响应
- streaming 响应
- 单 Provider
- 单模型或简单权重模型选择
- 基础 retry
- 基础 fallback
- usage 事件输出
- 最小 `Plugin / PluginBinding` 模型
- 最小 `AIExecutionTarget` 模型

第一阶段不做：

- 完整插件市场
- 多协议 Provider 全量适配
- MCP
- RAG
- 语义缓存
- 复杂计费
- Admin UI

第一阶段的执行目标可以按工程风险选择：

- 产品默认形态按 `Wasm` 设计
- 如果 Wasm 实现成本影响主链路，可以先用 `GoRuntime` 或 `ExternalProcessor` 打通同一套资源模型
- 无论先实现哪种，控制面 API 和 RuntimeSnapshot 都不能绑定死到单一执行方式

## 后续阶段

第二阶段：

- 完善 Plugin / PluginBinding
- Wasm AI Proxy 插件
- Builtin 插件执行点
- AI Token 统计插件
- Token Quota 插件
- 请求审计插件

第三阶段：

- External 插件接入 ext_proc 或独立 gRPC
- 内容安全插件
- Prompt 模板插件
- PII 脱敏插件

第四阶段：

- MCP Gateway
- Tool 调用治理
- RAG 插件
- Agent Gateway
- 多 target 支持

## 关键取舍

Ingate 的 AI Gateway 不把所有能力都做进核心，也不把产品语义全部交给插件 JSON。

核心资源负责表达稳定语义：

```text
AIProvider / AIModel / AIRoute / AIPolicy
```

插件负责表达产品运行形态：

```text
AI Proxy / 缓存 / 限流 / 安全 / 审计 / RAG / Prompt 模板
```

执行目标负责表达能力落点：

```text
Wasm / ExternalProcessor / GoRuntime
```

Wasm 是更好的产品主形态，Go Runtime 是复杂能力和快速验证的补充，External Processor 是隔离型扩展。这个边界能让 Ingate 更接近产品级 AI Gateway，同时避免后续核心能力被某一种插件实现绑死。
