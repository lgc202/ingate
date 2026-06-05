# Route、Policy 和 Plugin 长期模型设计

## 背景

当前控制台已经支持在路由上添加 Header 改写、超时和重试配置。第一版实现为了快速跑通链路，把这些配置保存到 `route.ingate.io/policy-bindings` annotation，由 compiler 解析后写入 IR，再由 xDS target 翻译成 Envoy route 配置。

这个方案适合验证闭环，但不适合作为长期后端模型：

- annotation 绕开了 `RouteSpec` 的类型系统、OpenAPI schema 和 apiserver 校验
- 后端协议依赖控制台中文名称，例如 `请求 Header 改写`
- `parameters map[string]any` 让 admin-api、compiler 和前端各自理解参数结构，容易漂移
- `RouteRule.TimeoutMillis` 和 annotation 中的超时策略形成两个配置来源
- 控制台产品模型、声明式资源模型和运行时模型边界不清

项目还处于初始阶段，应该直接把长期模型边界定清楚，不继续在临时 annotation 方案上扩展。

## 目标

建立三类能力边界：

```text
Route 原生能力
  表达一条 HTTP 路由如何匹配、改写、转发、超时和重试

Policy 治理策略
  表达可复用、可审计、可绑定的治理规则如何作用到资源

Plugin 扩展能力
  表达非标准能力、AI 能力和外部扩展如何加载、排序和按范围生效
```

这三类能力都进入正式声明式资源模型，compiler 只读取正式资源，不再解析控制台私有 annotation 作为运行主链路。

## 设计原则

- 后端协议使用稳定英文枚举，不使用控制台中文展示名
- Route 自身能力直接进入 `RouteSpec`，不包装成通用 Policy
- Policy 只用于可复用治理能力，不承载每条 Route 的普通转发行为
- Plugin 只用于运行时扩展，不把产品核心语义退化成大块插件 JSON
- admin-api 负责产品 DTO 到声明式资源的转换，前端不直接构造底层资源
- compiler 负责资源解析和领域校验，不负责理解控制台表单协议
- target translator 只消费 IR，不反向理解产品概念

## Route 原生能力

Route 负责表达 HTTP 请求进入网关后的原生 L7 行为。

第一阶段需要直接模型化：

- 请求匹配：host、path、method、header
- 加权转发：多个 `UpstreamRef`
- 请求 Header 修改
- 响应 Header 修改
- URL rewrite
- 请求 mirror
- CORS
- route timeout
- route retry

建议模型：

```go
type RouteRule struct {
	Matches      []RouteMatch  `json:"matches,omitempty"`
	PathPrefix   string        `json:"pathPrefix,omitempty"` // 兼容当前简单模型，可后续迁入 Matches
	Methods      []string      `json:"methods,omitempty"`
	Headers      []HeaderMatch `json:"headers,omitempty"`
	UpstreamRefs []UpstreamRef `json:"upstreamRefs"`
	Filters      []RouteFilter `json:"filters,omitempty"`
	Timeout      *RouteTimeout `json:"timeout,omitempty"`
	Retry        *RouteRetry   `json:"retry,omitempty"`
}

type RouteFilterType string

const (
	RouteFilterRequestHeaderModifier  RouteFilterType = "RequestHeaderModifier"
	RouteFilterResponseHeaderModifier RouteFilterType = "ResponseHeaderModifier"
	RouteFilterURLRewrite             RouteFilterType = "URLRewrite"
	RouteFilterRequestMirror          RouteFilterType = "RequestMirror"
	RouteFilterCORS                   RouteFilterType = "CORS"
)

type RouteFilter struct {
	Type                   RouteFilterType       `json:"type"`
	RequestHeaderModifier  *HeaderModifier       `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HeaderModifier       `json:"responseHeaderModifier,omitempty"`
	URLRewrite             *URLRewrite           `json:"urlRewrite,omitempty"`
	RequestMirror          *RequestMirror        `json:"requestMirror,omitempty"`
	CORS                   *CORSPolicy           `json:"cors,omitempty"`
}

type HeaderModifier struct {
	Set    []HeaderValue `json:"set,omitempty"`
	Add    []HeaderValue `json:"add,omitempty"`
	Remove []string      `json:"remove,omitempty"`
}

type RouteTimeout struct {
	RequestMillis int `json:"requestMillis,omitempty"`
}

type RouteRetry struct {
	Attempts            int      `json:"attempts,omitempty"`
	PerTryTimeoutMillis int      `json:"perTryTimeoutMillis,omitempty"`
	RetryOn             []string `json:"retryOn,omitempty"`
}
```

Route 原生能力的判断标准：

- 它只描述当前 route 的匹配、请求处理或转发行为
- 它直接映射到 Envoy route、virtual host 或 route action
- 它不需要独立生命周期、复用、审计或跨资源绑定

因此 Header 改写、超时、重试、URL rewrite、mirror、CORS 都不应该建成 Policy 资源。

## Policy 治理策略

Policy 表达可复用的治理规则。它们通常需要独立状态、审计、启停、权限控制、复用和绑定范围。

第一批 Policy 资源建议保持克制：

```text
AuthPolicy
RateLimitPolicy
AccessControlPolicy
PolicyBinding
```

建议模型：

```go
type PolicyBindingSpec struct {
	TargetRef PolicyTargetRef `json:"targetRef"`
	Policies  []PolicyRef     `json:"policies"`
}

type PolicyTargetRef struct {
	Kind     Kind   `json:"kind"`
	Name     string `json:"name"`
	RuleName string `json:"ruleName,omitempty"`
}

type PolicyRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}
```

Policy 的判断标准：

- 同一份配置可能被多个 Gateway、Route、Upstream 或 AIRoute 复用
- 需要独立展示状态、审计记录或权限控制
- 需要被显式绑定到目标资源
- 后续可能有优先级、继承、冲突检测或条件生效

不建议现在做一个万能 `Policy` + `map[string]any`。这会把 annotation 的问题搬到 CRD 里。每类高价值治理能力应该有自己的强类型资源。

## Plugin 扩展能力

Plugin 表达运行时扩展能力，适合承载变化快、组合方式多、与数据面执行机制强相关的能力。

典型能力：

- AI proxy
- AI token 统计
- 内容安全
- PII 脱敏
- OPA
- Bot 检测
- 自定义响应
- 复杂鉴权
- 外部处理器

建议继续保持 `Plugin` 和 `PluginBinding` 分离：

```go
type PluginSpec struct {
	Runtime  PluginRuntime `json:"runtime"`
	Version  string        `json:"version"`
	Endpoint string        `json:"endpoint,omitempty"`
	Image    string        `json:"image,omitempty"`
	Phases   []PluginPhase `json:"phases,omitempty"`
}

type PluginBindingSpec struct {
	TargetRef     PluginTargetRef     `json:"targetRef"`
	Phase         PluginPhase         `json:"phase"`
	Priority      int                 `json:"priority"`
	FailurePolicy PluginFailurePolicy `json:"failurePolicy"`
	Plugins       []PluginRef         `json:"plugins"`
}

type PluginRef struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config,omitempty"`
}
```

插件私有配置可以使用 JSON，因为插件能力本身是扩展边界。但以下字段必须结构化：

- 插件本体
- 运行时类型
- 版本
- 入口或镜像
- 生效目标
- 需要精确到 RouteRule 时使用稳定 `ruleName`
- 执行阶段
- 优先级
- 失败策略

也就是说，JSON 只能是插件私有配置，不能承担资源匹配、生命周期和运行时调度语义。

## Higress 参考取舍

Higress 提供了两个可借鉴点：

- 对 Ingress annotation，Higress 会先解析成内部强类型配置，再应用到 Istio `HTTPRoute`、`VirtualService` 或 `TrafficPolicy`，而不是让运行核心直接依赖控制台表单 JSON
- 对扩展能力，Higress 使用 `WasmPlugin` 独立资源，插件私有配置可以是 `Struct`，但插件阶段、优先级、失败策略和匹配范围仍有结构化字段

Ingate 不需要照搬 Higress：

- Ingate 的核心模型不绑定 Istio，也不把 Envoy xDS 当核心资源
- Ingate 已经有自己的 `Gateway / Route / Upstream / RuntimeSnapshot` 链路
- Ingate 的插件模型建议保留 `Plugin` 和 `PluginBinding` 分离，避免插件定义和生效范围耦合

## Admin API 和控制台边界

控制台可以继续用“策略配置”作为用户语言，因为用户关心的是治理、改写、安全和观测能力。

但 admin-api 保存时必须拆成正式资源。创建或更新 Route 时，只保存 Route 自身的匹配、目标和原生能力，不夹带治理 Policy 或 Plugin 绑定。

```text
控制台 Route 创建/编辑
  |
  |-- Gateway / Host / Rule / Upstream / Header 改写 / 超时 / 重试 / URL rewrite
  |     -> RouteSpec rule filters / timeout / retry
```

建议 Route 保存 DTO 直接贴近 `RouteSpec` 的长期边界：

```go
type CreateRouteReq struct {
	GatewayIDs []string       `json:"gatewayIDs"`
	Hostnames  []string       `json:"hostnames"`
	Enabled    *bool          `json:"enabled,omitempty"`
	Rules      []RouteRule    `json:"rules"`
}

type UpdateRouteReq struct {
	Version string `json:"version"`
	CreateRouteReq
}

type RouteRule struct {
	Name       string           `json:"name"`
	PathPrefix string           `json:"pathPrefix"`
	Methods    []string         `json:"methods,omitempty"`
	Headers    []HeaderMatchReq `json:"headers,omitempty"`
	Targets    []RouteTarget    `json:"targets"`

	RequestHeaderModifier  *HeaderModifierReq `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HeaderModifierReq `json:"responseHeaderModifier,omitempty"`
	Timeout                *RouteTimeoutReq   `json:"timeout,omitempty"`
	Retry                  *RouteRetryReq     `json:"retry,omitempty"`
}

type HeaderModifierReq struct {
	Set    []HeaderValueReq `json:"set,omitempty"`
	Remove []string         `json:"remove,omitempty"`
}

type RouteTarget struct {
	UpstreamID string `json:"upstreamID"`
	Weight     int    `json:"weight"`
}
```

`ServiceName` 不应继续和 `Targets` 并存。Route target 引用稳定 Upstream ID，不引用展示名。单目标路由也使用 `Targets` 表达：

```json
{
  "targets": [{ "upstreamID": "550e8400-e29b-41d4-a716-446655440000", "weight": 100 }]
}
```

Upstream 自身使用后端生成的 `metadata.name` 作为稳定 ID，用户输入名称保存到 `spec.displayName`。声明式资源内部仍使用 `UpstreamRef.Name` 引用 Upstream 的 `metadata.name`。

治理策略和插件由独立接口保存：

```text
控制台策略/插件配置
  |
  |-- API Key 鉴权 / 限流 / IP 访问控制
  |     -> AuthPolicy / RateLimitPolicy / AccessControlPolicy + PolicyBinding
  |
  |-- AI 内容安全 / Token 统计 / 自定义响应
        -> Plugin / PluginBinding 或 AI 专用资源编译出的 PluginBinding
```

如果产品上希望在“创建路由向导”里同时配置限流、认证或插件，前端可以保持一体化体验，但提交时应拆成多次调用：

```text
1. POST /routes
2. POST /policy-bindings
3. POST /plugin-bindings
```

这样用户体验可以是一个流程，后端模型仍保持 Route、Policy、Plugin 的独立生命周期。

后端不提供 `route-policy-capabilities` 这类产品能力目录接口。策略目录、展示名称、表单分组和默认参数属于控制台产品配置，由前端维护或由未来专门的产品配置服务提供；admin-api 只接收稳定的资源保存 DTO。

`displayName` 只用于显示，不进入 admin-api、compiler 或 target translator 的判断逻辑。

## 编译链路

长期编译链路应保持：

```text
正式声明式资源
  |
  v
Compiler
  |
  v
Logical IR
  |
  v
Target Translator
  |
  v
RuntimeSnapshot
```

compiler 输入包括：

- `RouteSpec` 中的 filters、timeout、retry
- `PolicyBinding` 引用的强类型 Policy 资源
- `PluginBinding` 引用的 Plugin 资源
- AI 资源编译出的 AI route、AI policy 和插件配置

compiler 不再解析：

- 控制台中文策略名
- `route.ingate.io/policy-bindings`
- 任意表单参数 map

## 迁移决策

项目仍处于初始阶段，不保留兼容路径。

执行方向：

1. 删除 `route.ingate.io/policy-bindings` 运行主链路
2. 将 Header 改写、超时、重试迁入 `RouteRule` 强类型字段
3. admin-api Route DTO 收敛为 `Rules []RouteRule`，转换为正式 `RouteSpec`
4. compiler 只从 `RouteSpec` 读取 route 原生能力
5. xDS translator 继续从 IR 生成 Envoy route action 和 header action
6. 删除后端 `route-policy-capabilities` 产品能力目录接口
7. 后续再补真正的 `PolicyBinding` 和 `PluginBinding` 产品接口

历史 plan `docs/superpowers/plans/2026-06-02-route-policy-runtime.md` 是 MVP 验证方案，后续实现应以本文档为准。

## 非目标

本文档不要求一次性实现：

- 完整插件市场
- 完整策略继承和冲突检测
- 全部 Gateway API filters
- AI runtime
- 真实 Wasm 插件生命周期
- 多租户权限模型

但资源边界必须按长期模型设计，避免后续再迁移一次核心协议。

## 后续实施顺序

建议按以下顺序实施：

1. 重构 `RouteRule` 原生能力模型，替换当前 annotation 策略
2. 调整 admin-api route DTO 为 `GatewayIDs / Hostnames / Enabled / Rules`，去掉 `ServiceName` 和顶层 route 原生能力字段
3. 调整控制台 repository，使 Route 保存只提交 Route 原生能力；策略目录不再来自 admin-api
4. 调整 compiler 和 IR，删除 `route_policy.go` 的 annotation 解析
5. 调整 xDS translator 和 ADS response 生成，保持运行行为不退化
6. 为 `PolicyBinding` 明确只承载可复用治理策略，支持绑定到 Gateway、Route、Upstream，必要时精确到 RouteRule
7. 为 `PluginBinding` 明确只承载插件执行和绑定语义，支持 target、phase、priority、failurePolicy 的结构化配置
8. 更新前端策略面板，使用户语言和后端模型显式解耦：一个向导可以触发 Route、PolicyBinding、PluginBinding 多次保存
