# RateLimit Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Ingate RateLimit Wasm plugin implementation that consumes the xDS rate-limit schema.

**Architecture:** The plugin lives under `plugins/ratelimit` and shares the root Go module. Core rate-limit config parsing, key extraction, fixed-window decisions, and dataplane calls live in ordinary Go packages; the Wasm entrypoint only registers Envoy/proxy-wasm lifecycle hooks. The plugin can reference Higress implementation ideas, but must not depend on Higress `wasm-go/pkg/wrapper` or its product config model.

**Tech Stack:** Go, TinyGo, proxy-wasm Go SDK, Ingate rate-limit xDS schema.

---

### Task 1: Plugin Module Skeleton

**Files:**
- Create: `plugins/ratelimit/README.md`
- Modify: `Makefile`

- [ ] Create the rate-limit plugin package under the root Go module
- [ ] Add a root `ratelimit-plugin-build` target that builds the plugin package
- [ ] Document that the built artifact is installed to `/opt/ingate/plugins/ratelimit.wasm`

### Task 2: Runtime Schema and Core Logic

**Files:**
- Create: `pkg/plugin/ratelimit/types.go`
- Create: `plugins/ratelimit/internal/policy/policy.go`
- Create: `plugins/ratelimit/internal/policy/key.go`
- Create: `plugins/ratelimit/internal/policy/cookie.go`

- [ ] Define plugin-level config with `schemaVersion`, `redisStores`, and executable route configs
- [ ] Define route config with `gatewayName`, `routeName`, `ruleName`, and expanded `bindings`
- [ ] Parse the Ingate `RateLimitPolicy` schema without depending on the control-plane module
- [ ] Implement local fixed-window limiter
- [ ] Implement key extraction for IP, Header, Query, Cookie, Consumer, Route, and Gateway

### Task 3: Wasm Entrypoint

**Files:**
- Create: `plugins/ratelimit/main.go`

- [ ] Register plugin config parsing
- [ ] Read Listener-level executable route config by current xDS route name
- [ ] Execute local rules synchronously
- [ ] Keep Redis global execution behind the Ingate-owned `ingate-dataplane` boundary
- [ ] Return over-limit responses and quota headers
- [ ] Honor `FailOpen` and `FailClose`

### Task 4: Verification

**Files:**
- Test: `plugins/ratelimit/internal/...`

- [ ] Run plugin unit tests with normal Go
- [ ] Run root `make test`
- [ ] Run root `make build`
- [ ] Run `ratelimit-plugin-build` if TinyGo is available
