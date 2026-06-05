# Gateway 长期模型和 admin-api 设计

## 背景

当前 Gateway 后端接口能支撑控制台第一版页面，但不适合作为长期模型继续扩展：

- `GET /api/v1/gateways` 同时返回列表数据和证书下拉选项，接口职责不清
- `GET /api/v1/gateways/:id` 和历史 `GET /api/v1/gateways/:id/overview` 都在表达详情，边界重复
- 创建和更新请求包含 `runtimeGroupName`、`certificateName` 等展示字段，后端实际不会持久化这些字段
- `description`、`enabled`、`hostnames` 等 Gateway 核心语义通过 annotation 保存，绕开了声明式资源模型
- HTTPS 证书字段出现在控制台请求里，但 `GatewaySpec` 没有证书引用，运行链路无法真正生效
- 多域名绑定时，监听器 `hostname` 只能表达单个 Host，剩余 Host 只能依赖控制台 annotation
- DTO 命名过于泛化，例如 `GatewayRequest`、`MutationResponse`，不符合按接口动作定义请求响应模型的规范

项目仍处于初始阶段，应该直接建立长期模型，不继续在临时 annotation 和页面字段上补丁式演进。

## 目标

- 把 Gateway 作为后端接口和资源模型重构的第一套样板
- `GatewaySpec` 明确表达入口监听、运行组、启停、域名绑定和 TLS 证书引用
- admin-api 面向控制台产品语义定义 DTO，不直接暴露 CR
- Handler 层只负责绑定、校验、调用 Service 和写统一响应
- Service 层承载 Gateway 用例语义和产品 DTO 到声明式资源的转换
- 列表、详情、表单选项、变更接口职责拆清楚
- 后端默认返回单资源或明确资源集合，不为页面便利提供不必要的聚合接口
- 删除不会被真实模型承载的请求字段，避免前端和后端能力不一致

## 非目标

- 不在本轮实现真实 xDS 协议服务
- 不在本轮实现 etcd、Kubernetes operator、AI runtime 或数据面 agent
- 不在 Gateway 表单中维护证书内容，证书是独立资源，Gateway 只引用证书
- 不为了当前控制台页面保留无长期价值的临时接口

## 设计原则

- Gateway 是控制面入口资源，不是 Envoy listener 的简单包装
- 运行组、监听器、域名绑定、TLS 证书引用是 Gateway 的核心声明式语义
- annotation 只能承载非核心元信息，不能作为运行主链路配置来源
- 请求体只包含可变更字段，不包含展示名称、派生文案和运行态聚合
- 响应体可以包含展示字段，但展示字段必须从稳定资源或选项数据派生
- 前端能通过多个资源接口自行组装的数据，后端不默认做页面聚合
- 只有存在性能、一致性、权限裁剪或复杂计算需求时，才新增专门聚合接口
- 后端协议使用稳定英文枚举，不使用控制台中文展示文案作为 key
- DTO 名称按接口动作定义，例如 `CreateGatewayReq`、`GetGatewayResp`
- helper 函数不调用 `response.WriteResult`、`ctx.JSON` 等响应方法

## Gateway 声明式模型

Gateway 需要直接表达入口资源的长期领域语义：

```go
type GatewaySpec struct {
	Description     string          `json:"description,omitempty"`
	Enabled         bool            `json:"enabled"`
	RuntimeGroupRef RuntimeGroupRef `json:"runtimeGroupRef,omitempty"`
	Listeners       []Listener      `json:"listeners"`
	HostBindings    []HostBinding   `json:"hostBindings,omitempty"`
}

type RuntimeGroupRef struct {
	Name string `json:"name"`
}

type Protocol string

const (
	ProtocolHTTP  Protocol = "HTTP"
	ProtocolHTTPS Protocol = "HTTPS"
)

type Listener struct {
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
	Port     int      `json:"port"`
}

type HostBinding struct {
	Hostname     string      `json:"hostname,omitempty"`
	ListenerRefs []string    `json:"listenerRefs,omitempty"`
	TLS          *GatewayTLS `json:"tls,omitempty"`
}

type GatewayTLS struct {
	CertificateRef string `json:"certificateRef,omitempty"`
}
```

字段含义：

- `Description` 是产品说明，属于 Gateway 自身属性，不再放 annotation
- `Enabled` 表示控制面是否允许该 Gateway 参与编译和下发
- `RuntimeGroupRef` 表示 Gateway 绑定的数据面运行组，第一阶段可以只有 `default`
- `Listeners` 只表达协议和端口，不再承担多域名绑定
- `HostBindings` 表达域名到监听器的绑定关系
- `GatewayTLS.CertificateRef` 引用独立证书资源，不存证书内容

域名绑定的设计重点：

- 一个监听器可以被多个域名绑定引用
- 一个域名绑定可以引用多个监听器，例如同一 Host 同时绑定 HTTP 和 HTTPS
- 不同域名绑定可以引用不同证书
- `Hostname` 为空表示 catch-all 绑定，也就是不限制 Host
- 空 `HostBindings` 只适用于不需要 TLS 证书的 HTTP-only Gateway
- HTTPS 监听器必须通过 `HostBindings` 提供证书引用，包括 catch-all HTTPS 场景
- 同一个 Gateway 中只能存在一个 catch-all host binding，避免入口语义歧义

## admin-api 接口

Gateway 接口按页面和用例职责拆分：

```text
GET    /api/v1/gateways
GET    /api/v1/gateways/:id
POST   /api/v1/gateways
PUT    /api/v1/gateways/:id
PATCH  /api/v1/gateways/:id/enabled
DELETE /api/v1/gateways/:id
GET    /api/v1/runtime-groups
```

接口职责：

- `GET /api/v1/gateways` 返回列表摘要，服务 Gateway 列表页和其他页面的 Gateway 选择器
- `GET /api/v1/gateways/:id` 返回 Gateway 配置详情，不返回关联 Route、Upstream 或 RuntimeSnapshot
- `POST /api/v1/gateways` 创建 Gateway
- `PUT /api/v1/gateways/:id` 更新 Gateway，不允许修改资源 ID
- `PATCH /api/v1/gateways/:id/enabled` 只更新启停状态
- `DELETE /api/v1/gateways/:id` 删除 Gateway，仍有关联 Route 时拒绝删除
- `GET /api/v1/runtime-groups` 返回运行组资源列表，前端自行组装 Gateway 表单选项

删除 `GET /api/v1/gateways/:id/overview`。详情页需要的关联 Route、Upstream、RuntimeSnapshot 由前端通过对应资源接口获取并组装。后续如果出现明显性能或一致性问题，再增加带明确语义的查询接口，例如 `GET /api/v1/routes?gateway=gw-public`。

## DTO 命名和结构

请求模型按动作命名：

```go
type CreateGatewayReq struct {
	Name            string                  `json:"name" binding:"required"`
	Description     string                  `json:"description"`
	RuntimeGroup    string                  `json:"runtimeGroup"`
	Listeners       []GatewayListenerReq    `json:"listeners" binding:"required"`
	HostBindings    []GatewayHostBindingReq `json:"hostBindings"`
}

type UpdateGatewayReq struct {
	Version         string                  `json:"version" binding:"required"`
	Description     string                  `json:"description"`
	RuntimeGroup    string                  `json:"runtimeGroup"`
	Listeners       []GatewayListenerReq    `json:"listeners" binding:"required"`
	HostBindings    []GatewayHostBindingReq `json:"hostBindings"`
}

type SetGatewayEnabledReq struct {
	Enabled *bool `json:"enabled" binding:"required"`
}
```

请求体不包含：

- `runtimeGroupName`，展示名由选项接口返回
- `certificateName`，展示名由证书资源或选项接口返回
- `hostPolicy`，这是派生展示文案
- `listenerSummary`，这是派生展示文案
- `routeCount`、`serviceCount`、`runtimeStatus`，这些是查询聚合结果

响应模型按页面语义命名：

```go
type ListGatewaysResp struct {
	Gateways []GatewaySummary `json:"gateways"`
}

type GetGatewayResp struct {
	Gateway GatewayDetail `json:"gateway"`
}
```

`GatewaySummary` 保持列表可扫描：

- `id`
- `version`
- `name`
- `description`
- `runtimeGroup`
- `runtimeGroupName`
- `listenerSummary`
- `hostBindingSummary`
- `enabled`
- `healthStatus`
- `lastChangedAt`

列表摘要默认不包含 `routeCount`、`upstreamCount`、`runtimeStatus`、`latestSnapshotVersion` 等聚合字段。确实需要时，优先由前端并行调用 Route、Upstream、RuntimeSnapshot 资源接口后组装；如果后续数据量导致前端组装成本过高，再设计独立统计接口。

`GatewayDetail` 保持配置可编辑：

- `id`
- `version`
- `name`
- `description`
- `runtimeGroup`
- `runtimeGroupName`
- `listeners`
- `hostBindings`
- `enabled`
- `healthStatus`
- `runtimeStatus`
- `lastChangedAt`

## Handler 和 Service 分层

Handler 层规则：

- 使用 `ShouldBindJSON`、`ShouldBindQuery`、`ShouldBindUri` 绑定参数
- 调用请求 DTO 的 `Validate()` 做字段校验和简单归一化
- 从 URI 获取的 `name` 明确传给 Service
- 调用 Service 方法，传递已校验的请求 DTO 或 Service Params
- 根据错误类型返回标准化响应
- 不编写业务逻辑，不构造 CR，不统计关联资源
- helper 函数不写响应

Service 层规则：

- 承载 Gateway 创建、更新、启停、删除和详情查询等用例
- 将 admin-api DTO 或 Service Params 转换为 `resource.Gateway`
- 校验跨资源约束，例如默认 Host 冲突、删除前仍有关联 Route
- 除跨资源校验外，不为 Gateway 详情读取 Route、Upstream、RuntimeSnapshot 做页面聚合
- 不返回面向 gin 的响应对象

Store 层规则：

- 只封装资源 IO
- 不理解控制台 DTO
- 不做业务聚合和产品展示转换

## 校验规则

请求 DTO 负责单请求字段校验：

- Gateway name 必须是 DNS label
- listener name 不为空且在当前 Gateway 内唯一
- listener protocol 只能是 `HTTP` 或 `HTTPS`
- listener port 在 `1..65535` 内，端口不能重复
- hostname 必须是合法 DNS subdomain 或通配域名
- host binding 引用的 listener 必须存在
- HTTPS listener 被 host binding 引用时必须配置 certificate ref
- HTTP listener 的 host binding 不能配置 certificate ref
- catch-all host binding 在同一个 Gateway 内最多一个

Service 负责跨资源校验：

- 创建时 Gateway name 不能重复
- 更新时 URI name 和资源 name 不能冲突
- 更新时 ResourceVersion 必须匹配
- 启用状态下，不限制 Host 的 Gateway 不能和另一个启用中的不限制 Host Gateway 共享监听入口
- 删除时仍有关联 Route 则拒绝删除
- 证书资源接入后，HTTPS 域名绑定引用的证书必须存在

## 编译和运行时影响

compiler 后续只读取 `GatewaySpec` 的正式字段：

- `Enabled=false` 的 Gateway 不参与运行配置生成
- `Listeners` 生成逻辑监听入口
- `HostBindings` 生成 Host 过滤和 TLS 证书引用
- `RuntimeGroupRef` 决定该 Gateway 编译到哪个运行组

Envoy xDS 只是第一个 target。Gateway 模型不能以 Envoy 专有结构作为核心表达。

## 实现范围

本轮代码实现建议包含：

- 更新 `pkg/apis/gateway/v1` 的 Gateway 类型
- 更新内部资源转换和生成代码
- 更新 Gateway admin-api DTO、Handler、Service 和测试
- 更新前端 Gateway domain、form 和 live repository 调用
- 删除 `/api/v1/gateways/:id/overview`
- 将列表接口和表单选项接口拆开
- 移除 Gateway 相关 annotation 主链路依赖

本轮不要求实现：

- 真实证书资源 CRUD
- 真实 runtime group CRUD
- 真实 xDS ADS/SotW/Delta 协议服务
- 运行时回执状态

证书和运行组在第一阶段可以用只读 option stub，但 Gateway 资源中的引用字段必须稳定。

## 测试要求

后端测试：

- Gateway 请求 DTO 校验
- 创建、更新、启停和删除 Service 用例
- host binding 引用 listener 的校验
- 不限制 Host Gateway 共享监听入口冲突
- 删除 Gateway 时仍有关联 Route 的拒绝逻辑

前端验证：

- `npm run build --prefix web/console`

仓库验证：

- `make test`
- `make build`
