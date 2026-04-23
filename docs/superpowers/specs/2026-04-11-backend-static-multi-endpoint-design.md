# Backend Static Multi-Endpoint Design

## Background

The current `Backend` capability in `ingate` already supports multiple static endpoints in the domain model and admin API:

- `pkg/apis/gateway/v1alpha1.StaticBackendSpec.Endpoints`
- `internal/adminapi/handler/dto.StaticBackendSpec.Endpoints`
- `internal/adminapi/convert/backend.go`

However, the current console implementation collapses the static backend experience into a single address plus single port:

- `staticAddress`
- `staticPort`

This creates a product mismatch:

1. `loadBalancePolicy` is exposed to the user.
2. The UI only allows one static endpoint.
3. For a single endpoint, load balancing has no practical meaning.

The result is a product flow that looks enterprise-oriented, but whose behavior is incomplete.

## Goal

Upgrade the console and admin workflow so that a `Static` backend can be configured with multiple `IP:Port` endpoints, while keeping the first iteration simple and product-coherent.

## Non-Goals

This iteration does not include:

- manual weight editing
- endpoint-level health editing
- drag-and-drop ordering
- advanced traffic distribution explanation
- richer service-discovery backends beyond current `Static` and `DNS`

## Product Decision

### Keep the current high-level backend model

The product continues to expose two backend types:

- `Static`
- `DNS`

This keeps the current conceptual model stable for users and does not introduce new backend categories.

### Upgrade only the `Static` experience

`Static` will support multiple endpoints.

Each endpoint is:

- `address`
- `port`

The user will be able to add and remove multiple endpoints from the console form.

### Keep weights internal for v1

The backend model already supports:

- `weight`
- `healthy`

But v1 will not expose endpoint weights in the form.

Instead:

- the console will submit `weight: 100` for every static endpoint
- the console will not allow editing `healthy`

This keeps the product simple while preserving a forward-compatible backend structure.

### Keep load balancing, but move it to advanced configuration

`loadBalancePolicy` remains valid and useful once multiple endpoints are supported.

Product treatment:

- keep the field
- move it to `高级配置`
- de-emphasize it when only one endpoint is configured

This keeps the UI professional without forcing unnecessary complexity onto the default path.

## UX Design

### Backend form

For `Static` backends, replace the current single-address section with a repeated endpoint list.

Each row contains:

- `IP/地址`
- `端口`

User actions:

- add endpoint
- remove endpoint

Validation:

- at least one endpoint is required for `Static`
- port must be `1..65535`
- address must be non-empty

Notes:

- v1 does not expose `weight`
- v1 does not expose `healthy`

### Field wording

Recommended wording for the static backend area:

- section title: `服务地址`
- hint text: `可配置一个或多个 IP:Port；配置多个地址时，请求会按负载策略分发。`

### Detail view

The backend detail drawer should show:

- backend type
- default port
- load-balance policy
- endpoint list

For static backends, the endpoint list should be shown explicitly instead of compressing the backend into a single `address:port` summary.

### List view

The backend list page does not need to show all endpoint details.

Recommended v1 behavior:

- keep `名称`
- keep `状态`
- replace single-address thinking with either:
  - endpoint count, or
  - a short address summary

The simplest and clearest v1 choice is endpoint count plus detail drawer expansion.

## Technical Scope

### Console changes

The main console work is localized to the backend form, request mapping, and backend detail display.

#### Form model

Change `BackendFormValues` from:

- `staticAddress`
- `staticPort`

to:

- `staticEndpoints: Array<{ address: string; port: number }>`

#### Form UI

Update `BackendFormDrawer` to use a dynamic list for static endpoints.

#### Request mapping

Update request builders so `Static` produces:

```json
{
  "static": {
    "endpoints": [
      { "address": "10.0.1.10", "port": 80, "weight": 100 },
      { "address": "10.0.1.11", "port": 80, "weight": 100 }
    ]
  }
}
```

#### Edit mapping

Update backend-to-form mapping so existing static backends with multiple endpoints round-trip correctly.

#### Page display

Update backend detail rendering to show all configured endpoints.

### Admin API changes

No structural admin API redesign is required.

The admin API already supports multiple endpoints in request/response DTOs.

Possible work items are limited to:

- verifying request validation remains correct for multiple endpoints
- verifying create/update/list/get flows return the expected endpoint arrays

### Domain model changes

No domain-model redesign is required.

The Kubernetes-style resource and validation layer already supports multiple static endpoints.

## Complexity Assessment

This is not a large architectural change.

It is a medium-small feature because:

- the backend model already supports the desired structure
- the admin API already supports the desired structure
- the current mismatch is mainly in the console form and mapping layer

Expected complexity:

- console: medium
- admin API: small
- resource model: none or minimal

## Rollout Recommendation

### v1

Deliver:

- multi-endpoint static backend form
- equal-weight submission
- updated detail display
- load-balance retained as advanced configuration

### v2

Consider:

- endpoint weight editing
- richer endpoint health display
- better address summary in tables
- more advanced backend traffic controls

## Why this is the right cut

This design fixes a real product inconsistency without opening a large configuration surface.

It is enterprise-grade because:

- the product behavior now matches the underlying model
- load balancing becomes semantically justified
- the UI supports common real-world multi-instance backends

It remains user-friendly because:

- the first version avoids endpoint-level tuning overload
- the form stays focused on the two pieces of information users actually need first
- advanced traffic behavior is preserved, but not overexposed
