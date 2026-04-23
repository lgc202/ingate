# Ingate 目录与接口草案

## 1. 文档目标

本文档把长期架构蓝图进一步下沉到代码组织和接口抽象层，重点回答：

1. 如果引入 IR、translator、插件体系，目录该怎么调整
2. 哪些接口应该先抽出来
3. AI 网关相关代码未来适合放在哪些包中

本文档是面向实现的草案，不要求一次到位，但要求演进路径清晰。

---

## 2. 当前目录的基本判断

当前仓库目录大方向是合理的：

- `cmd/`：入口
- `internal/controlplane/`：核心控制面
- `internal/adminapi/`：控制台语义层
- `pkg/apis/`：资源定义
- `pkg/generated/`：生成产物

未来不建议推翻当前大结构，而建议在 `internal/controlplane/` 下继续细化层次。

---

## 3. 建议的目标目录结构

下面是建议逐步演进出来的结构，不要求一次全部创建。

```text
internal/
  controlplane/
    apiserver/
    compiler/
      pipeline/
      ir/
      normalize/
      resolve/
      merge/
      patch/
      translate/
      publish/
      trace/
    controller/
      gateway/
      route/
      backend/
      certificate/
      authpolicy/
      trafficpolicy/
      resolvedgateway/
    plugin/
      model/
      registry/
      runtime/
      validator/
    runtime/
      envoy/
        translate/
        publish/
        snapshot/
      ai/
        translate/
        snapshot/
      capability/
    ai/
      provider/
      route/
      prompt/
      guardrail/
      tool/
      session/
      cache/
    shared/
    status/
    index/
  adminapi/
  pkg/
pkg/
  apis/
    gateway/
    policy/
    ai/
  generated/
```

其中重点是新增：

- `compiler/`
- `plugin/`
- `runtime/`
- `ai/`

---

## 4. `compiler/` 目录建议

### 4.1 目标

把今天散落在 `resolvedgateway` builder、controller 收敛逻辑、xDS translate 前置逻辑中的“编译行为”显式收拢。

### 4.2 子目录建议

#### `compiler/ir/`

放 IR 结构定义，例如：

- `logical.go`
- `listener.go`
- `route.go`
- `policy.go`
- `extension.go`
- `metadata.go`

#### `compiler/pipeline/`

放编译流水线编排逻辑，例如：

- `pipeline.go`
- `context.go`
- `result.go`

#### `compiler/normalize/`

放默认值与语义归一化。

#### `compiler/resolve/`

放依赖解析逻辑。

#### `compiler/merge/`

放策略合并逻辑。

#### `compiler/patch/`

放 IR patch / 扩展 pass。

#### `compiler/translate/`

放 translator 抽象，而不是具体 Envoy 实现。

#### `compiler/publish/`

放发布抽象，例如 snapshot publisher、状态回写接口。

#### `compiler/trace/`

放来源追踪、merge trace、冲突解释等。

---

## 5. `plugin/` 目录建议

### 5.1 `plugin/model/`

放插件元模型：

- plugin descriptor
- phase
- scope
- capability
- compatibility
- dependency

### 5.2 `plugin/registry/`

放插件注册、查找、排序、依赖解析。

### 5.3 `plugin/runtime/`

放插件运行时上下文，例如：

- compile context
- plugin execution context
- telemetry hooks

### 5.4 `plugin/validator/`

放插件配置与兼容性校验。

---

## 6. `runtime/` 目录建议

### 6.1 目标

把“控制面怎么翻译到不同执行目标”的职责集中收口。

### 6.2 `runtime/envoy/`

放 Envoy 相关实现：

- translator
- snapshot model
- publisher
- capability mapping

当前 `internal/controlplane/xds/` 可逐步向这里靠拢。

### 6.3 `runtime/ai/`

放 AI runtime 相关实现：

- AI translator
- AI runtime snapshot
- provider adapter bridge

### 6.4 `runtime/capability/`

放不同 target 支持能力的声明、比较与 negotiation。

---

## 7. `ai/` 目录建议

### 7.1 目标

让 AI 领域能力在控制面内部有独立归属，而不是散落在 plugin、runtime、route 逻辑里。

### 7.2 子目录建议

- `provider/`：provider adapter 抽象与目录
- `route/`：model route 相关逻辑
- `prompt/`：prompt policy 相关逻辑
- `guardrail/`：guardrail 相关逻辑
- `tool/`：tool route / tool policy
- `session/`：session 语义与状态治理
- `cache/`：AI cache 策略

---

## 8. `pkg/apis/ai/` 建议

当 AI 资源成熟后，建议在 `pkg/apis/ai/` 下独立定义，而不是混进 `gateway` 或 `policy` 中。

例如：

```text
pkg/apis/ai/v1alpha1/
  modelprovider_types.go
  modelroute_types.go
  promptpolicy_types.go
  guardrailpolicy_types.go
  toolpolicy_types.go
  sessionpolicy_types.go
  aicachepolicy_types.go
```

这样更利于：

- API 版本治理
- AI 领域边界清晰
- 生成代码和客户端隔离

---

## 9. 建议优先抽出来的接口

下面这些接口是最值得优先引入的。

### 9.1 Compile Pipeline

```go
type Pipeline interface {
    Compile(ctx context.Context, input Input) (*Result, error)
}
```

用于统一收口 controller 的收敛编译过程。

### 9.2 Phase Step

```go
type Step interface {
    Name() string
    Run(ctx context.Context, state *State) error
}
```

用于把 Normalize、Resolve、Merge、PatchIR、Translate 拆成稳定阶段。

### 9.3 Translator

```go
type Translator interface {
    Name() string
    Supports(cap CapabilitySet) error
    Translate(ctx context.Context, ir *ir.LogicalGateway) (Snapshot, error)
}
```

让 xDS 成为 translator 的一个实现。

### 9.4 Publisher

```go
type Publisher interface {
    Publish(ctx context.Context, snapshot Snapshot) error
}
```

让发布逻辑和翻译逻辑分离。

### 9.5 Plugin Descriptor

```go
type Descriptor interface {
    Name() string
    Version() string
    Kind() Kind
    Phase() Phase
    Scope() []Scope
    Capabilities() []Capability
    Compatibility() Compatibility
}
```

### 9.6 Phase-specific Plugins

```go
type ValidatorPlugin interface {
    Validate(ctx context.Context, state *State) error
}

type IRPatchPlugin interface {
    PatchIR(ctx context.Context, state *State) error
}

type TargetTranslatorPlugin interface {
    Translate(ctx context.Context, ir *ir.LogicalGateway) (Snapshot, error)
}
```

重点是不要搞一个什么都能干的巨大插件接口。

### 9.7 AI Provider Adapter

```go
type ProviderAdapter interface {
    Name() string
    Capabilities() CapabilitySet
    Invoke(ctx context.Context, req *ModelRequest) (*ModelResponse, error)
    Stream(ctx context.Context, req *ModelRequest) (Stream, error)
}
```

---

## 10. 与现有代码的演进关系

### 10.1 不建议直接迁移所有代码

建议采用“壳先搭起来，逻辑逐步搬迁”的方式。

例如：

1. 先定义 `compiler/pipeline` 接口
2. 先在 `resolvedgateway` builder 外包一层 pipeline
3. 再逐步把 normalize/resolve/merge 逻辑迁进去

### 10.2 `xds/` 目录的演进建议

当前 `internal/controlplane/xds/` 很自然，短期不必强拆。

建议方式：

- 先抽象 translator 接口
- 再让 `xds/translate` 实现该接口
- 后续逐步把 `xds/` 视为 `runtime/envoy/` 的实现来源

### 10.3 `resolvedgateway/` 目录的演进建议

短期继续保留，但它的职责应逐步从：

- 直接承担全部收敛构建

演进为：

- 驱动 compile pipeline
- 兼容输出 `ResolvedGateway`
- 回写状态

---

## 11. 建议的落地顺序

### 第一批

1. `compiler/pipeline`
2. `compiler/ir`
3. `compiler/translate`
4. `plugin/model`

### 第二批

1. `plugin/registry`
2. `runtime/capability`
3. `runtime/envoy`
4. `compiler/trace`

### 第三批

1. `ai/provider`
2. `runtime/ai`
3. `pkg/apis/ai/`
4. `plugin/runtime`

---

## 12. 结论

目录和接口设计的目标不是“为了好看”，而是为了让未来新能力能找到稳定落点。

这份草案最重要的意图有三点：

1. 把“编译”从 controller 细节里提炼出来
2. 把“运行时目标”从 xDS 单实现里抽象出来
3. 把“AI 与插件扩展”提前纳入包结构与接口边界

如果这几层边界立住，Ingate 后续无论做插件平台、多 target，还是 AI 网关，演进成本都会低很多。
