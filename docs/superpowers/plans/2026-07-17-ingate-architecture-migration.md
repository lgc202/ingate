# Ingate Simplified Architecture Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 `Resource -> Logical IR -> Target -> RuntimeSnapshot -> 独立 xDS` 链路迁移为一套 Ingate 配置域直接编译并原子下发同一份 Envoy 配置，同时把 Redis 作为系统组件、删除 `ingate-dataplane`，并保留可靠的 ACK/NACK、Last Good 和运行状态能力。

**Architecture:** `ingate-controller` 监听完整声明式资源集合，调用纯 `Envoy Config Compiler` 生成 LDS/RDS/CDS/EDS protobuf；`Config Delivery` 是 Snapshot Cache 的唯一写入者，负责 Candidate、Active、Baseline 和 Last Good 生命周期；标准 go-control-plane SotW ADS 在同一进程提供 xDS。Higress 只提供带 Redis ABI 的 Envoy 二进制，生产 Go 代码通过 Ingate 自己的最小 `redisabi` 包调用该 ABI，不引入 Higress Go 依赖。

**Tech Stack:** Go 1.26、Kubernetes generic apiserver/client-go、etcd v3、`github.com/envoyproxy/go-control-plane v0.14.0`、`github.com/envoyproxy/go-control-plane/envoy v1.36.0`、Envoy 1.36.4（Higress gateway v2.2.3 二进制）、Proxy-Wasm、Redis Standalone、React 19、TypeScript、Vitest、Docker。

---

## 范围与执行约束

本迁移虽然覆盖 API、Compiler、xDS、插件和部署，但这些部分共享同一个编译期依赖闭包和同一个不可兼容部署边界，因此保留为一份顺序计划。AI Proxy、Upstream protocol、模型改写、Provider 凭据和 Token usage 属于后续独立设计，不在本计划中实现。

执行期间必须保持以下约束：

- 不创建新的 IR package，不保留 `LogicalGateway`、`Target`、`Translator`、`RuntimeGroup` 或 `RuntimeSnapshot` 的替代抽象。
- `internal/envoy/config` 可以有未导出的局部规范化结构；跨 package 只暴露 `ResourceSet`、`CompileResult`、结构化 `Diagnostic` 和最终 Envoy protobuf 集合。
- `internal/envoy/xds` 不得 import `internal/envoy/delivery`；xDS 只通过 typed event sink 上报事件。
- `Config Delivery` 是唯一允许调用 Snapshot Cache `SetSnapshot` 的业务模块。
- 一套 Ingate 只使用一个固定 cache key；Node ID 只用于连接唯一性、ACK/NACK 和观测。
- API 字段只能“正确映射”或“明确 Unsupported”，不能接受后静默忽略。
- API 最终删除要等所有生产代码消费者切换完毕；管理面可以先停止读写旧字段。
- 每个任务先写失败测试，再实现最小代码；每个提交只完成一类变化。
- Go 代码注释使用中文；不要引入只为测试服务的生产接口。
- 所有生成文件只通过 `make generate` 更新，不手改。

## 最终目录职责

```text
internal/controller/app          进程装配、启动顺序和参数
internal/controller/reconcile    informer、全局 queue、ResourceSet 和收敛流程
internal/controller/status       Accepted Condition 与内部运行状态 HTTP API
internal/envoy/config            直接 Envoy 配置编译
internal/envoy/xds               标准 SotW ADS、callbacks、stream/node 记录
internal/envoy/delivery          Candidate/Active/Baseline/Last Good 状态机
internal/envoy/lastgood          内部 etcd 单记录持久化
internal/storage/schema          schema marker 检查与 reset/bootstrap
plugins/internal/redisabi        Ingate 最小 Redis hostcall ABI
plugins/ratelimit/internal/redis RESP、Lua 算法和异步执行器
```

依赖方向固定为：

```text
controller/app
  -> controller/reconcile -> envoy/config, envoy/delivery, controller/status
  -> envoy/delivery       -> envoy/config, envoy/xds events, envoy/lastgood
  -> envoy/xds            -> go-control-plane cache/server
  -> controller/status
```

---

### Task 1: 固定 go-control-plane 版本并验证 SotW callback 契约

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-concurrency, @go-context

**Files:**

- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/envoy/xds/events.go`
- Create: `internal/envoy/xds/node.go`
- Create: `internal/envoy/xds/cache.go`
- Create: `internal/envoy/xds/callbacks.go`
- Create: `internal/envoy/xds/server.go`
- Create: `internal/envoy/xds/node_test.go`
- Create: `internal/envoy/xds/callbacks_test.go`
- Create: `internal/envoy/xds/callbacks_integration_test.go`

- [ ] **Step 1: 写依赖版本和 callback 顺序的失败测试**

在 `callbacks_integration_test.go` 使用真实 `cache/v3.SnapshotCache`、SotW server 和 `bufconn` 建立 ADS stream，覆盖：

1. `OnStreamResponse` 能记录最终 `version` 与 `nonce`；
2. 下一次 `OnStreamRequest` 能用已发送 nonce 分类 ACK/NACK；
3. NACK sink 返回错误时 callback 返回错误并关闭 stream；
4. callback 内同步替换旧 snapshot 后，坏 Candidate 不会被同一 watch 立即重发；
5. Delta RPC 返回 `codes.Unimplemented`。

事件协议先固定为：

```go
type EventKind string

const (
	EventStreamOpened EventKind = "StreamOpened"
	EventStreamClosed EventKind = "StreamClosed"
	EventResponseSent EventKind = "ResponseSent"
	EventACK EventKind = "ACK"
	EventNACK EventKind = "NACK"
)

type Event struct {
	Kind            EventKind
	StreamID        int64
	NodeID          string
	TypeURL         string
	Version         string
	AcceptedVersion string
	Nonce           string
	ErrorCode       int32
	ErrorMessage    string
}

type EventSink func(context.Context, Event) error
```

`EventSink` 是真实跨 package 协作边界；xDS package 不知道 Delivery 类型。

- [ ] **Step 2: 运行测试并确认因根模块和实现缺失而失败**

Run:

```bash
go test ./internal/envoy/xds -run 'TestCallbacks|TestNodeHash' -count=1
```

Expected: FAIL，提示缺少 go-control-plane root module 或未定义 callbacks/node hash。

- [ ] **Step 3: 固定依赖版本**

将直接依赖固定为：

```text
github.com/envoyproxy/go-control-plane v0.14.0
github.com/envoyproxy/go-control-plane/envoy v1.36.0
```

使用 `go get github.com/envoyproxy/go-control-plane@v0.14.0 github.com/envoyproxy/go-control-plane/envoy@v1.36.0` 更新 `go.mod`/`go.sum`，然后检查 MVS 结果：

```bash
go list -m all | rg 'github.com/envoyproxy/go-control-plane'
```

Expected: root module 为 `v0.14.0`，Envoy protobuf 子模块为 `v1.36.0`。

- [ ] **Step 4: 实现固定 cache key 和最小 callback registry**

`node.go`/`cache.go` 定义：

```go
const CacheKey = "ingate"

type NodeHash struct{}

func (NodeHash) ID(*corev3.Node) string { return CacheKey }
```

共享 cache 使用 `cachev3.NewSnapshotCache(true, NodeHash{}, logger)`；`true` 表示 ADS cache。`callbacks.go` 内部按 stream/type 保存 latest `(nonce, sentVersion)`，并单独保存 `streamID -> nodeID`；第一次带 Node 的请求注册 Node ID，空 Node ID 或另一个活动 stream 使用相同 Node ID 时返回错误。`OnStreamClosed` 只用本地 stream registry 清理，不能依赖传入 node。

`server.go` 使用 `sotwv3.NewServer` 和薄 ADS adapter，只转发 `StreamAggregatedResources` 到 `StreamHandler(stream, resource.AnyType)`；Delta 保持 generated Unimplemented。

- [ ] **Step 5: 用真实 SotW server 验证同步 NACK 契约**

测试必须使用 `github.com/envoyproxy/go-control-plane/pkg/server/sotw/v3`，不能只直接调用 callback 方法。先把坏 snapshot 放入 cache，收到 NACK 时在 sink 内同步放回旧 snapshot，再允许 callback 返回。

Run:

```bash
go test -race ./internal/envoy/xds -run 'TestCallbacks|TestNodeHash|TestNACKRollbackContract' -count=1
```

Expected: PASS。

- [ ] **Step 6: 验证 xDS package 没有反向依赖**

Run:

```bash
go list -deps ./internal/envoy/xds | rg 'github.com/lgc202/ingate/internal/envoy/delivery'
```

Expected: 无输出。

- [ ] **Step 7: 提交**

```bash
git add go.mod go.sum internal/envoy/xds
git commit -m "test(xds): lock SotW callback contract"
```

---

### Task 2: 验证 Ingate Redis ABI 与 Higress Envoy 真实 PING

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-error-handling

**Files:**

- Create: `plugins/internal/redisabi/doc.go`
- Create: `plugins/internal/redisabi/status.go`
- Create: `plugins/internal/redisabi/hostcall_wasm.go`
- Create: `plugins/internal/redisabi/hostcall_native.go`
- Create: `plugins/internal/redisabi/callback_wasm.go`
- Create: `plugins/internal/redisabi/client.go`
- Create: `plugins/internal/redisabi/client_test.go`
- Create: `plugins/redisabi-smoke/main.go`
- Create: `test/redisabi/Dockerfile`
- Create: `test/redisabi/bootstrap.yaml`
- Create: `test/redisabi/run.sh`
- Modify: `Makefile`

- [ ] **Step 1: 写 ABI 常量和 client 行为的失败测试**

`client_test.go` 覆盖：固定 cluster 为 `ingate-system-redis`、命令超时为 `50ms`、初始化只执行一次、同步 hostcall 错误原样返回、dispatch 返回唯一 callout ID。

最小公开边界：

```go
const (
	SystemCluster  = "ingate-system-redis"
	CommandTimeout = 50 * time.Millisecond
)

type HostStatus uint32
type RedisStatus int32
type BufferType uint32

type Result struct {
	Status RedisStatus
	Data   []byte
}

type Callback func(Result)
```

ABI 契约使用精确的 ptr/len 签名：

```go
//go:wasmimport env proxy_redis_init
func proxyRedisInit(
	clusterData *byte, clusterSize int32,
	usernameData *byte, usernameSize int32,
	passwordData *byte, passwordSize int32,
	timeoutMilliseconds uint32,
) HostStatus

//go:wasmimport env proxy_redis_call
func proxyRedisCall(
	clusterData *byte, clusterSize int32,
	queryData *byte, querySize int32,
	calloutID *uint32,
) HostStatus

//go:wasmimport env proxy_get_buffer_bytes
func proxyGetBufferBytes(
	bufferType BufferType,
	start int32,
	maxSize int32,
	returnBufferData unsafe.Pointer,
	returnBufferSize *int32,
) HostStatus

//go:wasmexport proxy_on_redis_call_response
func proxyOnRedisCallResponse(
	pluginContextID uint32,
	calloutID uint32,
	status int32,
	responseSize int32,
)
```

三个 hostcall 返回 `HostStatus`，callback 的 Redis 执行状态使用 `RedisStatus`，两类状态不能复用同一类型。`HostStatus(0)` 表示 hostcall 成功；非零统一转换为稳定 hostcall error，不依赖未稳定的细分数值。Redis response buffer 类型固定为 `BufferType(9)`，且只在 callback 期间有效，必须在返回前复制。

- [ ] **Step 2: 运行普通 Go 测试并确认 stub 尚未实现**

Run:

```bash
go test ./plugins/internal/redisabi -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现最小 wasip1 ABI 和非 Wasm test stub**

`hostcall_wasm.go` 只声明：

```text
proxy_redis_init
proxy_redis_call
proxy_get_buffer_bytes
```

`callback_wasm.go` 只导出：

```text
proxy_on_redis_call_response
```

生产代码继续使用 `github.com/proxy-wasm/proxy-wasm-go-sdk`，执行 `rg -n 'github.com/higress-group/' plugins pkg` 必须无输出。

初始化字符串固定为：

```text
ingate-system-redis?buffer_flush_timeout=0&max_buffer_size_before_flush=0
```

username/password 为空时传 `nil, 0`，DB 固定 0，timeout 固定 50ms。空 query 必须返回 Go error，不能对空 slice 取 `&value[0]` 导致 panic。

- [ ] **Step 4: 构建只做 PING 的最小 Wasm**

`plugins/redisabi-smoke/main.go` 在 Root Context 启动时初始化 `ingate-system-redis` 并立即发送一次 RESP `PING`。异步 callback 只复制 Redis response buffer 到该 plugin/root context 的状态，不切换 HTTP context，也不 Resume/Respond；后续测试 HTTP 请求在正常 `OnHttpRequestHeaders` callback 中同步读取该状态，成功返回 HTTP 200 和 `PONG`，未完成返回 503，ABI/status/RESP 错误返回 500 并写明稳定错误类别。

Task 2 只验证 Redis ABI、buffer 和 callback export，不提前实现 HTTP callout context registry；`(pluginContextID, calloutID)`、late callback 和 direct Resume/Respond 在 Task 5 接入。

Smoke plugin 必须放在 `plugins/` 下，因为 Go 的 `internal` import 规则只允许 `plugins` 子树导入 `plugins/internal/redisabi`；`test/redisabi` 只保存 Docker 和运行 harness。

- [ ] **Step 5: 写真实 Higress Envoy + Redis smoke harness**

`test/redisabi/run.sh` 使用隔离 Docker network，并把 Higress 镜像中的 Envoy 复制进与最终 all-in-one 相同的 Debian bookworm 基础环境，再启动：

- 官方 Redis Standalone；
- 从官方 Higress gateway v2.2.3 镜像取得的 Envoy 1.36.4；
- 静态 bootstrap 中名为 `ingate-system-redis` 的 Redis cluster；
- 最小 Wasm filter。

先以 tag 拉取并验证，再将镜像引用替换为 `@sha256:` digest；脚本必须在发现仍是可变 tag 时失败。清理使用 trap，不复用开发环境容器。

启动前必须在最终 Debian 环境中依次验证：

```text
ldd /usr/local/bin/envoy 无 not found
envoy --version 为 1.36.4
envoy --mode validate -c /etc/ingate/envoy/bootstrap.yaml 成功
```

- [ ] **Step 6: 运行真实 ABI smoke**

Run:

```bash
make redis-abi-smoke
```

Expected: HTTP 200、响应包含 `PONG`，Envoy 日志无 unknown import、ABI status、buffer type 或 callback export 错误。

- [ ] **Step 7: 提交**

```bash
git add plugins/internal/redisabi plugins/redisabi-smoke test/redisabi Makefile
git commit -m "test(ratelimit): verify Ingate Redis ABI"
```

若真实 smoke 不能通过，停止后续 RateLimit 迁移并记录具体 ABI 差异；不能回退为 Higress SDK 生产依赖或重新保留 `ingate-dataplane`。

---

### Task 3: 从管理面移除 RuntimeGroup

**Required skills:** @superpowers:test-driven-development, @go-testing

**Files:**

- Modify: `internal/adminapi/handler/gateway/dto/request.go`
- Modify: `internal/adminapi/handler/gateway/dto/response.go`
- Modify: `internal/adminapi/handler/gateway/dto/types.go`
- Modify: `internal/adminapi/handler/gateway/handler.go`
- Modify: `internal/adminapi/service/gateway/service.go`
- Modify: `internal/adminapi/service/gateway/types.go`
- Modify: `internal/adminapi/handler/handler.go`
- Modify: `internal/adminapi/service/service.go`
- Modify: `internal/adminapi/store/store.go`
- Modify: `internal/adminapi/server/router.go`
- Modify: `internal/apiserver/registry/gateway/strategy.go`
- Delete: `internal/adminapi/handler/runtimegroup/handler.go`
- Delete: `internal/adminapi/handler/runtimegroup/dto/response.go`
- Delete: `internal/adminapi/handler/runtimegroup/dto/types.go`
- Delete: `internal/adminapi/service/runtimegroup/service.go`
- Delete: `internal/adminapi/service/runtimegroup/types.go`
- Delete: `internal/adminapi/store/runtimegroup/store.go`
- Delete: `internal/adminapi/store/runtime/store.go`
- Create: `internal/adminapi/handler/gateway/dto/request_test.go`
- Create: `internal/adminapi/service/gateway/service_test.go`
- Create: `internal/adminapi/server/router_test.go`
- Create: `internal/apiserver/registry/gateway/strategy_test.go`
- Modify: `web/console/package.json`
- Modify: `web/console/package-lock.json`
- Modify: `web/console/src/domain/gateway.ts`
- Modify: `web/console/src/api/contracts.ts`
- Modify: `web/console/src/api/liveConsoleRepository.ts`
- Modify: `web/console/src/features/gateways/form.ts`
- Modify: `web/console/src/features/gateways/GatewayPage.tsx`
- Modify: `web/console/src/mocks/consoleRepository.ts`
- Create: `web/console/src/features/gateways/form.test.ts`
- Modify: `Makefile`

- [ ] **Step 1: 写后端失败测试**

测试以下产品契约：

- Gateway 请求没有 `runtimeGroup` 仍可通过 DTO 校验；
- Create/Update 不查询 RuntimeGroup，也不写 `Spec.RuntimeGroupRef`；
- `/api/v1/runtime-groups` 返回 404；
- apiserver Gateway strategy 不再要求 `runtimeGroupRef.name`。

Run:

```bash
go test ./internal/adminapi/handler/gateway/dto ./internal/adminapi/service/gateway ./internal/adminapi/server ./internal/apiserver/registry/gateway -count=1
```

Expected: FAIL。

- [ ] **Step 2: 删除 Admin API RuntimeGroup 协作链**

Gateway service 构造函数改为：

```go
func New(store *gatewaystore.Store, routes *routestore.Store) *Service
```

`GatewayParams`、`GatewayConfig`、响应 DTO 和 Handler 参数转换全部删除 RuntimeGroup 字段；启停 Gateway 不再检查运行组。路由聚合、store 聚合和 router 同步删除 RuntimeGroup。

此时声明式 API 类型暂时保留 `RuntimeGroupRef`，但管理面不再读写它；最终在 Task 16 统一删除和重新生成代码。

- [ ] **Step 3: 建立 Console 单元测试基础**

为 `web/console` 增加 Vitest 与 `npm test -- --run` 脚本；Makefile 增加 `console-test`。`form.test.ts` 断言 Gateway payload、校验报告和 workspace 均不包含 RuntimeGroup，repository 不请求 `/runtime-groups`。

- [ ] **Step 4: 修改 Console Gateway 模型和页面**

删除：

- `Gateway.runtimeGroup`
- `Gateway.runtimeGroupName`
- `GatewayRuntimeGroupOption`
- `GatewayWorkspace.runtimeGroups`
- `GatewayMutationPayload.runtimeGroup`
- `ConsoleRepository.listRuntimeGroups()`
- 页面中的运行组选择器和列表列

`liveConsoleRepository.listGateways()` 只请求 `/gateways`；`getRouteWorkspace()` 不再为了 Gateway 展示请求 RuntimeGroup。

- [ ] **Step 5: 运行后端与前端测试**

Run:

```bash
go test ./internal/adminapi/... ./internal/apiserver/registry/gateway -count=1
make console-test
make console-build
```

Expected: PASS；构建产物协议不含 `runtimeGroup`。

- [ ] **Step 6: 提交**

```bash
git add internal/adminapi internal/apiserver/registry/gateway web/console Makefile
git commit -m "refactor(adminapi): remove runtime group management"
```

---

### Task 4: 迁移纯 Redis RESP 与三种限流算法

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-error-handling

**Files:**

- Create: `plugins/ratelimit/internal/redis/resp.go`
- Create: `plugins/ratelimit/internal/redis/resp_test.go`
- Create: `plugins/ratelimit/internal/redis/algorithm.go`
- Create: `plugins/ratelimit/internal/redis/algorithm_test.go`
- [ ] **Step 1: 为 RESP 和三种 Lua 算法写普通 Go 失败测试**

从 `internal/dataplane/service/ratelimit/algorithm.go` 迁移脚本和结果语义，但新 package 不能 import `go-redis`、Proxy-Wasm 或 HTTP DTO。覆盖：

- RESP bulk string、simple string、integer、array、nil、Redis error；
- FixedWindow、SlidingWindow、TokenBucket 的 EVAL argv；
- Allowed、Current、Limit、Remaining、ResetSeconds、RetryAfterSeconds；
- Redis error、空响应、截断、错误类型、数字溢出和尾随数据；
- FixedWindow TTL 毫秒向上取整；
- TokenBucket capacity 等于 requests + burst。

- [ ] **Step 2: 实现无运行时依赖的 RESP/算法层**

只实现本项目需要的 RESP2：bulk-string array 编码，以及 array/integer/bulk/simple/error 解码。Lua 脚本内容从现有 dataplane 原样迁移，删除 `redis.NewScript` 包装。时间通过可注入 clock 测试，生产使用 `time.Now`。

这个任务只新增未接线的纯代码，不修改当前 plugin schema、旧 xDS producer 或运行时。最终 producer/consumer 切换在 Task 5 原子完成。

- [ ] **Step 3: 运行纯算法测试**

Run:

```bash
go test -race ./plugins/ratelimit/internal/redis -count=1
```

Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add plugins/ratelimit/internal/redis
git commit -m "refactor(ratelimit): extract Redis algorithms"
```

---

### Task 5: 原子切换 RateLimit 配置、Redis ABI consumer 和 Envoy 二进制

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-concurrency, @go-context

**Files:**

- Create: `plugins/internal/redisabi/registry.go`
- Create: `plugins/internal/redisabi/registry_test.go`
- Modify: `plugins/internal/redisabi/status.go`
- Modify: `plugins/internal/redisabi/hostcall_wasm.go`
- Modify: `plugins/internal/redisabi/hostcall_native.go`
- Modify: `plugins/internal/redisabi/callback_wasm.go`
- Modify: `plugins/internal/redisabi/client.go`
- Modify: `plugins/internal/redisabi/client_test.go`
- Create: `plugins/ratelimit/internal/redis/client.go`
- Create: `plugins/ratelimit/internal/redis/client_test.go`
- Create: `plugins/ratelimit/internal/redis/execution.go`
- Create: `plugins/ratelimit/internal/redis/execution_test.go`
- Modify: `plugins/ratelimit/internal/app/app.go`
- Modify: `plugins/ratelimit/internal/runtime/runtime.go`
- Modify: `plugins/ratelimit/internal/runtime/runtime_test.go`
- Modify: `plugins/ratelimit/internal/wasm/http.go`
- Modify: `plugins/ratelimit/internal/wasm/plugin.go`
- Modify: `plugins/ratelimit/internal/wasm/request.go`
- Modify: `plugins/ratelimit/internal/wasm/route.go`
- Modify: `plugins/ratelimit/README.md`
- Modify: `pkg/plugin/ratelimit/doc.go`
- Modify: `pkg/plugin/ratelimit/types.go`
- Modify: `pkg/plugin/ratelimit/types_test.go`
- Modify: `plugins/ratelimit/internal/policy/decision.go`
- Modify: `plugins/ratelimit/internal/policy/global.go`
- Modify: `plugins/ratelimit/internal/policy/key.go`
- Modify: `plugins/ratelimit/internal/policy/policy.go`
- Modify: `plugins/ratelimit/internal/policy/policy_test.go`
- Modify: `plugins/ratelimit/internal/policy/runner.go`
- Modify: `internal/core/compiler/compiler.go`
- Modify: `internal/core/compiler/compiler_test.go`
- Modify: `internal/core/target/xds/ratelimit.go`
- Modify: `internal/core/target/xds/translator.go`
- Modify: `internal/core/target/xds/translator_test.go`
- Modify: `internal/xds/server/ratelimit_builder.go`
- Modify: `internal/xds/server/listener_builder.go`
- Modify: `internal/xds/server/listener_builder_test.go`
- Modify: `internal/adminapi/handler/ratelimitpolicy/dto/request.go`
- Modify: `internal/adminapi/handler/ratelimitpolicy/dto/response.go`
- Modify: `internal/adminapi/handler/ratelimitpolicy/dto/types.go`
- Modify: `internal/adminapi/handler/ratelimitpolicy/handler.go`
- Modify: `internal/adminapi/service/ratelimitpolicy/service.go`
- Modify: `internal/adminapi/service/ratelimitpolicy/types.go`
- Modify: `internal/adminapi/handler/handler.go`
- Modify: `internal/adminapi/service/service.go`
- Modify: `internal/adminapi/store/store.go`
- Modify: `internal/adminapi/store/resource/store.go`
- Modify: `internal/adminapi/server/router.go`
- Delete: `internal/adminapi/handler/redisstore/handler.go`
- Delete: `internal/adminapi/handler/redisstore/dto/request.go`
- Delete: `internal/adminapi/handler/redisstore/dto/response.go`
- Delete: `internal/adminapi/handler/redisstore/dto/types.go`
- Delete: `internal/adminapi/service/redisstore/service.go`
- Delete: `internal/adminapi/service/redisstore/types.go`
- Delete: `internal/adminapi/store/redisstore/store.go`
- Create: `internal/adminapi/handler/ratelimitpolicy/dto/request_test.go`
- Create: `internal/adminapi/service/ratelimitpolicy/service_test.go`
- Modify: `web/console/src/domain/policy.ts`
- Modify: `web/console/src/api/liveConsoleRepository.ts`
- Create: `web/console/src/features/policies/form.ts`
- Create: `web/console/src/features/policies/form.test.ts`
- Modify: `web/console/src/features/policies/PolicyPage.tsx`
- Modify: `web/console/src/mocks/consoleRepository.ts`
- Modify: `deploy/all-in-one/Dockerfile`
- Modify: `deploy/all-in-one/entrypoint.sh`
- Modify: `deploy/all-in-one/default.env`
- Modify: `deploy/all-in-one/envoy/bootstrap.yaml`
- Create: `deploy/all-in-one/redis/redis.conf`
- Create: `test/backend/Dockerfile`
- Create: `test/ratelimit-runtime/run.sh`
- Modify: `Makefile`
- Delete: `plugins/ratelimit/internal/dataplane/client.go`
- Delete: `plugins/ratelimit/internal/dataplane/http_transport.go`
- Delete: `plugins/ratelimit/internal/dataplane/request.go`
- Delete: `plugins/ratelimit/internal/dataplane/request_test.go`

- [ ] **Step 1: 写严格执行配置和 producer/consumer 契约失败测试**

固定最终 JSON 为生效架构规格中的 routes/bindings/policies 结构，不携带 schema version。测试拒绝任意未知字段、`schemaVersion`、`redisStores`、`dataPlane`、Policy `global`、Policy `displayName`、RouteConfig 顶层 `ruleName` 和旧 envelope；展示字段只存在于产品资源/DTO，不进入可执行插件配置。

同一提交证明当前 xDS producer 只输出最终结构，且 JSON 不含 schema version、RedisStore、dataplane cluster、Redis 地址或 timeout。项目按全新配置域开发，不增加旧 RuntimeSnapshot 的兼容读取或迁移分支。

- [ ] **Step 2: 写管理面系统 Redis 失败测试**

覆盖：

- `mode=Global` 不需要 `global`/`redisRef`；
- Create/Update 不查询 RedisStore，也不写 `Spec.Global`；
- 旧 compiler 过渡实现不再因 `Mode=Global, Global=nil` 报错，且不收集 RedisStore；
- 旧 compiler 过渡实现拒绝用户 Upstream ID/生成的 Envoy cluster name 使用 `ingate-system-*` 保留前缀；
- `/api/v1/redis-stores` 返回 404；
- Console workspace/payload 不含 `redisStores`、`global` 或 `redisRef`，Global 编辑器只显示“使用系统 Redis”。

- [ ] **Step 3: 写 callout registry、rule scope 和执行生命周期失败测试**

覆盖：

- registry key 固定为 `(pluginContextID, calloutID)`，不同 plugin 的相同 callout ID 不碰撞；
- HTTP context 创建时登记存活，`OnHttpStreamDone` 时只标记/移除 liveness，不提前删除仍在飞行的 callout；迟到 callback 负责最终删除 callout 并记录 ignored；
- runtime 先按 `(gatewayName, routeName)` 找 RouteConfig，再按当前 xDS `RuleName` 过滤 bindings；
- Gateway binding、整条 Route binding 和 rule-specific Route binding 的作用域正确；
- callback 先恢复始终有效的 plugin/root context，再查找但不删除 callout 记录；确认 HTTP context 存活与否后才删除记录；
- response buffer 在 callback 返回前复制，status 非零时不读取 buffer；
- 已销毁、未知、重复或跨 plugin callback 不 Resume/Respond、不 panic；
- 已销毁 context 的 callback 必须产生稳定的 `late_callback_ignored` 计数或日志，供真实 E2E 证明 callback 确实到达；
- registry 显式保存 pluginContextID 和 httpContextID；自定义 Redis callback 内只调用 Ingate 直接 hostcall（buffer、Resume/Respond、后续 Redis dispatch），不能调用依赖 SDK 私有 `activeContextID` 的异步注册 API，例如 `DispatchHttpCall`；
- GlobalCheck 严格串行且全部完成后统一裁决；同步/异步错误进入相同 fail-open/fail-close 顺序。

- [ ] **Step 4: 实现严格配置、binding 过滤和固定 key**

`pkg/plugin/ratelimit` 删除 RedisStore、DataPlane、Global、DisplayName 和 schema version 字段，使用 `json.Decoder.DisallowUnknownFields()` 且要求单一 JSON value。RouteConfig 只按 GatewayName + RouteName 建索引；当前 xDS RuleName 用于过滤：Gateway target 作用于所有 rule，Route target 无 ruleName 作用于整条 Route，有 ruleName 时只匹配同名 rule。

`policy.GlobalCheck` 删除 RedisStore/timeout。Redis key 用长度编码依次包含 `ingate-rate-limit`、Policy ID、Route ID、Route rule、RateLimit rule 和请求维度 key；每段使用 `字节长度:原始字节` 编码，测试空值、冒号、斜杠以及 `("a", "b:c")` / `("a:b", "c")` 这类碰撞输入。

- [ ] **Step 5: 实现 Redis ABI execution 并删除插件 HTTP transport**

Root Context 只初始化一次 `ingate-system-redis?buffer_flush_timeout=0&max_buffer_size_before_flush=0`。Execution 流程固定为：

```text
Apply -> GlobalChecks -> Pause
dispatch 当前 check
callback -> set plugin/root context -> 查找但暂不删除 (plugin, callout) 记录
-> 尝试切换 HTTP context 并判断是否仍存活 -> 删除记录
-> 已销毁则记录 late_callback_ignored 并返回
-> 存活则复制/解析 buffer -> 下一项
全部完成 -> CompleteGlobalChecks -> Resume 或 Respond
```

`proxywasm.SetEffectiveContext` 只改变 host context，不会同步 SDK 私有的 `activeContextID`。因此 Redis callback 路径不能借用 SDK 的 callback registry；plugin/http context ID 由 Ingate registry 显式管理，Resume/Respond/读取 buffer/继续 Redis call 都走本包可审计的直接 hostcall，并用单元测试证明不会把 callback 注册到错误 context。

删除 `plugins/ratelimit/internal/dataplane`，但独立 dataplane 服务暂留到 Task 17，保证 root build；插件不再 import `pkg/dataplane/ratelimit`。

- [ ] **Step 6: 同步切换过渡 compiler/xDS producer 和管理面**

修改当前 `internal/core/compiler`，Global 不再要求 Global config/RedisStore，并拒绝用户资源占用 `ingate-system-*`；修改旧 target/xDS translator 和 listener/ratelimit builder，只生成最终 routes/bindings/policies。Admin API/Console 同一提交删除 RedisStore CRUD、Redis 选择和 `spec.global` 写入；`plugins/ratelimit/README.md` 同步改为系统 Redis + 内置 ABI，不再描述 `ingate-dataplane` 或插件 HTTP transport。

这些过渡修改在新 Compiler/Controller 切换后由 Task 12/16 删除，不形成长期兼容层。

- [ ] **Step 7: 同步切换 all-in-one 到 Higress Envoy 和系统 Redis**

使用 Task 2 已固定 digest 的 Higress Envoy 1.36.4；bootstrap 加静态 `ingate-system-redis`；镜像加入固定 digest 的官方 Redis；entrypoint 使用 `/etc/ingate/redis/redis.conf` 启动 Redis。此阶段暂时保留独立 ingate-xds/ingate-dataplane 进程，但 dataplane 已无调用者。

`redis.conf` 固定：`bind 127.0.0.1`、`protected-mode yes`、`port 6379`、`daemonize no`、`dir /var/lib/ingate/redis`、`save ""`、`appendonly no`；限流状态允许随 Redis 重启清空，不把它当作控制面持久化数据。smoke 必须证明容器内 `redis-cli -h 127.0.0.1 ping` 成功，而同 Docker network 的 peer container 无法连接 6379。

同时修复 defaults 加载：`/etc/ingate/default.env` 只为尚未设置的变量赋默认值，不能覆盖 Docker `--env-file`/`-e`。runtime smoke 至少使用非默认 Console 地址和数据目录证明外部值生效；Controller internal/status 参数在 Task 12/13 出现后继续覆盖验证。

从这一任务开始就使用 `critical_pids`/`auxiliary_pids` 分组并只等待关键 pid，确保 Redis 退出不终止容器。完整健康检查和进程删除在后续任务继续收口。

- [ ] **Step 8: 运行原子切换验证**

Run:

```bash
go test -race ./pkg/plugin/ratelimit ./plugins/internal/redisabi ./plugins/ratelimit/internal/... ./internal/core/... ./internal/xds/... ./internal/adminapi/... -count=1
make ratelimit-plugin-build
make console-test
make console-build
make redis-abi-smoke
make all-in-one-image
make ratelimit-runtime-smoke
```

`test/backend/Dockerfile` 从当前源码构建仅测试使用的 `cmd/ingate-httpbin` image，不进入 all-in-one，也不依赖 mutable 公共 echo/httpbin tag。`ratelimit-runtime-smoke` 必须使用刚构建的精确 `ALL_IN_ONE_IMAGE` 和本地 test backend image，在隔离 network 启动两者，创建最小 Gateway/Route/Upstream/Global RateLimitPolicy/PolicyBinding，证明额度内请求成功、超额请求返回 429，且 Redis key 只存在于内置实例。

Expected: 全部 PASS；all-in-one 使用 Higress Envoy，最终 filter 配置可加载，Global 请求走系统 Redis；peer container 不能直连 Redis；源码无插件到 dataplane HTTP import。

- [ ] **Step 9: 提交**

```bash
git add pkg/plugin/ratelimit plugins internal/core internal/xds internal/adminapi web/console deploy/all-in-one test/backend test/ratelimit-runtime Makefile
git commit -m "feat(ratelimit): switch to system Redis runtime"
```

---

### Task 6: 建立直接的全局 Envoy Config Compiler 和 Listener 合并

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-data-structures, @go-error-handling

**Files:**

- Create: `internal/envoy/config/types.go`
- Create: `internal/envoy/config/compiler.go`
- Create: `internal/envoy/config/listeners.go`
- Create: `internal/envoy/config/envoy.go`
- Create: `internal/envoy/config/compiler_test.go`
- Create: `internal/envoy/config/listeners_test.go`
- Create: `internal/envoy/config/envoy_test.go`

- [ ] **Step 1: 写 Compiler 边界和确定性 snapshot 的失败测试**

跨 package 边界固定为：

```go
type ResourceSet struct {
	Gateways              []resource.Gateway
	Routes                []resource.Route
	Upstreams             []resource.Upstream
	RateLimitPolicies     []resource.RateLimitPolicy
	AccessControlPolicies []resource.AccessControlPolicy
	PolicyBindings        []resource.PolicyBinding
}

type Severity string

const (
	SeverityError Severity = "Error"
	SeverityWarning Severity = "Warning"
)

type Reason string

const (
	ReasonAccepted Reason = "Accepted"
	ReasonInvalidSpec Reason = "InvalidSpec"
	ReasonReferenceNotFound Reason = "ReferenceNotFound"
	ReasonConflict Reason = "Conflict"
	ReasonUnsupported Reason = "Unsupported"
	ReasonCompileFailed Reason = "CompileFailed"
)

type Diagnostic struct {
	Severity Severity
	Kind     resource.Kind
	ID       string
	Reason   Reason
	Message  string
}

type Config struct {
	Listeners []*listenerv3.Listener
	Routes    []*routev3.RouteConfiguration
	Clusters  []*clusterv3.Cluster
	Endpoints []*endpointv3.ClusterLoadAssignment
}

type CompileResult struct {
	Version     string
	Config      Config
	Diagnostics []Diagnostic
}
```

`Compiler.Compile(ResourceSet) CompileResult` 不返回 Logical IR。测试断言任意 Error 时 `Config` 不可发布；Warning 不阻止发布。

- [ ] **Step 2: 写多 Gateway Listener/hostname 失败测试**

覆盖：

- 两个 HTTP Gateway 同端口、不同且不重叠 hostname 合并为一个 Listener 和一个 RDS 名称；
- 同一 bind/port 出现 HTTP 与 HTTPS 返回 `Conflict`；
- exact、wildcard、catch-all 的 hostname 所有权重叠返回 `Conflict`；
- 同一 Listener catch-all 最多一个；
- `HostBinding.ListenerRefs` 为空或引用不存在返回 `InvalidSpec`；
- 没有 HostBinding 引用某 Listener 时，该 Listener 默认拥有 `*`；
- HTTPS Listener 或非空 CertificateRef 返回 `Unsupported`，不生成明文 Listener；
- disabled Gateway 被排除但不产生 Error。

Run:

```bash
go test ./internal/envoy/config -run 'TestCompiler|TestListeners|TestSnapshot' -count=1
```

Expected: FAIL。

- [ ] **Step 3: 实现索引、诊断和 Listener 规范化**

`compiler.go` 只组织主流程：建立 ID 索引、校验重复资源、调用 listener/route/upstream/policy 编译步骤、收集 diagnostics。Listener 分组键固定为：

```go
type listenerKey struct {
	address  string
	port     int
	protocol resource.Protocol
}
```

第一阶段 address 固定为 `0.0.0.0`。所有 map 输出必须先按规范化 key 排序。

- [ ] **Step 4: 实现最终 protobuf 和稳定版本**

`envoy.go` 将四类 typed protobuf 转成 `map[resource.Type][]types.Resource`，每次都显式包含：

```text
resource.ListenerType
resource.RouteType
resource.ClusterType
resource.EndpointType
```

空列表也必须存在。调用 `cachev3.NewSnapshot(version, resources)` 后执行 `Consistent()`。版本由按资源类型、资源名排序后的 deterministic protobuf bytes 计算 SHA-256，格式固定为 `ingate/<完整十六进制 hash>`；相同内容得到相同版本，不创建无意义 Candidate，任一内容变化使四类 snapshot version 一起变化。

- [ ] **Step 5: 运行确定性和多 Gateway 测试**

Run:

```bash
go test -race ./internal/envoy/config -run 'TestCompiler|TestListeners|TestSnapshot' -count=1
```

Expected: PASS；将输入 slice 顺序反转后 version 和 `proto.Equal` 结果不变。

- [ ] **Step 6: 提交**

```bash
git add internal/envoy/config
git commit -m "feat(envoy): add global config compiler foundation"
```

---

### Task 7: 完成 Route、Upstream 和内置治理插件的 Envoy 编译

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-data-structures

**Files:**

- Create: `internal/envoy/config/routes.go`
- Create: `internal/envoy/config/upstreams.go`
- Create: `internal/envoy/config/policies.go`
- Create: `internal/envoy/config/routes_test.go`
- Create: `internal/envoy/config/upstreams_test.go`
- Create: `internal/envoy/config/policies_test.go`
- Modify: `internal/envoy/config/compiler.go`
- Modify: `internal/envoy/config/listeners.go`
- Modify: `internal/envoy/config/envoy.go`

- [ ] **Step 1: 写 Route 展开、冲突和排序的失败测试**

按 `(Gateway, Listener, Route)` 展开 effective hostname，覆盖：

- Route 没有 hostnames 时继承 Listener 的全部有效 Host；
- Route hostname 必须等于或属于 Listener 的 Host 所有权；
- exact hostname 是 wildcard 子集；两个 wildcard 重叠视为冲突；
- Route 绑定多个 ParentRef 时独立展开；
- 没有任何可挂载 Listener 时返回 `Conflict`；
- 同一 virtual host 中完全相同 match 返回 `Conflict`；
- Route 顺序依次按更长 PathPrefix、更多 Method/Header 约束、Route ID、Rule Name；
- methods 展开后 Envoy route name 稳定包含 Gateway ID、Route ID、Rule Name 和 method。

- [ ] **Step 2: 写公开 Route 字段映射失败测试**

覆盖：

- Header `Set` 使用 `OVERWRITE_IF_EXISTS_OR_ADD`；
- Header `Add` 使用 `APPEND_IF_EXISTS_OR_ADD`；
- Remove 保持原语义；
- Timeout 进入 `RouteAction.Timeout`；
- Retry attempts、per-try timeout 和用户 `RetryOn` 全部进入 Envoy，不使用固定 retry 条件覆盖；
- `RouteRule.UpstreamRefs[].Weight` 映射到 `WeightedCluster`，两个及以上 Upstream 的比例和总权重保持，单 Upstream 也不丢失显式 weight；
- 未知 RouteFilter 返回 `Unsupported`。

- [ ] **Step 3: 写 Upstream CDS/EDS 失败测试**

覆盖：

- Upstream ID 是全局 cluster identity，多 Route 引用只生成一份；
- `ingate-system-*` ID 返回 `InvalidSpec`；
- round_robin、least_request、random 映射到 Envoy LB policy；
- Enabled endpoint 才进入 EDS；
- endpoint weight 进入 `LbEndpoint.LoadBalancingWeight`；
- Endpoint 地址、端口非法返回 Error；
- `HealthCheck.Enabled=true` 返回 `Unsupported`；
- 同名不同 protobuf 内容不得 first-wins。

- [ ] **Step 4: 写内置 Policy 配置失败测试**

Compiler 直接从强类型 Policy/Binding 构造 `pkg/plugin/acl` 与 `pkg/plugin/ratelimit` 的 route index。RateLimit JSON 使用无版本的最终结构，且不含 Redis 地址、RedisStore、cluster、timeout、`global` 或 dataplane 字段。Listener/HCM 每种内置插件只注入一次 Wasm filter，per-route config 通过稳定 xDS route name 定位。

- [ ] **Step 5: 实现 Route、Upstream 和 Policy 编译**

可以在同 package 使用未导出的 `compiledRoute`、`effectiveHost` 等临时类型降低复杂度，但禁止建立 `internal/envoy/ir`、导出 Logical 类型或跨 package 中间协议。现有 `internal/xds/server/*_builder.go` 中确认正确的 protobuf 组装逻辑可以迁移，不能继续 import `internal/core/target/xds`。

- [ ] **Step 6: 运行 Compiler 全量测试**

Run:

```bash
go test -race ./internal/envoy/config -count=1
```

Expected: PASS；每个当前公开字段都有正确映射或明确 Unsupported 测试。

- [ ] **Step 7: 提交**

```bash
git add internal/envoy/config
git commit -m "feat(envoy): compile routes upstreams and policies"
```

---

### Task 8: 完成标准 SotW Snapshot Cache 与 ADS Server

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-concurrency, @go-context

**Files:**

- Modify: `internal/envoy/xds/cache.go`
- Create: `internal/envoy/xds/logger.go`
- Modify: `internal/envoy/xds/server.go`
- Create: `internal/envoy/xds/cache_test.go`
- Create: `internal/envoy/xds/server_test.go`
- Modify: `internal/envoy/xds/events.go`
- Modify: `internal/envoy/xds/node.go`
- Modify: `internal/envoy/xds/callbacks.go`
- Modify: `internal/envoy/xds/callbacks_test.go`
- Modify: `internal/envoy/xds/callbacks_integration_test.go`

- [ ] **Step 1: 写 Snapshot Cache 与 ADS server 失败测试**

精确使用：

```go
cachev3.NewSnapshotCache(true, fixedNodeHash{}, logger)
sotwv3.NewServer(ctx, cache, callbacks, sotwv3.WithOrderedADS(), sotwv3.WithLogger(logger))
```

`true` 表示 ADS cache。测试两个不同 Node ID 映射同一 cache key 并收到同一配置；Node ID 为空或活动连接重复时拒绝。

- [ ] **Step 2: 实现只暴露 SotW 的薄 ADS adapter**

不要使用同时实现 Delta 的 `pkg/server/v3.NewServer`。`server.go` 嵌入 generated unimplemented server，只覆写：

```go
func (s *adsServer) StreamAggregatedResources(
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer,
) error {
	return s.sotw.StreamHandler(stream, resource.AnyType)
}
```

`DeltaAggregatedResources` 保持 generated `Unimplemented`，测试断言 gRPC code 为 `Unimplemented`。

- [ ] **Step 3: 完整实现 callbacks registry**

严格实现 v0.14 SotW callback 签名：

```go
OnStreamOpen(context.Context, int64, string) error
OnStreamClosed(int64, *corev3.Node)
OnStreamRequest(int64, *discoveryv3.DiscoveryRequest) error
OnStreamResponse(context.Context, int64, *discoveryv3.DiscoveryRequest, *discoveryv3.DiscoveryResponse)
```

`OnStreamResponse` 发生在真正 Send 前，但 nonce 已生成；记录 latest `(stream,type,nonce)->sentVersion/nodeID`。`OnStreamRequest` 发生在库自身 stale nonce 检查前，因此必须自行忽略未知或迟到 nonce。NACK 的被拒版本取 sent record，不能取 request.VersionInfo；后者只作为 acceptedVersion 诊断字段。

先释放 registry mutex 再调用外部 sink，防止同步 NACK 等待时和 StreamClosed 死锁。

- [ ] **Step 4: 验证空资源删除响应**

通过 bufconn 先发布带 RDS/CDS/EDS 的 v1，再发布这些类型显式空列表的 v2。客户端必须收到 version=v2 的空响应并能 ACK，而不是因 type version 为空继续保留旧资源。

- [ ] **Step 5: 运行 xDS 测试和依赖检查**

Run:

```bash
go test -race ./internal/envoy/xds -count=1
go list -deps ./internal/envoy/xds | rg 'github.com/lgc202/ingate/internal/envoy/delivery'
```

Expected: 第一条 PASS；第二条无输出。

- [ ] **Step 6: 提交**

```bash
git add internal/envoy/xds
git commit -m "feat(xds): serve standard SotW ADS"
```

---

### Task 9: 实现内部 etcd Last Good Store

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-error-handling, @go-defensive

**Files:**

- Create: `internal/envoy/lastgood/record.go`
- Create: `internal/envoy/lastgood/codec.go`
- Create: `internal/envoy/lastgood/store.go`
- Create: `internal/envoy/lastgood/codec_test.go`
- Create: `internal/envoy/lastgood/store_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: 写 record codec 的失败测试**

单 key 固定为：

```text
/ingate/internal/last-good/envoy
```

记录至少包含：

```go
type Record struct {
	SchemaVersion int       `json:"schemaVersion"`
	Version       string    `json:"version"`
	ContentHash   string    `json:"contentHash"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Listeners     [][]byte  `json:"listeners"`
	Routes        [][]byte  `json:"routes"`
	Clusters      [][]byte  `json:"clusters"`
	Endpoints     [][]byte  `json:"endpoints"`
}
```

每个 `[]byte` 是对应具体 Envoy protobuf 的 deterministic marshal 结果，不是用户 API JSON。

- [ ] **Step 2: 覆盖损坏和不兼容输入**

测试 schema 不匹配、hash 篡改、畸形 protobuf、重复资源名、缺失引用、snapshot `Consistent()` 失败时 Load 返回错误且不返回部分 Config；原 etcd 记录不删除。

错误必须可分类：

```go
var (
	ErrNotFound     = errors.New("last good not found")
	ErrCorrupt      = errors.New("last good corrupt")
	ErrIncompatible = errors.New("last good incompatible")
)
```

etcd transport、权限和超时错误保留为外部存储错误，不能误判为 NotFound/Corrupt。

- [ ] **Step 3: 实现 deterministic codec**

编码前按资源名排序；使用 `proto.MarshalOptions{Deterministic: true}`。ContentHash 对 version 之外的规范化资源内容计算并在 Load 时复验。解码后重新构造 `config.Config` 和 `cachev3.Snapshot`，再次执行 `Consistent()`。

- [ ] **Step 4: 实现 etcd Store**

将 `go.etcd.io/etcd/client/v3` 提升为直接依赖。Store 只接受真实外部边界 `clientv3.KV`，提供 `Load(ctx)` 与 `Save(ctx, record)`；不注册 API 资源、不生成 client/informer。Save 先在内存完成全部编码、hash 和 consistency 校验，最后单次 Put。

- [ ] **Step 5: 运行测试**

Run:

```bash
go test -race ./internal/envoy/lastgood -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/envoy/lastgood go.mod go.sum
git commit -m "feat(controller): persist last good Envoy config"
```

---

### Task 10: 实现 Candidate、Active、Baseline 和 Last Good Delivery 状态机

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-concurrency, @go-context, @go-error-handling

**Files:**

- Create: `internal/envoy/delivery/types.go`
- Create: `internal/envoy/delivery/baseline.go`
- Create: `internal/envoy/delivery/state.go`
- Create: `internal/envoy/delivery/delivery.go`
- Create: `internal/envoy/delivery/baseline_test.go`
- Create: `internal/envoy/delivery/state_test.go`
- Create: `internal/envoy/delivery/delivery_test.go`
- Create: `internal/envoy/delivery/nack_integration_test.go`

- [ ] **Step 1: 写状态枚举和 Baseline 失败测试**

固定状态：

```go
type State string

const (
	StateNoConfig State = "NoConfig"
	StateWaitingForEnvoy State = "WaitingForEnvoy"
	StateWaitingForACK State = "WaitingForACK"
	StateActive State = "Active"
	StateDegraded State = "Degraded"
)
```

Baseline 必须为 LDS/RDS/CDS/EDS 四类显式空资源、每类非空 baseline version，并通过 `Consistent()`；Baseline 不写入 Last Good，`configReady=false`。

- [ ] **Step 2: 写 Candidate supersede 和 ACK 完成条件失败测试**

覆盖：

- 无 Envoy 时新 Candidate 为 `WaitingForEnvoy`；
- 新 Candidate supersede 旧 Candidate；
- 旧版本迟到 ACK/NACK 永远不能改变状态；
- required type 是 Active 与 Candidate 实际动态类型并集，LDS 始终必需；
- 同一 stream/node 已实际发送并 ACK 全部 required type 后成为 Active；
- 至少一个同构实例完整 ACK 即可 Active；
- Candidate 版本与当前 Active 内容相同则 no-op。
- Last Good retry command 固定携带待持久化的 version 和 content hash；v1 retry 到达时若当前 Active 已是 v2，必须丢弃且不能覆盖 v2。
- 没有 Envoy 连接/实际 response 时保持 `WaitingForEnvoy` 且不启动 ACK timeout；首次发送订阅响应后才进入 `WaitingForACK` 并启动 30 秒 timer；timer 到期只记录 timeout，Candidate 仍留在 cache、Last Good 不变；新 Candidate supersede 时旧 timer 不能修改新状态。

- [ ] **Step 3: 写 NACK 同步回滚失败测试**

`nack_integration_test.go` 使用真实 Snapshot Cache + SotW server：

1. v1 已 Active；
2. 发布 v2；
3. 客户端用 `VersionInfo=v1` 和 v2 nonce NACK；
4. xDS callback 同步调用 Delivery；
5. Delivery 在 callback 返回前完成 `SetSnapshot(v1)`；
6. 断言 v2 不会再次发送；
7. 后续 v3 可正常发布。

无旧 Active 时恢复 Baseline 并回到 `NoConfig`。回滚失败或超过 `nackRollbackTimeout` 时 sink 返回 error，关闭 stream。

另覆盖多实例后置 NACK：node A 完整 ACK v2 后 v2 已 Active 且 Last Good=v2，node B 再对同一 v2 NACK 时只能更新 `Degraded`/lastNACK，不得把 fleet 回滚到 v1/Baseline，也不得覆盖 v2 Last Good。

- [ ] **Step 4: 实现单线程命令循环**

所有 Submit、xDS Event、ACK timeout 和 Last Good retry 都进入一个 command channel；只允许该 goroutine 修改 Candidate/Active/ACK map/Last Good 状态和调用 `SetSnapshot`。ACK 可以异步入队，NACK 通过 command reply 等待同步结果。

- [ ] **Step 5: 实现 Active 后持久化与 degraded 语义**

Candidate Active 后才 Save Last Good。Save 失败不回滚 Active，状态变为 Degraded 并重试持久化；旧 Last Good 保持。每个 retry command 携带目标 version/content hash，执行前再次比较当前 Active，只有完全匹配才允许 Save；因此旧 v1 retry 不能覆盖已经 Active 的 v2。只有同一版本成功持久化后才能清除对应的 persistence degraded，过期 retry 的成功/失败都不能改变当前状态。版本已经 Active 后，其他实例 NACK 只更新 degraded/lastNACK，不回滚 fleet。

默认 30 秒未完整 ACK 时保留 Candidate 在 Cache、保持 `WaitingForACK`，不自动覆盖 Last Good；新资源变化仍可 supersede。

- [ ] **Step 6: 运行状态机与真实回滚测试**

Run:

```bash
go test -race ./internal/envoy/delivery -count=1
```

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/envoy/delivery
git commit -m "feat(controller): add atomic Envoy config delivery"
```

---

### Task 11: 改为完整配置域 Reconciler 并写 Accepted 状态

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-concurrency

**Files:**

- Create: `internal/controller/reconcile/reconciler.go`
- Create: `internal/controller/reconcile/events.go`
- Create: `internal/controller/reconcile/resources.go`
- Create: `internal/controller/reconcile/reconciler_test.go`
- Create: `internal/controller/reconcile/events_test.go`
- Create: `internal/controller/reconcile/resources_test.go`
- Create: `internal/controller/status/types.go`
- Create: `internal/controller/status/runtime.go`
- Create: `internal/controller/status/resources.go`
- Create: `internal/controller/status/runtime_test.go`
- Create: `internal/controller/status/resources_test.go`

- [ ] **Step 1: 写唯一全局 queue key 的失败测试**

Gateway、Route、Upstream、RateLimitPolicy、AccessControlPolicy、PolicyBinding 的 Add/Delete/spec Update 全部只 enqueue：

```go
const queueKey = "config"
```

Update 时若 `old.Generation == new.Generation` 则忽略，防止 status-only update 自激；不再建立 per-Gateway、RedisRef、PolicyRef 或 UpstreamRef index。

- [ ] **Step 2: 写完整不可变 ResourceSet 构造测试**

`resources.go` 从所有 informer lister `List(labels.Everything())` 读取完整集合，并对每个对象做值拷贝/DeepCopy，确保 Compiler 期间不观察到 informer cache 变化。ResourceSet 不含 RuntimeGroup、RuntimeSnapshot 或 RedisStore。

- [ ] **Step 3: 写 reconcile 错误分类测试**

流程固定：

```text
build ResourceSet
-> Compiler.Compile
-> Status.ApplyDiagnostics
-> 有 Error：Forget，等待新的 spec 事件
-> 无 Error：Delivery.Submit
```

确定性 InvalidSpec/ReferenceNotFound/Conflict/Unsupported 不 rate-limit 重试；lister、client、Delivery channel 或 status API 等基础设施错误才 AddRateLimited。

- [ ] **Step 4: 写 Accepted Condition 失败测试**

对当前所有资源只维护一个 `Accepted` Condition。Error diagnostic 命中的资源写 `Status=False` 和对应稳定 reason；无 Error 的当前 generation 写 `Status=True, Reason=Accepted`。Condition 包含 ObservedGeneration、LastTransitionTime 和不泄漏内部堆栈的 message。

Condition 内容未变化时不 UpdateStatus；resourceVersion 冲突使用 `retry.RetryOnConflict` 重新 Get/UpdateStatus。

- [ ] **Step 5: 实现 Reconciler 和 Status writer**

Reconciler 只调用 Compiler、Delivery 和 Status，不生成 protobuf、不管理 stream、不读写 Last Good。`status.Runtime` 是线程安全的只读快照容器，由 Delivery/xDS/Reconciler 更新，后续 HTTP server 读取。

- [ ] **Step 6: 运行测试**

Run:

```bash
go test -race ./internal/controller/reconcile ./internal/controller/status -count=1
```

Expected: PASS，status 更新不会再次触发 reconcile。

- [ ] **Step 7: 提交**

```bash
git add internal/controller/reconcile internal/controller/status
git commit -m "refactor(controller): reconcile the full config domain"
```

---

### Task 12: 在 ingate-controller 内装配 xDS、Delivery、Reconciler 和状态服务

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-concurrency, @go-context, @go-logging

**Files:**

- Create: `internal/controller/app/options.go`
- Create: `internal/controller/app/options_test.go`
- Modify: `internal/controller/app/app.go`
- Modify: `cmd/ingate-controller/main.go`
- Create: `internal/controller/status/server.go`
- Create: `internal/controller/status/server_test.go`
- Delete: `internal/controller/controller/controller.go`
- Delete: `internal/controller/controller/events.go`
- Delete: `internal/controller/controller/events_test.go`
- Delete: `internal/controller/controller/reconcile.go`
- Delete: `internal/controller/controller/snapshot.go`
- Delete: `internal/controller/controller/snapshot_test.go`
- Delete: `internal/core/compiler/compiler.go`
- Delete: `internal/core/compiler/compiler_test.go`
- Delete: `internal/core/ir/gateway.go`
- Delete: `internal/core/pipeline/pipeline.go`
- Delete: `internal/core/pipeline/pipeline_test.go`
- Delete: `internal/core/runtime/snapshot.go`
- Delete: `internal/core/target/translator.go`
- Delete: `internal/core/target/registry.go`
- Delete: `internal/core/target/registry_test.go`
- Delete: `internal/core/target/builtin/registry.go`
- Delete: `internal/core/target/builtin/registry_test.go`
- Delete: `internal/core/target/debug/translator.go`
- Delete: `internal/core/target/debug/translator_test.go`
- Delete: `internal/core/target/xds/accesscontrol.go`
- Delete: `internal/core/target/xds/ratelimit.go`
- Delete: `internal/core/target/xds/translator.go`
- Delete: `internal/core/target/xds/translator_test.go`
- Delete: `cmd/ingate-xds/main.go`
- Delete: `internal/xds/app/app.go`
- Delete: `internal/xds/app/options.go`
- Delete: `internal/xds/server/accesscontrol_builder.go`
- Delete: `internal/xds/server/ads_server.go`
- Delete: `internal/xds/server/ads_stream_state.go`
- Delete: `internal/xds/server/ads_update_notifier.go`
- Delete: `internal/xds/server/cluster_builder.go`
- Delete: `internal/xds/server/cluster_builder_test.go`
- Delete: `internal/xds/server/endpoint_builder.go`
- Delete: `internal/xds/server/envoy_helpers.go`
- Delete: `internal/xds/server/listener_builder.go`
- Delete: `internal/xds/server/listener_builder_test.go`
- Delete: `internal/xds/server/ratelimit_builder.go`
- Delete: `internal/xds/server/response_builder.go`
- Delete: `internal/xds/server/route_builder.go`
- Delete: `internal/xds/server/route_builder_test.go`
- Delete: `internal/xds/server/server.go`
- Delete: `internal/xds/server/snapshot_store.go`
- Delete: `internal/xds/server/snapshot_watcher.go`
- Modify: `deploy/all-in-one/Dockerfile`
- Modify: `deploy/all-in-one/entrypoint.sh`
- Modify: `deploy/all-in-one/default.env`
- Modify: `install.sh`
- Modify: `Makefile`

- [ ] **Step 1: 写启动参数和 readiness 失败测试**

删除 `--target`，新增：

```text
--xds-listen-address=:18000
--internal-listen-address=127.0.0.1:18080
--etcd-endpoints=http://127.0.0.1:2379
--candidate-ack-timeout=30s
--nack-rollback-timeout=3s
--resync-period=0
```

all-in-one 显式把环境变量映射到 pflag，不能只写进 `default.env` 后假设二进制会自动读取：

```text
INGATE_CONTROLLER_XDS_ADDR       -> --xds-listen-address
INGATE_CONTROLLER_INTERNAL_ADDR  -> --internal-listen-address
INGATE_ETCD_ADDR                 -> --etcd-endpoints=http://<value>
INGATE_CANDIDATE_ACK_TIMEOUT     -> --candidate-ack-timeout
INGATE_NACK_ROLLBACK_TIMEOUT     -> --nack-rollback-timeout
INGATE_RESYNC_PERIOD             -> --resync-period
```

同一变更删除 `INGATE_XDS_ADDR`、Controller `--target` 以及独立 xDS 的 `--listen-address/--target` 接线。测试 `/readyz` 在 Last Good/Baseline 初始化完成且 ADS listener 已监听后返回 200；不等待 Envoy 连接、Candidate 或 ACK。`/healthz` 只表示进程存活。

二进制通用默认值可以保留 `:18000` 供独立部署显式控制，但 all-in-one 的 `default.env` 必须固定 `INGATE_CONTROLLER_XDS_ADDR=127.0.0.1:18000`，确保 ADS 仅在容器 loopback 可达。

- [ ] **Step 2: 实现 Controller Internal Status HTTP API**

`GET /internal/v1/status` 返回：

```json
{
  "candidateVersion": "",
  "activeVersion": "",
  "lastGoodVersion": "",
  "configReady": false,
  "deliveryState": "NoConfig",
  "connectedEnvoys": 0,
  "acks": {"required": 0, "received": 0},
  "nacks": {"count": 0},
  "lastNack": null
}
```

`lastNack` 只暴露 nodeID、typeURL、version、time 和截断后的稳定错误摘要。内部 server 默认只监听内部地址。

- [ ] **Step 3: 写 app 装配顺序测试**

装配顺序必须是：

```text
创建 shared Snapshot Cache
-> 创建 Last Good Store / Delivery
-> Restore Last Good；无记录则安装 Baseline
-> 启动 ADS listener 和内部 HTTP
-> 标记 ready
-> 启动 informer / Reconciler
-> cache sync 后 enqueue 全局 key
```

测试当前声明式资源编译失败时仍继续服务已恢复 Last Good，并覆盖启动分类：

- `ErrNotFound`：安装 Baseline，状态 NoConfig；
- `ErrCorrupt` / `ErrIncompatible`：保留原 etcd 记录，安装 Baseline，状态 Degraded，继续启动 ADS 和 informer，并尝试从当前声明式资源重新编译；
- etcd transport/权限或 Last Good store 初始化失败：启动失败，不伪装成空配置。schema marker 检查在 Task 16 与 API cutover 同时接入。

- [ ] **Step 4: 实现唯一 app 装配入口**

`app` 创建同一个 `cachev3.SnapshotCache` 并分别注入 Delivery 与 xDS Server。xDS callbacks 使用闭包调用 `delivery.HandleXDSEvent`；xDS package 不 import Delivery。使用 `errgroup.WithContext` 管理 ADS、Internal HTTP、Delivery loop 和 Reconciler 生命周期。

- [ ] **Step 5: 删除旧 Controller/Core/xDS 文件**

只有在新 app 测试通过后删除旧目录。不要搬迁 `Logical*` 类型；现有 builder 中仍有价值的 protobuf 逻辑应已在 Task 6/7 迁入 `internal/envoy/config`。

同时从 all-in-one Dockerfile 删除 `ingate-xds` binary COPY，从 entrypoint 删除独立进程启动和等待；由 `ingate-controller` 直接监听 18000。`default.env` 与 `install.sh` 生成的 env 文件同步改为 Controller 参数，保证删除源码的同一提交仍能构建并启动镜像。此时 `ingate-dataplane` 仍保留到 Task 17。

- [ ] **Step 6: 运行合并后的 Controller 测试和构建**

Run:

```bash
go test -race ./internal/controller/... ./internal/envoy/... -count=1
go build ./cmd/ingate-controller
make all-in-one-image
rg -n 'RuntimeSnapshot|LogicalGateway|internal/core/(ir|pipeline|target|runtime)|ingate-xds|--target|INGATE_XDS_ADDR' cmd internal/controller internal/envoy deploy/all-in-one install.sh --glob '*.go' --glob '*.sh' --glob '*.env' --glob 'Dockerfile'
```

Expected: 前三条 PASS；最后一条无输出；all-in-one 只运行合并后的 Controller xDS 端口。

- [ ] **Step 7: 提交**

```bash
git add cmd/ingate-controller internal/controller internal/envoy cmd/ingate-xds internal/xds internal/core deploy/all-in-one install.sh Makefile
git commit -m "refactor(controller): merge controller and xds process"
```

---

### Task 13: 通过 Admin API 暴露产品化运行状态

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-context, @go-error-handling

**Files:**

- Create: `internal/adminapi/client/controller/types.go`
- Create: `internal/adminapi/client/controller/client.go`
- Create: `internal/adminapi/client/controller/client_test.go`
- Create: `internal/adminapi/service/systemstatus/service.go`
- Create: `internal/adminapi/service/systemstatus/service_test.go`
- Create: `internal/adminapi/handler/systemstatus/dto/response.go`
- Create: `internal/adminapi/handler/systemstatus/handler.go`
- Create: `internal/adminapi/handler/systemstatus/handler_test.go`
- Modify: `internal/adminapi/app/options.go`
- Modify: `internal/adminapi/app/app.go`
- Modify: `internal/adminapi/server/server.go`
- Modify: `internal/adminapi/server/router.go`
- Modify: `internal/adminapi/handler/handler.go`
- Modify: `internal/adminapi/service/service.go`
- Modify: `internal/adminapi/store/store.go`
- Modify: `deploy/all-in-one/entrypoint.sh`
- Modify: `deploy/all-in-one/default.env`
- Modify: `install.sh`

- [ ] **Step 1: 写 Controller client 边界失败测试**

使用 `httptest.Server` 覆盖正常 JSON、超时、连接拒绝、非 2xx、非法 JSON 和超大错误文本。Client 使用独立 `http.Client`，默认 timeout 500ms，不把底层 URL/连接错误放入产品 DTO。

- [ ] **Step 2: 定义产品 DTO**

Admin API endpoint 固定为：

```text
GET /api/v1/system/status
```

响应 data：

```go
type GetStatusResp struct {
	Available       bool       `json:"available"`
	Message         string     `json:"message"`
	ConfigReady     bool       `json:"configReady"`
	DeliveryState   string     `json:"deliveryState"`
	CandidateVersion string    `json:"candidateVersion,omitempty"`
	ActiveVersion   string     `json:"activeVersion,omitempty"`
	LastGoodVersion string     `json:"lastGoodVersion,omitempty"`
	ConnectedEnvoys int        `json:"connectedEnvoys"`
	ACK             ACKSummary `json:"ack"`
	LastNACK        *NACK      `json:"lastNack,omitempty"`
}
```

Controller 不可用时 endpoint 仍返回统一 HTTP 200 响应体，`available=false`、`message="运行状态暂不可用"`；资源 CRUD 不依赖该调用。

- [ ] **Step 3: 实现 client、service、handler 和 router**

新增参数：

```text
--controller-status-url=http://127.0.0.1:18080
--controller-status-timeout=500ms
```

Service 捕获内部 client 错误并转换成 unavailable result，不向 Handler 返回底层错误。Handler 只调用 service 并使用 `response.GinJSONResponse`。

all-in-one 在同一提交显式接线：

```text
INGATE_CONTROLLER_INTERNAL_ADDR  -> --controller-status-url=http://<value>
INGATE_CONTROLLER_STATUS_TIMEOUT -> --controller-status-timeout
```

`default.env` 和 `install.sh` 生成的 env 文件增加 timeout，entrypoint 必须把两个值作为 pflag 传给 admin-api，不能只导出环境变量。

- [ ] **Step 4: 运行测试**

Run:

```bash
go test -race ./internal/adminapi/client/controller ./internal/adminapi/service/systemstatus ./internal/adminapi/handler/systemstatus ./internal/adminapi/server -count=1
make all-in-one-image
```

Expected: PASS；Controller unavailable 时 Gateway/Route router 测试仍可正常响应；启动 all-in-one 后 `GET /api/v1/system/status` 的 `available=true`，证明请求实际到达 Controller internal server。

- [ ] **Step 5: 提交**

```bash
git add internal/adminapi deploy/all-in-one install.sh
git commit -m "feat(adminapi): expose controller runtime status"
```

---

### Task 14: 将 Console 发布页迁移为单配置域运行状态

**Required skills:** @superpowers:test-driven-development

**Files:**

- Modify: `web/console/src/domain/publish.ts`
- Modify: `web/console/src/api/contracts.ts`
- Modify: `web/console/src/api/liveConsoleRepository.ts`
- Modify: `web/console/src/features/publish/PublishPage.tsx`
- Create: `web/console/src/features/publish/status.test.ts`
- Modify: `web/console/src/features/home/HomePage.tsx`
- Modify: `web/console/src/domain/home.ts`
- Modify: `web/console/src/domain/settings.ts`
- Modify: `web/console/src/features/settings/SettingsPage.tsx`
- Modify: `web/console/src/mocks/consoleRepository.ts`
- Modify: `web/console/src/app/navigation.ts`

- [ ] **Step 1: 写前端运行状态映射失败测试**

删除 `RuntimeTarget`、`PublishSnapshot`、逐 Gateway snapshot list 和 `xds/debug` target。测试新的 repository 方法只调用 `/system/status`，正确映射 Available、Candidate、Active、Last Good、connected Envoys、ACK 和 last NACK。

- [ ] **Step 2: 重写领域模型**

`PublishListView` 改为单配置域状态，例如：

```ts
export interface RuntimeStatusView {
  available: boolean;
  message: string;
  configReady: boolean;
  deliveryState: 'NoConfig' | 'WaitingForEnvoy' | 'WaitingForACK' | 'Active' | 'Degraded';
  candidateVersion?: string;
  activeVersion?: string;
  lastGoodVersion?: string;
  connectedEnvoys: number;
  ack: { required: number; received: number };
  lastNack?: { nodeID: string; typeURL: string; version: string; time: string; message: string };
}
```

- [ ] **Step 3: 重写运行状态页面**

页面不再展示“网关 / 生效目标 / snapshot 列表”。改为：

- 当前状态和 configReady；
- Candidate / Active / Last Good 版本卡片；
- 已连接 Envoy 数；
- ACK 进度；
- Last NACK 摘要；
- unavailable 时稳定提示。

首页“默认 target”改为“单一 Envoy 配置域”；设置页删除“快照保留数量”等不存在的产品参数。

- [ ] **Step 4: 运行前端测试和构建**

Run:

```bash
make console-test
make console-build
```

Expected: PASS；前端源码无 `RuntimeTarget`、`PublishSnapshot`、`debug target`。

- [ ] **Step 5: 提交**

```bash
git add web/console
git commit -m "refactor(console): show global Envoy runtime status"
```

---

### Task 15: 准备 schema marker 2 校验与 reset 核心库

**Required skills:** @superpowers:test-driven-development, @go-testing, @go-error-handling

**Files:**

- Create: `internal/storage/schema/schema.go`
- Create: `internal/storage/schema/reset.go`
- Create: `internal/storage/schema/schema_test.go`

- [ ] **Step 1: 写 marker 状态矩阵失败测试**

固定：

```go
const (
	MarkerKey = "/ingate/internal/schema-version"
	CurrentVersion = "2"
	ResourcePrefix = "/registry/gateway.ingate.io/"
	InternalPrefix = "/ingate/internal/"
)
```

`Check` 矩阵：值为 2 成功；marker 缺失（即使资源前缀为空）、旧值、未知值均失败并提示运行 bootstrap/reset。

- [ ] **Step 2: 写 bootstrap/reset 核心行为失败测试**

`Bootstrap` 仅在 marker 缺失且 Resource/Internal 前缀均为空时写入 2；已有有效 marker 时 no-op；已有旧数据或未知 marker 时拒绝。`Reset` 顺序删除 ResourcePrefix、删除 InternalPrefix、最后写 marker=2；任一步失败立即返回。崩溃在写 marker 前会使服务 fail-fast，是安全状态。

测试只通过 fake `clientv3.KV` 调用内部库，不创建可执行命令、部署入口或自动启动路径，避免旧 API 仍在运行时有人提前写入 marker=2。

- [ ] **Step 3: 实现 schema 核心库**

使用 `clientv3.KV` 作为真实外部边界，不创建临时 API 资源。库提供 `Check`、`Bootstrap`、`Reset`，但 Task 15 不从 `cmd/ingate`、apiserver、controller 或 entrypoint 暴露这些写操作。CLI、服务启动校验和部署接线必须等到 Task 16 与旧 API 删除一起启用。

- [ ] **Step 4: 运行测试**

Run:

```bash
go test ./internal/storage/schema -count=1
```

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/storage/schema
git commit -m "feat(storage): prepare schema version library"
```

---

### Task 16: 原子删除旧 API 并启用 schema marker 2

**Required skills:** @superpowers:test-driven-development, @go-testing

**Files:**

- Delete: `pkg/apis/gateway/runtimegroup.go`
- Delete: `pkg/apis/gateway/v1/runtimegroup.go`
- Delete: `pkg/apis/gateway/runtimesnapshot.go`
- Delete: `pkg/apis/gateway/v1/runtimesnapshot.go`
- Delete: `pkg/apis/gateway/redisstore.go`
- Delete: `pkg/apis/gateway/v1/redisstore.go`
- Modify: `pkg/apis/gateway/gateway.go`
- Modify: `pkg/apis/gateway/v1/gateway.go`
- Modify: `pkg/apis/gateway/ratelimit_policy.go`
- Modify: `pkg/apis/gateway/v1/ratelimit_policy.go`
- Modify: `pkg/apis/gateway/types.go`
- Modify: `pkg/apis/gateway/v1/types.go`
- Modify: `pkg/apis/gateway/register.go`
- Modify: `pkg/apis/gateway/v1/register.go`
- Delete: `internal/apiserver/registry/runtimegroup/storage.go`
- Delete: `internal/apiserver/registry/runtimegroup/strategy.go`
- Delete: `internal/apiserver/registry/runtimesnapshot/storage.go`
- Delete: `internal/apiserver/registry/runtimesnapshot/strategy.go`
- Delete: `internal/apiserver/registry/redisstore/storage.go`
- Delete: `internal/apiserver/registry/redisstore/strategy.go`
- Modify: `internal/apiserver/server/config.go`
- Modify: `internal/apiserver/server/options.go`
- Modify: `internal/apiserver/server/scheme_test.go`
- Modify: `internal/apiserver/app/app.go`
- Create: `internal/apiserver/app/schema_test.go`
- Modify: `internal/controller/app/options.go`
- Modify: `internal/controller/app/options_test.go`
- Modify: `internal/controller/app/app.go`
- Create: `internal/controller/app/schema_test.go`
- Create: `internal/cli/app/app.go`
- Create: `internal/cli/app/storage.go`
- Create: `internal/cli/app/storage_test.go`
- Create: `cmd/ingate/main.go`
- Modify: `deploy/all-in-one/Dockerfile`
- Modify: `deploy/all-in-one/entrypoint.sh`
- Modify: `deploy/all-in-one/default.env`
- Modify: `install.sh`
- Modify: `Makefile`
- Create: `test/schema/run.sh`
- Modify: `hack/openapi/api-rule-violations.report`
- Regenerate: `pkg/apis/gateway/zz_generated.deepcopy.go`
- Regenerate: `pkg/apis/gateway/v1/zz_generated.deepcopy.go`
- Regenerate: `pkg/apis/gateway/v1/zz_generated.conversion.go`
- Regenerate: `pkg/generated/clientset/versioned/**`
- Regenerate: `pkg/generated/informers/externalversions/**`
- Regenerate: `pkg/generated/listers/gateway/v1/**`
- Regenerate: `pkg/generated/openapi/zz_generated.openapi.go`

- [ ] **Step 1: 写最终 Scheme/OpenAPI 失败测试**

断言 RuntimeGroup、RuntimeSnapshot、RedisStore 的 internal/v1 GVK 均未注册；storage map 不安装对应资源；Gateway OpenAPI 没有 `runtimeGroupRef`；RateLimitPolicy OpenAPI 没有 `global`；仍保留 `RateLimitModeGlobal`。

- [ ] **Step 2: 写 schema 启动边界和 CLI 失败测试**

在 apiserver 和 controller 的真实入口测试：`schema.Check` 必须在 apiserver 打开服务 listener、controller 创建 informer/ADS listener 和启动 Reconciler 之前完成；marker 缺失、旧值、未知值都 fail-fast，不能把错误当成空配置继续运行。etcd transport/权限错误原样归为启动失败。

CLI 在同一任务首次出现并固定提供：

```text
ingate storage check
ingate storage bootstrap
ingate storage reset --confirm-services-stopped
```

`reset` 未带确认参数必须拒绝；输出必须说明它不会替调用方停止 apiserver/controller。测试 bootstrap 的空 volume、旧数据保护、unknown marker、reset 删除两个前缀后写 marker=2。

- [ ] **Step 3: 删除源 API 模型和常量**

删除：

```text
GatewaySpec.RuntimeGroupRef
RuntimeGroupRef
RateLimitPolicySpec.Global
GlobalRateLimitConfig
KindRuntimeGroup
KindRedisStore
RedisMode 及其常量
Bundle（整个旧 compiler helper，不只 RedisStores 字段）
RuntimeGroup/RuntimeSnapshot/RedisStore resource names
```

- [ ] **Step 4: 删除 registry 并更新 server storage**

`internal/apiserver/server/config.go` 只安装 Gateway、Route、Upstream、RateLimitPolicy、AccessControlPolicy、PolicyBinding 及其 status storage。

- [ ] **Step 5: 在同一原子边界接入服务检查、CLI 和部署 bootstrap/reset**

apiserver 从其实际 `--etcd-servers` 配置构造 schema checker；controller 使用 `--etcd-endpoints`，二者都在监听/缓存初始化前调用 `Check`。旧 API 删除、marker=2 检查和新 CLI 不拆成可独立发布的提交。

all-in-one entrypoint 在启动 apiserver/controller 前执行 `storage bootstrap`（仅空 volume）；检测到旧数据、缺 marker 或非 2 直接退出并保留 key。`install.sh reset` 先停止并删除容器，再用同一镜像启动 etcd one-shot action 执行带确认的 reset，之后才正常 start；不得用删除宿主目录替代 reset。`make dev-reset` 调用同一路径。Task 16 之后的每个 all-in-one 镜像都必须包含 `ingate` CLI，但最终镜像仍不自动迁移数据。

- [ ] **Step 6: 重新生成全部代码**

Run:

```bash
make generate
```

检查旧 generated client/informer/lister 文件已由 generator 删除，尤其：

```text
runtimegroup.go
runtimesnapshot.go
redisstore.go
fake_runtimegroup.go
fake_runtimesnapshot.go
fake_redisstore.go
```

禁止手改 generated 文件。

- [ ] **Step 7: 运行 API、schema 矩阵和全仓测试**

Run:

```bash
go test ./internal/apiserver/... ./pkg/apis/gateway/... -count=1
go test ./internal/storage/schema ./internal/cli/app ./internal/controller/app -count=1
make test
make build
make all-in-one-image
test/schema/run.sh "${ALL_IN_ONE_IMAGE:-ingate/all-in-one:dev}"
```

`test/schema/run.sh` 必须覆盖：全新空 volume 自动 bootstrap marker=2；缺 marker 但已有旧资源时 start 失败且 key 保留；old/unknown marker 失败且数据不删除；显式 reset 清空 ResourcePrefix/InternalPrefix 并写 marker=2；`make dev-reset` 走同一路径。Expected: 全部 PASS。

- [ ] **Step 8: 提交并验证生成幂等**

```bash
git add pkg/apis pkg/generated internal/apiserver internal/controller internal/cli cmd/ingate deploy/all-in-one install.sh Makefile test/schema hack/openapi
git commit -m "refactor(api): remove runtime and Redis resources"
make generate
git diff --exit-code
```

Expected: 最后一条无 diff。

---

### Task 17: 删除 ingate-dataplane、HTTP DTO、pkg/xredis 和 go-redis

**Required skills:** @superpowers:test-driven-development, @go-testing

**Files:**

- Delete: `cmd/ingate-dataplane/main.go`
- Delete: `internal/dataplane/app/app.go`
- Delete: `internal/dataplane/app/options.go`
- Delete: `internal/dataplane/handler/handler.go`
- Delete: `internal/dataplane/handler/ratelimit/handler.go`
- Delete: `internal/dataplane/server/router.go`
- Delete: `internal/dataplane/server/server.go`
- Delete: `internal/dataplane/service/service.go`
- Delete: `internal/dataplane/service/ratelimit/algorithm.go`
- Delete: `internal/dataplane/service/ratelimit/check.go`
- Delete: `internal/dataplane/service/ratelimit/redis.go`
- Delete: `internal/dataplane/service/ratelimit/service.go`
- Delete: `internal/dataplane/service/ratelimit/service_test.go`
- Delete: `pkg/dataplane/ratelimit/doc.go`
- Delete: `pkg/dataplane/ratelimit/types.go`
- Delete: `pkg/xredis/doc.go`
- Delete: `pkg/xredis/types.go`
- Delete: `pkg/xredis/client.go`
- Delete: `pkg/xredis/client_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `deploy/all-in-one/Dockerfile`
- Modify: `deploy/all-in-one/entrypoint.sh`
- Modify: `deploy/all-in-one/default.env`
- Modify: `install.sh`
- Modify: `Makefile`

- [ ] **Step 1: 写残留扫描测试命令**

先运行：

```bash
rg -n 'ingate-dataplane|pkg/dataplane|pkg/xredis|redis/go-redis' --glob '*.go' --glob 'go.mod' .
```

记录所有命中，确认插件已在 Task 5 切换，剩余命中仅属于待删除服务。

- [ ] **Step 2: 删除完整代码闭包和依赖**

删除上述目录，从 `go.mod` 删除 `github.com/redis/go-redis/v9`，运行 `go mod tidy`。不要保留 compatibility DTO、fallback HTTP transport 或空 package。

同一提交从 Dockerfile 删除 `ingate-dataplane` COPY，从 entrypoint 删除启动/等待，从 `default.env` 与 `install.sh` 删除 `INGATE_DATAPLANE_ADDR`，并移除只服务于该进程的 Makefile 目标。删除源码后必须立即保证 all-in-one 仍可构建和启动，不能留到 Task 18。

- [ ] **Step 3: 运行测试、构建和残留扫描**

Run:

```bash
make test
make build
make plugins-test
make plugins-build
make all-in-one-image
rg -n 'ingate-dataplane|pkg/dataplane|pkg/xredis|redis/go-redis|INGATE_DATAPLANE_ADDR' --glob '*.go' --glob 'go.mod' --glob '*.sh' --glob '*.env' --glob 'Dockerfile' .
```

Expected: 前五条 PASS；最后一条无输出。

- [ ] **Step 4: 提交**

```bash
git add cmd internal pkg go.mod go.sum deploy/all-in-one install.sh Makefile
git commit -m "refactor(dataplane): remove legacy rate limit service"
```

---

### Task 18: 收口 all-in-one 镜像、多架构与健康检查

**Required skills:** @superpowers:test-driven-development

**Files:**

- Modify: `deploy/all-in-one/redis/redis.conf`
- Create: `deploy/all-in-one/healthcheck.sh`
- Modify: `deploy/all-in-one/Dockerfile`
- Modify: `deploy/all-in-one/entrypoint.sh`
- Modify: `deploy/all-in-one/default.env`
- Modify: `deploy/all-in-one/envoy/bootstrap.yaml`
- Modify: `install.sh`
- Modify: `Makefile`

- [ ] **Step 1: 写镜像二进制闭包和进程集合 smoke**

Makefile 增加 `all-in-one-smoke`，检查：

```text
envoy --version 显示 1.36.4
envoy --mode validate -c /etc/ingate/envoy/bootstrap.yaml 成功
redis-server --version 成功
redis-cli -h 127.0.0.1 ping 返回 PONG
etcdctl version 成功
ldd envoy/redis-server 无 not found
镜像中没有 ingate-xds、ingate-dataplane、ingate-httpbin
```

Higress gateway v2.2.3 和官方 Redis bookworm 镜像都必须在 spike 验证后固定为 immutable digest；Dockerfile 不保留可变 tag。

- [ ] **Step 2: 修改 Dockerfile 并统一目标架构**

复制：

- etcd 和 `etcdctl`；`etcdctl` 只用于安装诊断与 E2E 数据损坏验证；
- `ingate`、ingate-apiserver、ingate-admin-api、ingate-controller；
- Higress Envoy 二进制；
- 官方 Redis `redis-server`/`redis-cli` 及运行所需动态库；
- Console 和内置 Wasm。

删除独立 xDS/dataplane 和 demo `ingate-httpbin` binary COPY；httpbin 只作为 E2E 外部 backend，不属于最终产品组件。增加 Docker `HEALTHCHECK` 调用 `/usr/local/bin/ingate-healthcheck`，并固定足以覆盖 etcd/schema/controller 冷启动的参数，例如 `--start-period=60s --interval=5s --timeout=3s --retries=6`，避免正常启动在 readiness 建立前被判 unhealthy。

删除 `ALL_IN_ONE_GOARCH ?= arm64`。使用同一个 `ALL_IN_ONE_ARCH ?= $(shell go env GOARCH)` 同时驱动 `GOARCH` 和 `docker build --platform=linux/$(ALL_IN_ONE_ARCH)`，Dockerfile/构建参数统一使用 `TARGETOS/TARGETARCH` 语义，保证 amd64 CI 不会得到“amd64 基础镜像 + arm64 Go binary”。本里程碑只要求 amd64/arm64 单平台构建都可用，不提前发布 buildx 多平台 manifest。

- [ ] **Step 3: 收口固定系统 Redis 配置**

Task 5 已增加静态 cluster；本任务把最终 bootstrap 固定并验证：

```yaml
- name: ingate-system-redis
  type: STATIC
  connect_timeout: 1s
  load_assignment:
    cluster_name: ingate-system-redis
    endpoints:
      - lb_endpoints:
          - endpoint:
              address:
                socket_address:
                  address: 127.0.0.1
                  port_value: 6379
```

该 cluster 不进入 CDS。ADS cluster 指向合并后的 controller 18000 端口。`redis.conf` 保持 loopback-only、无持久化约定，不重新引入用户地址配置。

- [ ] **Step 4: 收口 entrypoint 启动、依赖等待和进程监督**

启动顺序：

```text
etcd
-> 等待 etcd endpoint health
-> ingate storage bootstrap/check（reset action 已在 Task 16 接入）
-> Redis
-> 有界等待 redis-cli PING；失败只记录 degraded 并继续
-> apiserver
-> 等待 apiserver /readyz
-> controller
-> 等待 controller /readyz
-> Envoy
-> 等待 Envoy admin /ready
-> admin-api / Console
```

镜像内 `default.env` 只能为尚未设置的变量提供默认值，不得覆盖 Docker `--env-file` 或 `-e` 注入值；smoke 使用非默认 Console/Controller internal/status timeout 验证外部环境优先。

关键进程与非关键 Redis 明确分组：

```bash
critical_pids=()
auxiliary_pids=()
```

etcd、apiserver、controller、Envoy、admin-api 属于 `critical_pids`；Redis 属于 `auxiliary_pids`。主等待只能执行等价于 `wait -n "${critical_pids[@]}"` 的逻辑，不能对所有 shell child 使用裸 `wait -n`。Redis 首次启动使用有界 PING：成功后继续，超时或进程已退出则记录明确错误但仍启动 Envoy；运行期间由独立 auxiliary watcher `wait`/reap Redis 并记录退出，不结束主进程。普通代理和控制面继续运行，依赖 Redis 的策略按 FailOpen/FailClose 处理。shutdown trap 同时终止并 reap 两组 pid/watcher。

- [ ] **Step 5: 完成 install.sh 健康等待**

Task 16 已实现 reset/bootstrap。这里统一 `start`、`restart` 和 `reset` 的正常启动阶段调用有界 `wait_container_healthy`：每次轮询同时读取 `.State.Running`、`.State.ExitCode` 和 `.State.Health.Status`；进程提前退出时立即打印组件文件日志和 inspect 结果，不能一直等 `starting` 超时。只有 `healthy` 后才能 `print_success`，`unhealthy` 或超时也必须返回失败。

- [ ] **Step 6: 实现整体健康检查**

healthcheck 检查 apiserver `/readyz`、Controller `/readyz` 和 Envoy admin `/ready`；不要求业务 Gateway 存在，也不把 Redis 运行期故障变成整个容器 unhealthy。Redis PING 单独进入 status/smoke。所有组件日志写入 `/var/log/ingate/<component>.log`，失败诊断不能只依赖通常为空的 `docker logs`。

- [ ] **Step 7: 构建和 smoke**

Run:

```bash
make all-in-one-image
make all-in-one-smoke
```

Expected: PASS；`ps` 和镜像均无 ingate-xds/ingate-dataplane/ingate-httpbin；Controller 在 Envoy 启动前 ready；ADS 只监听 `127.0.0.1:18000`；config dump 中静态存在 `ingate-system-redis`；非默认环境变量未被镜像 defaults 覆盖；当前宿主架构与镜像/Go binary 一致。

Smoke 使用外部 backend container。必须证明同 network peer 无法连接 Redis 6379，而容器内 PING 成功；主动 kill Redis 进程，等待超过原裸 `wait -n` 的触发窗口，断言容器仍 running、Docker health 仍 healthy、普通无策略 Route 仍代理成功；随后重启 Redis 供后续测试使用。另以确定性失败的 Redis wrapper/config 启动全新容器，验证 `set -e` 不会让 entrypoint 退出、总体 health 仍 healthy、普通 Route 可代理且组件日志出现 Redis degraded。

- [ ] **Step 8: 提交**

```bash
git add deploy/all-in-one install.sh Makefile
git commit -m "build(all-in-one): embed Envoy and system Redis"
```

---

### Task 19: 增加真实 all-in-one、Delivery 和 Redis RateLimit E2E

**Required skills:** @superpowers:test-driven-development, @go-testing

**Files:**

- Create: `test/e2e/harness_test.go`
- Create: `test/e2e/proxy_test.go`
- Create: `test/e2e/delivery_test.go`
- Create: `test/e2e/ratelimit_test.go`
- Create: `test/e2e/process_test.go`
- Create: `test/e2e/xdsclient/main.go`
- Create: `test/e2e/xdsclient/Dockerfile`
- Create: `test/e2e/testdata/gateway.json`
- Create: `test/e2e/testdata/route.json`
- Create: `test/e2e/testdata/upstream.json`
- Create: `test/e2e/testdata/ratelimit-policies.json`
- Create: `test/e2e/testdata/policy-bindings.json`
- Reuse: `test/backend/Dockerfile`
- Modify: `Makefile`

- [ ] **Step 1: 建立隔离 E2E harness**

`make e2e` 必须依赖并传入本次刚构建的精确 `ALL_IN_ONE_IMAGE`，同时构建/传入 Task 5 的本地 test backend image，再运行带 `e2e` build tag 的 Go tests。Harness 为每组串行测试创建唯一容器名、数据目录和 Docker network，启动该外部 backend，等待 healthcheck，提供声明式 API create/update/delete、Admin status、Envoy admin、组件日志和清理 helper。重启/破坏存储/暂停进程等重型测试禁止 `t.Parallel`，`go test` 使用显式长 timeout 和 `-p=1`。

生产安装仍只映射 Admin/Gateway 端口。E2E 通过 `docker exec` 在容器内访问 apiserver 18443（TLS insecure test client）和 Envoy admin 15000；可控 SotW client 通过 `docker run --network container:<all-in-one>` 共享 network namespace，连接 `127.0.0.1:18000`，不把这些 loopback 内部端口发布到宿主机。sidecar 使用独立稳定 Node ID `ingate-e2e-controlled`，不复用被 SIGSTOP 的真实 Envoy Node ID。

entrypoint 把日志重定向到挂载的 `/var/log/ingate/<component>.log`，因此 harness 必须读取/复制这些文件，不能把 `docker logs` 当作 Envoy/Controller/ABI 错误来源。测试失败时保留数据、组件日志、inspect 和 config dump 到 `/tmp/ingate-e2e-*`，成功时清理。

- [ ] **Step 2: 实现普通代理和资源删除测试**

覆盖：

- 创建 Gateway、Route、Upstream 后 Envoy ADS ACK，HTTP 代理成功；
- 更新 Route path 后新行为生效；
- 先禁用/删除 Route 并等待 ACK，验证 RDS route 移除；再删除已无引用的 Upstream 并验证 CDS/EDS 移除；最后在无 child Route 后删除 Gateway 并验证 LDS/RDS 移除；
- 直接删除仍被启用 Route 引用的 Upstream/Gateway、非法引用、hostname 冲突和 Unsupported HealthCheck 都使当前资源 Accepted=False，并保持旧 Active；不假设父资源删除会隐式级联；
- 用户 Upstream ID 以 `ingate-system-` 开头时 Accepted=False。

- [ ] **Step 3: 实现 Delivery/Last Good 测试**

使用 `xdsclient` sidecar 做可控 SotW 序列：先 `SIGSTOP` 容器内真实 Envoy，sidecar 记录每个 type 的 version/nonce 并可延迟 ACK；v1 已 Active 后发布 v2 Candidate 但不 ACK，再发布 v3 supersede v2；此时发送 v2 nonce 的迟到 ACK/NACK，断言 Candidate 仍为 v3。随后对当前 v3 nonce 发送 NACK，断言 callback 返回前已恢复 v1；首次 Candidate/Baseline 使用独立 case，当前 nonce NACK 后恢复 Baseline、`configReady=false`。暂停期间允许 Docker health 暂时 unhealthy，但容器必须保持 running；停止 sidecar、`SIGCONT` Envoy 后重新等待 healthy，避免两个客户端互相影响。

另做真实 Envoy NACK：在全新容器、该 Wasm VM 从未加载的前提下暂时移走 `ratelimit.wasm`，首次创建需要该内置 filter 的 Policy/Binding，使 Envoy 必须创建新 VM 并 NACK；恢复文件并发布下一版本后成功，不能依赖已经缓存的 VmId。

Last Good 正常恢复测试先建立 v1 Active/LastGood，再提交确定性 compiler-invalid 更新，使当前资源 Accepted=False 且 Active 保持 v1；重启 Controller/容器后必须仍以 v1 代理，证明恢复发生在失败的重新编译之前。

损坏测试同样先保持当前资源 invalid，再用镜像内 `etcdctl` 篡改 `/ingate/internal/last-good/envoy` 后重启；由于当前资源仍 invalid，Degraded/Baseline 状态必须持续可观察，不能被立即成功编译掩盖。修复资源后验证生成新 Active、覆盖损坏记录并恢复代理。

- [ ] **Step 4: 实现三种 Redis 限流与故障语义测试**

覆盖 FixedWindow、SlidingWindow、TokenBucket、burst、quota headers、429 响应、FailOpen 和 FailClose。停止 Redis 后普通无策略 Route 仍可代理；Ingate 容器不退出。

迟到 callback 场景：对 Redis 执行短时 `CLIENT PAUSE`，发起 Global 请求并让客户端提前断开；等待 pause 结束后必须在组件日志/稳定计数中观察到 `late_callback_ignored`，证明 callback 确实在 context 销毁后到达，而不是被取消；同时 Envoy 不崩溃、无 invalid context trap、不会恢复已销毁请求。

- [ ] **Step 5: 实现进程、镜像和系统 cluster 测试**

断言：

- 进程和镜像无 ingate-xds、ingate-dataplane、ingate-httpbin；
- 动态 CDS 不包含 `ingate-system-redis`；
- 静态 bootstrap 包含该 cluster；
- `/var/log/ingate` 组件日志无缺失 `proxy_redis_*` import、缺失 callback export、BadArgument、Wasm trap；
- `/readyz` 不等待 Envoy ACK，整体 healthcheck 不等待业务 Gateway。
- apiserver、ADS、Envoy admin 保持 loopback/internal-only，测试只通过 exec/sidecar 访问。

- [ ] **Step 6: 运行真实 E2E**

Run:

```bash
make e2e ALL_IN_ONE_IMAGE=ingate/all-in-one:e2e
```

Makefile 中 `e2e` 依赖 `all-in-one-image`，并等价执行：

```bash
INGATE_E2E_IMAGE="$(ALL_IN_ONE_IMAGE)" INGATE_E2E_BACKEND_IMAGE="$(E2E_BACKEND_IMAGE)" go test -tags=e2e -timeout=45m -p=1 ./test/e2e -count=1
```

Expected: PASS；测试使用的 all-in-one/backend image ID 都与本次刚构建镜像一致。

- [ ] **Step 7: 提交**

```bash
git add test/e2e Makefile
git commit -m "test(e2e): cover Envoy delivery and Redis governance"
```

---

### Task 20: 更新现行文档、设计图并完成最终验收

**Required skills:** @superpowers:verification-before-completion, @go-code-review

**Files:**

- Modify: `README.md`
- Modify: `docs/2026-05-01-frontend-product-design.md`
- Modify: `docs/2026-06-01-all-in-one-design.md`
- Modify: `docs/2026-07-13-agent-platform-direction.md`
- Modify: `web/console/docs/frontend/data-architecture.md`
- Modify: `docs/images/frontend-create-route-step4-preview-ui.svg`
- Modify: `docs/images/frontend-gateway-config-ui.svg`
- Modify: `docs/images/frontend-gateway-detail-ui.svg`
- Modify: `docs/images/frontend-gateway-list-ui.svg`
- Modify: `docs/images/frontend-plugin-deploy-ui.svg`
- Modify: `docs/images/frontend-plugin-detail-ui.svg`
- Modify: `docs/images/frontend-plugin-list-ui.svg`
- Modify: `docs/images/frontend-product-architecture.svg`
- Modify: `docs/images/frontend-system-settings-ui.svg`
- Modify: `docs/superpowers/specs/2026-07-17-ingate-simplified-architecture-design.md`
- Modify: `docs/superpowers/specs/2026-06-05-gateway-admin-api-design.md`
- Modify: `docs/superpowers/specs/2026-06-07-rate-limit-policy-binding-design.md`
- Modify: `docs/superpowers/specs/2026-06-08-dataplane-capability-design.md`
- Modify: `docs/superpowers/specs/2026-06-08-enterprise-rate-limit-design.md`

- [ ] **Step 1: 更新现行产品和部署文档**

README、前端产品设计、all-in-one 设计和 Agent 方向文档统一描述：单配置域、多逻辑 Gateway、唯一 Envoy 数据平面、合并 Controller、etcd + Redis、无 RuntimeGroup/RuntimeSnapshot/RedisStore/ingate-dataplane。

前端文档保留“用户不直接操作内部运行配置”的原则，改写为“不暴露 Delivery/xDS 内部结构”。

- [ ] **Step 2: 更新设计图和历史文档状态**

当前产品图删除数据面组、独立 xDS/dataplane 和 Redis 配置页面。生效 spec 保持精确 Redis ABI、callback 生命周期和 SDK 边界；四份仍被用户检索的旧 design 顶部增加 superseded banner，指向 `2026-07-17-ingate-simplified-architecture-design.md`；不要重写历史 `docs/superpowers/plans/**`。

- [ ] **Step 3: 运行旧术语和第三方依赖扫描**

Run:

```bash
rg -n 'RuntimeGroup|RuntimeSnapshot|RedisStore|runtimeGroupRef|redisRef|json:"global|ingate-xds|ingate-dataplane|LogicalGateway|Target Translator' \
  --glob '!docs/superpowers/plans/**' \
  --glob '!docs/superpowers/specs/2026-04-28-control-plane-core-mvp-design.md' \
  --glob '!docs/superpowers/specs/2026-06-05-gateway-admin-api-design.md' \
  --glob '!docs/superpowers/specs/2026-06-07-rate-limit-policy-binding-design.md' \
  --glob '!docs/superpowers/specs/2026-06-08-dataplane-capability-design.md' \
  --glob '!docs/superpowers/specs/2026-06-08-enterprise-rate-limit-design.md' \
  --glob '!docs/superpowers/specs/2026-07-14-higress-envoy-standalone-ratelimit-design.md'
rg -n 'github.com/higress-group/' --glob '*.go' --glob 'go.mod' .
```

Expected: 第一条仅允许明确的“已删除/不支持”迁移说明；第二条无输出。

- [ ] **Step 4: 运行完整验证矩阵**

Run:

```bash
make test
make build
make plugins-test
make plugins-build
make console-test
make console-build
make all-in-one-image
make all-in-one-smoke
make e2e
```

Expected: 全部 PASS。

- [ ] **Step 5: 做最终 Go review 和工作树检查**

检查：

- xDS 不反向 import Delivery；
- 只有 Delivery 调用业务 `SetSnapshot`；
- 无用户可配置 Redis 地址；
- 无 IR/Target/RuntimeSnapshot 残留；
- 注释为中文；
- `_output/`、`.gocache/`、`.gomodcache/` 未被提交。

Run:

```bash
git status --short
git diff --check
```

Expected: 仅包含预期文档改动；无 whitespace error。

- [ ] **Step 6: 提交文档**

```bash
git add README.md docs web/console/docs
git commit -m "docs: align project with simplified Envoy architecture"
```

---

## 最终完成定义

只有以下条件全部满足，迁移才算完成：

- 一套 Ingate 编译并发布一份全局 Envoy 配置，多个 Gateway 可共享同一 Envoy fleet。
- 使用标准 go-control-plane SotW Snapshot Cache；Delta 明确未启用。
- RuntimeGroup、RuntimeSnapshot、Target、Translator、Registry、公开 Logical IR、独立 ingate-xds 已删除。
- RedisStore 和 `RateLimitPolicy.spec.global` 已删除；Global mode 自动使用系统 Redis。
- Ingate 生产 Go 代码不 import Higress；Higress 只作为 Envoy 二进制来源。
- Redis ABI PING、三种限流算法、fail-open/fail-close 和迟到 callback E2E 通过。
- ingate-dataplane、HTTP/JSON capability 协议、pkg/xredis 和 go-redis 已删除。
- Candidate NACK 不破坏 Active；首次 NACK 回到 Baseline；ACK 后才写 Last Good。
- Controller 重启可恢复 Last Good，损坏记录不部分恢复。
- Admin API/Console 展示单配置域 Candidate、Active、Last Good、ACK/NACK 和连接状态。
- schema marker 必须为 2；首次安装 bootstrap、旧数据显式 reset，服务不自动迁移。
- all-in-one 包含 Console、admin-api、apiserver、controller、Envoy、etcd、Redis，且不包含独立 xDS/dataplane 或 demo httpbin 进程；amd64/arm64 单平台构建都不会混入错误架构二进制。
- 最终完整验证矩阵全部通过。
