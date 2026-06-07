# RateLimitPolicy 和 PolicyBinding 专题设计

## 背景

`RateLimitPolicy` 和 `PolicyBinding` 已经出现在声明式资源、generated client、controller 和 compiler 中，但当前模型仍然是早期验证形态：

- `RateLimitPolicySpec` 只能表达单条 `requests/window/key/header`
- `RateLimitKey` 只支持 `IP` 和 `Header`
- `PolicyBinding` 只能绑定到资源本身，不能精确到 RouteRule
- xDS target 还没有消费限流策略，真实数据面不会生效
- admin-api 还没有面向控制台的限流策略和策略绑定接口

限流是典型的治理策略，不应该塞进 Route 原生字段，也不应该让前端直接编辑插件 JSON。长期模型需要保留强类型产品语义，同时允许数据面用内置插件执行 Redis/global limit。

## 目标

本专题一次性定义限流策略的长期形态：

- 使用强类型 `RateLimitPolicy` 表达限流策略本身
- 使用 `PolicyBinding` 表达策略绑定到哪个 Gateway、Route 或 RouteRule
- 支持本地限流和 Redis-backed global limit
- global limit 由内置 managed plugin 直连 Redis 执行，不新增独立 rate-limit 服务
- Redis 连接配置独立建模，策略通过引用使用，不把连接信息散落在策略规则里
- compiler 输出运行时无关 IR，xDS target 再翻译成内置限流插件配置
- admin-api 暴露产品 DTO，不暴露 Kubernetes 风格资源对象和插件私有 JSON

## 非目标

本专题不要求实现：

- 外部插件市场
- 独立 rate-limit service
- 多租户权限模型
- Secret 加密存储的完整实现
- 所有算法的运行时优化
- 前端页面重设计

但资源模型和编译链路要按长期边界设计，避免后续为了 Redis/global limit 再迁移一次核心协议。

## 总体结论

采用以下架构：

```text
RateLimitPolicy + PolicyBinding
  |
  v
Compiler
  |
  v
Logical IR
  |
  v
xDS target translator
  |
  v
managed rate-limit plugin config
  |
  v
Envoy 内置插件执行 local / Redis global limit
```

这里的“插件”是运行时执行机制，不是控制台产品协议。用户创建的是限流策略和绑定关系，不是 `Plugin` 资源，也不直接维护 WasmPlugin JSON。

## 内置治理插件

内置治理插件表示：能力在数据面通过插件机制执行，但在控制面和产品协议上不是用户插件。

用户只需要创建：

```text
RateLimitPolicy
PolicyBinding
RedisStore
```

用户不需要安装限流插件，也不需要创建 `Plugin` 或 `PluginBinding` 资源。系统会根据 `RateLimitPolicy + PolicyBinding` 自动生成 managed rate-limit plugin 配置，并随 `RuntimeSnapshot` 下发到数据面。

用户不关心以下实现细节：

- 插件包从哪里下载
- 插件版本
- Wasm 文件路径
- 插件执行 phase
- priority
- 插件私有 JSON schema
- Redis Lua 脚本
- xDS filter 细节

这样可以同时保留两个边界：

- 控制面是强类型治理模型，便于校验、审计、状态展示和长期演进
- 数据面使用插件执行，便于复用插件运行时能力并支持 Redis/global limit

限流、鉴权、访问控制这类核心治理能力不应该暴露为普通插件。普通插件适合自定义扩展能力；内置治理插件是 Ingate 自带能力，由 Ingate 版本管理、自动注入、自动配置。

## 和 Higress 的取舍

Higress 的限流能力主要通过 Wasm 插件配置实现，插件可以通过 `defaultConfig` 和 `matchRules` 在全局、域名、服务或路由范围生效。它的优点是落地快、运行时灵活，cluster/global limit 可以通过 Redis 支持。

Ingate 不直接照搬这个模型：

- 不把策略定义、匹配范围和插件私有配置混在一个 `WasmPlugin` 资源里
- 不让控制台协议退化成插件 JSON
- 不把 Redis 连接配置复制到每条规则

Ingate 借鉴它的数据面实现思路：global limit 由插件直连 Redis；但控制面仍保持 `RateLimitPolicy + PolicyBinding` 的强类型资源边界。

## 企业级验收标准

这个方案要满足企业级要求，不能只做到“能限流”。后续实现至少需要满足以下标准：

- Redis 支持 standalone 和 cluster 的模型边界，连接配置支持认证、TLS、DB、连接超时和命令超时
- Redis 密码不进入普通 DTO 响应，短期使用 `passwordRef` 占位，后续接 Secret
- global limit 计数必须使用 Lua 或 Redis 原子命令保证计数和过期时间一致
- local limit 和 global limit 的失败行为必须按 `FailurePolicy` 明确执行
- `FailOpen`、`FailClose`、Redis 错误、超限拒绝都必须有指标和日志
- `RedisStore`、`RateLimitPolicy`、`PolicyBinding` 必须有可展示状态，至少能表达引用是否解析成功、配置是否被接受、是否已下发
- 插件配置必须随 `RuntimeSnapshot` 版本下发，不能让插件自己绕过控制面拉配置
- 同一请求命中的 Gateway、Route、RouteRule 策略全部生效，不能出现隐式覆盖
- admin-api 的同名、引用存在性、删除保护等系统状态校验必须在 service 层完成
- 前端只消费产品 DTO，不直接维护底层资源对象或插件私有配置

## 资源模型

### RateLimitPolicy

`RateLimitPolicy` 表达一份可复用、可审计、可绑定的限流策略。

建议模型：

```go
type RateLimitPolicySpec struct {
	DisplayName   string                 `json:"displayName"`
	Description   string                 `json:"description,omitempty"`
	Enabled       bool                   `json:"enabled"`
	Mode          RateLimitMode          `json:"mode"`
	Rules         []RateLimitRule        `json:"rules"`
	Global        *GlobalRateLimitConfig `json:"global,omitempty"`
	Response      RateLimitResponse      `json:"response,omitempty"`
	FailurePolicy RateLimitFailurePolicy `json:"failurePolicy,omitempty"`
}

type RateLimitMode string

const (
	RateLimitModeLocal  RateLimitMode = "Local"
	RateLimitModeGlobal RateLimitMode = "Global"
)
```

`displayName` 是用户可见名称，同类资源内必须唯一。`metadata.name` 仍然是后端生成的 UUID，对 admin-api 和前端表现为 `id`。

`enabled=false` 表示策略保留但不生效。绑定关系可以继续存在，compiler 应跳过禁用策略。

### RateLimitRule

一条策略可以包含多条规则。这样可以表达同一绑定范围下的多层约束，例如“每 IP 每分钟 100 次，同时整个路由每分钟 1000 次”。

```go
type RateLimitRule struct {
	Name      string             `json:"name"`
	Key      RateLimitKey        `json:"key"`
	Limit    RateLimitQuota      `json:"limit"`
	Algorithm RateLimitAlgorithm `json:"algorithm,omitempty"`
}

type RateLimitQuota struct {
	Requests int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
}
```

`Rule.Name` 是策略内部稳定名称，用于日志、指标和插件配置，不作为跨资源引用键。

### RateLimitKey

限流 key 需要覆盖普通 API 网关和 AI 网关常见维度：

```go
type RateLimitKey struct {
	Parts []RateLimitKeyPart `json:"parts"`
}

type RateLimitKeyPart struct {
	Type RateLimitKeyType `json:"type"`
	Name string           `json:"name,omitempty"`
}

type RateLimitKeyType string

const (
	RateLimitKeyTypeIP       RateLimitKeyType = "IP"
	RateLimitKeyTypeHeader   RateLimitKeyType = "Header"
	RateLimitKeyTypeQuery    RateLimitKeyType = "Query"
	RateLimitKeyTypeCookie   RateLimitKeyType = "Cookie"
	RateLimitKeyTypeConsumer RateLimitKeyType = "Consumer"
	RateLimitKeyTypeRoute    RateLimitKeyType = "Route"
	RateLimitKeyTypeGateway  RateLimitKeyType = "Gateway"
)
```

`Parts` 支持组合 key。例如按租户和 API Key 联合限流：

```json
{
  "parts": [
    { "type": "Header", "name": "x-tenant-id" },
    { "type": "Consumer" }
  ]
}
```

组合 key 顺序有语义，compiler 不做排序。

### RateLimitAlgorithm

第一阶段支持固定窗口即可，模型预留算法字段：

```go
type RateLimitAlgorithm string

const (
	RateLimitAlgorithmFixedWindow RateLimitAlgorithm = "FixedWindow"
	RateLimitAlgorithmTokenBucket RateLimitAlgorithm = "TokenBucket"
)
```

本地限流优先使用插件运行时能稳定支持的算法。Redis global limit 初期使用固定窗口，后续可以升级滑动窗口或 token bucket，但不改变策略和绑定边界。

### GlobalRateLimitConfig

global limit 使用 Redis 存储计数状态：

```go
type GlobalRateLimitConfig struct {
	RedisRef string `json:"redisRef"`
	Prefix   string `json:"prefix,omitempty"`
	TimeoutMillis int `json:"timeoutMillis,omitempty"`
}
```

`redisRef` 引用独立 Redis 连接资源。`prefix` 用于隔离环境、租户或业务线，未配置时由 compiler 使用稳定默认值。

### RedisStore

Redis 配置不放进每个 `RateLimitPolicy`，避免重复、泄密和不可统一轮换。

建议新增资源：

```go
type RedisStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisStoreSpec `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

type RedisStoreSpec struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	Mode        RedisMode `json:"mode"`
	Address     string `json:"address"`
	DB          int    `json:"db,omitempty"`
	TLS         bool   `json:"tls,omitempty"`
	Username    string `json:"username,omitempty"`
	PasswordRef string `json:"passwordRef,omitempty"`
}
```

`PasswordRef` 先作为占位引用，后续接 Secret。不要把密码明文放进 admin-api 普通响应。

如果短期不实现完整 Secret，可以允许本地开发配置空密码，但接口模型保留 `passwordRef`。

### RateLimitResponse

超限响应是产品策略的一部分：

```go
type RateLimitResponse struct {
	StatusCode       int    `json:"statusCode,omitempty"`
	Message          string `json:"message,omitempty"`
	QuotaHeaderEnabled bool `json:"quotaHeaderEnabled,omitempty"`
}
```

默认 `statusCode=429`。`QuotaHeaderEnabled=true` 时，插件返回标准 quota header，便于客户端和调试工具识别剩余额度。

### FailurePolicy

Redis 或插件异常时的行为必须显式建模：

```go
type RateLimitFailurePolicy string

const (
	RateLimitFailurePolicyFailOpen  RateLimitFailurePolicy = "FailOpen"
	RateLimitFailurePolicyFailClose RateLimitFailurePolicy = "FailClose"
)
```

默认建议 `FailOpen`，避免 Redis 抖动直接放大成业务全站不可用。安全敏感路由可以配置 `FailClose`。

## PolicyBinding

`PolicyBinding` 只表达策略作用到哪里，不承载限流规则本身。

```go
type PolicyBindingSpec struct {
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Enabled     bool            `json:"enabled"`
	TargetRef   PolicyTargetRef `json:"targetRef"`
	Policies    []PolicyRef     `json:"policies"`
}

type PolicyTargetRef struct {
	Kind     Kind   `json:"kind"`
	Name     string `json:"name"`
	RuleName string `json:"ruleName,omitempty"`
}
```

`PolicyTargetRef.Name` 是目标资源 ID，也就是底层资源的 `metadata.name`。admin-api 对外字段使用 `targetID`，不要叫 `name`。

`RuleName` 只在 `Kind=Route` 或未来 `Kind=AIRoute` 时允许填写，用于精确绑定到某条 route rule。

第一阶段支持目标：

- `Gateway`
- `Route`
- `Route` + `RuleName`

暂不把 `Upstream` 作为控制台首批入口。Upstream 侧限流容易和连接池、负载均衡、健康检查混淆，后续如需要再开放。

## 生效语义

### 绑定范围

同一个请求可能同时命中多个绑定范围：

```text
Gateway binding
  -> Route binding
  -> RouteRule binding
```

策略从宽到窄全部生效，不做覆盖。只要任一限流规则超限，请求就被拒绝。

这种语义比“子级覆盖父级”更清晰，也更符合治理策略：全局配额和局部配额可以同时约束。

### 多策略绑定

一个 `PolicyBinding` 可以引用多个 policy。多个绑定也可以命中同一个请求。compiler 按资源名稳定排序，运行时按稳定顺序执行，但语义上不依赖执行顺序。

如果多个策略都返回超限，插件可以选择第一个命中的超限规则作为响应来源，但指标需要记录实际命中的 rule。

### 禁用语义

- `RateLimitPolicy.Enabled=false`：该策略不生效
- `PolicyBinding.Enabled=false`：该绑定不生效
- 目标资源禁用：对应 route/gateway 不进入运行时，策略自然不生效

禁用不删除资源，便于审计和后续恢复。

## Admin API

admin-api 提供产品接口：

```text
GET    /api/v1/rate-limit-policies
POST   /api/v1/rate-limit-policies
GET    /api/v1/rate-limit-policies/:id
PUT    /api/v1/rate-limit-policies/:id
PATCH  /api/v1/rate-limit-policies/:id/enabled
DELETE /api/v1/rate-limit-policies/:id

GET    /api/v1/policy-bindings
POST   /api/v1/policy-bindings
GET    /api/v1/policy-bindings/:id
PUT    /api/v1/policy-bindings/:id
PATCH  /api/v1/policy-bindings/:id/enabled
DELETE /api/v1/policy-bindings/:id

GET    /api/v1/redis-stores
POST   /api/v1/redis-stores
GET    /api/v1/redis-stores/:id
PUT    /api/v1/redis-stores/:id
DELETE /api/v1/redis-stores/:id
```

请求 DTO 使用 `Name` 表示用户可见名称，转换到底层资源时写入 `spec.displayName`。响应 DTO 使用 `id` 表示资源主键，使用 `name` 表示展示名称。

示例：

```go
type CreateRateLimitPolicyReq struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Enabled       bool                   `json:"enabled"`
	Mode          resource.RateLimitMode `json:"mode"`
	Rules         []RateLimitRuleReq      `json:"rules"`
	Global        *GlobalRateLimitReq     `json:"global,omitempty"`
	Response      RateLimitResponseReq    `json:"response,omitempty"`
	FailurePolicy string                 `json:"failurePolicy,omitempty"`
}
```

DTO `Validate` 只做字段存在性、枚举、数字范围和跨字段约束：

- name 必填
- mode 必填
- rules 至少一条
- requests > 0
- windowSeconds > 0
- Header/Query/Cookie key 必须填写 name
- `mode=Global` 必须填写 `global.redisRef`
- `mode=Local` 不能填写 Redis 配置

系统状态校验放在 service：

- 同类资源 name 唯一
- `redisRef` 存在
- `PolicyBinding.targetID` 存在
- `PolicyBinding.policies` 引用存在
- 删除 policy 时如果仍被 binding 引用，返回 `UserError`
- 删除 RedisStore 时如果仍被 global policy 引用，返回 `UserError`

## Compiler 和 IR

IR 不暴露 Redis 密码，也不暴露 admin-api DTO。

建议 IR：

```go
type LogicalRateLimitPolicy struct {
	Name          string
	DisplayName   string
	Mode          resource.RateLimitMode
	Rules         []LogicalRateLimitRule
	Global        *LogicalGlobalRateLimit
	Response      LogicalRateLimitResponse
	FailurePolicy resource.RateLimitFailurePolicy
}

type LogicalRateLimitRule struct {
	Name      string
	Key       []LogicalRateLimitKeyPart
	Limit     LogicalRateLimitQuota
	Algorithm resource.RateLimitAlgorithm
}

type LogicalPolicyTarget struct {
	Kind     resource.Kind
	Name     string
	RuleName string
}
```

compiler 负责：

- 建立 policy、binding、redis store 索引
- 校验 binding 引用存在
- 校验 route rule target 存在
- 过滤 disabled policy 和 disabled binding
- 输出与目标 Gateway 有关的 policy、binding 和 redis store

compiler 不负责生成插件 JSON。插件配置属于 target translator 的职责。

## xDS target 和内置插件

xDS target 把 IR 翻译成 managed rate-limit plugin 配置。

内部配置建议：

```go
type ManagedRateLimitPlugin struct {
	Bindings []RateLimitPluginBinding `json:"bindings"`
	RedisStores []RateLimitRedisStore `json:"redisStores,omitempty"`
}
```

每条 binding 带目标匹配条件和策略引用展开后的规则。这样数据面插件不需要理解 Ingate 的全量资源模型，只消费可执行配置。

local mode：

- 插件使用本地内存计数器
- 适合单实例或允许每实例独立配额的场景
- 不依赖 RedisStore

global mode：

- 插件按 RedisStore 建立连接池
- 使用稳定 key 前缀：环境前缀、gateway、route、rule、policy、rate rule、key parts
- 请求进入时计算 key，向 Redis 原子更新计数
- 超限时返回策略定义的响应

失败处理：

- Redis 超时、连接失败、命令失败按 `FailurePolicy` 处理
- `FailOpen` 放行并记录指标
- `FailClose` 拒绝并记录指标

插件必须暴露基础指标：

- 命中次数
- 超限次数
- Redis 错误次数
- FailOpen 放行次数
- FailClose 拒绝次数

## 和 Plugin 资源的关系

内置限流插件不是用户创建的 `Plugin` 资源。

原因：

- 限流是核心治理能力，需要强类型资源和明确 admin-api
- 用户不应该看到或维护插件版本、执行阶段、私有 JSON
- compiler 可以根据 policy/binding 自动生成 managed plugin 配置

未来如果用户自定义限流插件，可以另走 `Plugin + PluginBinding`，但不影响核心 `RateLimitPolicy` 模型。

## 实施顺序

建议按以下顺序实施：

1. 更新 `RateLimitPolicy`、`PolicyBinding`、`Bundle` 和 generated 代码
2. 新增 `RedisStore` 资源、apiserver registry、client、lister、informer 和 store 入口
3. 更新 compiler 索引、引用校验、RouteRule 级 target 支持和 IR
4. 更新 controller，把相关 RedisStore、PolicyBinding、RateLimitPolicy 收集进 Gateway bundle
5. 更新 xDS target，输出 managed rate-limit plugin 配置
6. 更新 xDS server，把 managed rate-limit plugin 注入 HTTP filter 链和 route/rule 匹配配置
7. 新增 admin-api RateLimitPolicy CRUD
8. 新增 admin-api PolicyBinding CRUD
9. 新增 admin-api RedisStore CRUD
10. 后续再做前端页面和联调

本专题先改后端和编译链路。前端在用户明确要求前不改。
