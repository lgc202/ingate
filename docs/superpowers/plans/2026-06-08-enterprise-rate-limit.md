# Enterprise RateLimit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the complete Ingate RateLimit capability with real Redis-backed global limiting, ingate-dataplane integration, xDS injection, enterprise Redis topology support, and end-to-end verification.

**Architecture:** `RateLimitPolicy`、`PolicyBinding` 和 `RedisStore` 仍是一等资源；compiler 和 xDS target 生成内置限流插件配置。Wasm 插件只负责匹配、key 生成、本地 shared-data 限流和 HTTP 调用 ingate-dataplane；`ingate-dataplane` 作为内置数据面组件负责 Redis 连接、算法执行、故障策略输入和观测数据。xDS 自动注入 Wasm filter 和 dataplane cluster，用户不手工安装插件或维护 Envoy 私有配置。

**Tech Stack:** Go 1.26, Envoy xDS, Proxy-Wasm Go SDK, go-redis, Redis Lua, all-in-one Docker runtime.

---

## File Map

- Modify: `pkg/apis/gateway/v1/types.go`，扩展 RateLimit/RedisStore 长期模型
- Modify: `pkg/apis/gateway/zz_generated.deepcopy.go` and generated client/openapi files through `make generate`
- Modify: `internal/core/ir/*.go`，把模型编译进逻辑 IR
- Modify: `internal/core/target/xds/translator.go`，把 IR 转成内置限流插件和 dataplane 配置
- Modify: `internal/xds/server/listener_response.go`，向 Wasm 插件下发 ingate-dataplane 协议配置
- Modify: `internal/xds/server/cluster_response.go`，注入 ingate-dataplane internal cluster
- Create: `pkg/dataplane/ratelimit/*.go`，定义插件和 ingate-dataplane 间稳定数据契约
- Create: `internal/dataplane/{app,server,handler,service}/**`，实现数据面服务入口、路由、handler、限流算法和 HTTP API
- Create: `pkg/xredis/**`，实现 Redis client 管理和连接池复用
- Create: `cmd/ingate-dataplane/main.go`，内置数据面服务进程入口
- Modify: `pkg/plugin/ratelimit/**`，同步插件可执行配置 schema
- Modify: `plugins/ratelimit/internal/{app,wasm,policy,dataplane}/**`，支持完整算法、global check 元数据和数据面调用
- Modify: `plugins/ratelimit/main.go`，保留精简 Wasm 入口，由 `app` 完成插件装配注册
- Modify: `internal/adminapi/handler/redisstore/**` and `internal/adminapi/service/redisstore/**`，补齐 RedisStore DTO 和连接测试
- Modify: `deploy/all-in-one/Dockerfile` and `deploy/all-in-one/entrypoint.sh`，打包并启动 ingate-dataplane
- Modify: `Makefile`，确保 build/dev-image 包含 ingate-dataplane 和插件
- Create or modify: `docs/*.md`，记录配置示例、故障策略、Redis 拓扑和端到端验证方式

## Task 1: Stable Protocol and Model

- [ ] Add shared dataplane rate-limit data contract package
- [ ] Extend `RateLimitAlgorithm` with `SlidingWindow`
- [ ] Extend `RateLimitQuota` for token bucket burst and refill semantics
- [ ] Extend `RateLimitKeyType` with `RouteRule`, `JWTClaim`, `APIKey`, `AIModel`, and `Tenant`
- [ ] Extend `RedisMode` with `Sentinel`
- [ ] Extend `RedisStoreSpec` with addresses, TLS server name, connect/command timeout, pool size, min idle, sentinel master, credential refs, and cluster/sentinel knobs
- [ ] Update Admin API DTO/service mapping for the model fields without adding frontend changes
- [ ] Run `make generate`

## Task 2: DataPlane HTTP API

- [ ] Create `cmd/ingate-dataplane`
- [ ] Add CLI flags for listen address, Redis config source, log level, and default timeout
- [ ] Implement `POST /v1/capabilities/rate-limit/check` using the stable protocol
- [ ] Implement `GET /healthz`
- [ ] Keep request validation at the external HTTP boundary
- [ ] Return protocol errors without leaking credentials

## Task 3: Redis Client Manager

- [ ] Implement standalone Redis client creation
- [ ] Implement sentinel Redis client creation
- [ ] Implement cluster Redis client creation
- [ ] Support username/password, DB, TLS, TLS server name, connect timeout, command timeout, pool size, and min idle
- [ ] Cache clients by RedisStore ID and config fingerprint
- [ ] Close replaced clients during hot update
- [ ] Add `RedisStore` connection test entry point for Admin API and ingate-dataplane bootstrap verification

## Task 4: Algorithms

- [ ] Implement FixedWindow with Lua so increment and expiry are atomic
- [ ] Implement SlidingWindow with sorted set Lua
- [ ] Implement TokenBucket with Lua
- [ ] Normalize quota result into allowed/current/limit/remaining/reset/retryAfter
- [ ] Keep algorithm errors separate from policy decisions so failOpen/failClose remains explicit

## Task 5: xDS Injection

- [ ] Add rate-limit ingate-dataplane constants and target model fields
- [ ] Inject dataplane cluster only when global policies are present
- [ ] Add dataplane cluster config to Wasm plugin config
- [ ] Preserve listener-level route config merging
- [ ] Keep wildcard virtual host merging behavior intact

## Task 6: Wasm Global Execution

- [ ] Convert `GlobalCheck` into ingate-dataplane protocol checks
- [ ] Dispatch one HTTP call to ingate-dataplane per request that needs global checks
- [ ] Pause request during ingate-dataplane call
- [ ] Parse dataplane response and reject on the first denied check
- [ ] Apply failOpen/failClose on timeout, 5xx, malformed response, and transport errors
- [ ] Merge quota headers across local and global decisions
- [ ] Remove the current placeholder log path that says global cannot run without ingate-dataplane

## Task 7: Deployment

- [ ] Include `ingate-dataplane` in all-in-one binaries and image
- [ ] Start ingate-dataplane before Envoy in `entrypoint.sh`
- [ ] Add all-in-one default ingate-dataplane address
- [ ] Document `/opt/ingate/plugins/ratelimit.wasm` and ingate-dataplane placement
- [ ] Ensure no `_output/` artifacts are committed

## Task 8: Verification

- [ ] Run `make test`
- [ ] Run `make build`
- [ ] Run `make ratelimit-plugin-test`
- [ ] Run `make ratelimit-plugin-build`
- [ ] Run `make dev-image`
- [ ] Run all-in-one E2E with Envoy + Wasm + ingate-dataplane + Redis standalone
- [ ] Verify first request allowed and second request rejected for global FixedWindow
- [ ] Verify Redis down with FailOpen allows and FailClose rejects
- [ ] Verify multiple Envoy workers share global quota through Redis
- [ ] Verify dataplane health endpoint and logs are usable for debugging

## Commit Strategy

- [ ] Commit model/protocol changes separately
- [ ] Commit dataplane implementation separately
- [ ] Commit xDS/plugin integration separately
- [ ] Commit deployment/E2E/documentation separately
