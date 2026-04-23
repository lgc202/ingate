# Backend Protocol Modeling Design

## Background

The current `Backend` experience in `ingate` and `ingate-console` has an inconsistent notion of service protocol.

Today:

- the console displays a backend protocol field in some places
- that value is inferred from the backend port
- there is no real `protocol` field in the backend resource model
- admin-api does not expose backend protocol as an explicit contract field

This creates a product problem:

1. users see the concept of protocol
2. the concept is not modeled as real data
3. the displayed value is heuristic rather than authoritative

For an enterprise control plane, that is not good enough. If a field is visible to the user, it should be a real field with stable semantics.

## Goal

Introduce a real `protocol` field for `Backend`, and support it consistently across:

- backend resource model
- defaults and validation
- admin-api DTOs and conversion
- console forms and display

Supported values in v1:

- `HTTP`
- `HTTPS`
- `gRPC`

Default value:

- `HTTP`

## Non-Goals

This design does **not** include a full upstream protocol behavior implementation.

Specifically out of scope:

- upstream TLS advanced settings
- SNI configuration
- upstream certificate verification controls
- h2c or other extended protocol options
- gRPC-specific transport or health behavior
- deep protocol-aware diagnostics

This is a modeling-and-configuration version, not a full data-plane protocol capability project.

## Product Decision

### Make protocol a first-class backend field

`Backend` should have a real protocol field instead of relying on port-based guessing.

That field must be:

- persisted in the resource
- accepted by admin-api create and update flows
- returned by list and get responses
- editable from the console
- displayed in the console from real data only

### Supported protocol values

The first version supports:

- `HTTP`
- `HTTPS`
- `gRPC`

We explicitly do **not** include `TCP` in this iteration because it expands the model boundary and pushes the product toward a broader upstream transport abstraction.

### Default protocol

Default to:

- `HTTP`

Reasoning:

- it is the most conservative and least surprising default
- it preserves compatibility with existing resources that have no protocol yet
- it matches the current product mental model more closely than `HTTPS` or `gRPC`

## Compatibility Strategy

### New backends

For newly created backends:

- if protocol is not explicitly provided, it defaults to `HTTP`

### Backend updates

For backend update requests:

- `protocol` is required in the admin-api update DTO
- `protocol: ""` is invalid in update requests
- the server does not interpret omission as "preserve old value"
- the server does not interpret omission as "reset to default"

This keeps update semantics explicit and avoids silent protocol changes during edit/save flows.

### Existing backends

For existing resources that do not yet contain `protocol`:

- treat them as `HTTP`

This avoids the need for data migration and keeps old resources readable and editable.

The source of truth for this compatibility behavior is:

- server-side defaulting on the resource model

During mixed-version rollout, a temporary compatibility fallback to `HTTP` is allowed only in:

- the API-to-console mapping layer

It is not allowed in:

- list rendering
- detail rendering
- any other UI presentation helper

That means the console UI still displays a real field from its view model, while the mapper may temporarily normalize missing backend responses to `HTTP` until the backend contract is fully rolled out.

### Console behavior

The console must stop inferring protocol from port numbers.

After this change:

- protocol shown in lists and detail views must always come from the real field
- no `443 -> HTTPS` guessing
- no `8443 -> HTTPS` guessing
- no `else -> HTTP` fallback for presentation logic

## Backend Resource Model Design

### Resource field

Add to `BackendSpec`:

```go
Protocol string `json:"protocol,omitempty"`
```

Allowed values:

- `HTTP`
- `HTTPS`
- `gRPC`

### Defaulting

When `Backend.Spec.Protocol` is empty:

- default it to `HTTP`

### Validation

Validation should reject any backend protocol outside:

- `HTTP`
- `HTTPS`
- `gRPC`

This validation applies consistently to:

- create
- update

## Admin API Design

### DTO changes

Add `protocol` to:

- `CreateBackendRequest`
- `UpdateBackendRequest`
- `BackendSpec`

This ensures create, update, list, and get all round-trip the same field.

For update semantics:

- `UpdateBackendRequest.protocol` is required
- `UpdateBackendRequest.protocol=""` must be rejected by validation
- console edit forms must always submit an explicit protocol value
- old resources that did not contain protocol must round-trip as `HTTP`

### Conversion

Update backend conversion so `protocol` is mapped both directions:

- DTO -> resource
- resource -> DTO

No other backend shape changes are needed in this topic.

## Console Design

### Form behavior

In the backend form:

- add a `服务协议` field
- place it in the basic configuration area
- expose:
  - `HTTP`
  - `HTTPS`
  - `gRPC`

Default:

- `HTTP`

### Why it belongs in basic configuration

Protocol is not a low-level tuning knob.

It is a core attribute of the backend target, and users expect it to be visible and editable as part of the primary backend definition.

### Display behavior

Backend list page:

- show a real protocol column again
- render it from the stored protocol field

Backend detail page:

- show the real protocol value

The detail-view surface for this topic is:

- the backend detail drawer rendered from `BackendsPage.tsx`

### Remove protocol guessing

Any console logic that derives protocol from port must be removed.

If the resource says `gRPC`, the UI must show `gRPC` even if the port is unusual.

If the resource says `HTTP`, the UI must show `HTTP` even on port `443`.

## Interaction with Existing Port Design

This topic does **not** redefine backend ports.

Current backend port treatment remains:

- static endpoints carry their own ports
- DNS targets specify a service port

The protocol field is orthogonal to the port field in this version.

That means:

- `HTTPS` does not force `443`
- `gRPC` does not force a specific port
- port and protocol remain independently configurable

This is intentional for v1 because we are modeling protocol, not introducing protocol-port coupling rules.

## Technical Scope

### Ingate repo

Expected files:

- `pkg/apis/gateway/v1alpha1/types_backend.go`
- `pkg/apis/gateway/v1alpha1/defaults.go`
- `pkg/apis/gateway/validation/validation.go`
- `internal/adminapi/handler/dto/backend.go`
- `internal/adminapi/convert/backend.go`
- `tools/hack/verify-admin-api.sh`

### Console repo

Expected files:

- `src/features/resources/forms.ts`
- `src/features/resources/BackendFormDrawer.tsx`
- `src/api/requests.ts`
- `src/api/types.ts`
- `src/api/mappers.ts`
- `src/pages/BackendsPage.tsx`

This console scope includes both:

- backend list rendering
- backend detail drawer rendering

### Existing logic to remove

Any helper equivalent to:

- infer protocol from port

must be removed or retired from backend display logic.

## Risks

### User expectation risk

Once users can configure `HTTPS` or `gRPC`, they may assume full protocol-specific upstream behavior already exists.

Mitigation:

- treat this as a real field, but document clearly that v1 is protocol modeling, not full advanced transport configuration

### Compatibility risk

Older resources do not have a protocol field.

Mitigation:

- apply defaulting to `HTTP`
- make DTO-to-console mapping treat empty protocol as `HTTP`

### Partial rollout risk

If only some layers are updated, the field may disappear during round-trips.

Mitigation:

- update resource model, DTOs, convertors, requests, mappers, and display together

## Verification Plan

### Resource model

Verify:

- defaulting to `HTTP`
- validation rejects unsupported protocol values

### Admin API

Verify:

- create backend with `protocol`
- update backend with `protocol`
- get/list responses return `protocol`

### Console

Verify:

- backend create form can select protocol
- edit form round-trips protocol correctly
- list and detail pages display real protocol
- protocol no longer changes based on port guessing

### Commands

Expected verification commands:

- `npm run typecheck`
- `npm run build`
- `make verify-admin-api`

## Why This Topic Is Worth Doing Now

This topic is worth doing now because the product already exposes the idea of backend protocol, but does so inconsistently.

Without this change:

- the UI keeps teaching users a concept that is not formally modeled
- protocol display remains heuristic
- future protocol work has no stable contract to build on

With this change:

- protocol becomes a real backend attribute
- console and backend API stay aligned
- future `HTTPS` and `gRPC` capability work has a clean base

## Final Decision

Implement a backend protocol modeling topic with these boundaries:

- add real `Backend.protocol`
- support `HTTP / HTTPS / gRPC`
- default to `HTTP`
- wire through resource model, admin-api, and console
- remove all backend protocol guessing from the console
- do not implement full protocol behavior differences yet
