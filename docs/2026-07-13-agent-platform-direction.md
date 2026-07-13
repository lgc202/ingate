# Ingate Agent 平台方向设计

本文档记录基于 Ingate 网关控制面演进 Agent 能力的方向性设计，供后续产品和架构讨论参考。

本文不是当前里程碑的实现规格。当前项目范围仍以 API 网关控制面、治理策略、RuntimeSnapshot 和 Envoy xDS target 为主，暂不提前实现完整 AI Runtime。文中涉及的 Higress Envoy、hostcall、Agent Runtime 和新资源类型都需要在进入实现前单独评审。

## 背景

Ingate 当前已经形成声明式配置编译链路：

```text
Resource -> Compiler -> Logical IR -> Target Translator -> RuntimeSnapshot
```

这条链路不仅适合传统 API 网关，也可以成为 Agent 的安全执行底座。Agent 不需要直接修改 Envoy、Redis 或内部 store，而是通过结构化资源和控制面 API 表达用户意图，再由现有编译、发布和状态收敛链路完成执行。

本次方向讨论参考了以下文章：

- [18、开源项目：云原生 API 网关 Higress](https://www.sharexiyue.cn/posts/e0325759/)
- [19、轻量级网关插件 Wasm 初体验](https://www.sharexiyue.cn/posts/2989f61c/)
- [20、Wasm 编程基础（上）](https://www.sharexiyue.cn/posts/faa223ba/)
- [21、Wasm 编程基础（下）](https://www.sharexiyue.cn/posts/7e7ea7d7/)
- [22、实践：用 Wasm 实现 AI Proxy](https://www.sharexiyue.cn/posts/742d9f5d/)
- [23、实践：用 Wasm 实现 API Agent](https://www.sharexiyue.cn/posts/1bea3d4d/)
- [24、实践：通过 Wasm API Agent 操控与运维 K8s](https://www.sharexiyue.cn/posts/8812a391/)
- [26、展望：云原生网关如何进化为 AI 网关](https://www.sharexiyue.cn/posts/11a29e35/)

## 文章带来的核心启发

### API 是 Agent 的工具边界

文章中的 API Agent 使用 OpenAPI 描述后端服务，让模型根据自然语言选择并调用 API。这说明网关天然掌握的 API、身份、路由和策略信息，可以进一步组织成 Agent Tool。

Ingate 可以把 OpenAPI、MCP 和内部服务定义统一纳入 Tool Registry，但不应把原始 OpenAPI 文档直接复制到每个 Wasm 插件配置中。工具定义、凭据、权限和运行时配置需要分别治理。

### AI Proxy 是 Agent 平台的基础能力

不同模型厂商在协议、认证、模型名称、流式响应和错误格式上存在差异。文章中的 AI Proxy 通过 provider adapter 把调用方协议统一为 OpenAI 风格。

Ingate 后续可以考虑 `ModelProvider` 这类强类型抽象，由 target translator 生成具体数据面的可执行配置。模型供应商差异属于 target/runtime adapter，不应进入 Gateway、Route 等核心资源模型。

### Wasm 适合治理，不适合承载完整 Agent 生命周期

文章证明了在 Wasm 中实现 ReAct 循环、模型调用和工具调用在技术上可行，但这种方式更适合作为原型，而不是 Ingate 的长期企业级 Agent Runtime。

完整 Agent 循环放在 Envoy Wasm 请求路径中会带来以下问题：

- 一次 Agent 请求可能暂停 HTTP stream 数十秒甚至更久
- 请求体、响应体、上下文和工具结果会占用 Envoy worker 内存
- 多轮 callback、取消、重试、超时和部分失败难以统一管理
- SSE 流式输出与多轮工具调用的状态机复杂
- 会话状态跟随 Envoy Pod 和 Wasm VM 生命周期消失
- 大型 OpenAPI 文档、模型 Key 和工具凭据不适合放入插件配置
- 长任务、人工审批、可恢复工作流不适合绑定 HTTP filter 生命周期

因此，Wasm 更适合承担：

- AI 协议转换
- 身份认证和租户识别
- RateLimit、AccessControl 和 TokenQuota
- 内容安全和敏感数据处理
- Token 统计、审计信息采集
- Agent Tool 出口鉴权和流量治理

独立 Agent Runtime 更适合承担：

- 多轮推理和 Tool 选择
- 会话、记忆和任务状态
- 超时、重试、取消和恢复
- JSON Schema 校验和结构化输出
- 人工审批和长时间工作流

### 网关需要同时治理 Agent 入口和 Tool 出口

普通 AI Gateway 通常只治理用户到模型的入口流量。Agent 场景还需要治理 Agent 到 Tool 的出口流量。

```text
用户
  -> Ingate Gateway
      -> Agent Runtime
          -> Ingate Tool Gateway
              -> OpenAPI / MCP / K8s / 企业内部服务
```

这样所有 Tool 调用都可以统一执行鉴权、限流、审计、脱敏、网络边界控制和风险阻断，这是 Ingate 相比普通 Agent 框架的重要差异。

## 产品定位

长期产品可以分为两个相互关联但边界不同的方向。

### Ingate Assistant

Ingate Assistant 面向网关管理员、平台工程师和研发人员，用自然语言配置、诊断和治理 Ingate。

它是 Ingate 自己的内置 Agent，也是最适合作为第一阶段落地的 Agent。

### Ingate Agent Gateway

Ingate Agent Gateway 面向用户创建的业务 Agent，负责治理模型、MCP、A2A 和 Tool 调用流量。

它不是单个 Agent，而是一套让企业安全运行和暴露 Agent 的网关基础设施。

## 统一入口和专家能力

产品上不建议提供大量互相割裂的聊天机器人。更合适的是一个统一的 Ingate Assistant，根据任务调用不同的内置专家能力。

```text
Ingate Assistant
  ├── Gateway Ops
  ├── Incident Diagnosis
  ├── API Integration
  ├── ModelOps
  ├── Security Governance
  ├── AI FinOps
  └── Gateway Migration
```

这些专家共享模型调用、Tool Registry、权限、审批、审计和会话能力，通过工具白名单、系统指令、输出 Schema 和权限范围形成不同能力边界。

## Skills 作为 Agent 扩展单元

Ingate Assistant 很适合引入 Skills。与其为每个场景实现一个独立 Agent，更合理的方式是保持统一 Agent 入口，通过 Skills 封装可复用的网关领域工作流。

```text
Agent
  -> 理解用户意图、选择 Skill、组织执行计划

Skill
  -> 封装领域步骤、Tool 白名单、输入输出、审批点和回滚方式

Tool
  -> 执行一个原子操作，例如查询 Route、模拟匹配或提交资源
```

三者职责不同：

- Agent 负责推理、澄清意图和选择合适的 Skill
- Skill 负责稳定、可测试、可审计的领域流程
- Tool 负责访问 Admin API、Apiserver、Compiler、xDS 状态和可观测系统
- Policy 负责判断当前用户是否有权调用 Skill 和其中的 Tool

候选内置 Skills 包括：

```text
create-gateway
create-route
bind-rate-limit-policy
bind-access-control-policy
diagnose-route-404
diagnose-upstream-502
explain-policy-impact
preview-runtime-snapshot
rollback-gateway-change
import-openapi
onboard-model-provider
plan-model-gray-migration
audit-gateway-security
migrate-higress-config
```

例如 `diagnose-route-404` Skill 可以固化以下流程：

```text
1. 解析请求的 Host、Path、Method 和 Header
2. 查询关联 Gateway 和 Listener
3. 执行 Route 匹配模拟
4. 检查 Logical IR 和 RuntimeSnapshot
5. 检查 xDS 发布和 ACK/NACK 状态
6. 输出证据、根因和建议
```

这套步骤不应每次都由模型自由规划。Skill 固化可靠流程，模型只负责补充参数、解释结果和处理必要分支，可以显著减少幻觉和不稳定调用。

### Skill 定义建议

Skill 后续可以采用结构化定义，但第一阶段不需要立即建模为公开资源。候选定义包含：

```yaml
name: diagnose-route-404
version: v1
description: 诊断请求未命中 Route 的原因
inputs:
  gatewayID: string
  host: string
  path: string
  method: string
tools:
  - get_gateway
  - explain_route_match
  - compile_preview
  - get_xds_status
approval: none
outputSchema: routeDiagnosis
```

涉及写操作的 Skill 还需要声明：

- 所需权限和资源范围
- 执行前置条件
- 人工审批点
- 最大操作次数和超时
- 幂等键和版本前置条件
- 执行后验证方式
- 回滚步骤

### Skill 分层

可以考虑三类 Skill：

- 内置 Skill：随 Ingate 发布，覆盖网关配置、诊断、策略和迁移等稳定能力
- 组织 Skill：由平台管理员基于企业流程配置，例如发布审批、值班排障和安全巡检
- Agent Tool Skill：把 OpenAPI、MCP 和内部 API 组合成业务 Agent 可复用的工作流

组织 Skill 不应默认允许执行任意代码、Shell 或 kubectl。第一阶段应限制为对已注册 Tool 的结构化编排，后续再评估受沙箱保护的扩展机制。

Skills 应运行在控制面或 Agent Runtime，不运行在 Envoy Wasm 中。Envoy 和 Wasm 负责流量治理和 Tool 出口策略，不承担长时间工作流编排。

## 内置 Agent 方向

### Gateway Ops Agent

负责配置和解释网关资源：

- 创建或修改 Gateway、Route、Upstream 和 RuntimeGroup 引用
- 创建 RateLimitPolicy、AccessControlPolicy 和 PolicyBinding
- 解释资源引用关系和策略影响范围
- 预览编译结果和 RuntimeSnapshot diff
- 用户确认后执行变更和回滚

典型问题：

```text
创建一个网关，把 api.example.com/orders 转发到订单服务
给支付接口增加每租户每分钟 100 次限流
只允许办公网段访问管理接口
这个 PolicyBinding 会影响哪些 Route
```

### Incident Diagnosis Agent

负责基于证据排查网关运行问题：

- 分析 404、502、503 和超时
- 模拟 Host、Path、Method、Header 等 Route 匹配过程
- 检查资源是否进入 Logical IR 和 RuntimeSnapshot
- 检查 xDS 版本以及 ACK/NACK 状态
- 分析 Upstream 健康、访问日志、指标和 trace
- 对比故障前后的配置和运行状态变化

第一阶段可以和 Gateway Ops Agent 合并，避免过早增加多个产品入口。

### API Integration Agent

负责把企业 API 接入网关并转换成 Agent Tool：

- 导入 OpenAPI 或 MCP Server 定义
- 识别服务地址、路径、参数和认证方式
- 创建 Upstream 和 Route
- 生成 ToolSet 和最小权限访问策略
- 执行连通性检查
- 输出变更计划和不兼容项

典型任务：

```text
导入订单系统 OpenAPI，只开放查询接口，并让订单 Agent 可以调用
```

### ModelOps Agent

负责模型接入和运行治理：

- 新增和验证 ModelProvider
- 检查协议兼容性和模型映射
- 配置灰度迁移和 fallback
- 比较质量、延迟、错误率和 Token 成本
- 观察新模型运行结果并建议扩大流量或回滚
- 管理供应商 API Token 的健康状态

### Security Governance Agent

负责持续检查和生成治理建议：

- 检查未鉴权 Route 和公网暴露的管理接口
- 检查过宽的 ACL、Tool 权限和 K8s RBAC
- 检查模型供应商 Key 是否暴露给调用方
- 检查 Prompt、Header、日志和 Tool 响应中的敏感数据
- 建议 RateLimitPolicy、AccessControlPolicy、ToolAccessPolicy 和内容安全策略

该 Agent 初期只提供巡检和建议，不自动修改安全策略。

### AI FinOps Agent

负责 AI 调用成本和配额优化：

- 按租户、Route、Agent 和模型分析 Token 使用量
- 分析模型成本、缓存命中率和错误重试成本
- 识别免费额度滥用和异常高消耗调用方
- 建议模型路由、缓存、quota 和 fallback 策略

### Gateway Migration Agent

负责从已有网关迁移到 Ingate：

- 导入 Nginx Ingress、Higress、APISIX、Kong 或 Envoy Gateway 配置
- 生成 Ingate 强类型资源
- 把第三方插件配置映射为 Route 原生能力或治理 Policy
- 输出不兼容项、迁移步骤、灰度方案和回滚方案

### Business API Agent

Business API Agent 是用户通过 Ingate 创建的业务 Agent，不属于 Ingate Assistant 的内置专家。

典型场景包括订单助手、客服助手、数据库查询助手和 K8s 助手。Ingate 负责这些 Agent 的模型入口、Tool 出口、安全、配额和审计，Agent Runtime 负责推理和任务执行。

## Gateway Ops Agent 第一阶段设计

Gateway Ops Agent 最适合作为第一个落地能力，因为它可以直接复用 Ingate 当前声明式控制面，并推动控制面形成稳定、可解释的工具接口。

### 只读工具

候选工具包括：

```text
list_resources
get_gateway
get_route
get_upstream
inspect_references
explain_route_match
simulate_request
compile_preview
diff_runtime_snapshot
get_xds_status
get_upstream_health
query_access_logs
query_metrics
```

Agent 的诊断必须基于这些结构化工具返回的证据，不应只读取一段日志后自由推测。

### 写入工具

候选写入工具包括：

```text
propose_change
apply_change
rollback_change
```

Agent 不直接操作 store、generated client、xDS cache 或 Envoy。所有资源变更必须通过 Admin API 或声明式 Apiserver，并继续经过 Compiler 和 target translator。

### 典型排障流程

```text
用户：为什么 POST api.example.com/orders 返回 404？

Agent：
1. 查找匹配的 Gateway 和 Listener
2. 模拟 Host / Path / Method 路由匹配
3. 检查 Route 是否进入 Logical IR
4. 检查 RuntimeSnapshot 是否包含该 Route
5. 检查 xDS 发布版本和 ACK/NACK
6. 输出证据、根因和修改建议
7. 用户确认后生成并提交资源变更
8. 观察新版本状态，必要时执行回滚
```

## 变更计划和审批

写操作必须采用结构化计划，而不是让模型直接连续调用修改接口。

```json
{
  "summary": "为订单路由增加租户限流",
  "operations": [
    {
      "action": "create",
      "resource": "RateLimitPolicy"
    },
    {
      "action": "create",
      "resource": "PolicyBinding"
    }
  ],
  "impact": ["route/orders"],
  "preconditions": ["route version = 12"],
  "rollback": "删除新增的 PolicyBinding 和 RateLimitPolicy"
}
```

第一阶段不一定需要把 ChangePlan 建模成持久化资源，可以先作为 Agent 与 Admin API 之间的稳定 DTO。是否升级为一等资源需要根据审计、审批和异步执行需求另行评审。

## 安全约束

企业级 Agent 必须从第一版建立以下约束：

- 查询工具和写入工具分开授权
- 每次工具调用携带当前用户身份和权限上下文
- Agent 不能获得超出当前用户的权限
- 写操作默认需要人工确认
- 使用资源版本前置条件，避免覆盖并发修改
- 记录用户输入、模型输出、工具调用、资源 diff、执行人和执行结果
- Secret、API Key、Authorization、Cookie 和敏感 Header 在进入模型前脱敏
- 日志、资源描述和 Tool 响应视为不可信输入，防止 prompt injection
- 不向 Agent 提供任意 Shell、kubectl 或网络访问能力
- 自动处置只允许预先授权的有限动作，并设置时间、次数和影响范围上限
- 所有自动变更必须具备可验证的回滚路径

## Higress Envoy 和 hostcall 方向

文章显示 Higress 的 Envoy 和 Wasm SDK 已经封装 HTTP、Redis 等 hostcall，并提供 Shared Data、Shared Queue 和 Wasm Service 等能力。这为删除独立 `ingate-dataplane` 进程提供了可行方向，但目前仍属于候选设计。

如果 Ingate 选择 Higress Envoy 作为第一个官方数据面，可以考虑：

```text
RateLimit / Cache / 简单状态能力
  -> Ingate runtime boundary
      -> Higress Redis hostcall adapter
```

必须保持以下边界：

- Policy 和插件 runtime 不直接依赖 Higress wrapper 类型
- Higress hostcall 只存在于 target/runtime adapter
- 核心配置模型不暴露 Higress cluster name、matchRules 或私有 JSON
- 其它 runtime target 可以提供自己的能力实现

删除 `ingate-dataplane` 前需要验证：

- Redis `EVAL` / `EVALSHA` 支持
- Standalone、Sentinel、Cluster 支持
- TLS、认证和 Secret 轮换
- 超时、取消和 callback 语义
- 连接池、Envoy worker 并发和故障恢复
- 指标、日志和错误分类
- 当前限流算法和 fail-open / fail-close 行为一致性

Shared Data 和 Shared Queue 适合小型缓存、计数、指标聚合和跨 VM 通知，不应作为 Agent 会话、长期任务或持久化工作流存储。

## 未来候选资源

以下资源只是方向候选，不代表已经进入当前实现范围：

```text
ModelProvider
Model
Agent
Skill
ToolProvider
ToolSet
AgentBinding
ToolAccessPolicy
TokenQuotaPolicy
ContentSafetyPolicy
```

设计这些资源时继续遵循 Ingate 的现有原则：

- 控制面表达网关和 AI 领域语义，不表达某个数据面实现
- 用户配置强类型资源，不直接编辑插件私有 JSON
- 资源之间使用不可变 ID 引用
- Compiler 解析资源关系并生成 Logical IR
- Target translator 生成具体运行时配置

## 建议演进路线

### 阶段一：控制面可解释性

- 增加 Route 匹配模拟和解释能力
- 增加资源引用关系查询
- 增加 compile preview 和 RuntimeSnapshot diff
- 增加 xDS 发布状态查询

这些能力即使没有 Agent，也能改善 Console、CLI 和故障排查体验。

### 阶段二：只读 Gateway Ops Agent

- 在 Console 提供统一 Agent 入口
- 建立第一批只读内置 Skills
- 只开放查询、模拟、解释和诊断工具
- 输出证据链和结构化建议
- 不允许写资源

### 阶段三：辅助配置

- Agent 生成结构化 ChangePlan
- Console 展示资源 diff、影响范围和风险
- 用户确认后通过 Admin API 提交
- 记录完整审计信息

### 阶段四：API 和 Tool 平台

- 导入 OpenAPI 和 MCP 定义
- 建立 Tool Registry
- 为 Tool 调用增加身份、权限、限流和审计
- 实现 API Integration Agent

### 阶段五：ModelOps 和 Agent Runtime

- 建立 ModelProvider 和 AI Proxy 能力
- 实现模型灰度、fallback、Token 统计和安全治理
- 引入独立 Agent Runtime
- 开放用户定义的 Business API Agent

### 阶段六：有限自动运维

- 只开放预先授权的自动动作
- 支持异常 Upstream 摘除、配置回滚和灰度调整等有限场景
- 自动动作必须有观察窗口、停止条件和回滚策略

## 待确认问题

- 是否将 Higress Envoy 定义为 Ingate 第一个官方数据面发行版
- 是否删除独立 `ingate-dataplane`，以及 hostcall 验证标准
- Agent 工具首先采用内部 Go API、HTTP API 还是 MCP 协议
- Gateway Ops Agent 是否作为独立服务 `ingate-agent` 部署
- ChangePlan 是否需要成为持久化一等资源
- Tool Registry 与现有 Upstream、Route 的资源关系
- Agent Runtime 的部署、租户隔离和状态存储边界
- Console 内置 Agent 与外部 MCP 客户端的身份和权限传递方式

## 当前建议

近期不直接实现完整 Agent Runtime。优先建设控制面的可解释性接口，包括 Route 模拟、编译预览、RuntimeSnapshot diff 和 xDS 状态。这些能力是 Gateway Ops Agent 的可靠工具基础，也可以独立提升 Ingate 当前产品质量。

第一个 Agent 建议采用只读 Gateway Ops Agent，通过真实控制面数据回答配置和排障问题。在只读能力稳定后，再加入变更计划、人工审批和受控执行。
