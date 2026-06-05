# Route Native Policy Plugin Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Header 改写、超时和重试从临时 Route annotation 迁入正式 `RouteSpec`，为 Route 原生能力、Policy 治理策略和 Plugin 扩展能力建立清晰代码边界

**Architecture:** Route 原生能力进入 `RouteRule` 强类型字段，admin-api DTO 将控制台产品参数转换为正式 `RouteSpec`。compiler 只读取正式资源模型，不再解析 `route.ingate.io/policy-bindings`，xDS target 继续从 IR 生成 Envoy header action、timeout 和 retry 配置。

**Tech Stack:** Go 1.26, Kubernetes-style API types/code-generator, Gin admin-api, Envoy xDS

---

### Task 1: Route API 增加原生能力模型

**Files:**
- Modify: `pkg/apis/gateway/v1/types.go`
- Modify: `pkg/apis/gateway/types.go`
- Regenerate: `pkg/apis/gateway/zz_generated.deepcopy.go`
- Regenerate: `pkg/apis/gateway/v1/zz_generated.deepcopy.go`
- Regenerate: `pkg/apis/gateway/v1/zz_generated.conversion.go`
- Regenerate: `pkg/generated/openapi/zz_generated.openapi.go`

- [x] Write failing compiler/admin-api tests that expect typed route filters, timeout and retry
- [x] Add `RouteFilterType`, `RouteFilter`, `HeaderModifier`, `HeaderValue`, `RouteTimeout`, and `RouteRetry`
- [x] Remove `AnnotationRoutePolicyBindings` from the API model
- [x] Run code generation

### Task 2: Admin API writes typed RouteSpec

**Files:**
- Modify: `internal/adminapi/handler/route/dto/request.go`
- Modify: `internal/adminapi/handler/route/dto/response.go`
- Modify: `internal/adminapi/handler/route/dto/types.go`
- Test: `internal/adminapi/handler/route/dto/request_test.go`
- Test: `internal/adminapi/handler/route/dto/response_test.go`

- [x] Update DTO tests to assert policy UI inputs become typed `RouteRule` fields
- [x] Replace annotation marshaling with typed route filters, timeout and retry
- [x] Keep display names only in admin-api capability catalog, never in compiler input
- [x] Preserve current validation behavior for header names, timeout bounds and retry bounds

### Task 3: Compiler consumes RouteSpec only

**Files:**
- Modify: `internal/core/compiler/compiler.go`
- Delete: `internal/core/compiler/route_policy.go`
- Test: `internal/core/compiler/compiler_test.go`

- [x] Update compiler tests to build routes with typed filters, timeout and retry
- [x] Translate route filters into `ir.LogicalRouteRule`
- [x] Delete annotation parsing and unsupported policy-name checks
- [x] Keep missing upstream and weight validation behavior unchanged

### Task 4: Runtime translation stays behavior-compatible

**Files:**
- Modify if needed: `internal/core/target/xds/translator.go`
- Modify if needed: `internal/xds/server/route_response.go`
- Test: `internal/core/target/xds/translator_test.go`
- Test: `internal/xds/server/route_response_test.go`

- [x] Ensure existing IR-to-xDS behavior remains unchanged
- [x] Adjust test fixtures only where source Route model changed
- [x] Verify Envoy route action still includes timeout, retry and header operations

### Task 5: Verification and docs cleanup

**Files:**
- Modify if needed: `docs/superpowers/specs/2026-06-05-route-policy-plugin-model-design.md`

- [x] Run focused Go tests for admin-api DTO, compiler and xDS route response
- [x] Run `make generate`
- [x] Run `make test`
- [x] Run `make build`
- [x] Run `git diff --check`
