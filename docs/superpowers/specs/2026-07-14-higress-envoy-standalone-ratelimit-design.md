# Higress Envoy Standalone Redis 限流设计

## 背景

Ingate 当前使用原生 Envoy 处理网关流量，同时运行独立的 ingate-dataplane 进程。内置 RateLimit Wasm 插件通过标准 Proxy-Wasm DispatchHttpCall 调用该进程，由 ingate-dataplane 使用 go-redis 执行 Redis Lua 脚本。

当前链路为：

    RateLimitPolicy + RedisStore
      -> Compiler
      -> Logical IR
      -> xDS target
      -> RuntimeSnapshot
      -> Envoy + ratelimit.wasm
      -> HTTP/JSON
      -> ingate-dataplane
      -> Redis

Higress Envoy 已扩展 Proxy-Wasm Redis hostcall，支持 RedisInit、DispatchRedisCall 和 EVAL 所需的原始 RESP 命令调用。Ingate 可以把 Redis 执行能力迁入 Envoy 和 Wasm 的运行时边界，删除额外的数据面进程。

已经验证官方 Higress gateway:v2.2.3 镜像包含 Envoy 1.36.4，二进制位于 /usr/local/bin/envoy，当前 Ingate bootstrap 可以通过该二进制的配置校验。第一阶段使用未经修改的官方 Higress Envoy，不维护 Envoy fork。

Higress proxy-wasm-go-sdk 的 v1.0.1 tag 尚未实现 Redis callout 测试驱动。本阶段固定到 Go pseudo-version v0.0.0-20260525073613-662ed045bf0b（commit 662ed045bf0b58eb0ab67ba9954ae2fa65343072）或包含同等 Redis proxytest 能力的后续稳定版本，不使用缺少 callout ID、调用记录和 callback 驱动的旧 tag。

## 决策

第一阶段只支持 Standalone Redis，并直接删除 ingate-dataplane。

Sentinel 和 Cluster 不走旧进程 fallback，也不静默退化为 Standalone。xDS target 遇到当前阶段无法等价执行的 RedisStore 配置时返回明确错误。后续阶段在 Higress 能力得到验证或补齐后恢复这些模式。

PasswordRef 继续保持现有占位语义，本阶段不实现新的 Secret 系统。为避免产生看似可用但实际无法认证的配置，xDS target 遇到非空 PasswordRef 时返回明确错误。本地开发和第一阶段 E2E 使用无认证 Redis。

## 目标

- 将 all-in-one 数据面切换到官方 Higress Envoy
- 使用 Higress Redis hostcall 执行 Global RateLimit
- 保持 FixedWindow、SlidingWindow 和 TokenBucket 三种算法
- 保持 GlobalCheck 顺序、fail-open / fail-close、拒绝响应和配额响应头语义
- 支持 Standalone Redis 的 DB、TLS、连接超时和命令超时
- 删除 ingate-dataplane 的二进制、代码、HTTP 协议、部署入口和内部 Envoy cluster
- 删除只服务于 ingate-dataplane 的 go-redis 依赖
- 通过单元测试、Wasm callback 测试和真实 Higress Envoy + Redis E2E 验证链路
- 完成 make test 和 make build

## 非目标

- 本阶段不支持 Redis Sentinel
- 本阶段不支持 Redis Cluster
- 本阶段不实现 PasswordRef 的 Secret 解析和轮换
- 本阶段不修改 Higress Envoy 源码
- 本阶段不引入 Higress Pilot、Higress Controller 或 Higress 产品资源
- 本阶段不把 Higress wrapper、matchRules 或私有 JSON 暴露到 Ingate 产品模型
- 本阶段不顺带实现 Cache、AI quota 或其它数据面 capability

## 方案比较

### 方案一：Ingate runtime adapter 直接使用 Higress Proxy-Wasm SDK

这是采用方案。

插件通过 Ingate 自己的 Redis runtime adapter 调用 Higress Proxy-Wasm SDK 的 RedisInit、DispatchRedisCall 和 GetRedisCallResponse。RESP 编解码、Lua 脚本、结果映射和错误分类由 Ingate 维护。

优点：

- Higress 依赖被限制在插件运行时边界
- 不依赖 Higress wasm-go wrapper 的产品配置和生命周期抽象
- 后续可以为其它 runtime target 提供不同 adapter
- 可以精确保持现有 RateLimit 行为

代价：

- Ingate 需要维护少量 RESP 编解码和异步调用编排
- 内置 Wasm 插件需要统一切换到 Higress 的 Proxy-Wasm SDK fork

### 方案二：直接使用 Higress wasm-go Redis wrapper

该方案能减少初始代码，但会把 wrapper 的 Cluster 类型、配置习惯和回调模型扩散到插件 runtime。它会模糊 Ingate runtime 边界，也增加后续支持其它 target 的迁移成本，因此不采用。

### 方案三：在 Envoy 内配置本地 Redis proxy listener

该方案更适合后续研究 Redis Cluster 路由，Standalone 阶段会额外引入 listener、network filter 和本地回环连接，复杂度与收益不匹配，因此不采用。

## 总体架构

迁移后的链路为：

    RateLimitPolicy + RedisStore
      -> Compiler
      -> Logical IR
      -> xDS target
          -> Redis runtime client
          -> Redis Envoy cluster
          -> RateLimit plugin config v2
      -> RuntimeSnapshot
      -> Higress Envoy
      -> ratelimit.wasm
      -> Ingate Redis runtime adapter
      -> Higress Redis hostcall
      -> Redis

Resource、Compiler 和 Logical IR 继续表达 Ingate 网关领域语义。只有 xDS target、xDS server 和插件 runtime 感知 Higress Redis hostcall。

## 数据模型边界

### 核心资源和 Logical IR

RedisStore、RateLimitPolicy 和 PolicyBinding 的产品模型保持不变。Compiler 继续解析引用关系并把 RedisStore 放入 Logical IR，不在核心编译层判断某个 target 是否支持 Sentinel 或 Cluster。

### xDS target

xDS target 新增 Redis runtime client 和 Redis cluster 的目标配置。

Redis runtime client 至少包含：

- 稳定的 client name
- RedisStore ID
- Envoy cluster name
- DB
- Username
- 生效的 command timeout

Redis cluster 至少包含：

- 稳定的 cluster name
- host 和 port
- connect timeout
- TLS 开关
- TLS server name

一个 RedisStore 可以产生多个 runtime client。原因是 Higress RedisInit 的命令超时绑定到 Envoy cluster 的 thread-local Redis client，而 RateLimitPolicy 可以覆盖 RedisStore 的命令超时。xDS target 按 RedisStore ID 和生效 command timeout 生成独立 cluster name，避免不同策略重配同一个共享 client。

稳定 identity 使用规范化字段的 SHA-256：

    identity input = RedisStore ID、effective address、DB、Username、TLS、
                     effective TLS server name、connect timeout、command timeout
    digest = SHA-256(identity input)
    Redis runtime client = ingate.redis.client.<完整十六进制 digest>
    Envoy cluster = ingate.redis.cluster.<完整十六进制 digest>

identity input 使用固定字段顺序和无歧义长度编码。RedisStore 配置变化时产生新名称，使多个 Gateway 的 RuntimeSnapshot 在逐步收敛期间可以同时携带新旧 cluster；相同 RedisStore ID 和相同有效配置会稳定去重。不同 RedisStore ID 即使连接配置相同也保持独立 identity，避免一个资源的后续修改影响另一个资源。固定前缀把 Redis runtime 资源与普通 Upstream 名称区分开，完整 digest 避免有损字符替换导致的静默碰撞。这些名称只属于 xDS target，不进入 Admin API 或核心资源协议。

xDS server 构建 CDS 时必须检查全局名称冲突：

- 同名且 protobuf 内容相同的 cluster 可以去重
- 同名但内容不同的 cluster 必须返回错误
- 不再沿用 first-wins 的静默去重行为

### 插件配置

RateLimit 插件配置 schema 从 v1 升级为 v2。

v2 不再包含 DataPlane、HTTP path 或 ingate-dataplane cluster。Global policy 的 RedisRef 在 target translation 后解析成 runtime client ref。插件只根据该引用找到 cluster、DB、用户名和命令超时。

插件配置不携带 Redis 地址和 TLS 私有细节；这些由 Envoy cluster 承载。

## xDS target 校验

以下校验属于 Higress xDS target 能力边界，不进入核心 Compiler：

- Mode 为空时按 Standalone 兼容处理
- Mode 为 Sentinel 或 Cluster 时返回明确不支持错误
- Addresses 为空时使用 Address
- Addresses 只有一个元素时使用该元素
- Addresses 多于一个元素时返回明确错误
- 最终地址必须是有效的 host:port
- PasswordRef 非空时返回明确错误
- PoolSize 或 MinIdleConns 非零时返回明确错误，因为 Higress hostcall 不能等价映射这些参数
- DB 必须在 0 到 math.MaxInt32 范围内
- ConnectTimeoutMillis 和 CommandTimeoutMillis 必须在 0 到 math.MaxUint32 范围内
- RateLimitPolicy Global.TimeoutMillis 必须在 0 到 math.MaxUint32 范围内
- 地址使用 net.SplitHostPort 解析，端口必须在 1 到 65535 范围内，IPv6 必须使用带方括号形式

错误必须包含 RedisStore ID 和不支持的字段或模式，方便 controller 状态和日志定位。

## 超时

连接超时由 Envoy cluster connect_timeout 承载。

- ConnectTimeoutMillis 大于零时使用用户值
- 未配置时使用五秒，与当前 go-redis 默认建连超时一致

命令超时由 Higress RedisInit 的 timeout 承载。

- Policy Global.TimeoutMillis 大于零时优先使用
- 否则使用 RedisStore.CommandTimeoutMillis
- 两者都未配置时使用 50ms，与当前 ingate-dataplane 默认值一致

RedisInit 使用独立的 runtime cluster name 初始化 client。为避免 Higress SDK 默认的短时请求缓冲增加限流延迟，runtime adapter 显式设置：

    buffer_flush_timeout=0
    max_buffer_size_before_flush=0

DB 通过 RedisInit cluster query 参数传递。

## TLS

TLS 由 xDS server 在 Redis Envoy cluster 上生成 upstream TLS transport socket。

- TLSServerName 非空时作为 effective TLS server name
- TLSServerName 为空时从 Redis 地址 host 派生 effective TLS server name
- effective TLS server name 是 DNS 名称时，UpstreamTlsContext.Sni 使用该名称，并生成 DNS 类型的精确 SAN matcher
- DNS SAN matcher 以请求的完整主机名为 exact 值，由 Envoy 执行标准 DNS 主机名校验，包括合法的通配符证书匹配
- effective TLS server name 是 IPv4 或 IPv6 地址时，不发送 SNI，并生成 IP_ADDRESS 类型的精确 SAN matcher
- CommonTlsContext.ValidationContext.TrustedCa 使用 all-in-one 中的系统 CA bundle
- MatchTypedSubjectAltNames 必须按上述 DNS 或 IP 类型匹配 effective TLS server name，不能只配置 SNI 或只验证 CA 链
- TLS 配置不进入插件 JSON

TLS 测试必须包含配置单元测试和真实连接测试。真实连接测试至少覆盖受信任且 SAN 匹配时成功，以及 SAN 不匹配时连接被拒绝。

## 插件运行时

### SDK 边界

内置插件统一使用 github.com/higress-group/proxy-wasm-go-sdk，避免共享 helper 和不同插件继续分裂在两个 Proxy-Wasm SDK 上。迁移范围包括 ratelimit、acl 和 plugins/internal/wasm。

Ingate 不使用 Higress wasm-go/pkg/wrapper。Higress SDK import 只出现在 Wasm 生命周期适配和 Redis runtime adapter 中。

### Redis runtime adapter

新增 plugins/ratelimit/internal/redis，职责包括：

- 根据插件 v2 配置初始化 runtime clients
- 构造 EVAL RESP 命令
- 调用 DispatchRedisCall
- 读取并解析 Redis RESP 返回
- 把结果转换成 Ingate 的 GlobalResult
- 分类同步 dispatch、网络、超时、Redis error 和返回格式错误

原 pkg/dataplane/ratelimit 的跨进程 DTO 不再存在。Policy 层改用插件内部的纯领域结果类型，不依赖 HTTP/JSON 协议。

### Lua 算法

FixedWindow、SlidingWindow 和 TokenBucket 的 Lua 脚本迁入 Redis runtime adapter。脚本内容和结果字段保持现有实现一致。

算法层不调用 Proxy-Wasm API，只负责：

- 构造脚本参数
- 解析 RESP array
- 计算 Allowed、Current、Limit、Remaining、ResetSeconds 和 RetryAfterSeconds

这样脚本结果映射可以作为普通 Go 代码单独测试。

## 请求执行流程

当前 ingate-dataplane 按 CheckRequest.Checks 顺序逐条执行。迁移后继续保持串行语义，不能为了降低延迟改成并行。

请求流程：

1. Wasm 根据当前 xDS route identity 找到 RouteConfig
2. Policy runner 执行 local limit，并生成有序 GlobalCheck
3. 没有 GlobalCheck 时直接执行现有动作
4. 有 GlobalCheck 时 HTTP 请求进入 Pause
5. HTTP context 创建一次 global execution，保存 checks、results 和当前索引
6. runtime adapter 对当前 check 执行一条 EVAL
7. callback 将结果写入相同索引，然后执行下一条 check
8. 所有 checks 完成后调用 CompleteGlobalChecks
9. 如果结果要求拒绝，发送现有拒绝响应
10. 否则保存 quota headers 并 ResumeHttpRequest

串行执行保证：

- 结果和 checks 一一对应
- 同一个请求中的相同 Redis key 保持原有更新顺序
- 第一个拒绝结果和 quota header 选择逻辑不变

HTTP context 销毁后不再恢复请求。callback 必须检查 execution 是否仍有效，并防止重复完成。

## 错误处理

配置和 target 能力错误在 translation 阶段返回，不生成不可执行的 RuntimeSnapshot。

请求执行错误按单条 GlobalCheck 记录：

- 参数和 runtime client 引用错误：InvalidRequest
- 不支持的算法：UnsupportedAlgorithm
- 超时：Timeout
- 网络、Redis error 和协议错误：RedisError

Higress callback 不能提供足够细的所有底层错误时，runtime adapter 保留稳定的上层分类，不依赖错误文本做 Policy 分支。

Policy 行为保持不变：

- FailOpen 的单条检查错误不阻止请求
- FailClose 的单条检查错误返回该策略配置的拒绝响应
- 整体执行无法完成时拒绝第一个 FailClose check
- Redis 明确返回超限时始终按策略拒绝
- QuotaHeaderEnabled 继续控制 X-RateLimit-Limit、Remaining 和 Reset

用户响应不暴露 Redis 地址、cluster name、底层错误或堆栈。

## 部署

all-in-one Dockerfile 使用官方 Higress gateway:v2.2.3 作为 Envoy binary source，只复制 /usr/local/bin/envoy 到最终 Debian 镜像。

Ingate 继续使用自己的 bootstrap、ADS server、RuntimeSnapshot 和插件目录，不启动 Higress Pilot 或其它 Higress 控制面组件。

删除：

- cmd/ingate-dataplane
- internal/dataplane
- pkg/dataplane/ratelimit
- INGATE_DATAPLANE_ADDR
- entrypoint 中的 dataplane 启动和健康等待
- all-in-one 中的 ingate-dataplane binary
- xDS target 中的 127.0.0.1:18081 internal cluster
- 仅由该进程使用的 go-redis 依赖和 pkg/xredis

ratelimit.wasm 和 acl.wasm 继续安装在 /opt/ingate/plugins。

## 测试

### 普通 Go 单元测试

- 三套算法的参数和 RESP 结果映射
- Redis 地址归一化
- 生效 connect timeout 和 command timeout
- runtime client identity 稳定性
- 非 Standalone、多个地址、PasswordRef 和连接池参数的 target 错误
- GlobalResult 的 fail-open / fail-close、拒绝顺序和 quota headers

### Proxy-Wasm 测试

使用固定 SDK revision 中已经实现 Redis callout ID、调用记录和 callback 驱动的 proxytest host：

- RedisInit 输入由纯配置构造测试覆盖，真实调用由 E2E 覆盖
- EVAL RESP 命令正确
- 有 global checks 时 Pause
- callback 后按顺序发送下一条 Redis call
- 全部成功后 Resume
- 超限时发送拒绝响应
- 同步 dispatch 失败和 callback 失败按 failure policy 处理
- HTTP context 已销毁时不重复完成

如果选定的后续稳定 SDK release 再次缺少上述 proxytest 能力，不能为此在生产代码中增加只为测试存在的接口；应继续固定已验证 revision，或把缺失能力贡献到 Higress SDK。

### xDS 测试

- Standalone Redis cluster 自动注入
- 没有 global policy 时不注入 Redis cluster
- 相同 store 和 timeout 去重
- 不同 timeout 生成不同 runtime client
- Redis cluster name 使用固定前缀和完整 SHA-256
- 同名不同内容的 cluster 返回构建错误
- TLS transport socket、SNI 和 CA 配置
- TLS validation context 包含 effective server name 的 SAN matcher
- RateLimit plugin config v2 不再包含 DataPlane
- xDS 输出不再包含 ingate-dataplane cluster

### 真实 E2E

新增独立 E2E 脚本，在隔离的 Docker network 中启动 Ingate all-in-one 镜像、真实 Redis Standalone 和测试 Upstream。该 Ingate 镜像使用从官方 Higress gateway:v2.2.3 复制的 Envoy 二进制，不直接运行官方 Higress gateway 容器。脚本负责创建资源、发送请求、收集 Envoy 日志并清理临时容器，不把 Redis 加入 all-in-one 的生产进程集合。

TLS E2E 生成一次性测试 CA 和服务端证书，并构建仅用于测试的派生 Ingate 镜像，把测试 CA 加入系统 CA bundle。生产镜像和 RedisStore 产品协议不增加测试 CA 开关。正确证书使用 Docker network DNS 名称作为 SAN；错误证书使用不匹配的 SAN 验证拒绝路径。

至少验证：

- FixedWindow 在额度内放行，超限返回 429
- SlidingWindow 在额度内放行，超限返回 429
- TokenBucket 支持 burst，并在耗尽后拒绝
- quota headers 的 limit、remaining 和 reset
- Redis 不可用时 FailOpen 放行
- Redis 不可用时 FailClose 拒绝
- TLS Redis 的受信任 SAN 成功和错误 SAN 拒绝
- Envoy 日志中没有 Wasm ABI、插件加载或 Redis hostcall 初始化错误
- 进程列表和镜像中不存在 ingate-dataplane

最终 all-in-one 镜像还需要执行启动 smoke test，证明从 Higress gateway 镜像复制出的 Envoy 二进制在最终 Debian 文件系统中依赖完整，并能使用 Ingate bootstrap 启动。

## 验收标准

第一阶段完成必须同时满足：

- 数据面二进制来自官方 Higress gateway:v2.2.3
- 当前 Ingate bootstrap 和 ADS 链路可用
- 三种 Global RateLimit 算法通过真实 Redis E2E
- fail-open / fail-close 和 quota headers 与迁移前一致
- Sentinel、Cluster 和其它无法映射的配置返回明确 target 错误
- 仓库不再构建、打包、启动或调用 ingate-dataplane
- 非历史设计文档中不存在仍有效的 ingate-dataplane 运行说明
- make test 通过
- make build 通过

## 后续阶段

本设计不会把最终产品范围永久缩小为 Standalone。

后续单独设计：

- Redis Cluster 的 key slot 路由、MOVED / ASK 和拓扑刷新
- Redis Sentinel 的 master 发现和故障切换
- PasswordRef 的 Secret 解析、缓存和轮换
- Higress Redis hostcall 的连接池参数和指标映射

只有这些能力得到等价验证后，才移除 xDS target 对相应配置的临时拒绝。
