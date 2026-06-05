# Route Policy Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Status:** 这是一版用于验证控制台路由策略闭环的 MVP 计划。项目后续不继续沿用 `route.ingate.io/policy-bindings` annotation 作为长期模型；Route、Policy 和 Plugin 的正式边界以 `docs/superpowers/specs/2026-06-05-route-policy-plugin-model-design.md` 为准。

**Goal:** 实现路由级企业策略最小闭环：控制台选择策略，Route 保存策略参数，compiler/xDS 翻译策略，Envoy 运行时真正生效

**Architecture:** 第一阶段不新增通用策略 CRD，也不做插件市场。路由级内置策略从 `route.ingate.io/policy-bindings` annotation 进入 IR，并由 xDS target 翻译成 Envoy route action/header action。该方案只用于早期验证，长期实现应将 Header 改写、超时和重试迁入 `RouteSpec` 的强类型字段或 route filters。

**Tech Stack:** Go, React/TypeScript, Envoy xDS RouteConfiguration

---

### Task 1: 后端返回内置策略模板

**Files:**
- Modify: `internal/adminapi/handler/route/dto/response.go`

- [x] `composer.policies` 返回 Header 改写、超时、重试三个企业常用策略
- [x] 参数使用结构化字段，不让用户手写 JSON
- [x] AI 专属策略先不返回，避免不能生效的策略出现在普通路由里

### Task 2: IR 增加路由级策略效果

**Files:**
- Modify: `internal/core/ir/gateway.go`
- Modify: `internal/core/compiler/compiler.go`
- Test: `internal/core/compiler/compiler_test.go`

- [x] 从 Route annotation 解析 `policyBindings`
- [x] 支持请求 Header set/add/remove
- [x] 支持 route timeout override
- [x] 支持 retry attempts/perTryTimeout

### Task 3: xDS target 翻译策略效果

**Files:**
- Modify: `internal/core/target/xds/translator.go`
- Modify: `internal/xds/server/route_response.go`
- Test: `internal/core/target/xds/translator_test.go`
- Test: `internal/xds/server/route_response_test.go`

- [x] xDS 内部 Route 带上 header action 和 retry policy
- [x] Envoy route 生成 `request_headers_to_add/remove`
- [x] Envoy route action 生成 retry policy

### Task 4: 验证

**Commands:**
- `npm run build --prefix web/console`
- `make test`
- `make build`
- `git diff --check`
- `make dev-restart`

- [ ] all-in-one 创建临时服务和路由
- [ ] 路由绑定 Header 改写策略
- [ ] `curl http://127.0.0.1:8080/get` 可看到改写后的 header
- [ ] 清理临时资源
