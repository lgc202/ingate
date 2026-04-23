# Backend Protocol Modeling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a real `protocol` field to `Backend`, support `HTTP / HTTPS / gRPC` with default `HTTP`, and wire it through the resource model, admin-api, and console without introducing full upstream protocol behavior.

**Architecture:** The change is a modeling topic, not a data-plane protocol feature. `ingate` becomes the source of truth by storing and defaulting `Backend.spec.protocol`, `admin-api` exposes it explicitly in create/update/get/list flows, and `ingate-console` stops inferring protocol from port and instead displays and edits the real field.

**Tech Stack:** Go, Kubernetes-style API types/defaulting/validation, Gin, shell verification scripts, React, TypeScript, Ant Design, Vite

---

## File Map

### Ingate repo: `/Users/guangcaili/workplace/code/lgc202/ingate`

- Modify: `pkg/apis/gateway/v1alpha1/types_backend.go`
  - Add `BackendSpec.Protocol`.
- Modify: `pkg/apis/gateway/v1alpha1/defaults.go`
  - Default backend protocol to `HTTP`.
- Modify: `pkg/apis/gateway/validation/validation.go`
  - Validate protocol enum and reject empty string on update through DTO/validation flow.
- Modify: `internal/adminapi/handler/dto/backend.go`
  - Add `protocol` to backend create/update/response DTOs.
- Modify: `internal/adminapi/convert/backend.go`
  - Round-trip protocol between DTOs and resource model.
- Modify: `tools/hack/verify-admin-api.sh`
  - Include protocol in backend create/update verification payloads.

### Console repo: `/Users/guangcaili/workplace/code/lgc202/ingate-console`

- Modify: `src/features/resources/forms.ts`
  - Add backend `protocol` form field and defaults.
- Modify: `src/features/resources/BackendFormDrawer.tsx`
  - Add visible `服务协议` field in basic configuration.
- Modify: `src/api/requests.ts`
  - Send backend protocol in create/update requests.
- Modify: `src/api/types.ts`
  - Add protocol to the backend view model.
- Modify: `src/api/mappers.ts`
  - Map protocol from DTOs and remove protocol inference from port.
- Modify: `src/pages/BackendsPage.tsx`
  - Display the real protocol value in list/detail surfaces.
- Optional cleanup target if needed: any helper in console files that still guesses backend protocol from port.

## Plan Constraints

- Do not add upstream TLS configuration.
- Do not add gRPC-specific transport behavior.
- Do not add TCP protocol in this topic.
- Do not infer protocol from port in UI after this change.
- Do not silently reset protocol to `HTTP` on update when omitted or empty.
- `/Users/guangcaili/workplace/code/lgc202/ingate` is not currently a Git repository, so commit steps are intentionally omitted.

### Task 1: Extend Backend Resource Model

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/gateway/v1alpha1/types_backend.go`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/gateway/v1alpha1/defaults.go`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/gateway/validation/validation.go`

- [ ] **Step 1: Add the protocol field to the backend spec**

Add:

```go
Protocol string `json:"protocol,omitempty"`
```

to `BackendSpec`.

- [ ] **Step 2: Add a backend protocol default**

Introduce a backend protocol default constant:

```go
DefaultBackendProtocol = "HTTP"
```

and apply it in `SetDefaults_Backend`.

- [ ] **Step 3: Add protocol validation**

Extend backend validation so only these values are allowed:

```go
"HTTP", "HTTPS", "gRPC"
```

Reject empty protocol after DTO decoding on update paths by ensuring update requests must carry a valid non-empty value.

- [ ] **Step 4: Verify resource-model compilation**

Run: `go test ./pkg/apis/gateway/...`
Expected: PASS

### Task 2: Wire Protocol Through Admin API

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate/internal/adminapi/handler/dto/backend.go`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate/internal/adminapi/convert/backend.go`

- [ ] **Step 1: Add protocol to backend DTOs**

Add `Protocol string` to:

- `CreateBackendRequest`
- `UpdateBackendRequest`
- `BackendSpec`

Use validation/binding semantics so:

- create requests may omit protocol and rely on defaulting to `HTTP`
- update requests must provide a non-empty protocol value

- [ ] **Step 2: Map protocol from DTO to resource**

Update backend create/update convertors to populate:

```go
Spec.Protocol = req.Protocol
```

- [ ] **Step 3: Map protocol from resource to DTO**

Update response conversion to return:

```go
Protocol: backend.Spec.Protocol
```

- [ ] **Step 4: Verify admin-api package compilation**

Run: `go test ./internal/adminapi/...`
Expected: PASS

### Task 3: Update Admin API Verification Flow

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate/tools/hack/verify-admin-api.sh`

- [ ] **Step 1: Add protocol to backend create payload**

Update the backend create request body to include:

```json
"protocol":"HTTP"
```

Also add an explicit compatibility-path verification:

- create a backend request without `protocol`
- confirm it succeeds
- confirm the returned/listed backend resolves to `protocol: "HTTP"`

- [ ] **Step 2: Add protocol to backend update payload**

Update the backend update request body to include an explicit protocol value such as:

```json
"protocol":"gRPC"
```

This verifies update round-trip semantics instead of only create defaulting.

- [ ] **Step 3: Add negative-path update verification**

Add explicit verification that backend update requests:

- fail when `protocol` is omitted
- fail when `protocol` is `""`

Expected outcome:

- HTTP 400 from admin-api

- [ ] **Step 4: Verify get/list responses carry protocol**

After create and update, assert the backend list/get response bodies contain the expected `protocol` value.

- [ ] **Step 5: Keep the rest of the verification flow intact**

Do not change route/auth-policy/traffic-policy verification beyond what is necessary for backend protocol.

- [ ] **Step 6: Run full admin-api verification**

Run: `make verify-admin-api`
Expected:

- `ADMIN_API_BACKEND_CREATE_CODE=201`
- `ADMIN_API_UPDATE_VERIFY=yes`
- command exits with code 0

### Task 4: Add Backend Protocol to Console Form Model

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/features/resources/forms.ts`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/types.ts`

- [ ] **Step 1: Add protocol to backend form values**

Extend `BackendFormValues` with:

```ts
protocol: "HTTP" | "HTTPS" | "gRPC";
```

- [ ] **Step 2: Default backend form protocol**

Set the default backend form value to:

```ts
protocol: "HTTP"
```

- [ ] **Step 3: Round-trip protocol into edit values**

Update `backendToFormValues()` so old resources with no protocol become `HTTP`, while resources that already have a protocol preserve the real stored value.

- [ ] **Step 4: Extend backend view model**

Add `protocol` to the backend UI type so list/detail rendering no longer depends on heuristics.

- [ ] **Step 5: Verify console types after model update**

Run: `npm run typecheck`
Expected: Type errors remain only in the backend form/request/mapper callsites that still need protocol wiring.

### Task 5: Update Console Request and Mapping Logic

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/requests.ts`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/mappers.ts`

- [ ] **Step 1: Send protocol in backend create/update requests**

Add:

```ts
protocol: values.protocol
```

to backend create/update request builders.

- [ ] **Step 2: Remove backend protocol inference**

Delete or stop using any backend protocol helper that infers protocol from ports.

- [ ] **Step 3: Map protocol from DTOs**

Set backend protocol from the real DTO field, with temporary mapper-layer fallback to `HTTP` only if the backend response omits protocol during mixed-version rollout.

- [ ] **Step 4: Keep the fallback boundary strict**

Do not allow list/detail rendering to infer protocol themselves. Only the mapper may normalize missing data to `HTTP`.

- [ ] **Step 5: Verify console mapper/request integrity**

Run: `npm run typecheck`
Expected: PASS

### Task 6: Add Protocol to Console UI

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/features/resources/BackendFormDrawer.tsx`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/pages/BackendsPage.tsx`

- [ ] **Step 1: Add a visible 服务协议 field to the backend form**

Place the field in the basic configuration area with these options:

```ts
HTTP
HTTPS
gRPC
```

- [ ] **Step 2: Preserve current port simplifications**

Do not reintroduce the removed `默认端口` UI duplication. Keep the current simplified port treatment:

- static endpoints carry their own ports
- DNS uses a single service port field

- [ ] **Step 3: Show real protocol in the backend list**

Add or restore a protocol column in `BackendsPage.tsx` that displays the real backend protocol.

- [ ] **Step 4: Show real protocol in the backend detail drawer**

Update the detail drawer to render the same real backend protocol field.

- [ ] **Step 5: Verify backend list/detail UX**

Run: `npm run dev`
Expected:

- backend form shows `服务协议`
- edit form round-trips the selected protocol
- list page shows the real protocol
- detail drawer shows the real protocol
- protocol display does not change when the port changes

### Task 7: Final Verification

**Files:**
- Verify both repos

- [ ] **Step 1: Run console typecheck**

Run: `npm run typecheck`
Workdir: `/Users/guangcaili/workplace/code/lgc202/ingate-console`
Expected: PASS

- [ ] **Step 2: Run console build**

Run: `npm run build`
Workdir: `/Users/guangcaili/workplace/code/lgc202/ingate-console`
Expected: PASS

- [ ] **Step 3: Run ingate admin-api verification**

Run: `make verify-admin-api`
Workdir: `/Users/guangcaili/workplace/code/lgc202/ingate`
Expected: PASS

- [ ] **Step 4: Confirm contract-level protocol coverage**

Verify from admin-api responses:

- backend create/list/get/update flows all expose `protocol`
- create-with-omitted-`protocol` resolves to `HTTP`
- update omission and empty-string cases are rejected

- [ ] **Step 5: Run targeted Go verification**

Run:

```bash
go test ./pkg/apis/gateway/... ./internal/adminapi/...
```

Workdir: `/Users/guangcaili/workplace/code/lgc202/ingate`
Expected: PASS

- [ ] **Step 6: Manual product verification**

Check these flows manually:

- create `HTTP` static backend
- create `HTTPS` DNS backend
- create `gRPC` static backend
- edit an old backend that lacked protocol and confirm it round-trips as `HTTP`
- confirm backend list/detail show the configured protocol, not a port-based guess
