# Managed Rate Limit Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Ingate managed rate-limit Wasm plugin implementation that consumes the xDS managed rate-limit schema.

**Architecture:** The plugin is a separate Go module under `plugins/managed/rate-limit` so proxy-wasm and TinyGo dependencies do not pollute the control-plane module. Core rate-limit config parsing, key extraction, and fixed-window decisions live in ordinary Go packages; the Wasm entrypoint only adapts Envoy/proxy-wasm APIs to that core logic. Global Redis limit uses the same fixed-window Lua pattern as Higress but consumes Ingate's strongly typed schema.

**Tech Stack:** Go, TinyGo, proxy-wasm Go SDK, Higress wasm-go wrapper, Redis Eval, Ingate managed rate-limit xDS schema.

---

### Task 1: Plugin Module Skeleton

**Files:**
- Create: `plugins/managed/rate-limit/go.mod`
- Create: `plugins/managed/rate-limit/README.md`
- Modify: `Makefile`

- [ ] Create an independent Go module for the managed rate-limit plugin
- [ ] Add a root `managed-rate-limit-plugin-build` target that runs TinyGo from the plugin module
- [ ] Document that the built artifact is installed to `/opt/ingate/plugins/rate-limit.wasm`

### Task 2: Runtime Schema and Core Logic

**Files:**
- Create: `plugins/managed/rate-limit/internal/config/config.go`
- Create: `plugins/managed/rate-limit/internal/ratelimit/limiter.go`
- Create: `plugins/managed/rate-limit/internal/ratelimit/key.go`
- Create: `plugins/managed/rate-limit/internal/ratelimit/cookie.go`

- [ ] Define plugin-level config with `schemaVersion` and `redisStores`
- [ ] Define route-level config with `gatewayName`, `routeName`, `ruleName`, and expanded `bindings`
- [ ] Parse the Ingate `RateLimitPolicy` schema without depending on the control-plane module
- [ ] Implement local fixed-window limiter
- [ ] Implement key extraction for IP, Header, Query, Cookie, Consumer, Route, and Gateway

### Task 3: Wasm Entrypoint

**Files:**
- Create: `plugins/managed/rate-limit/main.go`

- [ ] Register plugin config parsing
- [ ] Read route-level per-filter config when available
- [ ] Execute local rules synchronously
- [ ] Execute Redis global rules through async Redis Eval
- [ ] Return over-limit responses and quota headers
- [ ] Honor `FailOpen` and `FailClose`

### Task 4: Verification

**Files:**
- Test: `plugins/managed/rate-limit/internal/...`

- [ ] Run plugin unit tests with normal Go
- [ ] Run root `make test`
- [ ] Run root `make build`
- [ ] Run `managed-rate-limit-plugin-build` if TinyGo is available
