# 07 OpenAPI 的生成和接线

## 1. 这篇只回答一个问题

**当前 `ingate-apiserver` 的 OpenAPI 到底是怎么接进去的。**

你已经在使用文档里看过：

- `/openapi/v2`
- `/openapi/v3`

这篇只讲代码链路。

## 2. 先讲主链

当前链路非常固定：

```text
pkg/apis/... 的类型定义
-> doc.go 上的 openapi 标记和 openapi model package 标记
-> tools/hack/generate-apis.sh 跑 openapi-gen 两段生成
-> pkg/apis/**/zz_generated.model_name.go
-> pkg/generated/openapi/zz_generated.openapi.go
-> internal/controlplane/apiserver/config.go 接进 generic apiserver
-> /openapi/v2 和 /openapi/v3 对外提供 schema
```

如果你把这条线记住了，后面看具体代码就不会迷路。

## 3. 先看源头在哪里

先看这两个文件：

- [pkg/apis/gateway/v1alpha1/doc.go](/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/gateway/v1alpha1/doc.go)
- [pkg/apis/policy/v1alpha1/doc.go](/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/policy/v1alpha1/doc.go)

你会看到：

```go
// +k8s:deepcopy-gen=package
// +k8s:openapi-gen=true
// +k8s:openapi-model-package=com.github.lgc202.ingate.pkg.apis.gateway.v1alpha1
```

这里的重点是：

```go
// +k8s:openapi-gen=true
// +k8s:openapi-model-package=...
```

它们的含义不是“马上生成文件”，而是：

**告诉 OpenAPI 生成器，这个包里的类型要参与 OpenAPI 定义生成，并且这些类型应该使用哪个稳定的 OpenAPI model name。**

为什么标记放在 `doc.go`？

因为这是 Kubernetes 代码生成链的常见做法。
它不是业务逻辑文件，而是“给生成器看的包级声明”。

为什么要有 `openapi-model-package`？

因为 Kubernetes 的 `DefinitionNamer` 会通过 model name 把 OpenAPI schema 和 scheme 里的 GVK 对起来。

如果 model name 不稳定，或者和 scheme 期望的不一致，generic apiserver 仍然能启动，但字段管理会找不到资源对应的结构化 schema。

这时日志里可能出现：

```text
failed to update managedFields
no corresponding type for gateway.ingate.io/v1alpha1, Kind=Gateway
```

这不是普通展示问题。

它说明：

- OpenAPI schema 有了
- scheme 里也注册了 GVK
- 但二者中间缺少正确的 model name 映射

## 4. 再看生成脚本

看：

- [tools/hack/generate-apis.sh](/Users/guangcaili/workplace/code/lgc202/ingate/tools/hack/generate-apis.sh)

这支脚本现在做两件事：

1. 跑 `deepcopy-gen`
2. 跑 `defaulter-gen`
3. 跑 `openapi-gen`

这里你先只抓 OpenAPI 这一段：

- 它先通过 `go list -m` 取出当前项目依赖的 `k8s.io/kube-openapi` 版本
- 然后用 `go run` 直接执行对应版本的 `openapi-gen`
- 第一段只处理 Ingate 自己的 API 包，生成：

```text
pkg/apis/gateway/v1alpha1/zz_generated.model_name.go
pkg/apis/policy/v1alpha1/zz_generated.model_name.go
```

- 第二段生成最终 OpenAPI schema provider，输出到：

```text
pkg/generated/openapi/zz_generated.openapi.go
```

为什么这里用 `go run`，而不是要求你先手工安装 `openapi-gen`？

因为当前项目更适合：

- 尽量减少外部前置安装
- 让工具版本跟 `go.mod` 里的依赖版本对齐

这对小团队和学习阶段都更稳。

为什么 OpenAPI 要分两段？

因为这两类输出职责不同：

- `zz_generated.model_name.go` 要落回具体 API 包，让类型自己知道 model name
- `zz_generated.openapi.go` 要集中在 `pkg/generated/openapi`，作为 apiserver 运行时统一读取的 schema provider

另外，完整 schema 需要带上 `k8s.io/apimachinery` 里的依赖类型。

但是 `model_name` 不应该生成到 Kubernetes 依赖包里，只应该生成到我们自己的 API 包里。

所以分两段更清楚。

## 5. 为什么输出放在 `pkg/generated/openapi`

因为这里是：

- 代码生成产物
- 供 apiserver 运行时导入

它不是：

- 手写业务逻辑
- 也不是单独的产品模块

所以它最适合放在：

```text
pkg/generated/openapi
```

这和：

- `pkg/generated/clientset`
- `pkg/generated/informers`
- `pkg/generated/listers`
- `pkg/generated/proto`

是同一类东西。

## 6. 生成出来的代码到底长什么样

看：

- [pkg/apis/gateway/v1alpha1/zz_generated.model_name.go](/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/gateway/v1alpha1/zz_generated.model_name.go)
- [pkg/apis/policy/v1alpha1/zz_generated.model_name.go](/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/policy/v1alpha1/zz_generated.model_name.go)
- [pkg/generated/openapi/zz_generated.openapi.go](/Users/guangcaili/workplace/code/lgc202/ingate/pkg/generated/openapi/zz_generated.openapi.go)

你不需要现在把整文件读完。

先只知道它们做的是：

### `zz_generated.model_name.go`

**给每个类型提供稳定的 OpenAPI model name。**

例如 `Gateway` 会有一个 `OpenAPIModelName()` 方法。

这个方法不是业务逻辑。

它是给 Kubernetes API machinery 用的。

### `zz_generated.openapi.go`

**为每个资源类型提供 OpenAPI definition。**

也就是：

- `Gateway`
- `Route`
- `Backend`
- `AuthPolicy`
- `TrafficPolicy`

这些对象在 schema 里的字段信息，都在这里被描述出来。

为什么这类代码必须生成？

因为它非常：

- 机械
- 冗长
- 容易出错
- 不值得人工维护

## 7. 真正把 OpenAPI 接进 apiserver 的地方在哪

看：

- [internal/controlplane/apiserver/config.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/config.go)

最关键的是这几件事：

1. 引入生成代码：

```go
generatedopenapi "github.com/lgc202/ingate/pkg/generated/openapi"
```

2. 创建 definition namer：

```go
namer := openapi.NewDefinitionNamer(ingatescheme.Scheme)
```

3. 拿到 definition provider：

```go
getDefinitions := generatedopenapi.GetOpenAPIDefinitions
```

4. 配置 v2：

```go
genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(getDefinitions, namer)
```

5. 配置 v3：

```go
genericConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(getDefinitions, namer)
```

你先不要纠结每个 API 名。

先记住一句话：

**OpenAPI 不是单独再起一个服务，而是直接挂进 generic apiserver 的配置里。**

## 8. 为什么是在 `config.go`，不是在 `server.go`

因为这更符合当前这层代码的职责：

- `config.go` 负责组装 generic apiserver 运行时配置
- `server.go` 负责用完成后的 config 真正创建服务

OpenAPI 属于“运行时配置能力”，所以放在 `config.go` 更合理。

如果放到 `server.go`，代码职责就会开始混。

## 9. 为什么要同时接 v2 和 v3

因为当前 `generic apiserver` 已经提供了这两套 OpenAPI 能力。

对我们现在来说，同时接的价值是：

- 更完整
- 更接近正式 apiserver
- 后面工具适配空间更大

而且实现成本不高。

所以这里没有必要只接一个。

## 10. 为什么这一步不单独加新顶层目录

因为 OpenAPI 在这里不是一个新的业务域。

它只是：

- `pkg/apis/...` 的一类生成物
- `generic apiserver` 的一项能力配置

所以当前目录关系已经够用了：

- `pkg/apis/...`：源
- `pkg/generated/openapi/...`：生成物
- `internal/controlplane/apiserver/config.go`：接线点

这比额外加顶层 `openapi/` 目录更清楚。

## 11. 哪些是手写的，哪些是生成的

### 手写的

- `pkg/apis/*/v1alpha1/doc.go`
- `tools/hack/generate-apis.sh`
- `internal/controlplane/apiserver/config.go`

### 自动生成的

- `pkg/apis/*/v1alpha1/zz_generated.model_name.go`
- `pkg/generated/openapi/zz_generated.openapi.go`

## 12. 现在这套做法为什么合理

因为它满足了 4 个条件：

1. 贴 Kubernetes/OneX 的生成思路
2. 不引入额外复杂目录
3. 生成物和手写代码边界清楚
4. 对小白来说链路还能讲得清楚

## 13. 读完这篇，你应该能回答什么

你现在应该能回答：

1. OpenAPI 的源头为什么在 `pkg/apis/...`
2. `openapi-gen` 为什么由 `generate-apis.sh` 统一调用
3. `zz_generated.openapi.go` 为什么必须生成而不是手写
4. `zz_generated.model_name.go` 为什么不是多余文件
5. OpenAPI 为什么挂在 `config.go` 里而不是单独起服务
6. 为什么当前目录设计不需要单独加一个顶层 `openapi/`
