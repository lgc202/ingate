# 06 日常开发时应该怎么改

这篇是开发流程清单。

你以后改 apiserver 时，先按这里判断要改哪里。

## 1. 改资源字段

例如给 `GatewaySpec` 加字段。

先改：

```text
pkg/apis/gateway/v1alpha1/types_gateway.go
```

然后根据需要改：

```text
pkg/apis/gateway/v1alpha1/defaults.go
pkg/apis/gateway/validation/...
internal/controlplane/apiserver/registry/gateway/table/...
```

最后跑：

```bash
make generate
make verify-generated
go test ./...
```

为什么要跑 generate？

因为字段变化会影响：

- DeepCopy
- OpenAPI
- client 类型使用
- informer/lister 类型引用

## 2. 改默认值

先改：

```text
pkg/apis/<group>/v1alpha1/defaults.go
```

然后跑：

```bash
make generate-apis
make verify-generated
```

为什么默认值也要生成？

因为手写的是具体规则，生成的是注册和调用链。

## 3. 改校验规则

改：

```text
pkg/apis/<group>/validation/...
```

然后跑对应验证脚本或 `go test ./...`。

validation 的职责是拒绝非法对象。

defaulting 的职责是给合法对象补默认值。

这两个不要混。

## 4. 改 REST 行为

如果要改 create/update/status/delete 的语义，通常看：

```text
internal/controlplane/apiserver/registry/<group>/<resource>/...
```

重点看 strategy 和 storage。

不要直接去写普通 HTTP handler。

## 5. 新增资源

新增资源时通常要做这些事：

1. 在 `pkg/apis/<group>/v1alpha1/` 新增类型文件
2. 给顶层资源类型加 `+genclient`
3. 给顶层资源类型和 List 类型加 `runtime.Object` deepcopy marker
4. 在 `register.go` 注册类型
5. 写默认值和校验
6. 写 registry / strategy / storage
7. 安装到 APIGroup
8. 更新 TableConvertor 和 discovery metadata
9. 跑 `make generate`
10. 跑 `make verify-generated`
11. 跑 apiserver 验证脚本
12. 更新文档

这就是为什么新增 Kubernetes 风格资源比普通 DTO 更重。

它不是只加一个 struct。

它要进入完整 API machinery。

## 6. 改 proto

改：

```text
proto/...
```

然后跑：

```bash
make generate-proto
make verify-generated
```

注意：

proto 是组件间通信契约。

它不是 apiserver 资源定义。

不要把资源 API 和 gRPC API 混成一类。

## 7. 改构建脚本

优先改：

```text
tools/hack/*.sh
```

顶层 `Makefile` 保持薄一点。

Makefile 负责给人提供入口。

复杂逻辑放脚本里。

这样更接近成熟项目做法，也方便 CI 复用。

## 8. 每次收尾至少跑什么

文档或小改动：

```bash
make help
```

生成链相关改动：

```bash
make verify-generated
```

apiserver 行为改动：

```bash
make verify-apiserver
make verify-apiserver-auth
make verify-apiserver-kubectl
make verify-apiserver-admission
make verify-apiserver-table
go test ./...
```

日志问题复查：

```bash
rg -n "failed to update managedFields|SHOULD NOT HAPPEN|no corresponding type|panic|fatal" _output/*/ingate-apiserver-*.log
```

## 9. 一个重要习惯

不要看到生成代码报错就直接改生成代码。

先问：

- 源类型是不是错了
- marker 是不是漏了
- register.go 是不是漏注册了
- generate 脚本是不是没覆盖这个包
- OpenAPI model name 是不是没生成
- Scheme 里是不是没有这个类型

生成代码只是结果。

真正要改的是源头。
