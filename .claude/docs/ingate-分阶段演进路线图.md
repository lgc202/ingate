# Ingate 分阶段演进路线图

## 1. 文档目标

本文档将长期架构蓝图拆解为更可执行的演进路线，重点回答：

1. 当前到未来 1 年、3-5 年、5-10 年分别优先做什么
2. 哪些能力必须先打地基，哪些能力可以后置
3. 如何避免一上来大改导致系统失稳

本文档默认前提是：

- 不推翻现有 `apiserver -> controller-manager -> ResolvedGateway -> xds-server` 主链路
- 采用渐进式演进，而不是重写

---

## 2. 路线图原则

### 2.1 先立边界，再加能力

优先建设：

- IR
- phase 化编译流水线
- target translator
- 插件元模型

而不是先做大量新功能。

### 2.2 先内部抽象，再外部 API

很多变化应先在控制器和翻译层内部完成，再决定是否上升到用户可见 API。

### 2.3 先兼容演进，再考虑替换

短期不要急于删除 `ResolvedGateway` 或重构所有 controller，而应让新抽象与旧链路并存。

### 2.4 先控制面，后平台化，最后 AI 一级化

建议节奏：

1. 控制面内核升级
2. 插件/多运行时/治理平台化
3. AI 领域正式一级化

---

## 3. 阶段 0：现状收敛期（现在 - 3 个月）

目标：形成统一认知，避免后续改造方向发散。

### 3.1 目标产出

1. 完成长线架构文档沉淀
2. 统一 `ResolvedGateway` 的现状语义边界
3. 识别当前收敛逻辑和翻译逻辑中的阶段边界
4. 明确哪些位置未来是插件点、哪些不是

### 3.2 建议动作

1. 梳理 `resolvedgateway` builder 的输入、输出、阶段
2. 梳理 xDS translator 的核心对象流转
3. 梳理当前策略资源与最终收敛结果的关系
4. 补充核心术语表
   - Resource
   - IR
   - Runtime Snapshot
   - Translator
   - Plugin
   - Capability

### 3.3 阶段完成标志

- 团队对当前主链路和未来目标架构有统一理解
- 已形成文档而非停留在口头讨论

---

## 4. 阶段 1：控制面内核抽象期（3 - 12 个月）

目标：不改主链路形态，但把控制面从“收敛代码集合”升级成“编译流水线”。

### 4.1 必须优先落地的事情

#### A. 抽象编译阶段

在 controller 内部明确拆出：

1. Normalize
2. ResolveReference
3. MergePolicy
4. PatchIR
5. Translate

这里可以先是内部包和接口，不一定立即暴露给外部。

#### B. 引入内存态 Logical IR

目标不是马上替换 `ResolvedGateway`，而是让控制面先围绕 IR 思考和编码。

建议路径：

- Resource -> Logical IR
- Logical IR -> `ResolvedGateway`
- Logical IR -> runtime-specific translator

#### C. 建立 Target Translator 接口

让 xDS 成为第一个 translator，而不是默认唯一 translator。

#### D. 建立插件元模型

先定义但不必一开始支持完整插件市场。

优先支持：

- phase
- scope
- capability
- order
- compatibility

### 4.2 可并行推进的事项

1. 为 IR 增加来源追踪与 merge trace
2. 为 translator 增加 capability 检查
3. 为 `ResolvedGateway` 字段做语义梳理和去运行时化

### 4.3 阶段完成标志

- xDS 发布链路已经跑在 translator 接口之上
- controller 内部已存在明确的编译阶段和内存态 IR
- 新增一类策略或扩展能力时，不需要继续把逻辑糊在 builder 里

---

## 5. 阶段 2：插件与平台化基础期（1 - 2 年）

目标：让 Ingate 从“可扩展代码结构”变成“有真实扩展机制的平台”。

### 5.1 必须落地的能力

#### A. 控制面插件机制 v1

优先支持：

- 注册式插件
- phase 化接口
- 插件顺序与冲突检测
- 编译错误可解释化

#### B. 数据面插件机制 v1

建议优先在以下两种中选一种作为起点：

1. Wasm
2. ext_proc

如果未来重点偏 AI，优先级建议是：

- 先 ext_proc
- 后 Wasm

#### C. 插件治理基础能力

包括：

- 插件状态
- 插件启停
- 插件升级
- 插件回滚
- 插件观测

#### D. 平台治理基础能力

包括：

- 审计
- diff
- 配额
- 多租户基础模型
- 变更记录

### 5.2 可选增强项

1. 插件 OCI 打包
2. 插件签名
3. 插件审批流
4. 插件市场原型

### 5.3 阶段完成标志

- 至少有一类控制面插件和一类请求期插件已经能独立扩展系统行为
- 扩展能力不再必须修改主链路核心代码

---

## 6. 阶段 3：多运行时目标期（2 - 4 年）

目标：从“单一 Envoy 控制面”升级为“多 target 编译平台”。

### 6.1 目标能力

1. 多 translator 并存
2. IR capability 到 runtime capability 的映射
3. 不同 runtime 的支持矩阵
4. 运行时降级与冲突可见

### 6.2 建议顺序

1. 固化 xDS translator 接口和快照元数据
2. 引入第二个 target
   - 可以是实验性的 AI runtime translator
   - 也可以是内部 mock translator，用于验证抽象正确性
3. 建立 capability negotiation
4. 建立 per-target conformance 测试

### 6.3 阶段完成标志

- 控制面语义已不再隐式绑定 Envoy
- 第二个 target 可以在不改上层资源模型的情况下接入

---

## 7. 阶段 4：AI 网关一级化（3 - 5 年）

目标：把 AI 从扩展场景升级为一级领域能力。

### 7.1 先做的能力

1. `ModelProvider`
2. `ModelRoute`
3. Provider Adapter
4. AI runtime
5. AI 专项观测

### 7.2 第二批能力

1. `PromptPolicy`
2. `GuardrailPolicy`
3. `ToolPolicy`
4. streaming 一级能力
5. token/cost/quota 治理

### 7.3 第三批能力

1. `SessionPolicy`
2. `AICachePolicy`
3. `AIEvalPolicy`
4. 多模型联合调度
5. 成本、延迟、质量三目标治理

### 7.4 阶段完成标志

- AI 能力已经不再是“传统网关上的外挂功能”
- AI 资源、IR、runtime、观测、治理形成完整闭环

---

## 8. 阶段 5：平台成熟期（5 - 10 年）

目标：支撑企业级长期运营和多业务线演进。

### 8.1 关键能力

1. 插件市场和生态治理
2. 企业级签名、审批、发布策略
3. 多租户插件隔离
4. 变更影响分析
5. 大规模兼容升级
6. API / IR / SDK / runtime 版本矩阵治理

### 8.2 关键风险

1. 插件碎片化
2. Core 被插件反向污染
3. 兼容债务累积
4. AI provider 变化过快导致 adapter 层失控
5. 运行时多样化导致测试矩阵膨胀

### 8.3 治理重点

- 合同测试
- conformance 测试
- 生命周期管理
- 版本矩阵治理
- 观测与审计

---

## 9. 我建议优先排进 roadmap 的 8 个实施项

如果要更落地，我建议最近几个里程碑优先按这个顺序：

1. 明确 controller 编译阶段
2. 设计并落地内存态 Logical IR
3. 抽象 xDS translator 接口
4. 为 IR 增加来源追踪与 merge trace
5. 定义插件元模型
6. 落地控制面插件 v1
7. 落地 AI runtime 原型或第二 target 原型
8. 引入 AI 核心资源 `ModelProvider` / `ModelRoute`

---

## 10. 不建议现在立刻做的事情

为了避免路线跑偏，当前不建议一上来就做：

1. 直接重写所有 controller
2. 立刻替换 `ResolvedGateway`
3. 一次性设计完整插件市场
4. 先做很多 AI API 再补核心抽象
5. 让所有能力都走 Wasm
6. 在缺少 IR 的情况下引入第二套复杂控制面

---

## 11. 结论

这条路线的核心思想是：

- 第一年打内核抽象地基
- 第二年做真实扩展机制
- 第三到第五年做多运行时和 AI 一级化
- 五到十年重点在生态与治理，而不是继续重写核心

如果节奏把握得当，Ingate 可以从当前控制面项目自然演进为长期可持续的平台，而不是经历一次高风险重构。
