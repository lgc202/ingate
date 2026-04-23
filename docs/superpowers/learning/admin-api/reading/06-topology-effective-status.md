# 06. topology 和 effective-status

这一篇讲 admin-api 为什么不只是 CRUD 壳。

## 为什么需要聚合接口

如果只有 CRUD，前端想展示一个 Gateway 下面的完整关系，需要自己调用很多接口：

```text
GET gateway
GET routes
GET backends
GET auth-policies
GET traffic-policies
```

然后前端自己判断：

- 哪些 route 挂在这个 gateway 上
- 哪些 backend 被 route 引用
- 哪些 policy 影响这个 gateway 或 route
- 哪些引用找不到

这会让前端理解太多底层资源关系。

admin-api 应该承担这类产品聚合。

## Gateway topology 是什么

接口：

```text
GET /admin/v1/gateways/:name/topology
```

返回内容包括：

- gateway 自己
- 挂在这个 gateway 下的 routes
- 这些 routes 引用的 backends
- 影响 gateway/routes 的 AuthPolicy
- 影响 gateway/routes/backends 的 TrafficPolicy
- 找不到的引用

它回答的问题是：

```text
这个 Gateway 背后到底连着哪些东西？
```

## Route effective-status 是什么

接口：

```text
GET /admin/v1/routes/:name/effective-status
```

返回内容包括：

- route 自己
- route 绑定的 gateways
- route 引用的 backends
- 影响 route/gateway 的 AuthPolicy
- 影响 route/gateway/backend 的 TrafficPolicy
- route 当前 conditions
- 找不到的引用

它回答的问题是：

```text
这个 Route 当前受哪些资源影响？
```

## 代码在哪里

业务逻辑在：

```text
internal/adminapi/biz/topology.go
```

HTTP handler 在：

```text
internal/adminapi/handler/topology.go
```

响应 DTO 在：

```text
internal/adminapi/handler/dto/topology.go
```

路由在：

```text
internal/adminapi/server/routes.go
```

## Gateway topology 代码路径

流程是：

```text
GetGatewayTopology(name)
-> store.GetGateway(name)
-> store.ListRoutes()
-> store.ListAuthPolicies()
-> store.ListTrafficPolicies()
-> filterRoutesByGateway(routes, name)
-> backendNamesFromRoutes(routes)
-> getBackends(backendNames)
-> filterAuthPolicies(...)
-> filterTrafficPolicies(...)
-> 组装 GatewayTopologyResponse
```

重点是：它会从 Route 的 `parentRefs` 找出挂在 Gateway 下的 route。

```text
Route.spec.parentRefs[].name == gatewayName
```

然后从 Route 的 backendRefs 找 backend：

```text
Route.spec.rules[].backendRefs[].name
```

## Route effective-status 代码路径

流程是：

```text
GetRouteEffectiveStatus(name)
-> store.GetRoute(name)
-> gatewayNamesFromRoute(route)
-> getGateways(gatewayNames)
-> backendNamesFromRoute(route)
-> getBackends(backendNames)
-> store.ListAuthPolicies()
-> store.ListTrafficPolicies()
-> filterAuthPolicies(...)
-> filterTrafficPolicies(...)
-> 组装 RouteEffectiveStatusResponse
```

重点是：它以 route 为中心，把会影响 route 的资源找出来。

## policy 怎么匹配

AuthPolicy 现在支持目标：

```text
Gateway
Route
```

TrafficPolicy 现在支持目标：

```text
Gateway
Route
Backend
```

所以 topology 会按 `targetRefs` 判断 policy 是否相关。

例如：

```json
{
  "targetRefs": [
    {"kind": "Route", "name": "route-a"}
  ]
}
```

如果 topology 里包含 `route-a`，这个 policy 就会出现在结果里。

## unresolvedRefs 是什么

如果 Route 引用了一个不存在的 Backend，admin-api 不应该直接失败。

因为 topology 的目的就是展示当前拓扑，包括坏引用。

所以返回里有：

```json
{
  "kind": "Backend",
  "name": "missing-backend",
  "reason": "NotFound"
}
```

这比直接 `500` 更有业务价值。

## 当前实现的限制

当前聚合是实时 list/get。

优点：

- 简单
- 易懂
- 不需要缓存
- 不需要 informer

缺点：

- 资源很多时性能一般
- 没有分页
- 没有缓存
- 没有复杂条件推导

后面如果资源规模变大，可以考虑：

- admin-api 内部缓存
- controller 预计算拓扑
- 通过 label/index 优化查询
- 使用 informer/lister 做本地只读缓存

当前阶段先保持直接、可读、可验证。
