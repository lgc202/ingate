# Managed Rate Limit Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Ingate managed rate-limit Wasm plugin implementation that consumes the xDS managed rate-limit schema.

**Architecture:** The plugin is a separate Go module under `plugins/managed/rate-limit` so proxy-wasm and TinyGo dependencies do not pollute the control-plane module. Core rate-limit config parsing, key extraction, and fixed-window decisions live in ordinary Go packages; the Wasm entrypoint only adapts Envoy/proxy-wasm APIs to that core logic. The plugin can reference Higress implementation ideas, but must not depend on Higress `wasm-go/pkg/wrapper` or its product config model.

**Tech Stack:** Go, TinyGo, proxy-wasm Go SDK, Ingate managed rate-limit xDS schema.

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

- [ ] Define plugin-level config with `schemaVersion`, `redisStores`, and executable route configs
- [ ] Define route config with `gatewayName`, `routeName`, `ruleName`, and expanded `bindings`
- [ ] Parse the Ingate `RateLimitPolicy` schema without depending on the control-plane module
- [ ] Implement local fixed-window limiter
- [ ] Implement key extraction for IP, Header, Query, Cookie, Consumer, Route, and Gateway

### Task 3: Wasm Entrypoint

**Files:**
- Create: `plugins/managed/rate-limit/main.go`

- [ ] Register plugin config parsing
- [ ] Read Listener-level executable route config by current xDS route name
- [ ] Execute local rules synchronously
- [ ] Keep Redis global execution behind an Ingate-owned data-plane executor boundary
- [ ] Return over-limit responses and quota headers
- [ ] Honor `FailOpen` and `FailClose`

### Task 4: Verification

**Files:**
- Test: `plugins/managed/rate-limit/internal/...`

- [ ] Run plugin unit tests with normal Go
- [ ] Run root `make test`
- [ ] Run root `make build`
- [ ] Run `managed-rate-limit-plugin-build` if TinyGo is available
