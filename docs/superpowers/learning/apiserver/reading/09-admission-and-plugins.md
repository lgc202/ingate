# 09 admission 插件是怎么接进去的

这篇解释当前 admission 这块最重要的几个点。

## 1. 当前 admission 在哪里

这次 admission 相关代码主要在：
- `internal/controlplane/apiserver/options/plugins.go`
- `internal/controlplane/apiserver/admission/plugin/reservedmetadata/admission.go`
- `internal/controlplane/apiserver/config.go`
- `tools/hack/verify-apiserver-admission.sh`

## 2. `options/plugins.go` 做了什么

这里集中管理三件事：
1. 默认插件顺序
2. 所有 admission 插件注册入口
3. 根据 `enable/disable` 计算最终启用的插件列表

为什么要单独有这个文件？

因为这和 OneX 的思路一样：
- admission 插件本身是一个体系
- 不应该把注册和排序逻辑散进 `options.go` 或 `config.go`

## 3. 为什么这次不用 `AdmissionOptions.ApplyTo`

这是这次最关键的实现取舍。

Kubernetes 当前版本里，`AdmissionOptions.ApplyTo(...)` 会顺带接一层更通用的 admission initializer。那一层默认依赖：
- Kubernetes core shared informer
- Kubernetes core client
- dynamic client

但我们当前这个 `ingate-apiserver` 还没有接那套 core API 依赖。

所以如果强行走 `ApplyTo(...)`，会把当前阶段拉进一堆我们还不需要的前提里。

因此现在采用的是更小的接法：
- 继续使用 `AdmissionOptions` 保存 flags 和插件集合
- 但真正组链时，直接调用 `Plugins.NewFromPlugins(...)`
- 只加载我们自己的插件

这样做的结果是：
- 结构仍然正规
- 但不会被 Kubernetes 更重的通用 admission 初始化器绑住

## 4. 当前 plugin 做了什么

当前启用的 plugin 是：
- `IngateReservedMetadata`

它的规则很简单：
- 检查对象上的 labels 和 annotations
- 如果 key 以 `internal.ingate.io/` 开头
- 就拒绝请求

为什么选择这个规则？

因为它满足 4 个要求：
1. 规则真实有价值
2. 行为稳定
3. 容易验证
4. 容易给小白讲清楚

这比一上来就做复杂 webhook 或依赖 informer 的 plugin 更合适当前阶段。

## 5. 这块哪些是手写的

当前 admission 这块全是手写：
- `options/plugins.go`
- `admission/plugin/reservedmetadata/admission.go`
- `verify-apiserver-admission.sh`

为什么没有生成代码？

因为 admission plugin 属于运行时行为，不是 schema、client、proto 这类生成问题。

## 6. 这一步对后面有什么价值

它的价值不是“多了一个小规则”。

更重要的是：
- admission 这一层已经被正式接进 apiserver 主链
- 插件注册和排序方式已经立住
- 后面如果要继续加 validating/mutating/plugin initializer，就有明确落点
