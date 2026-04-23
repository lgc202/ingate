# 04 registry、strategy、storage 怎么理解

第一次看 Kubernetes apiserver，最容易被这三个词绕晕：

```text
registry
strategy
storage
```

这篇只讲当前项目里它们怎么分工。

## 1. 最短结论

### registry

负责把资源组织起来，交给 apiserver 安装。

### strategy

负责资源语义。

比如：

- 创建前清理哪些字段
- 更新时哪些字段不能改
- generation 什么时候加一
- create/update/status 怎么校验

### storage

负责把资源接到底层通用 store。

也就是把 REST 行为最终接到 etcd。

## 2. 为什么不直接写 handler

普通 HTTP 服务可能会这样写：

```go
router.POST("/gateways", createGateway)
router.GET("/gateways/:name", getGateway)
```

Kubernetes apiserver 不这样做。

它更像这样：

```text
定义资源类型
-> 实现 strategy
-> 创建 REST storage
-> 安装 APIGroup
-> generic apiserver 自动暴露 REST 行为
```

为什么？

因为 Kubernetes 资源有很多通用语义：

- list
- watch
- resourceVersion
- status 子资源
- deletionTimestamp
- finalizers
- managedFields
- admission
- defaulting
- validation
- OpenAPI

这些都不应该每个资源手写一遍。

## 3. strategy 负责什么

strategy 是资源语义的集中点。

它通常回答：

- 这个资源是不是 namespace scoped
- 创建时哪些字段由系统填
- 用户能不能改 status
- status 更新能不能改 spec
- 更新时怎么比较 spec 是否变化
- 对象是否合法

当前项目里，strategy 在：

```text
internal/controlplane/apiserver/registry/gateway/...
internal/controlplane/apiserver/registry/policy/...
```

## 4. storage 负责什么

storage 会把资源接到 generic registry store。

你可以先理解成：

```text
storage = 资源 REST 行为和 etcd 存储之间的适配层
```

它不是直接手写 etcd put/get。

它会复用 Kubernetes apiserver 的 store 机制。

这样才能获得：

- create
- get
- list
- watch
- delete
- update
- status update
- resourceVersion
- optimistic concurrency

## 5. registry 负责什么

registry 更偏组织和安装。

它会把多个资源的 storage 放到 APIGroup 里。

例如 gateway 组里有：

- gateways
- routes
- backends

policy 组里有：

- authpolicies
- trafficpolicies

最终安装成：

```text
/apis/gateway.ingate.io/v1alpha1/gateways
/apis/gateway.ingate.io/v1alpha1/routes
/apis/gateway.ingate.io/v1alpha1/backends
/apis/policy.ingate.io/v1alpha1/authpolicies
/apis/policy.ingate.io/v1alpha1/trafficpolicies
```

## 6. 为什么 status 要单独处理

status 子资源的核心规则是：

- 普通 update 主要改 spec
- status update 主要改 status

这样可以避免用户和 controller 互相覆盖。

比如用户改 `spec.listeners`。

controller 回写 `status.conditions`。

这两件事应该互不干扰。

## 7. 为什么 generation 重要

`metadata.generation` 通常表示 spec 变化次数。

当用户改 spec 时，generation 增加。

controller 可以用它判断：

```text
我当前 status 反映的是不是最新 spec
```

这就是为什么 strategy 里会处理 generation。

## 8. 怎么读这层代码

建议顺序：

1. 先看 API 类型
2. 再看 validation
3. 再看 strategy
4. 再看 storage
5. 再看 REST storage provider
6. 最后看 APIGroup install

不要一开始就扎进 storage 细节。

先知道资源语义，再看它怎么接到底层。
