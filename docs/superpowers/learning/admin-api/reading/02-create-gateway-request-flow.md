# 02. 创建 Gateway 的完整代码路径

这一篇只读一条路径：创建 Gateway。

请求是：

```text
POST /admin/v1/gateways
```

## 第一步：路由匹配

路由在：

```text
internal/adminapi/server/routes.go
```

相关注册是：

```go
adminV1.POST(gatewaysPath, handlers.gateway.Create)
```

这表示：

```text
POST /admin/v1/gateways -> GatewayHandler.Create
```

为什么路由不直接写业务？

因为路由只负责“URL 到 handler 的映射”。如果把业务写在路由文件里，文件会很快变成大杂烩。

## 第二步：middleware

请求进入 handler 前，会先过 middleware。

当前有：

```text
requestIDMiddleware
bearerTokenAuthMiddleware
```

`requestIDMiddleware` 会保证响应带 `X-Request-ID`。

`bearerTokenAuthMiddleware` 会检查：

```text
Authorization: Bearer ingate-dev-admin-api-token
```

如果没带 token，请求不会进入 handler，直接返回 `401`。

## 第三步：handler 绑定 JSON

文件：

```text
internal/adminapi/handler/gateway.go
```

核心流程是：

```go
var req dto.CreateGatewayRequest
if err := c.ShouldBindJSON(&req); err != nil {
    writeBindError(c, err)
    return
}
```

`ShouldBindJSON` 做两件事：

1. 把 HTTP body 里的 JSON 解析到 Go struct
2. 根据 `binding` tag 做基础校验

对应 DTO：

```text
internal/adminapi/handler/dto/gateway.go
```

例如：

```go
Name string `json:"name" binding:"required"`
```

意思是：

- JSON 字段叫 `name`
- 这个字段必填

如果用户没传 `name`，handler 会返回 `400 BadRequest`。

## 第四步：handler 调 biz

handler 不自己创建底层资源对象，而是调用：

```go
resp, err := h.service.Create(c.Request.Context(), req)
```

这里的 `h.service` 是：

```text
*biz.GatewayService
```

为什么 handler 不直接调 store？

因为 handler 是 HTTP 层。它应该只理解 HTTP 请求和响应。

业务规则应该放在 biz 层。现在 create 很薄，但后面如果加：

- 名称规范
- 权限检查
- 审计
- 幂等
- 默认值补充

都应该在 biz 层做，而不是让 handler 变复杂。

## 第五步：biz 做编排

文件：

```text
internal/adminapi/biz/gateway.go
```

创建 Gateway 的流程是：

```go
created, err := s.store.CreateGateway(ctx, convert.GatewayFromCreateRequest(req))
```

这行包含两个动作：

```text
DTO -> 底层 Gateway 资源
底层 Gateway 资源 -> apiserver
```

## 第六步：convert 转底层资源对象

文件：

```text
internal/adminapi/convert/gateway.go
```

`GatewayFromCreateRequest` 会构造：

```go
&gatewayv1alpha1.Gateway{
    TypeMeta: metav1.TypeMeta{
        APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
        Kind:       "Gateway",
    },
    ObjectMeta: metav1.ObjectMeta{Name: req.Name},
    Spec: gatewayv1alpha1.GatewaySpec{...},
}
```

这里开始进入 Kubernetes API Machinery 的世界。

### `APIVersion` 是什么

`APIVersion` 表示资源属于哪个 API 版本。

当前 Gateway 是：

```text
gateway.ingate.io/v1alpha1
```

### `Kind` 是什么

`Kind` 表示资源类型。

这里是：

```text
Gateway
```

### `ObjectMeta` 是什么

`ObjectMeta` 是 Kubernetes 对象通用元数据。

里面有：

- `name`
- `resourceVersion`
- `generation`
- `labels`
- `annotations`
- `creationTimestamp`

创建时只需要 name。其他字段由 apiserver 填。

### `Spec` 是什么

`Spec` 是用户期望状态。

例如 Gateway 的监听端口、协议、域名。

## 第七步：store 调 generated clientset

文件：

```text
internal/adminapi/store/apiserver.go
```

创建 Gateway 的代码是：

```go
return s.client.GatewayV1alpha1().Gateways().Create(ctx, gateway, createOptions)
```

这不是手写 HTTP 请求。

这里用的是 Kubernetes code-generator 生成的 clientset。

它帮我们封装了：

- 请求路径
- JSON 序列化
- REST 调用
- 返回对象反序列化
- Kubernetes API 参数编码

最终访问的是：

```text
POST /apis/gateway.ingate.io/v1alpha1/gateways
```

## 第八步：ingate-apiserver 处理资源

请求到达 ingate-apiserver 后，会进入 apiserver 的 registry/storage/strategy。

那部分在 apiserver 阅读文档里讲。

admin-api 只需要知道：

```text
admin-api 不写 etcd
admin-api 通过 apiserver 写资源
apiserver 负责校验、默认值、存储、watch、status 等 Kubernetes 语义
```

## 第九步：响应返回

apiserver 返回创建后的 Gateway 对象。

biz 调用：

```go
convert.GatewayToResponse(created)
```

把底层资源对象转换成产品响应 DTO。

handler 最后：

```go
c.JSON(http.StatusCreated, resp)
```

返回 HTTP `201 Created`。

## 总结

完整链路是：

```text
routes.go
-> middleware.go
-> handler/gateway.go
-> handler/dto/gateway.go
-> biz/gateway.go
-> convert/gateway.go
-> store/apiserver.go
-> pkg/generated/clientset/versioned
-> ingate-apiserver
-> etcd
```

读 admin-api 时，一定要按这个顺序读。不要一开始就跳进 generated clientset，否则会迷路。
