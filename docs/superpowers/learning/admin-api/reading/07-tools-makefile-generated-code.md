# 07. 工具、Makefile 和生成代码

这一篇讲 admin-api 用到的工程化工具。

## admin-api 哪些代码是手写的

这些是手写代码：

```text
cmd/admin-api/main.go
internal/adminapi/app
internal/adminapi/app/options
internal/adminapi/config
internal/adminapi/server
internal/adminapi/handler
internal/adminapi/handler/dto
internal/adminapi/biz
internal/adminapi/convert
internal/adminapi/store
```

这些也是手写脚本和文档：

```text
tools/hack/run-admin-api.sh
tools/hack/verify-admin-api.sh
docs/superpowers/learning/admin-api
```

## admin-api 用到了哪些生成代码

admin-api 自己没有生成 handler、DTO、biz、convert。

但它使用了生成出来的 clientset：

```text
pkg/generated/clientset/versioned
```

例如 store 里调用：

```go
s.client.GatewayV1alpha1().Gateways().Create(...)
s.client.PolicyV1alpha1().TrafficPolicies().Update(...)
```

这些方法来自生成代码，不是手写的。

## generated clientset 从哪里来

生成源头是资源类型定义：

```text
pkg/apis/gateway/v1alpha1
pkg/apis/policy/v1alpha1
```

这些 API 类型上有 code-generator 需要的注释，比如：

```go
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
```

这些注释告诉生成器：

- 这个类型要生成 client
- 这个类型不是 namespace 级资源
- 这个类型要生成 DeepCopyObject

然后脚本生成：

```text
pkg/generated/clientset/versioned
pkg/generated/informers/externalversions
pkg/generated/listers
```

admin-api 当前只直接用 clientset。

controller-manager 后面会用 informer/lister。

## Makefile 相关目标

构建 admin-api：

```bash
make build-admin-api
```

运行 admin-api：

```bash
make run-admin-api
```

验证 admin-api：

```bash
make verify-admin-api
```

全量编译：

```bash
go test ./...
```

校验生成代码是否最新：

```bash
make verify-generated
```

## build-admin-api 做什么

Makefile 目标会调用：

```text
tools/hack/build.sh
```

并指定：

```text
BINS=ingate-admin-api
```

输出到：

```text
_output/<os>_<arch>/ingate-admin-api
```

为什么输出到 `_output`？

因为构建产物不应该和源码混在一起，也不应该提交到仓库。

## run-admin-api.sh 做什么

脚本：

```text
tools/hack/run-admin-api.sh
```

它负责组装启动参数：

```text
--bind-address
--port
--apiserver-address
--apiserver-token
--apiserver-insecure-skip-tls-verify
--admin-token
```

环境变量可以覆盖默认值：

```text
ADMIN_API_TOKEN
APISERVER_TOKEN
APISERVER_ADDRESS
ADMIN_API_PORT
```

这比每次手敲一长串参数更稳定。

## verify-admin-api.sh 做什么

脚本：

```text
tools/hack/verify-admin-api.sh
```

它是端到端验证，不是单元测试。

它会真的启动两个进程：

```text
ingate-apiserver
ingate-admin-api
```

然后真的发 HTTP 请求。

它验证的是系统行为，而不是某个函数。

## 为什么不用 mock

当前阶段我们最关心的是链路是否真的通：

```text
admin-api -> generated clientset -> ingate-apiserver -> etcd
```

mock 只能证明代码调用了某个假接口，不能证明 apiserver 路径、认证、JSON、资源校验、存储都正确。

所以当前最重要的是 `make verify-admin-api`。

后面如果某些 biz 逻辑复杂了，再补单元测试。

## verify-generated 为什么要跑

虽然本次 admin-api 代码主要是手写的，但项目里有很多生成代码。

`make verify-generated` 会重新生成一遍，然后确认生成物没有变化。

如果它失败，说明有人改了 API 类型、proto 或生成配置，但没有提交对应生成代码。

这会导致别人拉代码后构建结果不一致。

## admin-api 后续可能增加的生成物

当前还没有为 admin-api 生成 OpenAPI/Swagger。

后面如果要做对外 API 文档，可以考虑：

- 从 Gin handler 注释生成 Swagger
- 手写 OpenAPI YAML
- 从 proto 定义 HTTP API 再生成文档

但这些都不是当前阶段必须项。

当前最重要的是：

```text
代码结构清楚
链路能跑通
验证脚本可靠
学习路径完整
```
