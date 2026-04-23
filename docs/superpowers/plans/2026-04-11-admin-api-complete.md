# Admin API Complete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the current-phase `ingate-admin-api` as a usable product API layer over `ingate-apiserver`.

**Architecture:** Keep the existing Gin + DTO + biz + convert + store layering. `admin-api` remains an HTTP JSON product API and uses generated clientset to call `ingate-apiserver`; it does not access etcd directly.

**Tech Stack:** Go, Gin, Kubernetes generated clientset, shell-based e2e verification.

---

### Task 1: Full Resource CRUD

**Files:**
- Modify: `internal/adminapi/handler/dto/*.go`
- Modify: `internal/adminapi/convert/*.go`
- Modify: `internal/adminapi/biz/*.go`
- Modify: `internal/adminapi/handler/*.go`
- Modify: `internal/adminapi/store/*.go`
- Modify: `internal/adminapi/server/routes.go`

- [ ] Add update request DTOs for all five resources.
- [ ] Add convert functions from update requests to resource objects.
- [ ] Add store update/delete methods using generated clientset.
- [ ] In biz update methods, get current object first, preserve resourceVersion, labels, and annotations, then update spec.
- [ ] Add PUT and DELETE handlers.
- [ ] Register routes.

### Task 2: Product Aggregation APIs

**Files:**
- Create: `internal/adminapi/handler/dto/topology.go`
- Create: `internal/adminapi/biz/topology.go`
- Create: `internal/adminapi/handler/topology.go`
- Modify: `internal/adminapi/server/routes.go`
- Modify: `internal/adminapi/server/server.go`

- [ ] Add `GET /admin/v1/gateways/:name/topology`.
- [ ] Add `GET /admin/v1/routes/:name/effective-status`.
- [ ] Topology should load gateway, attached routes, referenced backends, and policies targeting the gateway/routes/backends.
- [ ] Effective status should return route, referenced gateway/backend objects, and policies that affect the route.

### Task 3: Basic Admin API Middleware

**Files:**
- Create: `internal/adminapi/server/middleware.go`
- Modify: `internal/adminapi/app/options/options.go`
- Modify: `internal/adminapi/config/config.go`
- Modify: `internal/adminapi/server/server.go`
- Modify: `tools/hack/run-admin-api.sh`

- [ ] Add `--admin-token` option with development default.
- [ ] Add request-id middleware.
- [ ] Add Bearer Token middleware for `/admin/v1/*`.
- [ ] Keep `/healthz` and `/readyz` public.

### Task 4: Verification and Docs

**Files:**
- Modify: `tools/hack/verify-admin-api.sh`
- Modify: `docs/superpowers/learning/admin-api/README.md`

- [ ] Verify unauthenticated admin request returns 401.
- [ ] Verify authenticated create/get/list/update/delete for all five resources.
- [ ] Verify topology and effective-status endpoints.
- [ ] Run `make build-admin-api`, `make verify-admin-api`, `go test ./...`, and `make verify-generated`.
