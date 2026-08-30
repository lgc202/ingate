# Admin API 错误处理设计

## 文档状态

- 状态：实现基线候选稿
- 受众：Admin API、Console 和内部客户端的开发者
- 范围：`ingate-admin-api` 的 HTTP/gRPC 请求从 `service -> biz -> data` 返回到 `server` 的错误流
- 不包含：Controller 编译诊断、Envoy 请求失败、Analytics 查询结果和 Assistant 工具结果
- 参考基线：Kratos v3 及 `go-kratos/kratos-layout` 当前分层实践

本文细化 Admin API 的错误处理。与其他设计文档中的通用描述冲突时，Admin API 实现以本文为准。

## 1. 核心决策

Admin API 不建立独立于 Kratos 的错误框架。预期错误直接使用 `github.com/go-kratos/kratos/v3/errors` 表达；未知错误保持普通 Go error，由 server 统一转换为脱敏的内部错误。

```text
service ──调用──> biz <──实现接口── data
   │                │                 │
   │ 参数错误       │ 业务错误        │ 外部错误翻译
   └──── Kratos Error 或原始 error ────┘
                         │
                         v
                server 统一编码和记录
```

错误流遵循五条规则：

1. 声明式资源共有的静态错误在根 `biz` 包定义一次；领域特有错误留在对应领域包。
2. 包含动态安全文案的业务错误，在作出业务判断的位置直接构造。
3. data 只翻译能够确定语义的外部错误，未知错误原样返回。
4. service 不重新包装 biz 错误，server 不逐类映射业务错误。
5. 同一错误只在拥有最终处理责任的边界记录一次。

## 2. 目标与非目标

### 2.1 目标

- 从代码位置即可判断错误由哪条规则产生。
- HTTP、gRPC、日志和内部调用使用一致的状态与 reason。
- 保留排障所需的内部 cause，同时不向客户端泄露实现细节。
- 新增业务规则不需要修改全局错误类型、映射表和中间件。
- 调用方可以根据稳定 reason 或 Kratos 状态分类，而不是解析 message。

### 2.2 非目标

- 不建立覆盖所有组件的通用错误库。
- 不为每个校验分支、字段或用户文案定义错误码。
- 不通过错误类型承载正常业务数据。
- 不在当前阶段设计多语言框架；message 只要求安全且可直接展示。
- 不为了统一形式包装所有原始 error。

## 3. 错误模型

### 3.1 预期错误

预期错误是系统已经理解，并且能够向调用方给出明确状态的失败，例如：

- 请求字段格式错误；
- 资源不存在；
- 资源版本冲突；
- 资源内部成员名称冲突；
- 引用不存在或资源状态不允许当前操作；
- 已知依赖暂时不可用。

预期错误使用 Kratos Error：

| 字段 | 作用 | 约束 |
| --- | --- | --- |
| `Code` | HTTP 状态，并由 Kratos 映射到 gRPC 状态 | 使用标准状态语义 |
| `Reason` | 稳定机器分类 | 使用 `api/admin/v1.ErrorReason` |
| `Message` | 安全、可直接展示的默认文案 | 不包含内部实现和敏感数据 |
| `Cause` | 内部排障原因 | 只进入责任边界日志，不返回客户端 |
| `Metadata` | 可选机器数据 | 只有真实调用方消费时才增加 |

### 3.2 未知错误

未知错误包括数据库、网络、序列化和程序实现产生的非预期失败。产生位置不得猜测其业务含义，也不得把原始文本作为用户提示。

未知错误沿调用链原样返回。server 将其编码为 `500 INTERNAL_ERROR`，向客户端返回稳定文案，并在该边界记录原始 cause。

请求上下文本身的取消和超时不是依赖故障，也不能作为未知错误返回。server 在网络边界统一把 `context.Canceled` 转为 `499 REQUEST_CANCELED`，把 `context.DeadlineExceeded` 转为 `504 REQUEST_TIMEOUT`；业务和 data 层只需原样传播上下文错误。

### 3.3 Message 与 Cause

`Message` 和 `Cause` 服务于不同读者：

- Message 面向产品调用方，必须安全、明确、可采取行动。
- Cause 面向开发和运维，保留真实依赖错误与调用上下文。

以下写法只在翻译错误时使用：

```go
return errors.Conflict(
	adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
	fmt.Sprintf("关联服务 %q 不存在", upstreamID),
).WithCause(err)
```

没有下层错误时不调用 `WithCause`，也不为了形式制造 cause。

## 4. 各层职责

### 4.1 API

`api/admin/v1/error_reason.proto` 定义调用方、gRPC ErrorInfo 和日志使用的稳定 reason。

新增 reason 必须同时满足：

1. 调用方需要采取不同动作，或者运维需要独立聚合；
2. 现有 reason 无法准确表达；
3. 名称描述稳定语义，而不是当前实现或一句文案。

例如版本冲突要求调用方重新读取资源，因此应与普通资源冲突区分：

```proto
RESOURCE_NOT_FOUND = 8;
RESOURCE_CONFLICT = 12;
RESOURCE_VERSION_CONFLICT = 7;
```

资源内部成员名称冲突、引用缺失和资源类型不匹配，如果调用方的恢复动作都是修改输入后重试，可以共用 `RESOURCE_CONFLICT`。资源展示名称允许重复，不产生冲突错误。

不得增加 `ROUTE_NAME_CONFLICT`、`UPSTREAM_REFERENCE_MISSING` 等没有独立调用方行为的细粒度 reason。

### 4.2 service

service 负责协议边界：

- 校验必填字段、枚举、格式和 DTO 组合；
- 把 Proto 请求转换成 biz 输入；
- 把可确认的请求错误直接转换为 `BadRequest`；
- 原样返回 biz 结果和错误。

```go
spec, err := parseRouteSpec(request.GetName(), request.GetConfig())
if err != nil {
	return nil, err
}

route, err := s.routes.Replace(ctx, request.GetId(), request.GetVersion(), spec)
if err != nil {
	return nil, err
}
return routeResponse(route), nil
```

service 不执行以下操作：

- 根据 biz error 再构造一次 Kratos Error；
- 维护业务错误 switch；
- 访问数据库或判断跨资源规则；
- 记录随后还会返回的同一错误。

### 4.3 biz

biz 负责业务规则和用例编排，并拥有 data 需要返回的稳定业务错误。

允许 biz 导入 `api/admin/v1` 的 `ErrorReason` 和 Kratos errors。这里依赖的是 Admin API 的产品错误契约，不是 HTTP/gRPC server 实现；biz 仍然禁止导入 Kratos transport、server 和具体数据客户端。

#### 静态共享错误

只有同时满足以下条件才定义 sentinel：

- 错误不需要动态机器字段；
- data、biz 或多个调用点需要返回同一语义；
- 调用方需要通过 Kratos 状态或 `errors.Is` 识别它。

声明式资源不存在、持久化身份已存在、资源版本冲突和分页游标无效是资源存储共享的语义，集中放在根 `biz/errors.go`，供 data 和各领域用例复用：

```go
var ErrResourceVersionConflict = errors.Conflict(
	adminv1.ErrorReason_RESOURCE_VERSION_CONFLICT.String(),
	"资源已被其他用户修改，请刷新后重试",
)
```

这些 sentinel 必须保持数量少、语义稳定，并且确实跨资源复用。只属于某一领域的静态错误放在该领域包；带动态上下文的错误仍在规则判断处直接构造。根 `biz/errors.go` 不是通用错误工厂集合。

#### 动态业务错误

动态文案在规则成立的位置直接构造：

```go
if errors.IsNotFound(err) {
	return errors.Conflict(
		adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
		fmt.Sprintf("关联网关 %q 不存在", gatewayID),
	).WithCause(err)
}
```

不要为这一行错误创建 `routeNameConflict`、`NewRuleViolation` 或 `NewConflictError`。只有当函数封装了可复用的判断或转换逻辑，而不只是缩短构造表达式时，才值得抽取。

### 4.4 data

data 负责把外部依赖语义翻译为 biz 已定义的稳定错误。

```go
route, err := client.Get(ctx, routeID)
if err != nil {
	if apierrors.IsNotFound(err) {
		return nil, biz.ErrResourceNotFound
	}
	return nil, err
}
```

翻译规则：

- 外部 `NotFound` 转换为 `biz.ErrResourceNotFound`；
- 外部 `AlreadyExists` 转换为 `biz.ErrResourceAlreadyExists`；
- 乐观锁或条件写失败转换为 `biz.ErrResourceVersionConflict`；
- 外部输入拒绝只有在能够确定为调用方配置错误时才转换为 `BadRequest`；
- 超时和连接失败只有在客户端提供可靠分类时才转换为领域的依赖不可用错误，否则保留原始 error；
- 无法确认语义的响应和其他未知错误原样返回；
- 不根据 `err.Error()` 文本判断错误类型。

data 不创建面向具体页面的动态中文文案。只有 biz 知道某个缺失对象在当前操作中的含义，例如“关联服务不存在”。

### 4.5 server

server 只处理网络边界：

- 中间件使用一次 `errors.As` 识别可信的 Admin Kratos Error，并把未知错误替换成脱敏内部错误；
- HTTP 编码器使用 `errors.FromError(err)` 读取已经标准化的 Kratos Error；gRPC 直接使用同一个标准化错误；
- 编码 status、reason 和安全 message；
- 未知错误统一转换为脱敏的 `INTERNAL_ERROR`；
- 记录一次请求结果和内部 cause；
- recovery 把 panic 转换为内部错误。

server 不维护业务错误映射表，也不逐个 `errors.As` 解析领域类型。

如果继续使用自定义 HTTP envelope，错误响应应保留以下最小信息：

```json
{
  "code": 409,
  "reason": "RESOURCE_VERSION_CONFLICT",
  "msg": "资源已被其他用户修改，请刷新后重试",
  "data": null
}
```

Console 可以暂时只展示 `msg`，但自动化客户端不得解析 msg。需要基于错误采取动作时必须使用 `reason`。

## 5. 状态与 Reason

| 场景 | Kratos 构造 | HTTP | Reason | 恢复动作 |
| --- | --- | --- | --- | --- |
| 请求格式或字段错误 | `errors.BadRequest` | 400 | `INVALID_ARGUMENT` | 修改请求 |
| 目标资源不存在 | `errors.NotFound` | 404 | `RESOURCE_NOT_FOUND` | 刷新或选择其他资源 |
| 名称、引用或状态冲突 | `errors.Conflict` | 409 | `RESOURCE_CONFLICT` | 修改输入后重试 |
| 乐观并发冲突 | `errors.Conflict` | 409 | `RESOURCE_VERSION_CONFLICT` | 重新读取后重试 |
| 已知依赖暂时不可用 | `errors.ServiceUnavailable` | 503 | `DEPENDENCY_UNAVAILABLE` | 稍后重试 |
| 未知内部失败 | server 统一转换 | 500 | `INTERNAL_ERROR` | 使用 request ID 排障 |

HTTP 状态表达通用协议语义，Reason 表达产品级恢复动作。两者都不能从 message 推断。

## 6. Route Replace 参考链路

### 6.1 biz 预检

```go
func (uc *Usecase) Replace(
	ctx context.Context,
	routeID string,
	expectedGeneration int64,
	spec resource.RouteSpec,
) (*resource.Route, error) {
	current, err := uc.store.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}
	if current.Generation != expectedGeneration {
		return nil, biz.ErrResourceVersionConflict
	}
	if err := uc.checkReferences(ctx, spec); err != nil {
		return nil, err
	}
	return uc.store.ReplaceSpec(ctx, routeID, expectedGeneration, spec)
}
```

biz 的版本预检用于尽早拒绝过期请求，不能代替 data 的原子条件写。

### 6.2 data 条件写

data 必须在实际写入时再次比较版本。比较失败返回同一个 `biz.ErrResourceVersionConflict`；底层 ResourceVersion 因 Controller status 更新而冲突时，只重试能够确认不会覆盖用户 spec 的操作。

### 6.3 返回调用方

service 原样返回 `Replace` 的错误。server 中间件先完成一次可信错误识别和未知错误脱敏，HTTP 与 gRPC 随后写出 409、`RESOURCE_VERSION_CONFLICT` 和安全 message，不再调用 Route 专属映射函数。

## 7. Cause、日志与安全

- 预期的 4xx 错误不记录为系统异常；请求日志可以记录 code、reason、operation、resource ID 和 request ID。
- 未知 5xx、panic 和依赖故障在 server 或明确的降级边界记录一次。
- 只有翻译外部错误时使用 `WithCause`；动态业务判断没有 cause。
- Cause、堆栈、数据库错误、网络地址和完整响应体不得进入客户端 message。
- 访问密钥、私钥、模型凭据、Cookie、Authorization Header 和完整请求不得进入 error、metadata 或日志。
- message 不作为指标 label；聚合使用 reason 和 operation。
- 返回错误的函数不同时记录该错误，除非它已经完成降级并不会继续向上传播。

## 8. 明确拒绝的设计

### 无边界的全局错误工具箱

根 `biz/errors.go` 只容纳被多个声明式资源共同使用的稳定 sentinel，以及真正跨资源的数据错误翻译。不得在其中聚集领域特有 sentinel、动态业务工厂和纯粹缩短一行构造代码的帮助函数。

### 重复的 errcode 包

不再建立一份字符串常量与 `api/admin/v1.ErrorReason` 重复。Reason 的唯一事实来源是 Proto enum。

### 一行错误工厂

不创建只代理 `errors.Conflict` 的 `routeNameConflict`、`NewRuleViolation` 和类似函数。它们隐藏信息但不封装规则。

### 全局业务 mapper

server 不通过 `errors.As + switch` 把每个 biz error 映射成 HTTP/gRPC。预期错误在语义明确的位置已经携带完整状态。

### 为文案定义错误类型

调用方不需要读取动态字段时，不定义 `ViolationError`、`ConflictError` 等类型。Message 不是创建类型化错误的理由。

### Metadata 承载普通展示文案

安全展示文案直接放在 Message。Metadata 只承载真实机器调用方需要的结构化字段，不能成为第二套响应协议。

## 9. 渐进迁移

错误处理按领域链路迁移，不做一次性机械重写：

1. 判断错误是跨资源存储语义、领域特有语义还是动态业务判断。
2. data 把已知外部错误翻译到根 biz sentinel 或对应领域错误。
3. 删除重复 sentinel、全局错误工厂和重复 errcode。
4. 动态业务错误改为判断位置直接构造。
5. service 保持透传，server 删除该领域的映射分支。
6. 根 `biz/errors.go` 只保留经过实际调用证明的跨资源语义。

每次迁移以一条可运行链路为单位，避免为了临时编译同时保留新旧两套正式抽象。

## 10. 评审清单

新增或修改错误处理时逐项确认：

- 这是预期错误还是未知错误？
- 当前层是否真的知道它的业务语义？
- 调用方是否需要根据该错误采取不同动作？
- 是否可以复用现有 Reason？
- sentinel 是否至少被 data/biz 多个位置共享或需要匹配？
- 动态错误是否直接位于规则判断点？
- 是否错误地暴露了 cause、敏感信息或实现细节？
- 是否出现 log 后继续 return 同一错误？
- server 是否新增了不必要的业务映射？
- 是否为了减少几行构造代码增加了新类型或工厂？

## 11. 参考

- [Kratos Layout：biz 定义静态错误](https://github.com/go-kratos/kratos-layout/blob/c79b2ccfaa360fdce54a43e64325a8796df26b11/internal/biz/todo.go)
- [Kratos Layout：data 翻译存储错误](https://github.com/go-kratos/kratos-layout/blob/c79b2ccfaa360fdce54a43e64325a8796df26b11/internal/data/todo.go)
- [Kratos Layout：service 透传错误](https://github.com/go-kratos/kratos-layout/blob/c79b2ccfaa360fdce54a43e64325a8796df26b11/internal/service/todo.go)
- [Kratos Layout：server 不映射业务错误](https://github.com/go-kratos/kratos-layout/blob/c79b2ccfaa360fdce54a43e64325a8796df26b11/internal/server/http.go)
- [Go 1.13 Errors](https://go.dev/blog/go1.13-errors)
- [gRPC Status Codes](https://grpc.io/docs/guides/status-codes/)
- [Google AIP-193 Errors](https://google.aip.dev/193)
- [Kubernetes API Errors](https://github.com/kubernetes/apimachinery/blob/master/pkg/api/errors/errors.go)
