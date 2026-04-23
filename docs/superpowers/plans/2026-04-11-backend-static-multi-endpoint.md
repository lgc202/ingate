# Backend Static Multi-Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade `Static` backends from single-address console input to multi-endpoint configuration, while keeping endpoint weights internal and preserving the existing `DNS` flow.

**Architecture:** The resource model and admin API already support `static.endpoints[]`, so implementation should focus on removing the console's single-address assumption and tightening verification around existing backend validation. The product cut for v1 is: multiple static endpoints, equal default weight, unchanged backend API contract.

**Tech Stack:** React, TypeScript, Ant Design, Vite, Go, Gin, Kubernetes-style API types, shell verification scripts

---

## File Map

### Console repo: `/Users/guangcaili/workplace/code/lgc202/ingate-console`

- Modify: `src/features/resources/forms.ts`
  - Replace single static backend fields with a repeated endpoint form model.
- Modify: `src/features/resources/BackendFormDrawer.tsx`
  - Render repeated static endpoint rows, keep `DNS` path unchanged, move load balancing into advanced configuration if implemented in the same pass.
- Modify: `src/api/requests.ts`
  - Build `static.endpoints[]` payloads with default `weight: 100`.
- Modify: `src/api/types.ts`
  - Extend backend view model so backend pages can display endpoint arrays without collapsing to a single address.
- Modify: `src/api/mappers.ts`
  - Round-trip backend DTOs that contain multiple endpoints.
- Modify: `src/pages/BackendsPage.tsx`
  - Update list/detail display to reflect multiple endpoints coherently.
- Modify: `src/api/mock.ts`
  - Keep mock backends aligned with the new frontend backend shape.

### Ingate repo: `/Users/guangcaili/workplace/code/lgc202/ingate`

- Verify only: `internal/adminapi/handler/dto/backend.go`
  - Already supports `StaticBackendSpec.Endpoints`.
- Verify only: `internal/adminapi/convert/backend.go`
  - Already converts `[]BackendEndpoint` both directions.
- Verify only: `pkg/apis/gateway/v1alpha1/types_backend.go`
  - Already supports `StaticBackendSpec.Endpoints`.
- Verify and possibly update: `tools/hack/verify-admin-api.sh`
  - Extend backend create/update examples to include multiple static endpoints.

## Plan Constraints

- Do not redesign backend API shapes.
- Do not expose endpoint `weight` editing in v1.
- Do not expose endpoint `healthy` editing in v1.
- Do not redesign `DNS` backend behavior.
- Do not mix unrelated frontend polish into this change.
- This workspace is currently not a git repository, so commit steps are intentionally omitted from the plan.

### Task 1: Refactor Console Backend Form Model

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/features/resources/forms.ts`
- Reference: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/types.ts`

- [ ] **Step 1: Replace single static backend fields in the form model**

Change `BackendFormValues` from:

```ts
staticAddress: string;
staticPort: number;
```

to:

```ts
staticEndpoints: Array<{
  address: string;
  port: number;
}>;
```

- [ ] **Step 2: Update backend form defaults**

Set the default `Static` backend shape to:

```ts
staticEndpoints: [{ address: "", port: 80 }]
```

Run: `npm run typecheck`
Expected: Type errors in backend form and request mapping callsites.

- [ ] **Step 3: Update edit mapping from backend view model to form values**

Implement round-trip behavior so edit mode hydrates:

```ts
backend.endpoints -> form.staticEndpoints
```

Fallback rule:

- if no static endpoints exist, provide one empty row

Run: `npm run typecheck`
Expected: Typecheck still fails only in remaining backend UI/request files.

### Task 2: Update Console Request Builders

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/requests.ts`
- Reference: `/Users/guangcaili/workplace/code/lgc202/ingate/internal/adminapi/handler/dto/backend.go`

- [ ] **Step 1: Replace single-endpoint request building**

Remove the current single-endpoint shape:

```ts
endpoints: [
  {
    address: values.staticAddress.trim(),
    port: Number(values.staticPort) || Number(values.defaultPort)
  }
]
```

with a mapped array:

```ts
endpoints: values.staticEndpoints
  .map((endpoint) => ({
    address: endpoint.address.trim(),
    port: Number(endpoint.port) || Number(values.defaultPort),
    weight: 100
  }))
  .filter((endpoint) => endpoint.address)
```

- [ ] **Step 2: Keep DNS request building unchanged**

Ensure only the `Static` branch changes.

- [ ] **Step 3: Verify request builder output**

Run: `npm run typecheck`
Expected: No request-builder type errors.

### Task 3: Rebuild Static Backend Form UI

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/features/resources/BackendFormDrawer.tsx`

- [ ] **Step 1: Replace single static address section with a dynamic endpoint list**

Use repeated rows for:

- `地址`
- `端口`

Provide:

- add endpoint button
- remove endpoint button

- [ ] **Step 2: Keep validation minimal and product-coherent**

Validation rules:

- at least one endpoint for `Static`
- non-empty address
- port `1..65535`

- [ ] **Step 3: Keep weights internal**

Do not add a form field for:

- `weight`
- `healthy`

- [ ] **Step 4: Add the product hint text**

Static section hint:

```text
可配置一个或多个 IP:Port；配置多个地址时，请求会按负载策略分发。
```

- [ ] **Step 5: Verify form rendering**

Run: `npm run dev`
Expected: Static backend form supports add/remove endpoint rows and still renders the DNS path correctly.

### Task 4: Upgrade Console Backend View Model and Mapping

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/types.ts`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/mappers.ts`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/api/mock.ts`

- [ ] **Step 1: Extend the backend view model**

Add:

```ts
endpoints: Array<{
  address: string;
  port: number;
  weight?: number;
  healthy?: boolean;
}>;
```

Keep existing summary fields if still useful for list rendering.

- [ ] **Step 2: Stop collapsing backend DTOs to only the first endpoint**

Remove the current first-endpoint assumption from:

`mapBackend()`

and map the full endpoint array from either:

- `dto.status.endpoints`
- or `dto.spec.static.endpoints`

- [ ] **Step 3: Update mock backends**

Make mock data reflect the new endpoint-aware backend shape so UI fallback remains coherent.

- [ ] **Step 4: Verify type and mapper integrity**

Run: `npm run typecheck`
Expected: Backend page/display errors remain only where summary/detail UI still assumes one address.

### Task 5: Update Backend List and Detail UX

**Files:**
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/pages/BackendsPage.tsx`
- Reference: `/Users/guangcaili/workplace/code/lgc202/ingate-console/src/utils/display.ts`

- [ ] **Step 1: Replace single-address thinking in the list view**

Choose one v1 summary:

- endpoint count

Recommended column behavior:

```ts
title: "服务地址"
render: (backend) => backend.type === "Static" ? `${backend.endpoints.length} 个地址` : backend.address
```

- [ ] **Step 2: Update search logic**

Search should include:

- backend name
- DNS host
- static endpoint addresses
- route references

- [ ] **Step 3: Update detail drawer**

For static backends, show an explicit endpoint list:

```text
10.0.1.10:80
10.0.1.11:80
```

Keep:

- status
- protocol
- load-balance policy
- referenced routes

- [ ] **Step 4: Verify the backend page manually**

Run: `npm run dev`
Expected:

- create static backend with multiple endpoints
- edit existing static backend and preserve endpoints
- detail drawer shows endpoint list

### Task 6: Strengthen Admin API Verification

**Files:**
- Verify: `/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/gateway/validation/validation.go`
- Modify: `/Users/guangcaili/workplace/code/lgc202/ingate/tools/hack/verify-admin-api.sh`

- [ ] **Step 1: Confirm validation already supports multiple endpoints**

Review:

- `ValidateBackend`
- `validateEndpoint`

No code change needed unless a real gap is found.

- [ ] **Step 2: Update admin API verification script to use multiple endpoints**

Replace single-endpoint backend create/update payloads with multiple endpoint examples.

Use a payload shape like:

```json
{
  "type": "Static",
  "defaultPort": 8080,
  "static": {
    "endpoints": [
      { "address": "127.0.0.1", "port": 8080, "weight": 100, "healthy": true },
      { "address": "127.0.0.2", "port": 8080, "weight": 100, "healthy": true }
    ]
  }
}
```

- [ ] **Step 3: Run backend verification**

Run: `make verify-admin-api`
Expected: backend create/list/update/delete verification still passes with multi-endpoint payloads.

### Task 7: End-to-End Verification

**Files:**
- Verify: `/Users/guangcaili/workplace/code/lgc202/ingate-console`
- Verify: `/Users/guangcaili/workplace/code/lgc202/ingate`

- [ ] **Step 1: Run console typecheck**

Run: `npm run typecheck`
Expected: PASS

- [ ] **Step 2: Run console build**

Run: `npm run build`
Expected: PASS

- [ ] **Step 3: Run admin API verification**

Run: `make verify-admin-api`
Expected: PASS

- [ ] **Step 4: Manually verify the product flow**

Check:

- create static backend with one endpoint
- create static backend with multiple endpoints
- edit static backend and add/remove endpoints
- verify load-balance field still works
- confirm DNS backends remain unchanged

- [ ] **Step 5: Verify no whitespace or formatting regressions**

Run in console repo:

```bash
git diff --check
```

Expected: no whitespace errors

If the repo is still not initialized as Git, skip this step and rely on build plus manual inspection.
