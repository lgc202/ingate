# 01. admin-api 整体图

这一篇看全局。先知道每个目录负责什么，再去读单个文件。

## admin-api 是什么

`ingate-admin-api` 是产品 API 层。

它给用户、控制台、外部系统提供 HTTP JSON 接口。

它不是 Kubernetes-style apiserver。它也不是 etcd client。

它的位置是：

```text
用户 / 控制台 / 外部系统
-> admin-api
-> ingate-apiserver
-> etcd
```

## 为什么需要 admin-api

`ingate-apiserver` 暴露的是 Kubernetes 风格资源 API，比如：

```text
/apis/gateway.ingate.io/v1alpha1/gateways
/apis/gateway.ingate.io/v1alpha1/routes
```

这种接口适合 controller、kubectl、client-go。

但是产品侧更希望看到：

```text
/admin/v1/gateways
/admin/v1/gateways/:name/topology
/admin/v1/routes/:name/effective-status
```

也就是说，admin-api 的价值是：

- 把底层资源 API 包成产品 API
- 做权限、审计、幂等、聚合等产品逻辑
- 屏蔽底层 Kubernetes API 细节
- 给前端或外部用户一个更稳定的入口

当前阶段先实现了资源管理闭环和两个聚合查询。

## 一次请求的总流程

以创建 Gateway 为例：

```text
POST /admin/v1/gateways
-> Gin router
-> request-id middleware
-> bearer-token middleware
-> GatewayHandler.Create
-> dto.CreateGatewayRequest
-> GatewayService.Create
-> convert.GatewayFromCreateRequest
-> Store.CreateGateway
-> generated clientset GatewayV1alpha1().Gateways().Create
-> ingate-apiserver
-> etcd
```

每一层都有边界。

## 目录职责

### `cmd/admin-api`

进程入口。

它只负责启动命令，真正逻辑不写在这里。

### `internal/adminapi/app`

命令行程序组装。

主要文件：

```text
internal/adminapi/app/server.go
```

它负责：

- 创建 cobra command
- 解析参数
- 校验参数
- 调用 server 运行

### `internal/adminapi/app/options`

命令行参数定义。

比如：

```text
--bind-address
--port
--apiserver-address
--apiserver-token
--apiserver-insecure-skip-tls-verify
--admin-token
```

为什么单独放 options？

因为命令行参数是“输入”，不是运行时业务逻辑。单独放便于后面支持配置文件、环境变量、默认值覆盖。

### `internal/adminapi/config`

运行配置。

options 是“命令行输入”，config 是“程序内部最终使用的配置”。

现在两者很像，但后面会变得不一样。比如配置文件、环境变量、默认值合并后，最终都应该变成 config。

### `internal/adminapi/server`

HTTP server、middleware、路由注册。

主要负责：

- 创建 Gin router
- 注册 `/healthz`、`/readyz`
- 注册 `/admin/v1/*`
- 给 `/admin/v1/*` 加认证 middleware
- 创建 `http.Server`

它不应该写具体业务逻辑。

### `internal/adminapi/handler`

HTTP handler。

handler 只做三件事：

1. 绑定请求 JSON
2. 调用 biz service
3. 写 HTTP 响应

handler 不应该知道 apiserver clientset 怎么用，也不应该直接拼底层资源对象。

### `internal/adminapi/handler/dto`

HTTP JSON 的请求和响应结构。

DTO 是 Data Transfer Object，意思是“接口传输对象”。

为什么不用 `pkg/apis/...` 里的结构直接作为 HTTP 请求？

因为 `pkg/apis/...` 是底层资源模型，里面有 Kubernetes API Machinery 的概念，比如：

- `TypeMeta`
- `ObjectMeta`
- `ResourceVersion`
- `Status`
- `metav1.Condition`

这些对产品用户不友好。admin-api 应该暴露更简单的 JSON。

### `internal/adminapi/biz`

业务编排。

现在很多 biz 方法还比较薄，这是正常的。

比如 create 只是：

```text
DTO -> resource -> store.Create -> response
```

但 update、topology、effective-status 已经开始体现 biz 层价值。

后面会放：

- 权限判断
- 幂等逻辑
- 业务校验
- 资源组合
- 状态聚合
- 审计埋点

### `internal/adminapi/convert`

DTO 和底层资源对象互转。

它负责把：

```text
dto.CreateGatewayRequest
```

转换成：

```text
pkg/apis/gateway/v1alpha1.Gateway
```

也负责把底层资源对象转换回响应 DTO。

为什么不放在 handler 或 biz？

因为转换代码很细、很机械。如果混在 handler 或 biz 里，业务代码会变脏。

### `internal/adminapi/store`

存储访问层。

但它不直接访问 etcd。

它通过 generated clientset 访问 ingate-apiserver：

```text
client.GatewayV1alpha1().Gateways().Create(...)
client.PolicyV1alpha1().AuthPolicies().Update(...)
```

为什么叫 store？

因为从 admin-api 视角看，它是资源存取接口。底层现在是 apiserver，将来也可以封装缓存、重试、限流、监控。

## 手写代码和生成代码

admin-api 这部分基本都是手写代码。

它使用的 generated clientset 是生成代码，位置在：

```text
pkg/generated/clientset/versioned
pkg/generated/informers/externalversions
pkg/generated/listers
```

admin-api 当前直接用的是：

```text
pkg/generated/clientset/versioned
```

这些 generated clientset 不是手写的，是根据 `pkg/apis/...` 资源类型生成的。

## 当前阶段为什么没有 informer

admin-api 是请求响应模型。

它收到 HTTP 请求后，直接调用 apiserver。

informer 更适合 controller-manager，因为 controller 需要持续 watch 资源变化。

所以当前阶段：

```text
admin-api: request -> clientset -> apiserver
controller-manager: informer -> watch -> workqueue -> reconcile
```
