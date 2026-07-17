# Control Plane Core MVP Design

> 状态：历史 MVP 设计。本文中的 Logical IR、Target、Translator 和 RuntimeSnapshot 长期架构已被 [2026-07-17-ingate-simplified-architecture-design.md](./2026-07-17-ingate-simplified-architecture-design.md) 取代。

## 1. Project Positioning

Ingate Next is a declarative control-plane project for API gateways, AI gateways, and future traffic runtimes.

The first milestone is intentionally small: prove the control-plane core shape before adding storage, Envoy, plugins, AI providers, or deployment automation.

The MVP should answer one question:

```text
Can a small set of declared resources be compiled into a runtime-neutral logical model and then translated into a target-specific snapshot?
```

## 2. Architecture Principles

### Keep The Core Small

The core owns only resource modeling, compilation, IR construction, runtime snapshot construction, and target translation contracts.

It does not own request execution, proxy process management, model provider calls, persistence, admin UI behavior, or Kubernetes integration.

### Compile Before Publish

User-facing resources should not be pushed directly to a runtime target.

The control-plane path is:

```text
Resource -> Compiler -> Logical IR -> Target Translator -> RuntimeSnapshot
```

Publishing is a later phase.

### Runtime Targets Are Pluggable

`xds` will be the first real target, but the core must not assume Envoy-specific concepts in its logical model.

Target-specific data belongs behind translator implementations and inside `RuntimeSnapshot`.

### AI Is Deferred

The MVP does not implement AI resources. AI gateway capability will be redesigned as a separate domain when its product boundary is clear.

## 3. Non-Goals

This design does not implement:

- API server
- etcd or persistent storage
- Envoy xDS server
- data-plane agent
- Kubernetes operator
- plugin system
- AI Gateway runtime
- authentication, RBAC, tenants, or billing
- generated clients, informers, or CRDs

These are later milestones.

## 4. Core Concepts

### Resource

A `Resource` is a declared object with metadata and spec.

The MVP resource set is:

- `Gateway`
- `Route`
- `Upstream`

These are enough to prove the compilation path:

```text
Gateway
  -> Route
    -> Upstream
```

### Metadata

Every resource has minimal metadata:

```go
type Metadata struct {
    Name string
}
```

No namespace or tenant support is required in the MVP. Those can be added later without changing the compiler shape.

### Gateway

`Gateway` represents an entry point.

MVP fields:

```go
type GatewaySpec struct {
    Listeners []Listener
}

type Listener struct {
    Name     string
    Protocol string
    Port     int
    Hostname string
}
```

### Route

`Route` connects request matches to upstream references.

MVP fields:

```go
type RouteSpec struct {
    ParentRefs []string
    Hostnames  []string
    Rules      []RouteRule
}

type RouteRule struct {
    PathPrefix  string
    UpstreamRefs []UpstreamRef
}

type UpstreamRef struct {
    Name   string
    Weight int
}
```

### Upstream

`Upstream` represents a logical upstream service and its endpoints.

MVP fields:

```go
type UpstreamSpec struct {
    Endpoints []Endpoint
}

type Endpoint struct {
    Address string
    Port    int
}
```

## 5. Logical IR

The logical IR expresses runtime-neutral gateway intent.

It is not a copy of the resource model and not an Envoy model.

MVP shape:

```go
type LogicalGateway struct {
    Name      string
    Listeners []LogicalListener
    Routes    []LogicalRoute
    Upstreams []LogicalUpstream
}
```

The compiler should resolve references before producing IR:

- `Route.ParentRefs` must reference an existing `Gateway`.
- `Route.UpstreamRefs` must reference existing `Upstream` objects.
- Only routes attached to the selected gateway appear in that gateway's logical IR.

## 6. RuntimeSnapshot

`RuntimeSnapshot` is the stable handoff from core compilation to a runtime target.

It records:

- target name
- source gateway
- target-specific config payload

MVP shape:

```go
type RuntimeSnapshot struct {
    Target     string
    Gateway    string
    Version    string
    Config     any
}
```

The MVP target can be `debug`, not `xds`.

`debug` should produce a simple structured config that is easy to test without Envoy.

## 7. Compiler

The compiler consumes an in-memory bundle:

```go
type Bundle struct {
    Gateways []Gateway
    Routes   []Route
    Upstreams []Upstream
}
```

Compiler entry point:

```go
func CompileGateway(bundle Bundle, gatewayName string) (LogicalGateway, error)
```

Compiler responsibilities:

- find the selected gateway
- select attached routes
- validate upstream references
- build `LogicalGateway`
- return clear errors for missing references

Compiler non-responsibilities:

- storage
- watch loops
- status updates
- target translation
- publishing

## 8. Target Translator

A target translator converts logical IR into a runtime snapshot.

Contract:

```go
type Translator interface {
    Target() string
    Translate(logical LogicalGateway) (RuntimeSnapshot, error)
}
```

MVP translator:

```text
debug
```

The `debug` translator should preserve enough information to verify the pipeline in tests:

- listener names and ports
- route names and path prefixes
- upstream names and endpoints

## 9. First Directory Structure

Recommended first implementation layout:

```text
cmd/ingate/
  main.go

internal/core/resource/
  types.go

internal/core/ir/
  gateway.go

internal/core/compiler/
  compiler.go
  compiler_test.go

internal/core/runtime/
  snapshot.go

internal/core/target/
  translator.go

internal/core/target/debug/
  translator.go
  translator_test.go
```

Reasoning:

- `resource` owns declared user intent.
- `ir` owns runtime-neutral compiled intent.
- `compiler` owns resource-to-IR conversion.
- `runtime` owns target handoff types.
- `target` owns translator contracts and implementations.

## 10. MVP Acceptance Criteria

The first implementation is complete when:

1. A test can declare one gateway, one route, and one upstream in memory.
2. The compiler turns them into one `LogicalGateway`.
3. Missing gateway references return a clear error.
4. Missing upstream references return a clear error.
5. The debug translator turns `LogicalGateway` into `RuntimeSnapshot`.
6. `make test` passes.
7. `make build` passes.

## 11. Later Milestones

After this MVP, continue in this order:

1. Add `xds` translator contract and an Envoy-oriented runtime config.
2. Add persistent resource storage.
3. Add API server boundary.
4. Add data-plane model: `DataPlane` and `DataPlaneNode`.
5. Add explicit governance policy models as product requirements become clear.
6. Redesign AI gateway resources in a separate topic.
