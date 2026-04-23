# 02 生成文件逐个解释

这篇只回答一个问题：

**这些 `zz_generated` 和 `pkg/generated` 文件到底是什么。**

## 1. `zz_generated.deepcopy.go`

位置：

```text
pkg/apis/gateway/v1alpha1/zz_generated.deepcopy.go
pkg/apis/policy/v1alpha1/zz_generated.deepcopy.go
```

生成工具：

```text
deepcopy-gen
```

它生成：

- `DeepCopyInto`
- `DeepCopy`
- `DeepCopyObject`

什么时候用？

apiserver、client-go、cache、watch 都可能用。

你通常不直接调用。

但它必须存在。

## 2. `zz_generated.defaults.go`

位置：

```text
pkg/apis/gateway/v1alpha1/zz_generated.defaults.go
pkg/apis/policy/v1alpha1/zz_generated.defaults.go
```

生成工具：

```text
defaulter-gen
```

它把手写默认值函数接入统一入口。

手写部分在：

```text
pkg/apis/gateway/v1alpha1/defaults.go
pkg/apis/policy/v1alpha1/defaults.go
```

也就是说：

- 默认值规则是人写的
- 默认值注册和递归调用是生成的

## 3. `zz_generated.model_name.go`

位置：

```text
pkg/apis/gateway/v1alpha1/zz_generated.model_name.go
pkg/apis/policy/v1alpha1/zz_generated.model_name.go
```

生成工具：

```text
openapi-gen
```

它给类型生成类似能力：

```go
func (in Gateway) OpenAPIModelName() string
```

为什么需要？

因为 OpenAPI schema 不是只要“字段描述”。

generic apiserver 还要知道这个 schema 对应哪个 Go 类型、哪个 GVK。

`model_name` 就是这个关联的一部分。

## 4. `pkg/generated/openapi/zz_generated.openapi.go`

生成工具：

```text
openapi-gen
```

作用：

- 生成 OpenAPI definitions
- 供 apiserver 暴露 `/openapi/v2`
- 供 apiserver 暴露 `/openapi/v3`
- 供字段管理构建 type converter

这个文件通常很长。

不要从它开始读。

应该先读 `pkg/apis/...` 的类型定义。

## 5. `pkg/generated/clientset/...`

生成工具：

```text
client-gen
```

作用：

- 给每个 group/version 生成客户端入口
- 给每个资源生成 CRUD/list/watch 方法

以后 controller-manager 可能会这样用：

```go
client.GatewayV1alpha1().Gateways().List(ctx, opts)
```

## 6. `pkg/generated/informers/...`

生成工具：

```text
informer-gen
```

作用：

- 生成 informer factory
- 生成资源 informer
- 封装 list/watch/cache

controller-manager 后续会依赖 informer 来 watch apiserver。

## 7. `pkg/generated/listers/...`

生成工具：

```text
lister-gen
```

作用：

- 从 informer cache 里查对象
- 给 controller 提供只读查询接口

lister 通常和 informer 一起使用。

## 8. `pkg/generated/proto/...`

生成工具：

```text
protoc
protoc-gen-go
protoc-gen-go-grpc
```

作用：

- 生成 protobuf message 的 Go 结构
- 生成 gRPC client/server 接口

它和 `pkg/generated/clientset` 不是一类东西。

`clientset` 访问 apiserver 资源 API。

`proto` 服务于组件间 gRPC 通信。

## 9. 为什么生成文件都带 `DO NOT EDIT`

因为生成物的源头不是它自己。

如果你手改生成物：

1. 下一次生成会覆盖
2. 别人不知道你的改动源头是什么
3. CI 可能因为生成物不一致失败
4. 代码语义会变得不可追踪

正确做法是：

- 改 API 类型
- 改 marker
- 改默认值源函数
- 改生成脚本
- 重新生成
