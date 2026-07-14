# Higress Envoy Redis Hostcall Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the all-in-one standard Envoy and `ingate-dataplane` Redis forwarding path with Higress Envoy plus an Ingate-owned low-level Redis hostcall adapter.

**Architecture:** The control-plane model stays unchanged through compiler and logical IR. The xDS target emits referenced standalone Redis clusters and a compact plugin runtime projection; the RateLimit Wasm plugin keeps the standard proxy-wasm Go SDK and calls Higress Redis ABI through `plugins/internal/hostcall/redis`. Rate-limit Lua and RESP handling live in a plugin-owned `redislimit` package, while the old dataplane process and protocol are removed.

**Tech Stack:** Go 1.26, Envoy xDS v3, Higress Envoy 1.36.4 from gateway 2.2.3, standard proxy-wasm-go-sdk, WASI/Go Wasm, RESP, Redis Lua, Docker.

---

## File Structure

### New files

- `plugins/internal/hostcall/redis/client.go`: plugin-context-scoped Redis hostcall client, callback registry, ready state, context cleanup
- `plugins/internal/hostcall/redis/abi_wasm.go`: Higress-only Wasm imports and Redis callback export
- `plugins/internal/hostcall/redis/abi_nonwasm.go`: non-Wasm stubs required for ordinary Go tests
- `plugins/internal/hostcall/redis/client_test.go`: callback scoping, cleanup, late callback and readiness tests
- `plugins/ratelimit/internal/redislimit/algorithm.go`: Lua scripts, RESP command construction and algorithm result parsing
- `plugins/ratelimit/internal/redislimit/algorithm_test.go`: fixed-window, sliding-window, token-bucket and malformed-response tests
- `plugins/ratelimit/internal/redislimit/executor.go`: RedisStore lookup and one-check hostcall execution
- `plugins/ratelimit/internal/redislimit/executor_test.go`: lazy init, dispatch and error propagation tests
- `plugins/ratelimit/internal/runtime/global.go`: sequential per-request global-check state machine
- `hack/e2e-higress-redis-hostcall.sh`: deterministic Docker and Admin API verification with guaranteed cleanup

### Modified files

- `internal/core/target/xds/translator.go`: referenced RedisStore selection, validation, timeout normalization and Redis cluster generation
- `internal/core/target/xds/translator_test.go`: target translation behavior and unsupported configuration tests
- `internal/core/target/xds/ratelimit.go`: final RateLimit target payload without dataplane fields
- `internal/xds/server/cluster_builder.go`: per-cluster connect timeout
- `internal/xds/server/cluster_builder_test.go`: Redis cluster timeout and endpoint assertions
- `internal/xds/server/ratelimit_builder.go`: emit compact Redis runtime config without dataplane
- `internal/xds/server/listener_builder_test.go`: assert plugin JSON contains cluster projection and no dataplane
- `pkg/plugin/ratelimit/types.go`: Redis runtime projection, removal of `DataPlane`
- `pkg/plugin/ratelimit/types_test.go`: schema parsing and no-dataplane assertions
- `plugins/ratelimit/internal/policy/policy.go`: global result type owned by the plugin domain
- `plugins/ratelimit/internal/policy/global.go`: one-check success/error decision helper
- `plugins/ratelimit/internal/policy/policy_test.go`: FailOpen, FailClose and quota-header behavior without dataplane protocol types
- `plugins/ratelimit/internal/policy/runner.go`: remove dataplane-oriented timeout field from `GlobalCheck`
- `plugins/ratelimit/internal/runtime/runtime.go`: compile Redis executor and expose sequential global execution
- `plugins/ratelimit/internal/runtime/runtime_test.go`: sequential execution, early stop and final action tests
- `plugins/ratelimit/internal/wasm/plugin.go`: store plugin/http context IDs and own the hostcall client lifecycle
- `plugins/ratelimit/internal/wasm/http.go`: pass context ID into global execution and clean up old dataplane callbacks
- `deploy/all-in-one/Dockerfile`: use Higress Envoy and stop copying `ingate-dataplane`
- `deploy/all-in-one/entrypoint.sh`: remove dataplane startup and wait logic
- `deploy/all-in-one/default.env`: remove dataplane address
- `deploy/all-in-one/envoy/bootstrap.yaml`: use Envoy 1.36 typed HTTP/2 protocol options
- `Makefile`: keep plugin/all-in-one build targets valid after command deletion
- `go.mod`, `go.sum`: add RESP parser and remove go-redis after old dataplane deletion
- `README.md`: describe Higress Envoy and direct Redis hostcall runtime
- `plugins/ratelimit/README.md`: replace dataplane architecture with hostcall architecture

### Deleted files

- `cmd/ingate-dataplane/main.go`
- `internal/dataplane/**`
- `pkg/dataplane/**`
- `pkg/xredis/**`
- `plugins/ratelimit/internal/dataplane/**`

Historical specs and plans remain unchanged as decision records.

---

### Task 1: Translate Referenced Redis Stores Into xDS Clusters

**Files:**
- Modify: `pkg/plugin/ratelimit/types.go`
- Modify: `pkg/plugin/ratelimit/types_test.go`
- Modify: `internal/core/target/xds/ratelimit.go`
- Modify: `internal/core/target/xds/translator.go`
- Modify: `internal/core/target/xds/translator_test.go`

- [ ] **Step 1: Add failing translator tests for a referenced standalone RedisStore**

Add a helper that builds a logical Gateway with a Global RateLimitPolicy, a PolicyBinding and one RedisStore. Assert that translation produces:

```go
wantStore := pluginratelimit.RedisStore{
	Name:          "redis-main",
	ClusterName:   "ingate-redis-redis-main",
	DB:            0,
	TimeoutMillis: 50,
}

wantCluster := xds.Cluster{
	Name:                 "ingate-redis-redis-main",
	DiscoveryType:        xds.ClusterDiscoveryTypeLogicalDNS,
	Address:              "redis.internal",
	Port:                 6379,
	ConnectTimeoutMillis: 1000,
}
```

Also assert that no `ingate-dataplane` cluster is emitted.

- [ ] **Step 2: Add failing table tests for validation and reference filtering**

Cover these cases in `translator_test.go`:

```text
referenced Cluster mode -> error
referenced Sentinel mode -> error
referenced TLS store -> error
referenced username -> error
referenced passwordRef -> error
referenced non-empty addresses -> error
referenced non-empty sentinelMaster -> error
referenced malformed address -> error
referenced port 0 or greater than 65535 -> error
referenced negative connect timeout -> error
referenced negative command timeout -> error
referenced negative policy timeout -> error
missing referenced store -> error
unreferenced unsupported store -> translation succeeds and store is omitted
two policies with different effective timeout on one store -> error
```

Error assertions should check the RedisStore ID and the concrete reason, not the entire string.

- [ ] **Step 3: Run the focused tests and verify failure**

Run:

```bash
go test ./internal/core/target/xds ./pkg/plugin/ratelimit
```

Expected: FAIL because `RedisStore.ClusterName`, `TimeoutMillis`, `Cluster.ConnectTimeoutMillis` and translator validation do not exist.

- [ ] **Step 4: Extend the plugin runtime RedisStore projection**

Add the final execution fields while temporarily retaining old fields needed by the existing dataplane package until Task 5:

```go
type RedisStore struct {
	Name          string `json:"name"`
	ClusterName   string `json:"clusterName,omitempty"`
	DB            int    `json:"db,omitempty"`
	TimeoutMillis int    `json:"timeoutMillis,omitempty"`

	// Transitional fields removed in Task 5.
	DisplayName          string   `json:"displayName,omitempty"`
	Mode                 string   `json:"mode,omitempty"`
	Address              string   `json:"address,omitempty"`
	Addresses            []string `json:"addresses,omitempty"`
	TLS                  bool     `json:"tls,omitempty"`
	TLSServerName        string   `json:"tlsServerName,omitempty"`
	Username             string   `json:"username,omitempty"`
	PasswordRef          string   `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int      `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int      `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int      `json:"poolSize,omitempty"`
	MinIdleConns         int      `json:"minIdleConns,omitempty"`
	SentinelMaster       string   `json:"sentinelMaster,omitempty"`
}
```

Do not expose Higress ABI names in this type.

- [ ] **Step 5: Add connect timeout to the target Cluster model**

Extend `internal/core/target/xds.Cluster`:

```go
ConnectTimeoutMillis int `json:"connectTimeoutMillis,omitempty"`
```

- [ ] **Step 6: Refactor RateLimit translation to return clusters and errors**

Change the helper boundary to:

```go
func (t Translator) translateRateLimitConfig(logical ir.LogicalGateway) (*RateLimitConfig, []Cluster, error)
```

Implementation order:

1. Build `policiesByName` and `storesByName`
2. Walk PolicyBindings in stable order
3. Include only RateLimit policies referenced by a binding
4. For Global policies, resolve the referenced RedisStore
5. Validate only stores reached through those bindings
6. Parse `address` with `net.SplitHostPort` and `strconv.Atoi`, then require port `1..65535`
7. Calculate effective command timeout: policy, store, default 50
8. Ensure all policies for one store calculate the same timeout
9. Append each runtime RedisStore and Envoy cluster only once
10. Stop populating `RateLimitConfig.DataPlane` and remove the old `ingate-dataplane` cluster append from `Translate`; keep only the transitional Go type until Task 5 so existing plugin packages still compile

Use target-local constants:

```go
const (
	redisClusterNamePrefix             = "ingate-redis-"
	defaultRedisConnectTimeoutMillis   = 1000
	defaultRedisCommandTimeoutMillis   = 50
)
```

Keep the main `Translate` flow direct:

```go
rateLimit, redisClusters, err := t.translateRateLimitConfig(logical)
if err != nil {
	return runtime.RuntimeSnapshot{}, err
}
config.RateLimit = rateLimit
config.Clusters = append(config.Clusters, redisClusters...)
```

- [ ] **Step 7: Run focused and full tests**

Run:

```bash
go test ./internal/core/target/xds ./pkg/plugin/ratelimit
go test ./...
```

Expected: PASS. The real global path is not switched yet, but the repository must compile.

- [ ] **Step 8: Commit the target translation change**

```bash
git add pkg/plugin/ratelimit/types.go pkg/plugin/ratelimit/types_test.go internal/core/target/xds/ratelimit.go internal/core/target/xds/translator.go internal/core/target/xds/translator_test.go
git commit -m "feat(xds): translate Redis stores for hostcall runtime"
```

---

### Task 2: Build Redis Clusters With Explicit Timeouts and Modern Bootstrap HTTP/2

**Files:**
- Modify: `internal/xds/server/cluster_builder.go`
- Modify: `internal/xds/server/cluster_builder_test.go`
- Modify: `deploy/all-in-one/envoy/bootstrap.yaml`

- [ ] **Step 1: Add a failing cluster-builder test**

Create a logical DNS cluster with `ConnectTimeoutMillis: 1000` and assert:

```go
if got := cluster.GetConnectTimeout().AsDuration(); got != time.Second {
	t.Fatalf("ConnectTimeout = %s, want 1s", got)
}
```

Keep the existing TLS test and add a separate test for Redis so TLS behavior remains isolated.

- [ ] **Step 2: Run the focused test and verify failure**

Run:

```bash
go test ./internal/xds/server -run 'TestResponseBuilderBuildClusters'
```

Expected: FAIL because every cluster still receives the fixed 5-second timeout.

- [ ] **Step 3: Implement per-cluster connect timeout**

Add a named default and keep ordinary Upstream behavior unchanged:

```go
const defaultClusterConnectTimeout = 5 * time.Second

connectTimeout := defaultClusterConnectTimeout
if cluster.ConnectTimeoutMillis > 0 {
	connectTimeout = time.Duration(cluster.ConnectTimeoutMillis) * time.Millisecond
}
```

Use `connectTimeout` in the Envoy Cluster protobuf.

- [ ] **Step 4: Replace deprecated bootstrap HTTP/2 configuration**

Replace:

```yaml
http2_protocol_options: {}
```

with:

```yaml
typed_extension_protocol_options:
  envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
    "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
    explicit_http_config:
      http2_protocol_options: {}
```

- [ ] **Step 5: Verify unit tests and bootstrap validation with Higress Envoy**

Run:

```bash
go test ./internal/xds/server
docker run --rm \
  -v "$PWD/deploy/all-in-one/envoy/bootstrap.yaml:/etc/ingate/envoy/bootstrap.yaml:ro" \
  --entrypoint /usr/local/bin/envoy \
  higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/gateway:2.2.3 \
  --mode validate -c /etc/ingate/envoy/bootstrap.yaml
```

Expected: tests PASS and Envoy prints `configuration ... OK` without the deprecated `http2_protocol_options` warning.

- [ ] **Step 6: Commit cluster and bootstrap compatibility**

```bash
git add internal/xds/server/cluster_builder.go internal/xds/server/cluster_builder_test.go deploy/all-in-one/envoy/bootstrap.yaml
git commit -m "feat(xds): configure Redis cluster timeouts"
```

---

### Task 3: Add the Ingate-Owned Higress Redis ABI Adapter

**Files:**
- Create: `plugins/internal/hostcall/redis/client.go`
- Create: `plugins/internal/hostcall/redis/abi_wasm.go`
- Create: `plugins/internal/hostcall/redis/abi_nonwasm.go`
- Create: `plugins/internal/hostcall/redis/client_test.go`

- [ ] **Step 1: Write failing callback-registry tests**

Cover these behaviors without invoking a real host:

```text
same callout ID under two plugin contexts does not collide
callback completion removes only the matching entry
ForgetContext removes all callbacks owned by one HTTP context
Close removes the plugin-context client and readiness state
unknown or late callback is ignored
readiness is tracked per cluster name
```

The tests should construct Clients with different `pluginContextID` values and exercise unexported registry methods from the same package.

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./plugins/internal/hostcall/redis
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the plugin-context-scoped Client**

Use a concrete type and one small global routing map required by the exported callback:

```go
type Callback func(response []byte, err error)

type callbackEntry struct {
	contextID uint32
	callback  Callback
}

type Client struct {
	pluginContextID uint32
	callbacks       map[uint32]callbackEntry
	ready           map[string]clientConfig
}

type clientConfig struct {
	initCluster   string
	database      int
	timeoutMillis int
}

var clients = map[uint32]*Client{}
```

Required methods:

```go
func NewClient(pluginContextID uint32) *Client
func (c *Client) Init(cluster string, database, timeoutMillis int) error
func (c *Client) Call(contextID uint32, cluster string, query []byte, callback Callback) error
func (c *Client) ForgetContext(contextID uint32)
func (c *Client) Close()
```

`Init` appends `?db=<n>` only when DB is non-zero, invokes the host with that init key, and stores the successful configuration under the base Envoy cluster name. `Call` receives the base cluster name, requires a matching ready entry, and dispatches to that same base cluster; Higress keeps the initialized Redis client on the underlying ThreadLocalCluster. Reinitializing one base cluster with a different DB or timeout is an explicit configuration error. Failed initialization is not cached.

- [ ] **Step 4: Implement status conversion and callback handling**

Map known Proxy-Wasm statuses to the standard SDK errors and preserve unknown numeric values:

```go
func statusError(status uint32) error {
	switch status {
	case 0:
		return nil
	case 1:
		return types.ErrorStatusNotFound
	case 2:
		return types.ErrorStatusBadArgument
	case 10:
		return types.ErrorInternalFailure
	case 12:
		return types.ErrorUnimplemented
	default:
		return fmt.Errorf("higress redis hostcall status %d", status)
	}
}
```

The callback path must:

1. Resolve Client by `pluginContextID`
2. Resolve and delete callback by `calloutID`
3. Ignore unknown entries
4. Call `proxywasm.SetEffectiveContext(entry.contextID)`
5. Drop the callback if the HTTP context no longer exists
6. Treat callback status `!= 0` as a network error
7. Read buffer type `9` only for successful callbacks

- [ ] **Step 5: Add the Wasm ABI declarations**

`abi_wasm.go`:

```go
//go:build wasip1 && wasm

package redis

//go:wasmimport env proxy_redis_init
func proxyRedisInit(cluster *byte, clusterSize int32, username *byte, usernameSize int32, password *byte, passwordSize int32, timeout uint32) uint32

//go:wasmimport env proxy_redis_call
func proxyRedisCall(cluster *byte, clusterSize int32, query *byte, querySize int32, calloutID *uint32) uint32

//go:wasmimport env proxy_get_buffer_bytes
func proxyGetBufferBytes(bufferType uint32, start int32, maxSize int32, data unsafe.Pointer, size *int32) uint32

//go:wasmexport proxy_on_redis_call_response
func proxyOnRedisCallResponse(pluginContextID, calloutID uint32, status, responseSize int32) {
	handleRedisCallResponse(pluginContextID, calloutID, status, responseSize)
}
```

Keep pointer conversion and buffer copying in this build-tagged file. Do not import Higress Go modules.

- [ ] **Step 6: Add non-Wasm stubs**

`abi_nonwasm.go` must define raw functions with signatures identical to the Wasm declarations and return numeric status code `12` (`Unimplemented`). Shared wrappers in `client.go` convert that status to `types.ErrorUnimplemented`. Tests must not pretend that these stubs are a real Envoy host.

- [ ] **Step 7: Run host tests and a Wasm compile check**

Run:

```bash
go test ./plugins/internal/hostcall/redis
GOOS=wasip1 GOARCH=wasm go test -c \
  -o /tmp/ingate-redis-hostcall.test.wasm \
  ./plugins/internal/hostcall/redis
```

Expected: host tests PASS and the Wasm test binary compiles, proving the custom imports can coexist with the standard proxy-wasm SDK imports.

- [ ] **Step 8: Commit the ABI adapter**

```bash
git add plugins/internal/hostcall/redis
git commit -m "feat(plugin): add Higress Redis hostcall adapter"
```

---

### Task 4: Move Redis Lua and RESP Handling Into the RateLimit Plugin

**Files:**
- Create: `plugins/ratelimit/internal/redislimit/algorithm.go`
- Create: `plugins/ratelimit/internal/redislimit/algorithm_test.go`
- Create: `plugins/ratelimit/internal/redislimit/executor.go`
- Create: `plugins/ratelimit/internal/redislimit/executor_test.go`
- Modify: `plugins/ratelimit/internal/policy/policy.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the RESP dependency**

Run:

```bash
go get github.com/tidwall/resp@v0.1.1
```

Expected: `go.mod` adds `github.com/tidwall/resp v0.1.1` as a direct dependency.

- [ ] **Step 2: Write failing algorithm tests**

Test command construction and response parsing for:

```text
FixedWindow response: [current, ttlMillis]
SlidingWindow response: [allowed, current, ttlMillis]
TokenBucket response: [allowed, current, capacity, tokens, resetMillis]
Redis error value
non-array value
too few array items
non-integer item
```

Define the result in the policy package before writing redislimit tests:

```go
type GlobalResult struct {
	Allowed      bool
	Current      int
	Limit        int
	ResetSeconds int
}

want := policy.GlobalResult{
	Allowed:      true,
	Current:      3,
	Limit:        10,
	ResetSeconds: 30,
}
```

- [ ] **Step 3: Run algorithm tests and verify failure**

Run:

```bash
go test ./plugins/ratelimit/internal/redislimit
```

Expected: FAIL because the package is not implemented.

- [ ] **Step 4: Move the three Lua scripts and implement RESP EVAL encoding**

Move the scripts without changing their algorithm semantics from:

```text
internal/dataplane/service/ratelimit/algorithm.go
```

Build a RESP array equivalent to:

```text
EVAL <script> 1 <redis-key> <algorithm arguments...>
```

Use `resp.NewWriter`; do not construct RESP by string concatenation.

- [ ] **Step 5: Implement plugin-domain algorithm result parsing**

`redislimit` imports `policy` and returns `policy.GlobalResult`. The dependency direction is fixed for the entire implementation:

```text
policy <- redislimit <- runtime
```

The policy package must never import `redislimit`; runtime is the only package that coordinates both. Keep reset rounding behavior identical to the old dataplane implementation and include burst in token-bucket capacity.

- [ ] **Step 6: Write failing Executor tests**

Define the external boundary in the consumer package:

```go
type Caller interface {
	Init(cluster string, database, timeoutMillis int) error
	Call(contextID uint32, cluster string, query []byte, callback hostredis.Callback) error
}
```

Use a small fake Caller in tests to assert:

```text
store ID resolves to cluster name, DB and timeout
Init runs before Call
missing store returns a clear error
Init error is returned synchronously
dispatch error is returned synchronously
callback response is parsed into Result
callback network/RESP error reaches the caller
```

- [ ] **Step 7: Implement one-check Executor**

`Executor` should pre-index runtime stores by ID and expose:

```go
func NewExecutor(stores []config.RedisStore, caller Caller) (*Executor, error)
func (e *Executor) Check(contextID uint32, check policy.GlobalCheck, callback func(policy.GlobalResult, error)) error
```

Do not put FailOpen/FailClose policy decisions in this package. It executes one check and returns a domain result or error.

- [ ] **Step 8: Run tests and plugin compilation**

Run:

```bash
go test ./plugins/ratelimit/internal/redislimit
make ratelimit-plugin-build
```

Expected: tests PASS and the plugin still builds before runtime wiring.

- [ ] **Step 9: Commit Redis algorithm execution**

```bash
git add go.mod go.sum plugins/ratelimit/internal/policy/policy.go plugins/ratelimit/internal/redislimit
git commit -m "feat(ratelimit): execute Redis limit algorithms in Wasm"
```

---

### Task 5: Wire Sequential Global Checks Into the RateLimit Runtime

**Files:**
- Modify: `pkg/plugin/ratelimit/types.go`
- Modify: `pkg/plugin/ratelimit/types_test.go`
- Modify: `internal/core/target/xds/ratelimit.go`
- Modify: `internal/xds/server/ratelimit_builder.go`
- Modify: `internal/xds/server/listener_builder_test.go`
- Modify: `plugins/ratelimit/internal/policy/policy.go`
- Modify: `plugins/ratelimit/internal/policy/global.go`
- Modify: `plugins/ratelimit/internal/policy/policy_test.go`
- Modify: `plugins/ratelimit/internal/policy/runner.go`
- Modify: `plugins/ratelimit/internal/runtime/runtime.go`
- Create: `plugins/ratelimit/internal/runtime/global.go`
- Modify: `plugins/ratelimit/internal/runtime/runtime_test.go`
- Modify: `plugins/ratelimit/internal/wasm/plugin.go`
- Modify: `plugins/ratelimit/internal/wasm/http.go`
- Delete: `plugins/ratelimit/internal/dataplane/client.go`
- Delete: `plugins/ratelimit/internal/dataplane/http_transport.go`
- Delete: `plugins/ratelimit/internal/dataplane/request.go`
- Delete: `plugins/ratelimit/internal/dataplane/request_test.go`

- [ ] **Step 1: Rewrite policy global-result tests to remove dataplane protocol types**

Replace batch `CheckResponse` tests with one-check domain tests:

```go
func TestApplyGlobalResultRejectsRedisLimit(t *testing.T)
func TestApplyGlobalResultAllowsFailOpenError(t *testing.T)
func TestApplyGlobalResultRejectsFailCloseError(t *testing.T)
func TestApplyGlobalResultReturnsQuotaHeaders(t *testing.T)
```

The helper should accept one `GlobalCheck`, one plugin-domain result and an error.

- [ ] **Step 2: Add failing sequential runtime tests**

Use a fake `redislimit.Caller` through a real Executor. Cover:

```text
checks execute in stable order
second check starts only after first callback
first rejected result stops later checks
FailOpen error continues to next check
FailClose error stops and returns 429
all successful checks return Continue and final quota headers
synchronous Init/dispatch errors use the same failure policy path
all-synchronous FailOpen completion returns Continue without invoking the async completion callback
synchronous FailClose completion returns Respond without invoking ResumeHttpRequest
an actual Redis dispatch returns pending=true and completes through the async callback
Compile rejects a Global Policy whose RedisRef is missing from RedisStores
Compile rejects duplicate store IDs, empty cluster names and non-positive effective timeouts
```

- [ ] **Step 3: Run focused tests and verify failure**

Run:

```bash
go test ./plugins/ratelimit/internal/policy ./plugins/ratelimit/internal/runtime
```

Expected: FAIL because runtime still requires dataplane response types and HTTP transport.

- [ ] **Step 4: Replace batch dataplane decisions with one-check policy decisions**

Use the `policy.GlobalResult` type added in Task 4. `policy/global.go` must not import `redislimit`, preserving the dependency direction `policy <- redislimit <- runtime`.

Use a direct helper boundary:

```go
func ApplyGlobalResult(check GlobalCheck, result GlobalResult, err error) (Decision, bool)
```

Rules:

```text
error + FailOpen -> allowed
error + FailClose -> rejected
Allowed false -> rejected
Allowed true -> quota headers
```

Remove `RedisTimeoutMs` from `GlobalCheck`; timeout now comes from the runtime RedisStore projection.

- [ ] **Step 5: Implement the runtime globalExecution state object**

Keep the asynchronous state machine out of the Wasm entrypoint:

```go
type globalExecution struct {
	runtime      *Runtime
	contextID    uint32
	checks       []policy.GlobalCheck
	index        int
	quotaHeaders map[string]string
	complete     func(Result)
	returned     bool
	result       *Result
}

func (e *globalExecution) next()
func (e *globalExecution) handle(result policy.GlobalResult, err error)
func (e *globalExecution) finish(result Result)
```

`next` dispatches one check. `handle` applies policy semantics, stops on reject, continues on FailOpen error, and completes exactly once. `finish` stores a result when execution completes before the caller has returned; it invokes the async completion callback only after the execution has entered pending state.

Expose:

```go
func (r *Runtime) StartGlobalChecks(contextID uint32, checks []policy.GlobalCheck, complete func(Result)) (result Result, pending bool)
```

`StartGlobalChecks` calls `next`, marks `returned=true`, then returns the stored synchronous result with `pending=false` or reports `pending=true` after a real Redis dispatch. Synchronous initialization or dispatch failure must be passed through `handle`; do not duplicate failure-policy branches in the Wasm layer and do not call the async completion callback for synchronous completion.

- [ ] **Step 6: Compile Runtime with a required Redis caller**

Change the compile boundary to return validation errors:

```go
func Compile(cfg config.PluginConfig, runner *policy.Runner, caller redislimit.Caller) (*Runtime, error)
```

Before building route indexes or the Executor, `Compile` must:

1. Reject duplicate RedisStore IDs
2. Reject empty store IDs or cluster names
3. Reject effective timeout values less than or equal to zero
4. Walk every route, binding and Global Policy
5. Reject a Global Policy with nil Global config, empty RedisRef or a RedisRef absent from the projected stores

This is the plugin's cross-process configuration boundary and must fail `OnPluginStart` even though the xDS translator performs the same logical validation. The Executor keeps its request-time missing-store guard as defense against internal misuse, but generated invalid configuration must not reach request handling.

The plugin entrypoint always provides a concrete hostcall Client. Tests provide a fake Caller. Do not silently substitute a no-op caller. `OnPluginStart` logs the Compile error and returns `types.OnPluginStartStatusFailed`.

- [ ] **Step 7: Wire plugin and HTTP context lifecycle**

Store IDs explicitly:

```go
type pluginContext struct {
	types.DefaultPluginContext
	contextID uint32
	redis     *hostredis.Client
	runtime   *ratelimitruntime.Runtime
}

type httpContext struct {
	types.DefaultHttpContext
	contextID uint32
	plugin    *pluginContext
}
```

Required lifecycle behavior:

```go
func (p *pluginContext) OnPluginDone() bool {
	p.redis.Close()
	return true
}

func (h *httpContext) OnHttpStreamDone() {
	h.plugin.redis.ForgetContext(h.contextID)
}
```

`OnHttpRequestHeaders` uses the two-state result explicitly:

```go
result, pending := h.plugin.runtime.StartGlobalChecks(h.contextID, checks, h.handleAsyncGlobalResult)
if !pending {
	return h.applyRuntimeResult(result)
}
return types.ActionPause
```

Only `handleAsyncGlobalResult` may call `ResumeHttpRequest`; it is never invoked for synchronous completion. This prevents Resume-before-Pause and prevents a synchronous local response followed by an unnecessary Pause.

- [ ] **Step 8: Remove transitional plugin schema and dataplane transport**

Delete `PluginConfig.DataPlane`, the `DataPlane` type and old RedisStore connection fields. The final plugin projection is:

```go
type RedisStore struct {
	Name          string `json:"name"`
	ClusterName   string `json:"clusterName"`
	DB            int    `json:"db,omitempty"`
	TimeoutMillis int    `json:"timeoutMillis"`
}
```

Delete `plugins/ratelimit/internal/dataplane` and update xDS builder tests to assert:

```go
if bytes.Contains([]byte(pluginJSON.Value), []byte(`"dataPlane"`)) {
	t.Fatal("plugin config still contains dataPlane")
}
```

- [ ] **Step 9: Run focused, plugin and full tests**

Run:

```bash
go test ./plugins/ratelimit/...
go test ./internal/core/target/xds ./internal/xds/server ./pkg/plugin/ratelimit
make plugins-build
go test ./...
```

Expected: all PASS. `rg -n 'internal/dataplane|pkg/dataplane|DataPlane' plugins/ratelimit pkg/plugin/ratelimit internal/core/target/xds internal/xds/server` returns no production references.

- [ ] **Step 10: Commit the plugin runtime migration**

```bash
git add pkg/plugin/ratelimit internal/core/target/xds/ratelimit.go internal/xds/server/ratelimit_builder.go internal/xds/server/listener_builder_test.go plugins/ratelimit
git commit -m "feat(ratelimit): use Redis hostcall execution"
```

---

### Task 6: Delete the Ingate Dataplane Process and Redis Service Packages

**Files:**
- Delete: `cmd/ingate-dataplane/main.go`
- Delete: `internal/dataplane/**`
- Delete: `pkg/dataplane/**`
- Delete: `pkg/xredis/**`
- Modify: `deploy/all-in-one/Dockerfile`
- Modify: `deploy/all-in-one/entrypoint.sh`
- Modify: `deploy/all-in-one/default.env`
- Modify: `Makefile`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `README.md`
- Modify: `plugins/ratelimit/README.md`

- [ ] **Step 1: Record the production reference baseline**

Run:

```bash
rg -n 'ingate-dataplane|internal/dataplane|pkg/dataplane|pkg/xredis|INGATE_DATAPLANE_ADDR|go-redis' \
  --glob '!docs/superpowers/specs/**' \
  --glob '!docs/superpowers/plans/**' \
  --glob '!_output/**'
```

Expected: references are limited to the old command/service packages, all-in-one files, dependency files, README and plugin README.

- [ ] **Step 2: Delete the old Go packages**

Delete the command, service, protocol and xredis packages listed above. Do not leave empty package directories or compatibility aliases.

- [ ] **Step 3: Remove the all-in-one dataplane process**

Delete from Dockerfile:

```dockerfile
COPY _output/all-in-one/bin/ingate-dataplane /usr/local/bin/ingate-dataplane
```

Delete from entrypoint:

```text
DATAPLANE_ADDR
start_bg ingate-dataplane ...
DATAPLANE_HOST
DATAPLANE_PORT
wait_tcp ingate-dataplane ...
```

Delete `INGATE_DATAPLANE_ADDR` from `default.env`.

- [ ] **Step 4: Remove the Go dependency and tidy modules**

Run:

```bash
go mod tidy
```

Expected: `github.com/redis/go-redis/v9` is removed, while `github.com/tidwall/resp` remains direct.

- [ ] **Step 5: Update runtime documentation**

Update README component lists and RateLimit README architecture to:

```text
RateLimit Wasm -> Ingate Redis hostcall adapter -> Higress Envoy -> Redis
```

State the initial limitation: standalone, no TLS, no auth.

- [ ] **Step 6: Run reference, test and build checks**

Run:

```bash
rg -n 'ingate-dataplane|internal/dataplane|pkg/dataplane|pkg/xredis|INGATE_DATAPLANE_ADDR|go-redis' \
  --glob '!docs/superpowers/specs/**' \
  --glob '!docs/superpowers/plans/**' \
  --glob '!_output/**'
make test
make build
make plugins-build
```

Expected: `rg` returns no production references; all builds PASS; `_output/bin` no longer contains `ingate-dataplane` after rebuilding from a clean `_output/bin` directory or by checking the command list before build.

- [ ] **Step 7: Commit dataplane deletion**

```bash
git add -A cmd/ingate-dataplane internal/dataplane pkg/dataplane pkg/xredis deploy/all-in-one Makefile go.mod go.sum README.md plugins/ratelimit/README.md
git commit -m "refactor: remove ingate dataplane process"
```

---

### Task 7: Package Higress Envoy in the All-in-One Image

**Files:**
- Modify: `deploy/all-in-one/Dockerfile`

- [ ] **Step 1: Change the Envoy source image with a pinned version argument**

Declare the global build argument before the Dockerfile's first `FROM`, then consume it in the Envoy stage:

```dockerfile
ARG HIGRESS_GATEWAY_IMAGE=higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/gateway:2.2.3

FROM quay.io/coreos/etcd:v3.5.11 AS etcd
FROM ${HIGRESS_GATEWAY_IMAGE} AS envoy
```

Continue copying only:

```dockerfile
COPY --from=envoy /usr/local/bin/envoy /usr/local/bin/envoy
```

Do not copy `pilot-agent`, startup scripts or Higress configuration.

- [ ] **Step 2: Build the complete image**

Run:

```bash
make all-in-one-image
```

Expected: image `ingate/all-in-one:dev` builds successfully with the Higress Envoy binary and without `ingate-dataplane`.

- [ ] **Step 3: Verify the packaged runtime**

Run:

```bash
docker run --rm --entrypoint envoy ingate/all-in-one:dev --version
docker run --rm --entrypoint /bin/bash ingate/all-in-one:dev -lc \
  'test ! -e /usr/local/bin/ingate-dataplane && envoy --mode validate -c /etc/ingate/envoy/bootstrap.yaml'
```

Expected:

```text
Envoy version contains 1.36.4
ingate-dataplane test succeeds because the file is absent
bootstrap validation prints OK
```

- [ ] **Step 4: Commit the runtime image replacement**

```bash
git add deploy/all-in-one/Dockerfile
git commit -m "build: package Higress Envoy runtime"
```

---

### Task 8: Automate and Run the Real Standalone Redis Hostcall Flow

**Files:**
- Create: `hack/e2e-higress-redis-hostcall.sh`
- Update earlier task files only if verification exposes a defect

- [ ] **Step 1: Create a deterministic E2E script with guaranteed cleanup**

Start the script with:

```bash
#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-ingate/all-in-one:dev}"
NETWORK="ingate-hostcall-e2e"
REDIS_CONTAINER="ingate-hostcall-redis"
INGATE_CONTAINER="ingate-hostcall"
API="http://127.0.0.1:8001/api/v1"

cleanup() {
	docker rm -f "$INGATE_CONTAINER" "$REDIS_CONTAINER" >/dev/null 2>&1 || true
	docker network rm "$NETWORK" >/dev/null 2>&1 || true
}

cleanup
trap cleanup EXIT
```

This pre-cleans stale resources and guarantees cleanup on success or failure.

- [ ] **Step 2: Start Redis and Ingate, then wait for health**

The script must run:

```bash
docker network create "$NETWORK"
docker run -d --name "$REDIS_CONTAINER" --network "$NETWORK" redis:7.4-alpine
docker run -d --name "$INGATE_CONTAINER" --network "$NETWORK" \
  -p 8001:8001 -p 8080:8080 -p 8443:8443 "$IMAGE"

for _ in $(seq 1 60); do
	if curl -fsS http://127.0.0.1:8001/healthz >/dev/null; then
		break
	fi
	sleep 1
done
curl -fsS http://127.0.0.1:8001/healthz >/dev/null
```

- [ ] **Step 3: Add an Admin API helper and create Gateway, Upstream and Route**

Use a helper that fails unless the API returns an ID:

```bash
create_resource() {
	local path="$1"
	local payload="$2"
	curl -fsS -X POST "$API/$path" \
		-H 'content-type: application/json' \
		-d "$payload" | jq -er '.data.id'
}
```

Create the resources with exact product DTO payloads:

```bash
GATEWAY_ID=$(create_resource gateways '{
  "name":"hostcall-e2e",
  "description":"",
  "runtimeGroup":"default",
  "listeners":[{"name":"http","protocol":"HTTP","port":8080}],
  "hostBindings":[]
}')

UPSTREAM_ID=$(create_resource upstreams '{
  "name":"hostcall-httpbin",
  "type":"application",
  "endpoints":[{"id":"httpbin","address":"127.0.0.1","port":19090,"weight":100,"enabled":true}],
  "loadBalancePolicy":"round_robin"
}')

ROUTE_PAYLOAD=$(jq -cn --arg gateway "$GATEWAY_ID" --arg upstream "$UPSTREAM_ID" '{
  name:"hostcall-routes",
  gatewayIDs:[$gateway],
  hostnames:[],
  enabled:true,
  rules:[
    {name:"fail-close",pathPrefix:"/hostcall/fail-close",methods:["GET"],targets:[{upstreamID:$upstream,weight:100}]},
    {name:"fail-open",pathPrefix:"/hostcall/fail-open",methods:["GET"],targets:[{upstreamID:$upstream,weight:100}]}
  ]
}')
ROUTE_ID=$(create_resource routes "$ROUTE_PAYLOAD")
```

- [ ] **Step 4: Create RedisStore, two policies and rule-scoped bindings**

Create the supported RedisStore:

```bash
REDIS_ID=$(create_resource redis-stores '{
  "name":"hostcall-redis",
  "mode":"Standalone",
  "address":"ingate-hostcall-redis:6379",
  "tls":false,
  "connectTimeoutMillis":1000,
  "commandTimeoutMillis":50
}')
```

Add a `create_policy` helper that receives display name and failure policy, using this exact body shape:

```bash
create_policy() {
	local name="$1" failure="$2"
	local payload
	payload=$(jq -cn --arg name "$name" --arg redis "$REDIS_ID" --arg failure "$failure" '{
      name:$name,
      enabled:true,
      mode:"Global",
      rules:[{
        name:"fixed-window",
        key:{parts:[{type:"RouteRule"}]},
        limit:{requests:2,windowSeconds:60},
        algorithm:"FixedWindow"
      }],
      global:{redisRef:$redis,prefix:"ingate-hostcall-e2e",timeoutMillis:50},
      response:{statusCode:429,message:"Too many requests",quotaHeaderEnabled:true},
      failurePolicy:$failure
    }')
	create_resource rate-limit-policies "$payload"
}
```

Create `FailClose` and `FailOpen` policies, then create bindings with exact rule targets:

```bash
FAIL_CLOSE_POLICY_ID=$(create_policy hostcall-fail-close FailClose)
FAIL_OPEN_POLICY_ID=$(create_policy hostcall-fail-open FailOpen)

create_binding() {
	local name="$1" rule="$2" policy="$3"
	local payload
	payload=$(jq -cn --arg name "$name" --arg route "$ROUTE_ID" --arg rule "$rule" --arg policy "$policy" '{
      name:$name,
      enabled:true,
      targetRef:{kind:"Route",name:$route,ruleName:$rule},
      policies:[{kind:"RateLimitPolicy",name:$policy}]
    }')
	create_resource policy-bindings "$payload" >/dev/null
}

create_binding bind-hostcall-fail-close fail-close "$FAIL_CLOSE_POLICY_ID"
create_binding bind-hostcall-fail-open fail-open "$FAIL_OPEN_POLICY_ID"
sleep 3
```

- [ ] **Step 5: Add HTTP status assertions for quota enforcement**

Add:

```bash
request_status() {
	curl --max-time 5 -sS \
		-D /tmp/ingate-hostcall-headers \
		-o /tmp/ingate-hostcall-response \
		-w '%{http_code}' "$1"
}

test "$(request_status http://127.0.0.1:8080/hostcall/fail-close)" = "200"
test "$(request_status http://127.0.0.1:8080/hostcall/fail-close)" = "200"
test "$(request_status http://127.0.0.1:8080/hostcall/fail-close)" = "429"
grep -Eqi '^x-ratelimit-limit:' /tmp/ingate-hostcall-headers
grep -Eqi '^x-ratelimit-remaining:' /tmp/ingate-hostcall-headers
grep -Eqi '^x-ratelimit-reset:' /tmp/ingate-hostcall-headers

docker exec "$REDIS_CONTAINER" redis-cli --scan --pattern 'ingate-hostcall-e2e:*' | grep -q .
docker exec "$REDIS_CONTAINER" redis-cli FLUSHALL >/dev/null
```

- [ ] **Step 6: Verify FailClose, FailOpen and recovery without restarting Envoy**

Continue the script:

```bash
docker stop "$REDIS_CONTAINER" >/dev/null

test "$(request_status http://127.0.0.1:8080/hostcall/fail-close)" = "429"
test "$(request_status http://127.0.0.1:8080/hostcall/fail-open)" = "200"

docker start "$REDIS_CONTAINER" >/dev/null
sleep 2
docker exec "$REDIS_CONTAINER" redis-cli FLUSHALL >/dev/null

test "$(request_status http://127.0.0.1:8080/hostcall/fail-open)" = "200"
test "$(request_status http://127.0.0.1:8080/hostcall/fail-open)" = "200"
test "$(request_status http://127.0.0.1:8080/hostcall/fail-open)" = "429"
```

The `request_status` helper already uses finite `--max-time 5`, so a missing timeout callback fails the script instead of hanging indefinitely.

- [ ] **Step 7: Fail on ABI or callback errors in Envoy logs**

Add:

```bash
docker exec "$INGATE_CONTAINER" tail -n 300 /var/log/ingate/envoy.log

if docker exec "$INGATE_CONTAINER" grep -Eqi \
  'unknown host function|missing callback|invalid callout id|redis_call_response on invalid' \
  /var/log/ingate/envoy.log; then
	echo "unexpected Redis hostcall error in Envoy log" >&2
	exit 1
fi
```

- [ ] **Step 8: Run and commit the E2E script**

Run:

```bash
chmod +x hack/e2e-higress-redis-hostcall.sh
./hack/e2e-higress-redis-hostcall.sh
```

Expected: the script exits 0, proves `200, 200, 429`, FailClose, FailOpen and recovery, and removes both containers and the network through the EXIT trap.

Commit:

```bash
git add hack/e2e-higress-redis-hostcall.sh
git commit -m "test: verify Higress Redis hostcall flow"
```

- [ ] **Step 9: Run final repository verification**

Run:

```bash
make test
make build
make plugins-build
make all-in-one-image
./hack/e2e-higress-redis-hostcall.sh
git status --short
```

Expected: all commands PASS and the worktree contains only intended source/document changes.

If E2E verification exposes a defect, add a focused regression test, rerun the relevant task checks, and commit the fix separately with a message describing the defect.
