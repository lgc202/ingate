# admin-api 学习入口

`ingate-admin-api` 是面向用户、控制台、外部系统的产品 API 层。

它不直接操作 etcd。

当前链路是：

```text
HTTP JSON
-> Gin middleware
-> Gin handler
-> handler/dto
-> biz
-> convert
-> generated clientset
-> ingate-apiserver
-> etcd
```

## 当前实现到了什么

当前已经实现五类资源的完整资源管理接口：

```text
GET    /healthz
GET    /readyz

POST   /admin/v1/gateways
GET    /admin/v1/gateways
GET    /admin/v1/gateways/:name
PUT    /admin/v1/gateways/:name
DELETE /admin/v1/gateways/:name
GET    /admin/v1/gateways/:name/topology

POST   /admin/v1/backends
GET    /admin/v1/backends
GET    /admin/v1/backends/:name
PUT    /admin/v1/backends/:name
DELETE /admin/v1/backends/:name

POST   /admin/v1/routes
GET    /admin/v1/routes
GET    /admin/v1/routes/:name
PUT    /admin/v1/routes/:name
DELETE /admin/v1/routes/:name
GET    /admin/v1/routes/:name/effective-status

POST   /admin/v1/auth-policies
GET    /admin/v1/auth-policies
GET    /admin/v1/auth-policies/:name
PUT    /admin/v1/auth-policies/:name
DELETE /admin/v1/auth-policies/:name

POST   /admin/v1/traffic-policies
GET    /admin/v1/traffic-policies
GET    /admin/v1/traffic-policies/:name
PUT    /admin/v1/traffic-policies/:name
DELETE /admin/v1/traffic-policies/:name
```

这些接口证明：

- admin-api 可以接收产品化 JSON
- DTO 可以转换成底层资源对象
- generated clientset 可以访问 ingate-apiserver
- 资源最终会进入 apiserver/etcd
- admin-api 不需要绕过 apiserver 直接碰 etcd
- admin-api 可以做业务聚合，而不是只做 apiserver 代理

## 为什么用 Gin

当前 admin-api 是普通 HTTP JSON 产品接口。

用 Gin 的原因是：

- 路由声明清晰
- JSON binding 直接
- middleware 简单
- handler 写法直接
- 比手写 `net/http` 路由更适合后续扩展产品接口

## 认证方式

`/healthz` 和 `/readyz` 是公开接口。

`/admin/v1/*` 需要 Bearer Token：

```bash
curl -H 'Authorization: Bearer ingate-dev-admin-api-token' \
  http://127.0.0.1:18080/admin/v1/gateways
```

默认 token：

```text
ingate-dev-admin-api-token
```

可以通过启动参数覆盖：

```bash
ingate-admin-api --admin-token=your-token
```

## 目录职责

```text
internal/adminapi/app
```

启动命令入口。

```text
internal/adminapi/app/options
```

命令行参数。

```text
internal/adminapi/config
```

运行配置。

```text
internal/adminapi/server
```

Gin server、middleware、路由注册。

```text
internal/adminapi/handler
```

HTTP handler，只处理请求绑定、调用 biz、返回响应。

```text
internal/adminapi/handler/dto
```

HTTP JSON 请求/响应结构。

```text
internal/adminapi/biz
```

产品工作流编排。

这里已经开始有业务聚合逻辑，比如 topology 和 effective-status。

```text
internal/adminapi/convert
```

DTO 和 `pkg/apis/...` 资源对象互转。

```text
internal/adminapi/store
```

通过 generated clientset 访问 ingate-apiserver。

## 怎么构建

```bash
make build-admin-api
```

## 怎么运行

先运行 apiserver：

```bash
make run-apiserver
```

再运行 admin-api：

```bash
make run-admin-api
```

默认配置：

```text
admin-api:       http://127.0.0.1:18080
ingate-apiserver: https://127.0.0.1:18443
apiserver token: ingate-dev-admin-token
admin-api token: ingate-dev-admin-api-token
```

## 怎么自动验证

```bash
make verify-admin-api
```

这个脚本会：

1. 构建 apiserver
2. 构建 admin-api
3. 临时启动 apiserver
4. 临时启动 admin-api
5. 验证未带 admin token 的请求返回 401
6. 通过 admin-api 创建五类资源
7. 通过 admin-api 列表查询五类资源
8. 通过 admin-api 更新五类资源
9. 查询 gateway topology
10. 查询 route effective-status
11. 直接查询 apiserver，确认资源真的创建成功
12. 通过 admin-api 删除五类资源
13. 验证删除后查询返回 404

预期输出：

```text
ADMIN_API_HEALTHZ=ok
ADMIN_API_UNAUTH_CODE=401
ADMIN_API_GATEWAY_CREATE_CODE=201
ADMIN_API_BACKEND_CREATE_CODE=201
ADMIN_API_ROUTE_CREATE_CODE=201
ADMIN_API_AUTH_POLICY_CREATE_CODE=201
ADMIN_API_TRAFFIC_POLICY_CREATE_CODE=201
ADMIN_API_UPDATE_VERIFY=yes
ADMIN_API_TOPOLOGY_VERIFY=yes
ADMIN_API_DELETE_VERIFY=yes
ADMIN_API_APISERVER_RESOURCE_VERIFY=yes
```

## 当前还没做什么

暂时还没有做：

- 多用户登录系统
- RBAC 权限模型
- 多租户隔离
- 请求幂等键
- audit
- OpenAPI/Swagger 输出
- 前端控制台适配

这些是后续平台化阶段的内容，不属于当前阶段的 admin-api 完整闭环。

## 源码阅读文档

如果你准备开始读 admin-api 代码，按这个目录的顺序读：

```text
docs/superpowers/learning/admin-api/reading/
```

入口：

```text
docs/superpowers/learning/admin-api/reading/README.md
```
