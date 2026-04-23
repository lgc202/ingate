# Ingate 长期架构蓝图

## 1. 文档目的

本文档用于沉淀 Ingate 在未来 3-5 年到 10 年维度上的架构演进方向，重点回答三个问题：

1. Ingate 应该如何设计成一个长期可维护、可扩展的网关平台
2. 如何在不推倒现有主链路的前提下，引入稳定的插件体系
3. 如何为未来演进为 AI 网关预留清晰的模型和运行时边界

本文档不是当前实现说明，而是面向未来演进的目标架构与落地路径。

---

## 2. 架构目标

### 2.1 总体目标

将 Ingate 从“单一控制面项目”演进为：

**声明式资源 + 统一中间表示（IR） + 编译型控制面 + 可插拔策略/插件 + 多运行时数据面**

### 2.2 设计原则

1. **核心做小，边缘做活**
   - 核心只负责资源建模、依赖收敛、IR 编译、目标翻译、发布与观测
   - 变化快的能力通过扩展点承载，如策略、插件、Provider、AI 能力、控制台语义

2. **控制面做编排与翻译，数据面做执行**
   - 控制面不直接承载所有复杂执行逻辑
   - 数据面或外部执行组件负责请求期行为

3. **稳定边界优先于短期功能堆叠**
   - API 边界
   - IR 边界
   - Runtime 边界
   - Plugin 边界
   - Tenant / Security 边界

4. **默认面向多运行时，而不是绑定单一实现**
   - Envoy/xDS 是当前主要目标，但不应成为唯一目标

5. **AI 网关作为一级领域能力演进，而不是把 AI 当成特殊 Backend**

---

## 3. 当前架构的基本判断

Ingate 当前主链路是合理的，已经具备演进成平台型控制面的基础：

1. 资源写入 `ingate-apiserver`
2. `ingate-controller-manager` 监听 Gateway/Route/Backend/Certificate/AuthPolicy/TrafficPolicy
3. 各资源 controller 负责入队和依赖关系维护
4. `resolvedgateway` controller 收敛全量依赖，生成 `ResolvedGateway`
5. `ingate-xds-server` 监听 `ResolvedGateway` 并翻译发布到数据面
6. `ingate-admin-api` 提供控制台语义层

当前最重要的架构锚点是：

- `ResolvedGateway` 已经天然承担了“控制面中间结果”的角色
- controller-manager 到 xDS-server 的关系已经接近“编译 -> 发布”流水线

当前主要限制也很明确：

1. `ResolvedGateway` 更像面向当前数据面的派生产物，还不是稳定 IR
2. 控制器收敛过程中的多个阶段尚未显式抽象为可插拔 pass
3. xDS 翻译层对未来多运行时支持不够显式
4. 缺少插件生命周期、能力声明、作用域、顺序等统一模型
5. 缺少 AI 场景下需要的 provider、tool、session、streaming、guardrail 等抽象

---

## 4. 目标架构

### 4.1 目标架构总览

建议将 Ingate 演进为四层结构：

1. **Resource Layer**
   - 对外声明式 API
   - Gateway / Route / Backend / Policy / AI 相关资源

2. **IR & Compile Layer**
   - 资源归一化
   - 依赖解析
   - 策略合并
   - 插件 pass
   - 生成逻辑 IR 与目标运行时快照

3. **Runtime Adapter Layer**
   - Envoy/xDS translator
   - AI runtime translator
   - 未来其他代理/runtime translator

4. **Execution Layer**
   - Envoy / Wasm / ext_proc / 独立 AI runtime
   - 负责真实请求处理

### 4.2 控制面流水线

建议把当前控制面内部显式抽象成如下阶段：

1. **Source Ingest**
   - 接入 K8s CRD、Admin API、未来外部配置源

2. **Normalize**
   - 默认值填充
   - 版本兼容与语义统一

3. **Reference Resolve**
   - 解析 Gateway、Route、Backend、Certificate、Policy 依赖关系

4. **Policy Merge**
   - 统一合并全局、租户、命名空间、Gateway、Route、Backend 级策略

5. **Extension Pass**
   - 插件在这一阶段改写、补充或校验 IR

6. **Target Translate**
   - 把 IR 翻译到具体运行时目标

7. **Publish**
   - 发布 xDS/配置快照
   - 记录审计、状态与回滚元数据

这几个阶段应逐步从当前 `resolvedgateway` 相关实现中拆出，形成清晰的编译流水线。

---

## 5. IR 设计建议

### 5.1 为什么需要稳定 IR

如果未来要同时支持：

- 更多策略类型
- 多种插件
- 多运行时
- AI 网关能力

那么控制面核心不能直接绑定 Envoy 或当前 xDS 语义，必须拥有稳定 IR。

### 5.2 建议拆成两层 IR

#### 1. Logical IR

表达逻辑意图，不绑定具体运行时，主要包含：

- Listener / Host / Route / Match / BackendRef
- TLS
- AuthN / AuthZ
- RateLimit / Retry / Timeout / TrafficPolicy
- Observability
- Extension Hooks
- AI Route / Model Route / Tool Route 等未来能力

#### 2. Runtime Snapshot

表达某个运行时如何执行，按目标生成：

- Envoy xDS snapshot
- AI runtime config snapshot
- 未来其他 runtime snapshot

### 5.3 `ResolvedGateway` 的演进建议

不建议立即删除 `ResolvedGateway`。更现实的方式是：

1. 短期把 `ResolvedGateway` 视为当前阶段 IR 载体
2. 中期在控制器内部先引入更稳定的内存态 IR
3. 长期再决定是否引入更明确的持久化 IR 资源

演进方向应是：

- `ResolvedGateway` 从“为当前 xDS 服务的派生产物”
- 逐步演进为“网关逻辑配置的稳定收敛结果”

---

## 6. 插件体系设计

### 6.1 插件分层

建议从一开始就区分三类插件：

#### A. 控制面插件

作用在编译阶段，例如：

- admission 扩展
- 校验扩展
- policy merge 扩展
- route rewrite 扩展
- AI 语义增强扩展
- target translator 扩展

#### B. 数据面插件

作用在请求执行阶段，例如：

- Wasm 插件
- ext_proc 外部处理服务
- 内建 filter
- AI 推理/护栏/审计外部服务

#### C. 管理面插件

作用在控制台和运维运营层，例如：

- 插件市场
- 审批流
- 计费
- 多租户管理
- AI 模型目录与配额管理

### 6.2 插件模型的关键约束

插件必须具备以下长期属性：

1. **阶段（Phase）明确**
   - 插件只能运行在指定阶段

2. **作用域（Scope）明确**
   - global / tenant / namespace / gateway / route / backend / model-route / tool-route

3. **顺序（Order）明确**
   - 必须支持插件顺序和依赖关系声明

4. **能力（Capability）声明明确**
   - 是否允许访问 Secret
   - 是否允许改写 Header
   - 是否允许调用外部服务
   - 是否支持流式响应
   - 是否允许使用模型 Provider

5. **版本兼容（Compatibility）明确**
   - core version
   - plugin SDK version
   - runtime capability version

6. **发布与回滚能力明确**
   - 插件独立版本化
   - 最好支持 OCI 化、签名、热更新与回滚

### 6.3 插件装载建议

不建议使用 Go `plugin` 作为长期机制，原因是 ABI 和版本兼容成本高。

建议优先采用：

1. **注册式插件 + 模块化接口**
   - 适合控制面 pass 扩展

2. **进程外插件（gRPC / MCP / sidecar service）**
   - 适合复杂能力和长期演进

3. **Wasm**
   - 适合请求期轻逻辑和热更新场景

### 6.4 数据面插件的选型建议

1. **内建 filter**
   - 适合性能敏感、长期稳定的核心能力

2. **Wasm**
   - 适合轻量请求期扩展

3. **ext_proc / 外部处理服务**
   - 适合 AI、鉴权、风控、上下文拼装、复杂策略、审计等重逻辑

总体原则：

- 核心链路能力内建
- 业务差异能力插件化
- AI 重逻辑优先外部服务，不强塞 Wasm

---

## 7. 面向 AI 网关的扩展设计

### 7.1 核心原则

AI 网关不能只是“把模型服务当成一个特殊 Backend”。

更合理的方式是把 AI 能力建模为新的领域层，并让它们进入同一套控制面编译体系。

### 7.2 建议新增的 AI 领域资源

1. **ModelProvider**
   - 表示 OpenAI、Anthropic、Gemini、Azure OpenAI、本地 vLLM 等上游模型提供方
   - 负责 endpoint、认证、模型目录、配额能力描述

2. **ModelRoute**
   - 表示 AI 请求路由规则
   - 支持按模型、标签、租户、成本、延迟、可用性进行选择

3. **PromptPolicy**
   - 表示 prompt 模板、上下文拼装、变量注入、系统提示词约束

4. **GuardrailPolicy**
   - 表示输入输出审计、PII 脱敏、风险拦截、越权限制

5. **ToolPolicy**
   - 表示工具白名单、调用预算、超时、鉴权与审计策略

6. **SessionPolicy**
   - 表示会话保持、上下文记忆、TTL 与粘性策略

7. **AICachePolicy**
   - 表示 prompt cache、response cache、semantic cache 策略

8. **AIEvalPolicy / AIObservabilityPolicy**
   - 表示质量评估、成本、token、延迟、命中率等观测策略

### 7.3 AI 请求执行链建议

建议把 AI 请求路径抽象为：

1. Auth
2. Quota
3. Prompt / Context Assembly
4. ModelRoute
5. Guardrail（请求前）
6. ProviderAdapter
7. Guardrail（响应后）
8. Cache / Trace / Audit

其中：

- `ModelRoute` 是 AI 领域的核心路由能力
- `ProviderAdapter` 是 AI provider 标准化适配层
- `Guardrail` / `ToolPolicy` / `SessionPolicy` 应支持插件化
- 流式响应必须是一级能力

### 7.4 AI 运行时建议

不建议把所有 AI 执行逻辑都塞进 Envoy filter。

更合适的方式是：

- 通用 L4/L7 继续由 Envoy 负责
- AI 重逻辑通过 ext_proc、sidecar service 或独立 AI runtime 承载
- 控制面统一编译、路由和治理

也就是说：

- Envoy 更像交通调度器
- AI runtime 更像智能执行器

---

## 8. 多运行时目标设计

### 8.1 目标

控制面核心不应只面向 Envoy/xDS，而应支持多个 target translator。

### 8.2 建议的目标抽象

控制面输出应支持：

- Envoy xDS
- 独立 AI runtime 配置
- 未来其他代理/sidecar/runtime 配置

### 8.3 Target Translator 的职责

每个 translator 负责：

1. 声明支持的 IR capability
2. 校验目标运行时是否能承载某些能力
3. 把 IR 翻译成目标快照
4. 返回降级/不支持/冲突信息

这样可以避免上层 API 被某个数据面实现细节反向污染。

---

## 9. 工程治理建议

参考 OneX 的工程治理思路，Ingate 未来应补齐以下能力：

### 9.1 目录和模块边界

建议保持并强化以下边界：

- `cmd/`：应用入口
- `internal/<domain>/`：业务域
- `internal/pkg/`：仓内共享基础设施
- `pkg/`：对外复用能力与公共模型
- `pkg/generated/`：生成代码

### 9.2 统一基础设施

各二进制入口应逐步标准化：

- config
- logging
- healthz / readyz
- metrics
- tracing
- graceful shutdown
- pprof

### 9.3 自动化与生成

长期维护应依赖：

- codegen
- contract test
- conformance test
- compatibility test
- golden test

### 9.4 版本治理

建议逐步定义并治理：

- API version
- IR version
- Plugin SDK version
- Runtime capability version

### 9.5 插件兼容性验证

插件必须通过：

- contract test
- upgrade test
- rollback test
- performance budget test

---

## 10. 分阶段落地建议

### 10.1 第一阶段：1 年内

目标：不推翻现有主链路，补齐关键抽象。

建议动作：

1. 保留现有 `apiserver -> controller-manager -> ResolvedGateway -> xds-server` 主链路
2. 在 controller 内部抽象编译阶段：Normalize / Resolve / Merge / Extension / Translate
3. 把 `ResolvedGateway` 的字段语义向稳定 IR 方向梳理
4. 把 xDS translate 抽象成 target translator 接口
5. 引入插件元数据模型：phase、scope、order、capability、compatibility
6. 先支持一类控制面扩展和一类请求期扩展
   - 控制面：注册式 pass 插件
   - 数据面：Wasm 或 ext_proc

### 10.2 第二阶段：3-5 年

目标：从控制面项目演进为平台型网关。

建议动作：

1. 支持多配置源
2. 支持多运行时 target
3. 增加插件市场与策略中心
4. 把 admin-api 从控制台接口升级为平台管理面
5. 引入多租户、审计、配额、变更 diff、回滚能力

### 10.3 第三阶段：5-10 年

目标：把 AI 网关能力建设为一级公民。

建议动作：

1. 引入 AI 领域模型
2. 标准化 provider adapter
3. 把流式响应、工具调用、会话治理做成一级能力
4. 支持 prompt / guardrail / cache / eval 插件化
5. 支持成本、质量、延迟三目标联合调度

---

## 11. 建议优先落地的四件事

如果只选最重要的四件事，建议优先做：

1. **稳定 IR**
2. **插件阶段模型**
3. **多运行时 target 抽象**
4. **AI 领域模型独立化**

这四件事决定 Ingate 将来是继续做传统网关控制面，还是进一步演进为插件平台和 AI 网关，而不需要重写一遍系统。

---

## 12. 后续可继续补充的文档

后续建议继续在 `.claude/docs/` 下补充：

1. `ingate-插件体系设计.md`
   - 插件元模型
   - 生命周期
   - 装载方式
   - 顺序与冲突处理

2. `ingate-ir-设计草案.md`
   - Logical IR
   - Runtime Snapshot
   - 与现有 `ResolvedGateway` 的映射关系

3. `ingate-ai-网关设计草案.md`
   - AI 领域模型
   - Provider 抽象
   - 请求执行链
   - 流式与 Tool 调用模型

4. `ingate-分阶段演进路线图.md`
   - 按季度或里程碑拆解落地顺序

---

## 13. 结论

Ingate 当前最有价值的不是某个单点功能，而是已经具备了一条正确的控制面主链路。未来应避免在核心里不断堆功能，而应该把它升级成：

- 有稳定 IR 的编译型控制面
- 有明确边界的插件平台
- 有多运行时目标的网关平台
- 可以自然长出 AI 网关能力的长期架构

最重要的长期原则是：

**把核心做小，把扩展做强，把边界做稳。**
