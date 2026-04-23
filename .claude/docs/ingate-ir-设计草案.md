# Ingate IR 设计草案

## 1. 文档目标

本文档用于定义 Ingate 未来的中间表示（IR）设计方向，重点回答：

1. 为什么 Ingate 需要稳定 IR
2. IR 应该分几层，各自解决什么问题
3. IR 与现有 `ResolvedGateway` 的关系如何演进
4. IR 如何支撑多运行时目标和未来 AI 网关能力

本文档不是最终字段定义，而是设计草案。

---

## 2. 为什么需要 IR

当前 Ingate 的控制面主链路已经形成：

- 输入：Gateway / Route / Backend / Certificate / AuthPolicy / TrafficPolicy
- 收敛：`ResolvedGateway`
- 输出：xDS/runtime config

问题在于：

1. 当前收敛结果更偏向现有运行时目标
2. 未来如果支持更多策略、插件、AI 语义、多运行时，控制面不能直接绑定 Envoy/xDS
3. 缺少稳定 IR 时，控制面扩展会持续侵入收敛逻辑和翻译逻辑

因此，IR 的目标不是增加一个中间对象，而是为控制面建立**长期稳定的语义边界**。

---

## 3. IR 的设计目标

IR 应满足以下目标：

1. **稳定表达逻辑意图**
2. **不直接绑定具体运行时实现**
3. **可被插件修改、校验、增强**
4. **可翻译到多个 target runtime**
5. **可承载未来 AI 网关能力**
6. **便于做 diff、审计、回滚、兼容演进**

---

## 4. 建议的两层 IR 结构

建议把 IR 分成两层。

### 4.1 Logical IR

Logical IR 用于表达逻辑配置意图，不绑定 Envoy 或其他具体运行时。

它回答的问题是：

**系统希望流量如何被路由、治理、保护、观测。**

建议承载如下能力：

- Listener
- Host / VirtualHost
- Route
- Match
- BackendRef
- TLS
- AuthN / AuthZ
- RateLimit / Retry / Timeout
- Traffic split / failover
- Observability
- Extension hooks
- AI route / model route / tool route / session / guardrail 等未来能力

### 4.2 Runtime Snapshot

Runtime Snapshot 用于表达某个运行时如何执行。

它回答的问题是：

**这个运行时最终要拿到什么配置快照。**

例如：

- Envoy xDS snapshot
- AI runtime snapshot
- 未来其他 sidecar/runtime snapshot

它应该由 translator 从 Logical IR 生成，而不是成为上层 API 的一部分。

---

## 5. Logical IR 设计建议

### 5.1 分层原则

建议 Logical IR 至少拆成以下几层语义：

1. **入口层**
   - listener
   - host
   - port
   - protocol
   - TLS

2. **路由层**
   - route
   - match
   - rewrite
   - redirect
   - weighted destinations

3. **后端层**
   - backend refs
   - endpoint group
   - health / failover
   - load balance hints

4. **策略层**
   - authn
   - authz
   - ratelimit
   - timeout
   - retry
   - circuit breaking
   - observability

5. **扩展层**
   - plugin attachments
   - extension hooks
   - runtime hints

6. **AI 领域层**
   - model selection
   - prompt assembly hints
   - tool route
   - guardrail hooks
   - session semantics
   - cache semantics

### 5.2 设计原则

1. Logical IR 应表达“语义”，而不是表达“某个运行时字段名”
2. 能够在编译阶段做合并、diff、验证
3. 能够附带来源信息，便于追溯某个字段来自哪条资源或策略
4. 能够表达 capability requirement，便于 translator 判断目标是否支持

### 5.3 来源追踪建议

建议 IR 中保留来源元信息，例如：

- 来源资源名
- 来源 GVK
- 来源作用域
- 来源优先级
- merge 轨迹

这样有利于：

- 解释最终配置是怎么来的
- 做冲突诊断
- 做控制台 diff 展示
- 做审计与回滚

---

## 6. Runtime Snapshot 设计建议

### 6.1 定位

Runtime Snapshot 不应是通用模型，而应是目标运行时的具体编译产物。

### 6.2 建议职责

每个 Runtime Snapshot 负责：

1. 对应一个明确 target
2. 包含该 target 的完整执行配置
3. 带有编译时间、版本、源 IR 信息
4. 可直接用于发布或缓存

### 6.3 与 translator 的关系

建议关系如下：

- 输入：Logical IR
- 输出：Runtime Snapshot
- 执行者：Target Translator

也就是说，Runtime Snapshot 是 translator 的输出，不应反向约束 Logical IR 的语义建模。

---

## 7. `ResolvedGateway` 的演进建议

### 7.1 当前定位

当前 `ResolvedGateway` 已经天然承担了控制面中间结果的角色。

### 7.2 存在的问题

当前它更像：

- 以现有 Gateway 领域为中心的收敛结果
- 面向当前 xDS 发布链路的中间资源

它还不是严格意义上的稳定 IR，原因包括：

1. 与当前运行时目标关系较近
2. 编译阶段尚未完全显式分离
3. 插件 attach 点与 capability 边界还不清晰
4. 未来 AI 语义不一定自然落进去

### 7.3 演进路径

建议分三步走：

#### 第一步：把 `ResolvedGateway` 当作当前阶段 IR 载体

短期内不推翻现有资源模型，继续让 controller 输出 `ResolvedGateway`。

#### 第二步：在 controller 内部先建立内存态 Logical IR

controller 收敛时：

- 资源 -> 内存态 Logical IR
- Logical IR -> `ResolvedGateway`（当前兼容载体）
- Logical IR -> Runtime Snapshot

这样可以先把语义中心从 `ResolvedGateway` 转移到 IR，而不影响现有链路。

#### 第三步：再决定持久化策略

长期再评估：

- `ResolvedGateway` 是否继续存在
- 是否演进成稳定 IR 资源
- 是否拆成更通用的 `CompiledGateway` / `GatewayIR` 一类对象

---

## 8. IR 与插件的关系

IR 是插件体系真正稳定的作用面。

建议原则：

1. 大部分控制面插件都不要直接操作原始资源集合
2. 插件优先操作规范化后的 Logical IR
3. 只有少量插件运行在 Validate / Normalize 等前置阶段
4. Runtime 相关插件应作用在 Translate 之后的 target-specific 产物上

换句话说：

- 资源层解决“用户怎么声明”
- IR 层解决“系统怎么理解”
- Snapshot 层解决“运行时怎么执行”

---

## 9. IR 与多运行时的关系

未来控制面可能需要同时支持：

- Envoy/xDS
- AI runtime
- 其他 sidecar/proxy/runtime

因此 IR 中应避免出现过多 target-specific 字段。

建议做法：

1. 只在 Logical IR 中表达通用语义
2. 把 target 特定字段下沉到 translator 和 Runtime Snapshot
3. 如果确实需要 target hint，也应以 capability 或 hint 的方式表达，而不是把某个运行时的对象结构直接嵌入 IR

---

## 10. IR 与 AI 网关的关系

如果未来要支持 AI 网关，IR 需要具备承载 AI 语义的能力。

建议至少预留以下语义面：

- model routing
- provider selection
- prompt/context assembly hooks
- tool routing
- session semantics
- streaming semantics
- guardrail hooks
- cache semantics
- audit semantics

重点原则：

AI 语义应成为 Logical IR 的新领域层，而不是附着在传统 Backend 字段上的特殊 case。

---

## 11. 兼容性与版本治理

IR 是长期边界，必须版本化治理。

建议逐步引入：

- `irVersion`
- `compatibilityVersion`
- `capabilitySetVersion`

每次大语义变更都应考虑：

1. 是否改变了 Logical IR 语义
2. 是否需要 translator 同步升级
3. 是否影响插件 SDK
4. 是否需要兼容路径或编译时拒绝策略

---

## 12. 分阶段落地建议

### 第一阶段

1. 在 controller 内部显式抽象 Normalize / Resolve / Merge / PatchIR / Translate 阶段
2. 设计内存态 Logical IR 结构
3. 增加来源追踪与 merge 元数据
4. 保持 `ResolvedGateway` 作为兼容输出

### 第二阶段

1. 引入 Target Translator 接口
2. 让 xDS 成为一个具体 translator
3. 为 Runtime Snapshot 建立标准元数据
4. 让插件更多作用于 IR 而不是直接作用于收敛代码

### 第三阶段

1. 为 AI 领域扩展 IR
2. 让 IR 同时支撑传统网关与 AI 网关
3. 评估是否需要新的持久化 IR 资源

---

## 13. 结论

IR 不是单纯的数据结构调整，而是 Ingate 从“面向当前实现的控制面”走向“面向未来平台演进的控制面”的关键边界。

最重要的结论有三点：

1. Logical IR 必须稳定表达语义，而不是绑定运行时
2. Runtime Snapshot 应由 translator 生成，而不是反向定义上层模型
3. `ResolvedGateway` 不必立刻移除，但应逐步从派生产物演进为 IR 演化路径中的兼容载体
