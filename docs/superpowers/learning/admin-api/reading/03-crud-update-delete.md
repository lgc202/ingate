# 03. CRUD、Update 和 Delete

这一篇重点讲为什么 update 不是直接调用 `Update`，而是先 `Get`。

## 当前 CRUD 接口

每类资源都有：

```text
POST   /admin/v1/<resources>
GET    /admin/v1/<resources>
GET    /admin/v1/<resources>/:name
PUT    /admin/v1/<resources>/:name
DELETE /admin/v1/<resources>/:name
```

当前没有 PATCH。

为什么先不做 PATCH？

因为 PATCH 会引入：

- JSON Merge Patch
- Strategic Merge Patch
- Server-Side Apply
- field manager
- managedFields
- 局部更新冲突

这些是 Kubernetes API 的复杂内容。当前阶段先用 PUT 建立完整闭环。

## Create

Create 的输入 DTO 包含 name。

例如：

```go
type CreateGatewayRequest struct {
    Name      string
    Listeners []GatewayListener
}
```

因为创建一个资源时，需要告诉 apiserver 资源叫什么。

## Update

Update 的输入 DTO 不包含 name。

例如：

```go
type UpdateGatewayRequest struct {
    Listeners []GatewayListener
}
```

name 从 URL 来：

```text
PUT /admin/v1/gateways/:name
```

为什么 update DTO 不放 name？

因为 URL 已经表达了“你要更新哪个资源”。如果 body 里也放 name，就会出现两个 name：

```text
URL name = a
body name = b
```

还要额外处理冲突。当前设计直接避免这个问题。

## 为什么 update 要先 get

biz 里的 update 逻辑是：

```text
Get 当前对象
-> 用请求构造新 spec
-> 复制当前 ObjectMeta
-> 保留当前 Status
-> 调 Update
```

原因是 Kubernetes 资源对象不是只有 spec。

它还有：

- `metadata.resourceVersion`
- `metadata.uid`
- `metadata.creationTimestamp`
- `metadata.generation`
- `metadata.labels`
- `metadata.annotations`
- `status`

如果 update 时只构造一个新对象：

```go
Gateway{
    ObjectMeta: metav1.ObjectMeta{Name: name},
    Spec: newSpec,
}
```

就可能丢掉已有 metadata。

更重要的是，Kubernetes update 通常需要 `resourceVersion`。

`resourceVersion` 表示“我基于哪个版本更新”。

这能防止并发覆盖。

所以 update 必须先拿到当前对象。

## 为什么保留 Status

admin-api 更新的是用户期望，也就是 `spec`。

状态 `status` 通常由 controller-manager 回写。

如果 admin-api 更新 spec 时把 status 清掉，就会破坏 controller 写进去的状态。

所以当前 update 会保留：

```go
updated.Status = current.Status
```

更严格的 Kubernetes 设计里，status 应该通过 `/status` 子资源单独更新。admin-api 当前不写 status。

## Delete

Delete 很直接：

```text
handler -> biz.Delete -> store.Delete -> generated clientset Delete
```

删除成功返回：

```text
204 No Content
```

为什么不是返回删除后的对象？

HTTP API 常见做法是：删除成功但没有响应体，返回 `204`。

## 错误处理

统一错误处理在：

```text
internal/adminapi/handler/common.go
```

它会把 apiserver/client-go 错误转成 HTTP 错误：

```text
NotFound       -> 404
AlreadyExists  -> 409
Invalid        -> 400
BadRequest     -> 400
其他错误       -> 500
```

为什么 handler 统一处理？

因为 apiserver 返回的是 Kubernetes API error。产品 API 不应该把所有内部细节原样散落到每个 handler 里。

## 为什么 list 当前不分页

当前 list 是：

```go
List(ctx, metav1.ListOptions{})
```

没有 limit、continue、labelSelector。

这是阶段性选择。

后面产品化可以加：

- 分页参数
- 关键字搜索
- 标签过滤
- 排序

但这些需要先设计产品语义，不应该在第一阶段混进去。
