# Rate Limit Policy Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the backend long-term model for `RateLimitPolicy + PolicyBinding + RedisStore`, compiling it into managed rate-limit plugin configuration.

**Architecture:** Control-plane resources stay strongly typed and product-oriented. The compiler emits runtime-independent IR, and the xDS target translates that IR into managed rate-limit plugin config consumed by the data plane. Admin API exposes DTOs for policies, bindings, and Redis stores without exposing plugin JSON.

**Tech Stack:** Go 1.26, Kubernetes apiserver-style resources, generated clients, Gin admin API, Envoy xDS internal config.

---

### Task 1: Coding Standard

**Files:**
- Modify: `AGENTS.md`

- [x] Add the managed governance plugin boundary to the coding standards
- [x] Add RateLimitPolicy/PolicyBinding/RedisStore modeling rules

### Task 2: Declarative Resource Model

**Files:**
- Modify: `pkg/apis/gateway/v1/types.go`
- Modify: `pkg/apis/gateway/types.go`
- Modify: `pkg/apis/gateway/v1/register.go`
- Modify: `pkg/apis/gateway/register.go`
- Generate: `pkg/apis/gateway/**/zz_generated.*`
- Generate: `pkg/generated/**`

- [ ] Replace the early single-rule `RateLimitPolicySpec` with the long-term model
- [ ] Add `RedisStore`
- [ ] Add `RuleName` and lifecycle fields to `PolicyBindingSpec`
- [ ] Run `make generate`

### Task 3: Apiserver And Store

**Files:**
- Create: `internal/apiserver/registry/redisstore/storage.go`
- Create: `internal/apiserver/registry/redisstore/strategy.go`
- Modify: `internal/apiserver/server/config.go`
- Create: `internal/adminapi/store/redisstore/store.go`
- Create: `internal/adminapi/store/ratelimitpolicy/store.go`
- Create: `internal/adminapi/store/policybinding/store.go`
- Modify: `internal/adminapi/store/store.go`

- [ ] Register RedisStore REST storage
- [ ] Add dedicated admin stores for policy resources

### Task 4: Compiler, Controller, IR

**Files:**
- Modify: `internal/core/ir/gateway.go`
- Modify: `internal/core/compiler/compiler.go`
- Modify: `internal/controller/controller/reconcile.go`
- Modify: `internal/controller/controller/controller.go`
- Modify: `internal/controller/controller/events.go`

- [ ] Add RedisStore to bundle and controller collection
- [ ] Compile enabled policies/bindings only
- [ ] Support RouteRule binding target
- [ ] Emit logical rate-limit policies and Redis stores

### Task 5: xDS Target Managed Plugin Config

**Files:**
- Modify: `internal/core/target/xds/translator.go`
- Modify: `internal/core/target/debug/translator.go`

- [ ] Translate logical rate-limit data into managed rate-limit plugin config
- [ ] Include RedisStore config without password material
- [ ] Keep debug target aligned for local inspection

### Task 6: Admin API

**Files:**
- Create: `internal/adminapi/handler/ratelimitpolicy/**`
- Create: `internal/adminapi/handler/policybinding/**`
- Create: `internal/adminapi/handler/redisstore/**`
- Create: `internal/adminapi/service/ratelimitpolicy/**`
- Create: `internal/adminapi/service/policybinding/**`
- Create: `internal/adminapi/service/redisstore/**`
- Modify: `internal/adminapi/handler/handler.go`
- Modify: `internal/adminapi/service/service.go`
- Modify: `internal/adminapi/server/router.go`

- [ ] Implement CRUD and enable/disable endpoints
- [ ] Validate request-local rules in DTO
- [ ] Validate name uniqueness and references in service
- [ ] Return `UserError` for user-facing business failures

### Task 7: Verification

**Files:**
- All changed Go files

- [ ] Run `gofmt`
- [ ] Run `make generate`
- [ ] Run `make test`
- [ ] Run `make build`
- [ ] Commit focused changes

