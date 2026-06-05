# Gateway Admin API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild Gateway as a long-term declarative entry model and make admin-api expose resource-first Gateway interfaces with clean DTO and handler/service boundaries.

**Architecture:** Gateway core semantics move into `GatewaySpec`: description, enabled, runtime group, listeners, host bindings, and TLS certificate references. admin-api exposes product DTOs and service use-case methods, while frontend consumes resource-level APIs and assembles page data itself when needed.

**Tech Stack:** Go 1.26, Kubernetes apiserver/code-generator, Gin, TypeScript, React, Vite.

---

## File Structure

- Modify `pkg/apis/gateway/types.go`: internal Gateway model.
- Modify `pkg/apis/gateway/v1/types.go`: external Gateway API model.
- Regenerate `pkg/apis/**/zz_generated.deepcopy.go`, `pkg/apis/gateway/v1/zz_generated.conversion.go`, clientsets, informers, listers and OpenAPI using `make generate`.
- Modify `internal/core/compiler/compiler.go`: compile new GatewaySpec fields into IR.
- Modify `internal/core/ir/gateway.go`: keep logical listener host semantics compatible with new host bindings.
- Modify compiler/pipeline tests under `internal/core/**`.
- Replace Gateway DTO files under `internal/adminapi/handler/gateway/dto`.
- Modify `internal/adminapi/handler/gateway/handler.go`: align handler responsibilities and remove `Overview`.
- Modify `internal/adminapi/server/router.go`: remove `/overview`; Gateway form dependencies come from resource APIs such as `/runtime-groups`.
- Modify `internal/adminapi/service/gateway/service.go` and `types.go`: accept use-case params, no page aggregation.
- Add admin-api Gateway DTO and service tests under `internal/adminapi/**`.
- Modify frontend Gateway domain/form/page/repository files under `web/console/src/**`.

## Task 1: Update Gateway Resource Model

**Files:**
- Modify: `pkg/apis/gateway/types.go`
- Modify: `pkg/apis/gateway/v1/types.go`
- Test: generated compile checks in later tasks

- [ ] **Step 1: Replace GatewaySpec fields**

Add `Description`, `Enabled`, `RuntimeGroupRef`, `Listeners`, and `HostBindings`.

- [ ] **Step 2: Add typed Gateway substructures**

Add `RuntimeGroupRef`, typed `ListenerProtocol`, `Listener`, `HostBinding`, and `GatewayTLS` with Chinese comments.

- [ ] **Step 3: Remove Gateway annotation constants from main path**

Remove `AnnotationGatewayDescription`, `AnnotationGatewayEnabled`, and `AnnotationGatewayHostnames` unless a compatibility test proves they are still required.

- [ ] **Step 4: Run focused compile to expose generated-code gaps**

Run: `go test ./pkg/apis/...`

Expected: FAIL before generation if generated conversion/deepcopy is stale.

## Task 2: Regenerate API Code

**Files:**
- Modify: generated files under `pkg/apis`, `pkg/generated`, `pkg/generated/openapi`

- [ ] **Step 1: Run code generation**

Run: `make generate`

Expected: generated deepcopy, conversion, client and OpenAPI files update without manual edits.

- [ ] **Step 2: Run API package tests**

Run: `go test ./pkg/apis/...`

Expected: PASS.

- [ ] **Step 3: Commit resource model**

Run:

```bash
git add pkg/apis pkg/generated hack/openapi/api-rule-violations.report
git commit -m "feat: model gateway host bindings"
```

## Task 3: Update Compiler for New GatewaySpec

**Files:**
- Modify: `internal/core/compiler/compiler.go`
- Modify: `internal/core/ir/gateway.go`
- Modify: `internal/core/compiler/compiler_test.go`
- Modify: `internal/core/pipeline/pipeline_test.go`

- [ ] **Step 1: Update tests to construct new GatewaySpec**

Change test Gateway fixtures to set `Enabled: true`, listeners without `Hostname`, and host bindings when a host is needed.

- [ ] **Step 2: Run compiler tests and observe failures**

Run: `go test ./internal/core/...`

Expected: FAIL until compiler reads host bindings.

- [ ] **Step 3: Compile enabled Gateways only**

Update compiler entry path so `Enabled=false` Gateway does not produce a runtime snapshot.

- [ ] **Step 4: Map HostBindings to logical listener hostnames**

For this milestone, keep IR simple: listeners retain one `Hostname` string. If a listener has exactly one host binding, populate it; if it has catch-all or multiple host bindings, keep empty hostname and leave richer virtual-host handling for later target work.

- [ ] **Step 5: Run core tests**

Run: `go test ./internal/core/...`

Expected: PASS.

- [ ] **Step 6: Commit compiler update**

Run:

```bash
git add internal/core
git commit -m "feat: compile gateway host bindings"
```

## Task 4: Rebuild Gateway admin-api DTOs

**Files:**
- Replace: `internal/adminapi/handler/gateway/dto/types.go`
- Replace: `internal/adminapi/handler/gateway/dto/request.go`
- Replace: `internal/adminapi/handler/gateway/dto/response.go`
- Add: `internal/adminapi/handler/gateway/dto/request_test.go`
- Add: `internal/adminapi/handler/gateway/dto/response_test.go`

- [ ] **Step 1: Write request DTO tests**

Cover required name, DNS label validation, listener protocol/port validation, duplicate listener names and ports, host binding listener refs, catch-all duplication, and HTTPS certificate requirement.

- [ ] **Step 2: Run DTO tests and confirm failure**

Run: `go test ./internal/adminapi/handler/gateway/dto`

Expected: FAIL before DTO rewrite.

- [ ] **Step 3: Define action-named DTOs**

Create `CreateGatewayReq`, `UpdateGatewayReq`, `SetGatewayEnabledReq`, `GatewayListenerReq`, `GatewayHostBindingReq`, `GatewayTLSReq`, `ListGatewaysResp`, `GetGatewayResp`, `GatewaySummary`, `GatewayDetail`, `GatewayListener`, `GatewayHostBinding`, and `GatewayTLS`.

- [ ] **Step 4: Move request validation into `Validate()`**

Validation normalizes strings and returns meaningful errors. DTO helper functions must not write HTTP responses.

- [ ] **Step 5: Keep response conversion resource-only**

Convert `resource.Gateway` to summary/detail without reading Route, Upstream or RuntimeSnapshot.

- [ ] **Step 6: Run DTO tests**

Run: `go test ./internal/adminapi/handler/gateway/dto`

Expected: PASS.

## Task 5: Rebuild Gateway Service and Handler

**Files:**
- Modify: `internal/adminapi/service/gateway/types.go`
- Modify: `internal/adminapi/service/gateway/service.go`
- Modify: `internal/adminapi/handler/gateway/handler.go`
- Modify: `internal/adminapi/server/router.go`
- Add: `internal/adminapi/service/gateway/service_test.go`

- [ ] **Step 1: Write service tests**

Cover create duplicate name, update version conflict, set enabled, delete with attached Route rejected, and catch-all shared listener conflict.

- [ ] **Step 2: Run service tests and confirm failure**

Run: `go test ./internal/adminapi/service/gateway`

Expected: FAIL before service rewrite.

- [ ] **Step 3: Define service params**

Add `CreateGatewayParams` and `UpdateGatewayParams`. Service should accept params or resource-neutral values, not gin DTO response objects.

- [ ] **Step 4: Update service methods**

`List` lists only Gateways. `Get` fetches only one Gateway. `Create` and `Update` construct `resource.Gateway` from params. `SetEnabled` updates `Spec.Enabled`. `Delete` keeps Route reference guard.

- [ ] **Step 5: Update cross-resource validation**

Use `Spec.Enabled` and catch-all host bindings instead of annotations when checking hostless listener conflicts.

- [ ] **Step 6: Update handlers**

Handlers bind URI/body, call `Validate()`, convert DTOs to service params, call service, and write unified responses. Remove `Overview`.

- [ ] **Step 7: Update router**

Remove `GET /api/v1/gateways/:id/overview`; do not add a Gateway form aggregation endpoint.

- [ ] **Step 8: Run adminapi tests**

Run: `go test ./internal/adminapi/...`

Expected: PASS.

- [ ] **Step 9: Commit admin-api update**

Run:

```bash
git add internal/adminapi
git commit -m "refactor: clean gateway admin api"
```

## Task 6: Update Frontend Gateway Contract

**Files:**
- Modify: `web/console/src/domain/gateway.ts`
- Modify: `web/console/src/features/gateways/form.ts`
- Modify: `web/console/src/features/gateways/GatewayPage.tsx`
- Modify: `web/console/src/api/contracts.ts`
- Modify: `web/console/src/api/liveConsoleRepository.ts`
- Modify: `web/console/src/mocks/consoleRepository.ts`

- [ ] **Step 1: Update TypeScript Gateway types**

Rename request shape to align with backend: `runtimeGroup`, `listeners`, `hostBindings`. Remove mutation fields `runtimeGroupName`, `certificateName`, and `hostnames`.

- [ ] **Step 2: Update Gateway form draft**

Keep UI host mode if useful, but build payload as host bindings with TLS certificate references.

- [ ] **Step 3: Update live repository**

Call `GET /runtime-groups` separately for running group options. Keep `GET /gateways` resource-only.

- [ ] **Step 4: Update mock repository**

Make mocks match the new API contract so local UI still works.

- [ ] **Step 5: Run frontend build**

Run: `npm run build --prefix web/console`

Expected: PASS.

- [ ] **Step 6: Commit frontend update**

Run:

```bash
git add web/console
git commit -m "refactor: align gateway console contract"
```

## Task 7: Final Verification

**Files:**
- All modified files

- [ ] **Step 1: Run generated verification**

Run: `make generate`

Expected: no unexpected changes after generation.

- [ ] **Step 2: Run Go tests**

Run: `make test`

Expected: PASS.

- [ ] **Step 3: Run Go build**

Run: `make build`

Expected: PASS.

- [ ] **Step 4: Run frontend build**

Run: `npm run build --prefix web/console`

Expected: PASS.

- [ ] **Step 5: Check working tree and whitespace**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only intentional source changes before final commit if any.
