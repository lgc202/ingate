# 01 代码生成工具分别做什么

Kubernetes 风格项目里，代码生成不是锦上添花。

它是基础工程能力。

## 1. 当前生成入口

总入口：

```bash
make generate
```

分入口：

```bash
make generate-apis
make generate-clients
make generate-proto
```

校验入口：

```bash
make verify-generated
```

对应脚本：

```text
tools/hack/generate-all.sh
tools/hack/generate-apis.sh
tools/hack/generate-clients.sh
tools/hack/generate-proto.sh
tools/hack/verify-generated.sh
```

## 2. `deepcopy-gen`

输入：

```text
pkg/apis/gateway/v1alpha1
pkg/apis/policy/v1alpha1
```

输出：

```text
pkg/apis/gateway/v1alpha1/zz_generated.deepcopy.go
pkg/apis/policy/v1alpha1/zz_generated.deepcopy.go
```

作用：

- 生成 `DeepCopyInto`
- 生成 `DeepCopy`
- 给资源对象生成 `DeepCopyObject`

为什么重要？

因为 apiserver 和 informer cache 经常需要复制对象。

浅拷贝会让多个逻辑共享同一份 slice/map/pointer，容易产生隐蔽 bug。

## 3. `defaulter-gen`

输入：

```text
pkg/apis/gateway/v1alpha1/defaults.go
pkg/apis/policy/v1alpha1/defaults.go
```

输出：

```text
pkg/apis/gateway/v1alpha1/zz_generated.defaults.go
pkg/apis/policy/v1alpha1/zz_generated.defaults.go
```

作用：

- 把手写默认值函数注册成统一 defaulter
- 让 apiserver 在对象进入存储前补默认值

为什么不全手写？

因为每个类型如何递归调用默认值函数有固定模式。

让生成器处理更稳定。

## 4. `client-gen`

输入：

```text
pkg/apis/gateway/v1alpha1
pkg/apis/policy/v1alpha1
```

输出：

```text
pkg/generated/clientset/...
```

作用：

- 生成版本化客户端
- 生成资源 CRUD 方法
- 生成 list/watch 方法

后续 controller-manager 会使用这类 client 去访问 apiserver。

## 5. `lister-gen`

输出：

```text
pkg/generated/listers/...
```

作用：

- 给 informer cache 生成只读查询封装
- 让 controller 从本地缓存读对象

为什么 controller 不总是直接请求 apiserver？

因为控制器需要高频读资源。

直接打 apiserver 会增加压力，也会让调谐逻辑变慢。

## 6. `informer-gen`

输出：

```text
pkg/generated/informers/...
```

作用：

- 生成 watch/list/cache 机制的封装
- 给 controller 提供事件驱动入口

你可以先这样理解：

```text
informer = list + watch + 本地缓存 + 事件回调
```

## 7. `openapi-gen`

输出两类文件：

```text
pkg/apis/**/zz_generated.model_name.go
pkg/generated/openapi/zz_generated.openapi.go
```

作用：

- 生成 OpenAPI schema
- 生成 OpenAPI model name
- 让 generic apiserver 暴露 `/openapi/v2` 和 `/openapi/v3`
- 让字段管理能把 GVK 和 schema 对起来

## 8. `protoc`

输入：

```text
proto/...
```

输出：

```text
pkg/generated/proto/...
```

作用：

- 生成 protobuf Go 类型
- 生成 gRPC client/server 接口

注意：

`proto/` 是组件间通信契约，不等于 `pkg/apis/` 的资源 API。

资源 API 面向 apiserver。

proto 面向组件间调用。

## 9. 为什么工具版本要固定

当前脚本会从 `go.mod` 和常量里取工具版本。

这样做是为了避免：

- 你本机装了一个版本
- CI 装了另一个版本
- 生成结果不一致

成熟工程里，生成工具版本漂移是很常见的坑。
