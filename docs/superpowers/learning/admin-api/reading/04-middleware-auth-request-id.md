# 04. Middleware、认证和 Request ID

这一篇看 Gin middleware。

## middleware 是什么

middleware 是请求进入 handler 前、响应返回前执行的一段逻辑。

它适合放通用能力，比如：

- 日志
- panic recovery
- 认证
- request id
- 跨域
- 限流

当前 admin-api 用了：

```go
router.Use(gin.Logger(), gin.Recovery(), requestIDMiddleware())
```

`/admin/v1/*` 又单独用了：

```go
adminV1.Use(bearerTokenAuthMiddleware(adminToken))
```

## 为什么 healthz 不需要认证

路由是这样注册的：

```text
GET /healthz
GET /readyz
```

然后才创建：

```text
/admin/v1 group
```

认证 middleware 只挂在 `/admin/v1` group 上。

所以：

```text
/healthz 公开
/readyz 公开
/admin/v1/* 需要 token
```

这是故意的。

健康检查通常给 Kubernetes probe 或负载均衡器使用，不应该依赖业务 token。

## Bearer Token 认证

代码在：

```text
internal/adminapi/server/middleware.go
```

用户请求必须带：

```text
Authorization: Bearer ingate-dev-admin-api-token
```

middleware 会检查：

1. header 是否以 `Bearer ` 开头
2. token 是否和配置里的 admin token 相等

如果不通过：

```text
401 Unauthorized
```

返回 JSON：

```json
{
  "code": "Unauthorized",
  "message": "missing or invalid bearer token"
}
```

## 为什么用 ConstantTimeCompare

比较 token 时用了：

```go
subtle.ConstantTimeCompare
```

它是常量时间比较。

普通字符串比较可能在第几个字符不一样时提前返回，理论上会泄露时间差。

这对当前学习项目不是高风险点，但这是安全代码的良好习惯。

## admin token 从哪里来

默认值在：

```text
internal/adminapi/app/options/options.go
```

参数是：

```text
--admin-token
```

默认：

```text
ingate-dev-admin-api-token
```

运行脚本里也有：

```text
ADMIN_API_TOKEN
```

这表示你可以这样启动：

```bash
ADMIN_API_TOKEN=my-token make run-admin-api
```

## request id

每个响应会带：

```text
X-Request-ID
```

如果用户请求里已经带了 `X-Request-ID`，admin-api 会沿用。

如果没带，admin-api 会生成一个：

```text
req-<unix-nano>-<sequence>
```

为什么要 request id？

因为排查问题时需要把日志串起来。

比如用户说“这个请求失败了”，如果能提供 request id，就可以在 admin-api、apiserver、后续 controller 日志里追踪同一次请求。

当前只是把 response header 写出。后面可以继续做：

- Gin logger 打印 request id
- admin-api 调 apiserver 时透传 request id
- controller 日志关联资源和 request id

## 为什么现在不是完整用户系统

当前只是基础 Bearer Token。

不是：

- 用户登录
- JWT
- RBAC
- 多租户
- OAuth2
- OIDC

原因是这些属于另一个大模块。现在先保证 admin-api 有最基本保护，避免裸奔。
