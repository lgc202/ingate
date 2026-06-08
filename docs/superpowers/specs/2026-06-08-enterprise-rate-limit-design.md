# 企业级 RateLimit 专题设计

## 背景

Ingate 已经具备 `RateLimitPolicy`、`PolicyBinding`、`RedisStore`、compiler、xDS target 和内置限流 Wasm 插件的基础链路。当前已验证的能力是：

- `Local` 模式可通过 Envoy shared data 在单 Envoy 实例多 worker 间共享计数
- `RateLimitPolicy + PolicyBinding` 可以编译为内置限流插件配置
- all-in-one 镜像可以加载插件，真实 Envoy 请求能返回 200 / 429
- xDS 层可以自动注入限流 HTTP filter

但 `Global` 模式还没有完整闭环。当前插件只能把 global 规则转换成 `GlobalCheck`，没有真正访问 Redis。标准 Proxy-Wasm Go SDK 不提供 HTTP filter 主动拨 Redis TCP 的能力，Higress 的 Redis 能力依赖它自己的 wrapper / host function，不能作为 Ingate 的默认运行时假设。

本专题要把 RateLimit 当作企业级治理能力一次做完整，不做临时方案，不把后续必需能力留成需要推倒重来的结构。

## 目标

- `Local` 和 `Global` 两种模式都真实可用
- `Global` 模式通过 Redis 实现跨 Envoy 实例全局计数
- Redis 支持 Standalone、Sentinel、Cluster、TLS、认证、DB、超时、连接池和故障处理
- 限流算法支持 FixedWindow、SlidingWindow、TokenBucket
- 插件、数据面服务、xDS 配置协议有明确版本和兼容边界
- Admin API、控制台、状态、日志、指标、E2E 都围绕最终产品能力闭环
- 用户只创建 `RateLimitPolicy`、`PolicyBinding`、`RedisStore`，不手工安装插件、不配置 Envoy cluster、不维护插件私有 JSON

## 非目标

- 不实现独立对外售卖的 rate-limit SaaS
- 不把 Higress wrapper 作为运行时依赖
- 不把 Redis 密码明文暴露给前端或插件日志
- 不把限流策略塞回 Route 原生字段
- 不让前端直接编辑 Kubernetes 风格资源对象或 Wasm 插件配置

## 企业级完整能力清单

### 策略模型

- `RateLimitPolicy` 支持 `Local` 和 `Global`
- `RateLimitPolicy` 支持多条 rule
- rule 支持稳定名称、算法、额度、窗口、burst 参数
- key 支持组合维度：
  - Gateway
  - Route
  - RouteRule
  - Header
  - Query
  - Cookie
  - Consumer
  - IP
  - JWT claim
  - API key
  - AI model
  - Tenant
- key 生成规则版本化，避免升级后同一策略生成不同 Redis key
- 多策略命中规则明确：任一策略拒绝则拒绝
- 多 rule 命中规则明确：任一 rule 拒绝则拒绝
- quota header 合并规则明确：返回最先耗尽或剩余额度最低的规则
- `FailurePolicy` 支持 `FailOpen` 和 `FailClose`
- `Response` 支持状态码、body、content-type、自定义响应头、quota header 开关

### RedisStore

- 支持 Standalone
- 支持 Sentinel
- 支持 Cluster
- 支持 TLS
- 支持 username/password
- 支持 DB
- 支持连接超时、命令超时、最大连接数、最小空闲连接数
- 支持连接测试
- 支持 SecretRef / CredentialRef
- 支持健康状态和最近错误
- 支持删除保护：被 global policy 引用时不能删除

### 数据面执行

- Local 模式使用 Envoy shared data 保存单实例计数
- Local 模式有过期窗口清理和内存上限保护
- Global 模式通过内置 Redis 数据面服务访问 Redis
- FixedWindow 使用 Redis 原子命令或 Lua 保证计数和过期时间一致
- SlidingWindow 使用 sorted set / Lua 保证窗口计算一致
- TokenBucket 使用 Lua 原子更新 token 状态
- Redis 超时、连接错误、脚本错误按 `FailurePolicy` 决策
- Redis Cluster 支持 slot 路由、MOVED、ASK
- Redis Sentinel 支持主节点发现和故障切换
- 数据面服务支持连接复用、热更新、关闭旧连接
- 数据面服务不能阻塞 Envoy worker 过久，所有远程调用都有硬超时

### xDS 集成

- 自动注入限流 Wasm filter
- 自动注入插件调用 ingate-dataplane所需的 internal cluster
- 自动下发插件配置、RedisStore 引用和数据面服务协议配置
- 多 Gateway、多 Listener、多 RuntimeGroup 配置隔离
- 同一 listener 上多个 wildcard route 生成合法 Envoy VirtualHost
- Envoy NACK、Wasm 加载失败、配置解析失败能被观测并回写状态
- 配置有 schema version，插件拒绝未知不兼容版本

### Admin API 和前端

- `RedisStore` CRUD 和连接测试
- `RateLimitPolicy` CRUD、启停、引用校验
- `PolicyBinding` CRUD、启停、绑定目标校验
- DTO 不泄漏插件私有 JSON
- 表单能配置算法、key 维度、额度、窗口、失败策略、超限响应
- 详情页展示策略生效状态、最近错误、绑定目标、执行统计
- 后端业务错误使用 `UserError` 返回可展示文案
- 操作失败用 toast / dialog 告知，不让用户猜

### 可观测性

- 指标：
  - 请求允许数
  - 请求拒绝数
  - Redis 成功数
  - Redis 错误数
  - Redis 超时数
  - 数据面服务延迟
  - 插件配置版本
  - 策略 / 绑定 / 路由维度统计
- 日志：
  - 配置加载失败
  - Redis 连接失败
  - Redis 命令失败
  - failOpen / failClose 决策
  - 超限拒绝摘要
- Debug：
  - 当前插件配置版本
  - 当前数据面服务配置版本
  - RedisStore 健康状态
  - 最近错误列表

### 测试和交付

- 单元测试覆盖 key 生成、算法、响应选择、失败策略
- 插件测试覆盖 local/global 分支和异步回调
- 数据面服务测试覆盖 Redis standalone / sentinel / cluster
- all-in-one E2E 覆盖 Envoy + Wasm + Redis
- 故障测试覆盖 Redis down、超时、错误密码、cluster 重定向
- 并发测试覆盖多 worker、多 Envoy 实例共享 global quota
- 镜像构建包含插件、数据面服务和默认配置
- 文档包含配置示例、排障手册和状态说明

## 总体架构

```text
Admin API / Console
  |
  v
RateLimitPolicy + PolicyBinding + RedisStore
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
  +--> Envoy Listener / Route / Cluster
  |
  +--> RateLimit Plugin Config
  |
  +--> RateLimit DataPlane Cluster
          |
          v
Envoy HTTP filter chain
  |
  v
ratelimit.wasm
  |
  +--> Local counter: Envoy shared data
  |
  +--> Global counter: DispatchHttpCall
          |
          v
    ingate-dataplane
          |
          v
        Redis
```

## 核心设计决策

### 不让 Wasm 插件直接连接 Redis

标准 Proxy-Wasm Go SDK 没有提供 HTTP filter 主动建立任意 TCP 连接的能力。Wasm 插件可以使用 `DispatchHttpCall` 调用 Envoy 已知 cluster，也可以读写 shared data / shared queue，但不能像普通 Go 进程一样 `net.Dial("tcp", redis)`。

因此，Redis 访问必须放到 Ingate 自己的数据面执行边界中。这个边界不是用户手动部署的外部服务，而是 Ingate 数据面内置治理数据面服务，由 xDS 自动注入访问 cluster。

这样做的收益：

- 不依赖 Higress 专属 host function
- 不把 Redis client、连接池、TLS、Cluster slot 等复杂能力塞进 Wasm
- Wasm 插件保持轻量和稳定
- Redis 数据面服务可以用普通 Go 运行时和成熟 Redis client
- 后续可以统一承载鉴权、配额、AI token 预算等需要外部状态的治理能力

### 数据面服务是内置数据面组件，不是用户级外部服务

`ingate-dataplane` 是 Ingate 数据面的内部组件。部署形态由 RuntimeGroup 决定：

- all-in-one：作为同容器进程启动
- 单机数据面：作为本机进程或 sidecar
- Kubernetes 数据面：作为同 Pod sidecar 或 DaemonSet 内部服务

用户不需要配置它。控制面负责把它作为 Envoy cluster 注入，插件通过固定 cluster 名称调用。

### 插件和数据面服务协议版本化

插件调用 ingate-dataplane使用 Ingate 自定义协议，协议必须版本化。建议使用 JSON 作为 Wasm 到数据面服务的传输格式，原因是：

- 当前插件已经使用 JSON 配置
- Go WASI 下 JSON 成本可接受
- 便于调试 config dump 和数据面服务日志

协议字段必须稳定，不能直接复用内部 Go struct。

请求：

```json
{
  "checks": [
    {
      "policyName": "uuid",
      "ruleName": "per-route",
      "redisStore": {
        "id": "redis-main",
        "mode": "Standalone",
        "address": "127.0.0.1:6379",
        "db": 0
      },
      "redisKey": "ingate-rate-limit:...",
      "algorithm": "FixedWindow",
      "limit": {
        "requests": 100,
        "windowSeconds": 60,
        "burst": 0
      },
      "timeoutMillis": 50
    }
  ]
}
```

响应：

```json
{
  "results": [
    {
      "policyName": "uuid",
      "ruleName": "per-route",
      "allowed": false,
      "current": 101,
      "limit": 100,
      "remaining": 0,
      "resetSeconds": 42,
      "error": ""
    }
  ]
}
```

## 请求执行流程

### Local 模式

```text
OnHttpRequestHeaders
  |
  v
识别当前 route identity
  |
  v
读取插件配置中对应 route bindings
  |
  v
生成 composite key
  |
  v
Envoy shared data CAS 更新 fixed window
  |
  +--> allowed: Continue
  |
  +--> limited: SendHttpResponse(429)
```

Local 模式是单 Envoy 实例内限流，不承诺跨实例一致。跨实例一致必须使用 Global 模式。

### Global 模式

```text
OnHttpRequestHeaders
  |
  v
识别当前 route identity
  |
  v
生成 GlobalCheck 列表
  |
  v
DispatchHttpCall 到 ingate-dataplane
  |
  v
Pause 请求
  |
  v
数据面服务访问 Redis 并返回决策
  |
  +--> allowed: ContinueHttpRequest
  |
  +--> limited: SendHttpResponse(429)
  |
  +--> error: 按 FailurePolicy 决策
```

插件必须保证每个请求最多有一个未完成数据面服务调用。回调返回后必须清理请求上下文，避免泄漏。

## Redis 算法

### FixedWindow

Redis key：

```text
{prefix}:{gateway}:{route}:{rule}:{binding}:{policy}:{policyRule}:{keyHash}:{windowStart}
```

执行方式：

- 使用 Lua 保证 `INCR` 和 `PEXPIRE` 原子化
- 首次创建 key 时设置过期时间为窗口剩余时间加保护余量
- 返回 `current`、`remaining`、`resetSeconds`

### SlidingWindow

Redis key：

```text
{prefix}:sw:{scopeHash}
```

执行方式：

- 使用 sorted set 保存请求时间戳
- Lua 内删除窗口外成员、统计窗口内成员、按额度判断、写入当前请求
- 设置 key TTL 为窗口长度加保护余量

### TokenBucket

Redis key：

```text
{prefix}:tb:{scopeHash}
```

执行方式：

- 使用 hash 保存 `tokens` 和 `lastRefillUnixMillis`
- Lua 内按时间补充 token，再判断是否允许消耗
- 返回剩余 token 和下次可用时间

## RedisStore 设计

当前模型需要扩展为：

```go
type RedisStoreSpec struct {
	DisplayName          string
	Description          string
	Mode                 RedisMode
	Address              string
	Addresses            []string
	DB                   int
	TLS                  bool
	TLSServerName        string
	Username             string
	PasswordRef          string
	ConnectTimeoutMillis int
	CommandTimeoutMillis int
	MaxConns             int
	MinIdleConns         int
	Sentinel             *RedisSentinelConfig
	Cluster              *RedisClusterConfig
}
```

`Address` 用于 standalone 简化配置，`Addresses` 用于 sentinel / cluster。DTO 可以做得更产品化，但底层资源要能表达完整部署形态。

## 状态模型

RateLimit 需要状态回写，不能只靠日志排障。

### RedisStore status

- `Ready`
- `ConnectionFailed`
- `AuthFailed`
- `TLSFailed`
- `ClusterSlotsFailed`
- `SentinelMasterNotFound`
- `LastError`
- `LastCheckedAt`

### RateLimitPolicy status

- `Accepted`
- `InvalidReference`
- `UnsupportedAlgorithm`
- `NotBound`
- `Programmed`
- `LastError`

### PolicyBinding status

- `Accepted`
- `TargetNotFound`
- `PolicyNotFound`
- `Programmed`
- `LastError`

### Runtime status

- xDS 是否 ACK
- Wasm 是否加载成功
- 数据面服务 cluster 是否可达
- RedisStore 是否健康

## xDS 输出要求

xDS target 需要输出：

- ratelimit Wasm HTTP filter
- Wasm plugin config
- dataplane cluster
- dataplane endpoint
- RedisStore 信息进入数据面服务配置，而不是进入用户可见 route

cluster 命名建议：

```text
ingate-dataplane
```

插件内固定使用这个 cluster 名，xDS 保证 cluster 存在。没有 global 策略时可以不注入 dataplane cluster。

## 数据面服务设计

数据面服务是普通 Go 进程，职责只包含：

- 接收插件检查请求
- 维护 Redis client 池
- 按 RedisStore 配置热更新 client
- 执行 Lua / Redis 命令
- 返回限流决策
- 暴露健康检查和指标

数据面服务不负责：

- 拉取控制面资源
- 判断请求命中了哪个 Route
- 解析 PolicyBinding
- 生成业务 key
- 写 HTTP 超限响应

这些职责保留在控制面编译链路和 Wasm 插件内。

## 错误处理

### 插件错误

- 插件配置解析失败：插件启动失败，Envoy 按 filter failure policy 处理
- 当前 route 找不到配置：放行并计数 `config_miss`
- key 维度缺失：该 rule 不生效，记录 debug 级别统计
- shared data CAS 重试耗尽：按 policy failure policy
- dataplane 调用超时：按 policy failure policy
- dataplane 返回协议错误：按 policy failure policy

### 数据面服务错误

- RedisStore 不存在：返回 per-check error
- Redis 连接失败：返回 per-check error
- Redis 认证失败：返回 per-check error，并更新 RedisStore status
- Lua 脚本失败：返回 per-check error
- Cluster MOVED / ASK：数据面服务内部处理，失败后返回 error

### failOpen / failClose

- `FailOpen`：内部错误时放行，但必须记录指标和日志
- `FailClose`：内部错误时拒绝，请求响应使用策略的超限响应或系统错误响应

## 安全

- Redis 密码只通过 `PasswordRef` / SecretRef 进入数据面服务
- 插件配置不包含密码明文
- Admin API 响应不返回密码明文
- 日志中 Redis 地址可以打印，用户名和密码不可打印
- TLS Redis 必须支持 server name
- 数据面服务内部接口只对 Envoy / 本机可达，不暴露公网

## 兼容和迁移

这是新项目，不保留历史兼容代码。已有早期字段如果不符合最终模型，直接迁移到最终模型。

必须避免：

- 保留 Higress wrapper 依赖
- 保留 global limit 的占位执行路径
- 保留只支持 FixedWindow 的协议结构
- 保留无法表达 Sentinel / Cluster 的 RedisStore 模型

## 开发拆分

虽然专题目标是完整企业级能力，但实现可以按依赖顺序推进。每一步完成后都必须保持主干可构建、可验证。

### 1. 模型和文档定稿

- 更新 `RateLimitPolicy`
- 更新 `RedisStore`
- 更新 DTO 和编码规范
- 明确状态字段
- 明确插件-数据面服务协议

### 2. 数据面服务骨架

- 新增 `ingate-dataplane`
- 实现 HTTP API
- 实现配置加载和热更新接口
- 实现健康检查和指标基础

### 3. Redis 完整能力

- Standalone client
- Sentinel client
- Cluster client
- TLS / auth / DB
- 连接池和超时
- 连接测试

### 4. 算法

- FixedWindow
- SlidingWindow
- TokenBucket
- Redis Lua 脚本和 SHA 缓存
- 错误和超时行为

### 5. Wasm 插件 global 执行

- DispatchHttpCall
- 请求 pause / resume
- dataplane response 解析
- quota header 合并
- failOpen / failClose

### 6. xDS 注入

- dataplane cluster 注入
- dataplane endpoint 注入
- global 策略存在时自动启用
- config dump 可读

### 7. 状态回写

- RedisStore status
- RateLimitPolicy status
- PolicyBinding status
- Runtime / xDS ACK 状态

### 8. Admin API 和前端

- RedisStore 连接测试
- 策略表单最终字段
- 绑定表单最终字段
- 状态展示和错误提示

### 9. E2E 和故障验证

- all-in-one + Redis standalone
- Redis down
- Redis timeout
- wrong password
- failOpen
- failClose
- 多 Envoy 实例 global quota
- Sentinel failover
- Cluster MOVED / ASK

## 验收标准

专题完成时必须满足：

- `Local` 模式能在多 worker Envoy 中稳定限流
- `Global` 模式能跨多个 Envoy 实例共享 Redis 计数
- Standalone / Sentinel / Cluster Redis 都有 E2E 或集成验证
- FixedWindow / SlidingWindow / TokenBucket 都有算法测试和 Redis 集成测试
- Redis 故障时 failOpen / failClose 行为和指标正确
- xDS 无 NACK，Envoy 能加载 Wasm 和 dataplane cluster
- Admin API 不暴露内部错误和敏感信息
- 控制台能配置完整策略并看到状态
- `make test`、`make build`、插件构建、all-in-one E2E 全部通过

## 当前缺口

当前代码离本设计还缺：

- Global 模式真正调用 ingate-dataplane
- `ingate-dataplane`
- Redis Standalone / Sentinel / Cluster client
- Redis TLS / auth / DB / connection pool
- SlidingWindow / TokenBucket
- 插件异步 `DispatchHttpCall` 流程
- dataplane cluster 自动注入
- RedisStore 连接测试
- SecretRef 真正取密钥
- 状态回写
- 指标
- 前端完整配置和状态展示
- Redis 故障 E2E
- 多 Envoy 实例 global quota E2E
