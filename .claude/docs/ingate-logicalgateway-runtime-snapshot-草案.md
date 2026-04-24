# Ingate LogicalGateway 与 RuntimeSnapshot 草案

## 1. 目的

这份文档用于沉淀当前关于 `LogicalGateway` 与 `RuntimeSnapshot` 的讨论结果，作为现有实现继续演进时的近线说明。

它不替代 `.claude/docs/` 下已有的长期架构、IR、插件、AI 网关文档，而是回答一个更贴近当前代码的问题：

**如果 Ingate 要继续从现有 `ResolvedGateway -> xDS` 结构演进，下一步最核心的语义边界应该是什么**

---

## 2. 当前代码的两个雏形

当前仓库里已经有两个很重要的雏形。

### 2.1 `ResolvedGateway` builder

`internal/controlplane/controller/resolvedgateway/builder.go` 当前负责：

- 接收 `ResourceBundle`
- 聚合 Gateway、Route、Backend、Certificate、Policy
- 构造 `ResolvedGateway`

这部分已经很接近“逻辑收敛”阶段，只是目前产物还是 `ResolvedGateway`。

### 2.2 `RuntimeConfig` translator

`internal/controlplane/xds/translate/resolvedgateway.go` 当前负责：

- 读取 `ResolvedGateway`
- 生成面向 xDS 发布链路的 `RuntimeConfig`

这部分已经很接近“target-specific runtime output”阶段，只是当前 target 只有 Envoy/xDS。

---

## 3. 为什么要进一步引入 `LogicalGateway`

当前 `ResolvedGateway` 同时承担了两个角色：

1. 控制面收敛结果
2. 当前 xDS 链路的中间输入

短期这没有问题，但长期会带来几个限制：

1. 控制面语义和当前运行时形状耦合得太紧
2. 插件很难找到稳定作用面
3. 第二个 runtime 很难接入
4. AI 网关语义容易被迫塞进现有 Backend / Route 结构

因此下一步更合理的做法是：

- 把“逻辑语义层”提纯成 `LogicalGateway`
- 把“运行时输出层”明确成 `RuntimeSnapshot`

---

## 4. `LogicalGateway` 与 `RuntimeSnapshot` 的区别

### 4.1 `LogicalGateway`

`LogicalGateway` 表达的是：

**系统想让网关如何工作**

它应该聚焦逻辑语义，例如：

- listener
- route
- match
- backend
- TLS
- auth / traffic policy merge 结果
- extension attachments
- AI 相关语义入口

它不应该直接绑定 Envoy，也不应该直接出现 xDS 资源组织方式。

### 4.2 `RuntimeSnapshot`

`RuntimeSnapshot` 表达的是：

**某个具体 runtime 最终应该拿到什么执行配置**

它天然是 target-specific 的，例如：

- `EnvoySnapshot`
- `AIRuntimeSnapshot`
- 未来其他 runtime snapshot

同一个 `LogicalGateway` 可以被翻译成多个不同 target 的 snapshot。

---

## 5. 一种更清晰的对象流转

长期更推荐的对象流转是：

`Resources -> ResourceBundle -> LogicalGateway -> Translator -> RuntimeSnapshot -> Publisher`

而不是一直停留在：

`Resources -> ResolvedGateway -> RuntimeConfig`

这条新链路的意义是：

- `ResourceBundle` 负责输入聚合
- `LogicalGateway` 负责统一语义
- `Translator` 负责 target-specific 映射
- `RuntimeSnapshot` 负责执行配置输出
- `Publisher` 负责发布、缓存、同步

---

## 6. 为什么这种分层更适合插件

插件如果直接围绕资源对象或最终 ADS 结果工作，会很快失控。

更稳定的方式是：

- 控制面插件围绕编译阶段和 `LogicalGateway`
- 运行时插件围绕 target-specific snapshot

这意味着未来最自然的插件点会是：

- Validate
- Normalize
- Resolve
- MergePolicy
- PatchIR
- Translate
- Publish

其中最核心的插件作用面是 `LogicalGateway`，因为它代表统一语义，而不是某个 runtime 细节。

---

## 7. 为什么这种分层更适合 AI 网关

AI 网关不能简单视为特殊 Backend。

AI 领域天然需要新增语义，例如：

- `ModelProvider`
- `ModelRoute`
- `PromptPolicy`
- `GuardrailPolicy`
- `ToolPolicy`
- `SessionPolicy`
- `AICachePolicy`

如果没有统一的语义层，这些概念只能被硬塞进现有 route/backend/filter 结构，长期会让模型越来越别扭。

引入 `LogicalGateway` 后，可以把 AI 作为新的语义层挂进去，再由不同 translator 决定：

- 哪些能力下发给 Envoy
- 哪些能力下发给 AI runtime
- 哪些能力交给外部执行组件

所以：

- `LogicalGateway` 是插件和 AI 语义的稳定作用面
- `RuntimeSnapshot` 是多 runtime 执行输出的稳定边界

---

## 8. 与当前实现的关系

当前代码可以这样理解：

- `ResolvedGateway` 是尚未提纯完成的逻辑收敛结果
- `RuntimeConfig` 是 Envoy/xDS 方向的 runtime output 雏形

也就是说，今天的代码并不是方向错了，而是还没有把中间边界显式立出来。

---

## 9. 当前阶段建议的落点

为了避免一次性重写全链路，当前阶段更适合先做这一步：

1. 新增最小 `LogicalGateway` 结构
2. 从 `ResourceBundle` 提炼 `LogicalGateway`
3. 暂时保留 `LogicalGateway -> ResolvedGateway` 的兼容转换
4. 暂时保留现有 `ResolvedGateway -> RuntimeConfig` 路径

这样可以先把语义中心立住，再逐步推动后续演进。

---

## 10. 结论

这次演进最核心的判断是：

- `ResolvedGateway` 不应该长期同时承担语义中心和运行时输入两个角色
- `LogicalGateway` 应逐步成为统一语义边界
- `RuntimeSnapshot` 应逐步成为 target-specific 执行边界
- 这套分层是 Ingate 后续支持插件、多 runtime、AI 网关的关键基础

从当前代码出发，最现实的第一步不是重写整条链路，而是先把 `LogicalGateway` 这个对象落地。