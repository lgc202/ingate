# Higress Envoy 与 Redis Hostcall 迁移设计

## 背景

Ingate 当前在 all-in-one 镜像中使用标准 Envoy，并通过独立的 `ingate-dataplane` HTTP 服务完成 Redis-backed global rate limit：

```text
RateLimit Wasm
  -> dispatch_http_call
  -> ingate-dataplane
  -> go-redis
  -> Redis
```

这条链路能够工作，但增加了一个数据面进程、一层 HTTP/JSON 协议和一组只服务于插件的运行时类型。Higress Envoy 在 Proxy-Wasm host ABI 上扩展了 Redis 异步调用能力，可以让 Wasm 插件直接通过 Envoy 的 cluster manager 访问 Redis。

本次迁移的目标是将 all-in-one 数据面替换为 Higress Envoy，并用 Redis hostcall 删除 `ingate-dataplane` 转发层。Ingate 仍然保留自己的资源模型、compiler、RuntimeSnapshot、xDS 服务和插件运行时设计，不引入 Higress controller、pilot-agent、console 或 `wasm-go` wrapper。

## 目标

- all-in-one 使用 Higress `gateway:2.2.3` 中的 Envoy 二进制
- 保留 Ingate 自己的 Envoy bootstrap 和 ADS v3 xDS 服务
- RateLimit 插件通过 Higress Envoy Redis hostcall 执行 global limit
- 继续支持固定窗口、滑动窗口和令牌桶三种 Redis-backed 算法
- 继续支持 Policy 的 `FailOpen` 和 `FailClose` 语义
- 删除 `ingate-dataplane` 进程、HTTP transport 和相关协议代码
- 将 Higress 专属耦合限制在一个小型 hostcall adapter 中
- 保持 Policy、compiler 和核心 IR 不感知 Higress 私有 ABI

## 非目标

首个版本不实现：

- Redis Cluster
- Redis Sentinel
- Redis TLS
- Redis 用户名和密码认证
- `passwordRef` secret 解析
- Higress `wasm-go/pkg/wrapper`
- Higress controller、pilot-agent 或 Kubernetes 部署模型
- 通用 capability framework
- 为未来 hostcall 提前设计插件 SDK

首版只支持 `Standalone + 无 TLS + 无认证` 的 RedisStore。其他配置必须返回明确的不支持错误，不能静默降级。

## 核心决策

### 只使用 Higress Envoy

Ingate 继续依赖标准的：

```text
github.com/proxy-wasm/proxy-wasm-go-sdk
```

不整体切换到 Higress fork，也不引入：

```text
github.com/higress-group/wasm-go
github.com/higress-group/proxy-wasm-go-sdk
```

Higress Redis hostcall 不是 Proxy-Wasm 标准 ABI，因此 Wasm 模块仍然需要知道少量 Higress 扩展符号。这个耦合由 Ingate 自己的 adapter 封装，而不是扩散到 RateLimit policy、runtime 或其他插件。

需要封装的主机 ABI 包括：

```text
proxy_redis_init
proxy_redis_call
proxy_get_buffer_bytes
proxy_on_redis_call_response
```

其中 Redis response 使用 Higress 扩展的 buffer type `9`。这些符号和数值属于 target runtime 适配细节，不能出现在控制面资源协议中。

首版 adapter 按 Higress ABI 固定以下签名：

```go
//go:wasmimport env proxy_redis_init
func proxyRedisInit(cluster *byte, clusterSize int32, username *byte, usernameSize int32, password *byte, passwordSize int32, timeout uint32) uint32

//go:wasmimport env proxy_redis_call
func proxyRedisCall(cluster *byte, clusterSize int32, query *byte, querySize int32, calloutID *uint32) uint32

//go:wasmimport env proxy_get_buffer_bytes
func proxyGetBufferBytes(bufferType uint32, start int32, maxSize int32, data unsafe.Pointer, size *int32) uint32

//go:wasmexport proxy_on_redis_call_response
func proxyOnRedisCallResponse(pluginContextID uint32, calloutID uint32, status int32, responseSize int32)
```

hostcall 返回值沿用 Proxy-Wasm status：`0` 表示成功，其他值转换成结构化 adapter error。Redis callback 的 `status == 0` 表示调用成功，非零表示网络或连接失败。

### 固定 Higress Envoy 版本

all-in-one Dockerfile 使用官方镜像：

```text
higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/gateway:2.2.3
```

并只复制 `/usr/local/bin/envoy`。容器仍然直接执行：

```text
envoy -c /etc/ingate/envoy/bootstrap.yaml
```

不复制或启动 `pilot-agent`。Higress Envoy 当前版本为 Envoy `1.36.4`，现有 ADS v3 bootstrap 可以通过配置校验。实现时同时将 bootstrap 中已弃用的 `http2_protocol_options` 迁移到 Envoy 1.36 推荐的 typed extension protocol options。

## 目标架构

```text
RateLimitPolicy + PolicyBinding + RedisStore
  -> Compiler
  -> Logical IR
  -> xDS Target Translator
       -> Redis Envoy Cluster
       -> RateLimit Wasm Runtime Config
  -> ingate-xds
  -> Higress Envoy
       -> RateLimit Wasm
       -> Ingate Redis Hostcall Adapter
       -> Higress Redis ABI
       -> Standalone Redis
```

核心控制面链路仍然是：

```text
Resource -> Compiler -> Logical IR -> Target Translator -> RuntimeSnapshot
```

Higress 只影响 xDS target 的运行时输出和 Wasm 的底层 host adapter。

## xDS Target 设计

### Redis cluster

xDS translator 为每个被 Global RateLimit 实际引用的 RedisStore 生成一个 Envoy cluster。未被策略引用的 RedisStore 不进入当前 Gateway 的 RuntimeSnapshot。

Redis cluster 使用确定性名称，例如：

```text
ingate-redis-<redis-store-id>
```

名称生成属于 xDS target 逻辑，不能写回 RedisStore 资源。插件运行时配置通过 RedisStore ID 映射到该 cluster name。

首版 cluster 使用普通 Envoy upstream cluster 和 raw TCP 连接，不需要 Redis proxy network filter。Higress hostcall 会通过 cluster manager 选择 endpoint，并使用自己的 Redis async client 发送 RESP 数据。

Redis cluster 统一使用 `LOGICAL_DNS`，域名由 Envoy 解析，IP literal 也沿用同一条配置路径。cluster connect timeout 优先使用 RedisStore `connectTimeoutMillis`，值为 `0` 时使用默认 `1000ms`。

RedisStore 的 `address` 必须是标准 `host:port`。translator 使用结构化地址解析，解析失败时返回包含 RedisStore ID 的明确错误。

### 运行时配置

删除 RateLimit runtime config 中的 `DataPlane`：

```text
clusterName
path
timeoutMillis
```

RedisStore 的插件运行时投影保留执行所需信息：

```text
store ID
Envoy cluster name
database
effective command timeout
```

地址只用于 translator 生成 Envoy cluster，不需要再传给 Wasm。首版不向 Wasm 下发 TLS、Sentinel、Cluster、pool 或 secret 字段。

### 不支持配置的处理

translator 先根据当前 Gateway 实际生效的 PolicyBinding 和 Global RateLimitPolicy 收集被引用的 RedisStore ID，只校验并投影这些 store。未被当前 Gateway 引用的 RedisStore 即使使用暂不支持的模式，也不阻塞当前 RuntimeSnapshot。

对于被引用的 RedisStore，translator 在以下情况返回错误，并拒绝生成部分 RuntimeSnapshot：

- RedisStore mode 不是 `Standalone`
- `tls` 为 true
- `username` 非空
- `passwordRef` 非空
- `addresses`、`sentinelMaster` 等集群或 Sentinel 字段被配置
- `address` 不是合法 `host:port`
- `connectTimeoutMillis`、`commandTimeoutMillis` 或 Policy `global.timeoutMillis` 为负数

Admin API 资源模型暂时保留这些字段，避免把当前运行时限制误写成长期产品模型。首版限制由 xDS target 明确执行，未来其他 target 或新版 runtime 可以支持更多模式。

### timeout 规则

Higress `proxy_redis_init` 的 timeout 是 cluster Redis client 级别，不是单条命令级别。首版保持一个 RedisStore 对应一个 cluster，因此同一个 store 只能有一个有效 command timeout。

有效 timeout 按以下顺序确定：

1. Policy `global.timeoutMillis`
2. RedisStore `commandTimeoutMillis`
3. 默认 `50ms`

值为 `0` 表示未设置，负数属于非法配置。虽然 Admin API DTO 已经校验负数，translator 仍需在跨层配置边界再次拒绝非法值。

同一个 RedisStore 被多个 Global Policy 引用时，所有引用计算出的有效 timeout 必须一致。否则 translator 返回明确错误。未来可以通过为同一 store 生成多个 client alias cluster 解除限制，但首版不提前实现。

连接阶段由 Envoy cluster connect timeout 控制，连接建立后的 Redis 命令由 `proxy_redis_init` 的 operation timeout 控制，两者是不同配置。Higress Envoy 的 Redis raw client 在 connect timeout 或 operation timeout 后会关闭连接、触发 hostcall failure callback，因此插件不再维护第二套请求定时器，请求不会无限保持 Pause。

## Redis Hostcall Adapter

### 包边界

新增：

```text
plugins/internal/hostcall/redis
```

这个包只负责 Higress Redis ABI，不负责限流算法、Policy 语义或 RedisStore 产品模型。

建议暴露一个具体 `Client` 类型，核心能力保持最小：

```text
Init(cluster, database, timeout)
Call(httpContextID, cluster, rawRESP, callback)
ForgetContext(contextID)
Close()
```

每个 `Client` 由一个 Proxy-Wasm plugin context 创建并持有，Client 在构造时记录 `pluginContextID`。不实现类似 go-redis 的通用命令集合，也不复制 Higress wrapper。

### Wasm ABI 文件

ABI 声明只参与 `wasip1/wasm` 构建，避免污染普通 Go 测试：

```text
abi_wasm.go
```

该文件使用 `//go:wasmimport` 调用 Higress Envoy，并使用 `//go:wasmexport` 导出 Redis response callback。普通平台使用单独实现承载可测试的 callback registry，不伪造真实 Envoy 行为。

### callback 生命周期

每个 Redis call 保存：

```text
plugin context ID
  -> callout ID
      -> HTTP context ID + callback
```

callback registry 和 Redis ready 状态都按 plugin context 隔离。Redis ready 按 `(pluginContextID, clusterName)` 记录；即使同一个 Wasm VM 内创建多个 plugin context，也不会共享 callout 或初始化状态。

Higress Envoy 回调 `proxy_on_redis_call_response(pluginContextID, calloutID, status, responseSize)` 时：

1. 根据 pluginContextID 和 callout ID 取出并删除 callback
2. 调用标准 SDK 的 `SetEffectiveContext`
3. context 仍存在时读取 Redis response buffer
4. 将 status、response 或 error 交给调用方

HTTP stream 结束时调用 `ForgetContext` 删除该 context 的未完成 callback。Host 侧没有 cancel ABI，因此迟到的回调仍可能到达；adapter 对未知 callout ID 直接忽略，不能 panic。

plugin context 结束时调用 `Close`，删除该 pluginContextID 下的 callback 和 ready 状态。

Envoy 每个 Wasm VM 单线程执行 callback registry，不需要为 map 增加 mutex。

### Redis 初始化

Redis client 采用延迟初始化：

- 第一次访问 cluster 时调用 `proxy_redis_init`
- 初始化成功后在当前 plugin context 的 Client 内标记 ready
- 初始化失败不永久缓存，后续请求继续重试
- database 通过 Higress cluster query 参数传入
- username 和 password 首版固定为空

延迟初始化避免 Listener 和 CDS 更新顺序造成插件启动失败，同时允许临时配置或初始化失败后恢复。

## Global RateLimit 执行

### 包组织

现有：

```text
plugins/ratelimit/internal/dataplane
```

替换为表达真实职责的包，例如：

```text
plugins/ratelimit/internal/redislimit
```

该包负责：

- 将 GlobalCheck 转成 Redis Lua `EVAL`
- RESP 请求编码和响应解析
- 按顺序调度多条 global checks
- 将 Redis 结果转换成插件领域结果

通用 hostcall adapter 只返回原始 RESP 数据，避免把 RateLimit 逻辑下沉到共享包。

### RESP 编解码

使用成熟的 RESP parser，例如：

```text
github.com/tidwall/resp
```

不使用字符串拼接解析 RESP。编码命令时将 `EVAL`、Lua script、key 和参数构造成 RESP array；解析时显式处理 array、integer、bulk string 和 Redis error。

### Lua 算法

迁移当前 `internal/dataplane/service/ratelimit/algorithm.go` 中的三个 Lua script：

- FixedWindow
- SlidingWindow
- TokenBucket

算法语义、Redis key、quota header、reset 和 retry-after 计算保持不变。算法结果不再使用 `pkg/dataplane/ratelimit` 类型，而是转换为插件自己的 domain result。

### 请求状态机

一次 HTTP 请求的 global checks 顺序执行：

```text
Apply local rules
  -> no global checks: continue/respond
  -> global checks:
       pause request once
       execute check 0
       execute check 1
       ...
       continue or respond
```

顺序执行的原因：

- 保持实现直接，避免一套并发聚合状态机
- 控制单请求对 Redis 的瞬时调用数量
- 可以在第一条拒绝规则后立即停止
- 保持 PolicyBinding 和 rule 的稳定顺序

单条检查处理规则：

- Redis 返回 allowed：继续下一条
- Redis 返回 rejected：立即按对应 Policy 返回拒绝响应
- Redis 或 hostcall 错误且 Policy 为 FailOpen：记录错误并继续下一条
- Redis 或 hostcall 错误且 Policy 为 FailClose：立即按对应 Policy 返回拒绝响应
- 全部通过：保存最后一个需要输出的 quota headers 并 resume request

请求只调用一次 `Pause`，最终只调用一次 `ResumeHttpRequest` 或本地响应。

## 错误处理

### 配置错误

以下错误在控制面 target translation 阶段处理：

- RedisStore 不支持
- 地址非法
- store 引用不存在
- 同一 store timeout 冲突

错误文本必须包含资源 ID 和具体原因，便于 controller 状态和未来 Agent 排障使用。

### 插件启动错误

插件 JSON schema 或 runtime config 非法时，`OnPluginStart` 返回失败。由 Ingate 自己生成的配置也必须在插件边界重新校验，因为这是跨进程配置边界。

### 请求执行错误

以下情况统一转换为 Redis execution error：

- `proxy_redis_init` 返回非 OK
- `proxy_redis_call` dispatch 失败
- callback status 非成功
- response buffer 读取失败
- RESP 格式非法
- Redis 返回 error value
- Lua 返回值数量或类型不符合算法约定

日志至少包含 policy、rule、RedisStore ID、cluster name 和错误类别。错误文本用于日志，不作为稳定分支；FailOpen/FailClose 只依赖结构化错误结果。

## 删除范围

删除以下运行链路：

```text
cmd/ingate-dataplane
internal/dataplane
pkg/dataplane
plugins/ratelimit/internal/dataplane
```

如果删除后没有其他生产调用方，同时删除：

```text
pkg/xredis
github.com/redis/go-redis/v9
```

同步清理：

- all-in-one Dockerfile 中的 `ingate-dataplane` binary
- entrypoint 中的 dataplane 启动和端口等待
- `INGATE_DATAPLANE_ADDR`
- xDS translator 中的 dataplane cluster 常量和生成逻辑
- RateLimit plugin config 中的 `DataPlane`
- README 和插件文档中的旧链路说明

历史设计文档作为决策记录保留，不批量重写。

## 测试与验证

### 单元测试

- RESP `EVAL` 请求编码
- 三种算法响应解析
- Redis error 和非法返回处理
- callback 注册、完成、context 清理和迟到回调
- connect timeout 和 operation timeout 产生失败 callback 后的请求收敛
- 多条 global checks 顺序执行和提前结束
- FailOpen / FailClose
- xDS Redis cluster 生成
- 不支持 RedisStore 配置拒绝
- timeout 冲突拒绝
- dataplane 配置不再出现在插件 JSON 和 RuntimeSnapshot

测试优先覆盖领域行为，不为了 mock hostcall 污染生产接口。原始 Wasm ABI 由真实 Higress Envoy 集成验证。

### 构建验证

```text
make test
make build
make plugins-build
make all-in-one-image
```

确认：

- RateLimit 和 ACL 插件仍使用标准 proxy-wasm-go-sdk 构建
- RateLimit Wasm 包含 Redis callback export
- all-in-one 中 `envoy --version` 对应 Higress Envoy 1.36.4
- bootstrap 使用 Envoy 1.36 支持的配置字段
- 镜像中不再包含 `ingate-dataplane`

### 真实链路验证

使用 standalone Redis 和 all-in-one：

1. 创建 RedisStore、Global RateLimitPolicy 和 PolicyBinding
2. 连续请求验证允许响应后出现 429
3. 验证 quota headers、reset 和 retry-after
4. 停止 Redis，验证 FailOpen 放行
5. 停止 Redis，验证 FailClose 拒绝
6. 恢复 Redis，验证后续请求可以自动恢复
7. 检查 Envoy 日志中没有未知 host function、Wasm callback 或 cluster lookup 错误

## 后续扩展

本次设计保留以下演进路径，但不在首版实现：

- Redis 认证：引入正式 secret 资源和运行时 secret 下发后，将凭据传给 `proxy_redis_init`
- Redis TLS：由 xDS translator 为 Redis cluster 生成 transport socket
- 多 timeout：为同一个 RedisStore 生成不同 client alias cluster
- Cluster 和 Sentinel：先验证 Higress Envoy async Redis client 的路由与拓扑能力，再扩展 target adapter
- 其他 hostcall：仅在出现第二个真实能力时提炼共享 hostcall runtime，不提前建立 capability framework

无论后续如何扩展，核心 Policy、PolicyBinding、RedisStore 和 compiler 都不应依赖 Higress wrapper、matchRules 或私有 host function 类型。
