# 04 codegen 和生成代码怎么读

## 1. 为什么这里必须讲“生成代码”

因为当前 Ingate 的一大块工程价值，恰恰就在于：

**它不是手糊一个 API，而是接上了 Kubernetes 风格的代码生成链。**

如果你忽略这块，就会误以为：
- `pkg/generated/` 只是一些杂项
- 生成脚本只是附属品

这两个判断都不对。

## 2. 先分两类源

### 资源源
- `pkg/apis/...`

### 契约源
- `proto/...`

## 3. 对应两类生成物

### 从资源源生成
- `zz_generated.deepcopy.go`
- `zz_generated.defaults.go`
- `zz_generated.model_name.go`
- `pkg/generated/openapi/zz_generated.openapi.go`
- `clientset`
- `informers`
- `listers`

### 从契约源生成
- `*.pb.go`
- `*_grpc.pb.go`

## 4. 为什么不能把这些手写掉

因为这些代码的共同特点是：
- 机械重复
- 要求非常一致
- 一旦手写，后面会反复出错

这正是“应该生成”的典型场景。

## 5. 当前各脚本的职责

### `generate-apis.sh`
负责：
- `DeepCopy`
- 默认值函数
- OpenAPI schema
- OpenAPI model name

### `generate-clients.sh`
负责：
- `clientset`
- `informer`
- `lister`

### `generate-proto.sh`
负责：
- proto Go 代码

### `generate-all.sh`
负责：
- 串起来统一执行

### `verify-generated.sh`
负责：
- 先重新生成
- 再检查生成物是否存在
- 再检查仓库中的生成结果是否过期

## 6. 为什么 `verify-generated` 很重要

因为真正难的不是“生成一次”，而是：

**如何防止以后大家忘记更新生成物。**

这就是为什么成熟工程都会有 `verify-generated` 这类入口。

## 7. 为什么 OpenAPI 还会生成 `model_name`

你现在会看到两个看起来有点像的生成物：

- `pkg/generated/openapi/zz_generated.openapi.go`
- `pkg/apis/**/zz_generated.model_name.go`

它们不是重复。

### `zz_generated.openapi.go`

它描述的是：

**每个类型有哪些字段、字段是什么类型、OpenAPI schema 长什么样。**

例如：

- `Gateway.spec.listeners`
- `Route.spec.rules`
- `AuthPolicy.spec.jwt`

这些字段结构会进入 `/openapi/v2` 和 `/openapi/v3`。

### `zz_generated.model_name.go`

它描述的是：

**Go 类型对应哪个 OpenAPI model name。**

这个名字会被 Kubernetes 的 `DefinitionNamer` 用来把 OpenAPI schema 和真实的 GVK 关联起来。

这里的 GVK 指：

- group
- version
- kind

比如：

```text
gateway.ingate.io/v1alpha1, Kind=Gateway
```

为什么这个关联重要？

因为 generic apiserver 的字段管理能力需要知道：

**一个请求里的对象到底对应哪个结构化 schema。**

如果只有 schema，但 model name 和 scheme 对不上，apiserver 运行时可能会出现类似：

```text
failed to update managedFields
no corresponding type for gateway.ingate.io/v1alpha1, Kind=Gateway
```

所以 `model_name` 不是为了好看，也不是额外复杂化。

它是让：

- OpenAPI
- scheme
- GVK
- managedFields

这几件事能正确接起来的桥。

## 8. 为什么 `generate-apis.sh` 里 OpenAPI 分两段生成

现在脚本里 OpenAPI 生成分两次跑：

1. 先只对 Ingate 自己的 API 包生成 `zz_generated.model_name.go`
2. 再把 Ingate API 包和 Kubernetes 依赖包一起生成完整 OpenAPI schema

这样做是因为：

- `model_name` 只应该落在我们自己的 `pkg/apis/...` 包里
- 完整 OpenAPI schema 又需要引用 Kubernetes 的 `metav1`、`runtime`、`version` 等依赖类型

如果把这两件事混在一次生成里，输出位置和职责会变得不清楚。

分开以后边界更明确：

- API 包自己携带 model name
- `pkg/generated/openapi` 只承载最终 schema provider

## 9. 读生成代码时的顺序

建议按这个顺序读：

1. 先读 `pkg/apis/.../types.go`
2. 再看 `doc.go` 上的生成标记
3. 再看 `tools/hack/generate-apis.sh`
4. 再看 `zz_generated.deepcopy.go`
5. 再看 `zz_generated.defaults.go`
6. 再看 `zz_generated.model_name.go`
7. 最后再看 `pkg/generated/openapi/zz_generated.openapi.go`

不要一上来就读完整 OpenAPI 生成文件。

那个文件很长，很机械，不适合作为入口。
