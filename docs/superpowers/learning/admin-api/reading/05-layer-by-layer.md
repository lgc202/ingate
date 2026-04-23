# 05. 分层逐个看

这一篇把 admin-api 每一层拆开看。

## 为什么要分层

如果把所有代码写在一个 handler 里，创建 Gateway 可能会变成：

```text
解析 JSON
校验 token
构造 Gateway 对象
设置 TypeMeta
调用 REST client
处理 Kubernetes 错误
转换响应 JSON
```

这样短期看快，长期会很乱。

现在的分层是：

```text
server -> handler -> dto -> biz -> convert -> store -> generated clientset
```

## server 层

目录：

```text
internal/adminapi/server
```

职责：

- 创建 Gin router
- 注册 middleware
- 注册路由
- 创建 `http.Server`

不负责：

- 解析具体业务 JSON
- 构造 Gateway/Route 对象
- 调用 apiserver
- 业务聚合

## handler 层

目录：

```text
internal/adminapi/handler
```

职责：

- 从 HTTP 请求里拿参数
- `ShouldBindJSON`
- 调 biz
- 写 HTTP status code 和 JSON

典型代码：

```go
var req dto.CreateGatewayRequest
if err := c.ShouldBindJSON(&req); err != nil {
    writeBindError(c, err)
    return
}

resp, err := h.service.Create(c.Request.Context(), req)
if err != nil {
    writeStoreError(c, err)
    return
}
c.JSON(http.StatusCreated, resp)
```

handler 不应该直接 import `pkg/generated/clientset`。

如果 handler 直接使用 clientset，说明分层被破坏了。

## dto 层

目录：

```text
internal/adminapi/handler/dto
```

职责：

- 定义 HTTP 请求结构
- 定义 HTTP 响应结构
- 屏蔽底层 Kubernetes 资源细节

例如 Gateway create 请求是：

```json
{
  "name": "demo",
  "listeners": [
    {
      "name": "web",
      "protocol": "HTTP",
      "port": 80
    }
  ]
}
```

而底层 Kubernetes 资源需要：

```text
apiVersion
kind
metadata
spec
status
```

DTO 让产品 API 更干净。

## biz 层

目录：

```text
internal/adminapi/biz
```

职责：

- 编排业务流程
- 调 convert
- 调 store
- 做聚合查询
- 保留 update 时的 metadata/status

为什么有些 biz 文件看起来只是转调？

因为现在业务还简单。

但 update 已经体现 biz 价值：

```text
Get current
-> build updated object
-> preserve ObjectMeta
-> preserve Status
-> store.Update
```

聚合接口更明显：

```text
Get gateway
-> List routes
-> 找挂在这个 gateway 下的 routes
-> 找 routes 引用的 backends
-> 找影响这些资源的 policies
-> 组合成 topology response
```

这就不是简单转调了。

## convert 层

目录：

```text
internal/adminapi/convert
```

职责：

- DTO -> `pkg/apis/...` 资源对象
- `pkg/apis/...` 资源对象 -> DTO

为什么单独拆？

因为转换代码字段很多，容易污染业务逻辑。

例如：

```go
GatewayFromCreateRequest
GatewayFromUpdateRequest
GatewayToResponse
GatewayListToResponse
```

这类代码不该放在 handler，也不该放在 store。

## store 层

目录：

```text
internal/adminapi/store
```

职责：

- 定义资源存取接口
- 用 generated clientset 调 apiserver

它封装了底层访问方式：

```go
s.client.GatewayV1alpha1().Gateways().Create(...)
s.client.PolicyV1alpha1().AuthPolicies().Update(...)
```

为什么 store 不直接叫 client？

因为 `client` 容易让人以为只是 generated clientset 的薄包装。

从 admin-api 视角，它是资源存储访问接口。以后可以在这里加：

- 统一超时
- 重试
- 指标
- 缓存
- 限流

## generated clientset

目录：

```text
pkg/generated/clientset/versioned
```

这不是手写的。

它来自 Kubernetes code-generator，根据 `pkg/apis/...` 里的资源类型生成。

admin-api 当前用它来访问：

```text
GatewayV1alpha1().Gateways()
GatewayV1alpha1().Backends()
GatewayV1alpha1().Routes()
PolicyV1alpha1().AuthPolicies()
PolicyV1alpha1().TrafficPolicies()
```

## 一句话记忆

```text
server 管 HTTP 框架
handler 管 HTTP 请求响应
dto 管 JSON 形状
biz 管业务编排
convert 管对象转换
store 管资源存取
generated clientset 管 apiserver REST 调用
```
