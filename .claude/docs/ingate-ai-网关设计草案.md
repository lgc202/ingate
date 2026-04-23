# Ingate AI 网关设计草案

## 1. 文档目标

本文档聚焦 Ingate 未来演进为 AI 网关时的设计方向，重点回答：

1. AI 网关能力应该如何进入现有控制面体系
2. 需要新增哪些 AI 领域模型
3. AI 请求执行链应该如何抽象
4. Envoy、控制面、AI runtime 的边界如何划分

本文档目标不是定义具体 API 字段，而是明确架构方向和边界。

---

## 2. 核心判断

AI 网关不能简单理解为“把大模型服务当成一个特殊 Backend”。

这种做法短期也许能跑通，但长期会遇到以下问题：

1. 模型选择不等于普通后端选择
2. Prompt、上下文、工具调用、会话、流式输出都不是传统 L7 路由自然能表达的内容
3. Guardrail、缓存、评估、审计是 AI 场景中的一级能力
4. AI 能力变化极快，不能持续侵入传统网关核心路径

因此，AI 网关应该作为 Ingate 的一个新领域层进入控制面，而不是成为 Backend 的特例。

---

## 3. 设计目标

AI 网关设计应满足以下目标：

1. **复用现有控制面主链路**
   - 继续沿用声明式资源 -> 收敛 -> IR -> translator -> runtime 的主线

2. **把 AI 作为一级领域建模**
   - 明确 provider、model route、tool route、guardrail、session 等概念

3. **与传统网关能力协同**
   - 复用认证、限流、审计、可观测、多租户等平台能力

4. **支持快速变化**
   - AI provider、prompt 策略、tool 调用方式变化快，需要插件化与 adapter 化

5. **支持流式与工具调用**
   - 这些必须作为一级能力，而不是后补 feature

---

## 4. AI 领域模型建议

建议把 AI 网关新增资源独立出来，而不是强塞进现有 Backend/Route 模型。

### 4.1 ModelProvider

表示模型上游提供方，例如：

- OpenAI
- Anthropic
- Gemini
- Azure OpenAI
- 本地 vLLM
- 企业内部模型平台

它应描述：

- endpoint
- auth 方式
- 支持的模型目录
- 并发与速率限制
- streaming 能力
- tool use 能力
- 成本或能力标签

### 4.2 ModelRoute

表示 AI 请求该路由到哪个 provider / model。

应支持：

- 按租户/业务/标签路由
- 按成本、延迟、质量、可用性路由
- fallback
- canary
- A/B testing
- 多模型兜底

### 4.3 PromptPolicy

表示 prompt 相关策略，例如：

- 系统提示词模板
- 上下文注入
- 变量绑定
- 长度限制
- 多模板选择

### 4.4 GuardrailPolicy

表示 AI 安全与合规策略，例如：

- 输入审查
- 输出审查
- PII 脱敏
- 风险内容阻断
- 越权限制
- 工具调用保护

### 4.5 ToolPolicy

表示工具调用策略，例如：

- 可用工具白名单
- 工具调用预算
- 工具鉴权
- 超时与重试
- 审计要求

### 4.6 SessionPolicy

表示会话相关策略，例如：

- session stickiness
- 上下文窗口
- memory TTL
- 跨请求上下文引用策略

### 4.7 AICachePolicy

表示缓存策略，例如：

- prompt cache
- response cache
- semantic cache
- cache key 策略
- TTL

### 4.8 AIObservabilityPolicy / AIEvalPolicy

表示：

- token 消耗
- 成本
- 延迟
- cache hit rate
- 拒答率
- 工具调用成功率
- 模型质量评估

---

## 5. AI 请求执行链建议

AI 网关的执行链不应等同于传统 HTTP 代理链。

建议抽象为：

1. **Auth**
   - 请求鉴权、租户识别、调用身份建立

2. **Quota**
   - QPS、并发、token 预算、租户预算

3. **Prompt / Context Assembly**
   - prompt 模板应用
   - 上下文注入
   - session 记忆拼装

4. **ModelRoute**
   - 选择 provider / model / fallback 目标

5. **Guardrail（请求前）**
   - 对输入做审查、脱敏、限制

6. **ProviderAdapter**
   - 标准化调用上游模型 provider

7. **Tool Orchestration**
   - 如果有工具调用，执行工具路由、鉴权与审计

8. **Guardrail（响应后）**
   - 对输出做审查、过滤、脱敏

9. **Cache / Trace / Audit**
   - 记录 token、成本、延迟、工具调用链路、审计信息

其中需要特别强调：

- 流式输出应贯穿整条链路
- Tool 调用不应是外挂逻辑，而应进入执行模型
- Prompt / Session / Guardrail 都应支持插件化

---

## 6. Provider Adapter 设计建议

未来 AI provider 会持续变化，因此 Provider Adapter 必须是稳定抽象。

### 6.1 目标

Provider Adapter 负责把统一 AI 请求语义翻译为具体 provider 调用。

### 6.2 建议职责

每个 adapter 负责：

1. provider endpoint 与鉴权适配
2. 模型能力发现
3. 请求格式转换
4. streaming 适配
5. tool use 适配
6. 错误映射
7. 限流/重试/超时策略对接

### 6.3 设计原则

1. 上层不应直接感知某个 provider 的特定字段
2. 统一表达模型能力，再由 adapter 做映射
3. adapter 应声明自身支持的 capability

---

## 7. AI 运行时边界建议

### 7.1 为什么不能把所有逻辑塞进 Envoy

Envoy 很适合做：

- L4/L7 流量转发
- 基础认证
- 基础限流
- 标准代理治理

但它不适合承载所有 AI 逻辑，原因包括：

- AI prompt/context/tool/session 逻辑复杂度高
- provider 变化快
- 推理前后处理演进快
- 工具编排和 guardrail 经常需要进程外能力

### 7.2 建议边界

#### 控制面

负责：

- AI 资源建模
- IR 收敛
- 路由与策略编译
- 目标配置生成
- 治理与观测统一接入

#### Envoy / 通用代理层

负责：

- 基础入口
- 通用鉴权
- 基础限流
- 路由转发
- 部分轻量请求期扩展

#### AI runtime

负责：

- prompt/context assembly
- provider adapter
- tool orchestration
- guardrail 执行
- session 管理
- AI 结果增强与审计

### 7.3 总结

可以把未来运行时理解为：

- Envoy 是交通调度器
- AI runtime 是智能执行器
- 控制面是统一编排者

---

## 8. 与 IR 的关系

AI 网关能力应进入 Logical IR，而不是只在 translator 里做特殊分支。

建议在 IR 中逐步支持：

- model routing
- provider selection
- prompt/context hooks
- tool routing
- session semantics
- streaming semantics
- guardrail hooks
- AI cache semantics
- AI audit semantics

这样可以保证：

1. AI 能力和传统网关能力共享一套编译体系
2. AI 插件可以围绕 IR 工作
3. 多 runtime 可以共享上层语义

---

## 9. AI 插件设计建议

AI 网关中的高变化能力，尽量不要全部内建。

建议插件优先承载：

- PromptPolicy 扩展
- Guardrail 扩展
- ToolPolicy 扩展
- ProviderAdapter 扩展
- Eval 扩展
- Cache 扩展

建议插件形态：

- 控制面：注册式或进程外插件
- 请求期轻逻辑：Wasm
- 请求期重逻辑：ext_proc / sidecar / 独立 AI runtime

---

## 10. 观测与治理建议

AI 网关相较传统网关，必须新增更细的观测维度。

建议至少追踪：

- 请求数
- token 输入输出
- provider / model 选择结果
- fallback 次数
- tool 调用次数与成功率
- cache 命中率
- latency
- error class
- 拒答率
- 成本
- 租户维度配额消耗

同时应支持：

- 请求级 tracing
- prompt 与 response 审计索引
- 可配置的脱敏与留存策略

---

## 11. 分阶段落地建议

### 第一阶段

1. 明确 AI 不是 Backend 特例
2. 在 IR 设计中预留 AI 语义面
3. 先设计 `ModelProvider`、`ModelRoute` 两个核心概念
4. 选定一种 AI runtime 承载方式（推荐进程外）

### 第二阶段

1. 引入 PromptPolicy / GuardrailPolicy / ToolPolicy
2. 建立 Provider Adapter 抽象
3. 让 streaming 成为一级能力
4. 接入 AI 专项观测指标

### 第三阶段

1. 引入 SessionPolicy / AICachePolicy / AIEvalPolicy
2. 支持多模型联合调度
3. 支持成本、质量、延迟三目标治理
4. 支持企业级 AI 安全治理与审计

---

## 12. 结论

AI 网关最容易走错的路，是把它当成“传统 API 网关的一点增强”。

更合理的方向是：

- 复用现有控制面主链路
- 把 AI 作为新领域建模
- 让 IR 成为 AI 与传统网关共享的语义边界
- 让 Envoy 做通用代理，让 AI runtime 承载重逻辑
- 用插件体系承接高频变化能力

这样 Ingate 才能在未来既是一个传统网关平台，也能自然长成 AI 网关平台。
