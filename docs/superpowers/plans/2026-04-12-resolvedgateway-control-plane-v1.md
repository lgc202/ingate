# ResolvedGateway Control Plane V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first end-to-end control-plane reconciliation loop that watches gateway resources, produces `ResolvedGateway` objects, and lets `xds-server` consume them as the single publish source.

**Architecture:** Keep `controller-manager` and `xds-server` as separate processes. Use multiple resource-specific trigger controllers to enqueue affected `Gateway` keys, then a single `resolvedgateway` reconciliation controller loads related objects, resolves references, merges policy, writes `ResolvedGateway`, and updates status. `xds-server` watches `ResolvedGateway` only and translates it into publishable runtime state.

**Tech Stack:** Go, Cobra, client-go generated clientset/informers/listers, Kubernetes workqueue patterns, generic apiserver storage, existing code-generation scripts, gRPC/proto stubs already in `pkg/generated/proto`.

---

## File Map

### New API and storage files
- Create: `pkg/apis/gateway/v1alpha1/types_resolvedgateway.go`
- Modify: `pkg/apis/gateway/v1alpha1/resources.go`
- Modify: `pkg/apis/gateway/v1alpha1/register.go`
- Modify: `pkg/apis/gateway/defaulting/defaulting.go`
- Modify: `pkg/apis/gateway/validation/validation.go`
- Modify: `pkg/apis/gateway/validation/status.go`
- Create: `internal/controlplane/apiserver/registry/gateway/resolvedgateway/strategy.go`
- Create: `internal/controlplane/apiserver/registry/gateway/resolvedgateway/storage/storage.go`
- Modify: `internal/controlplane/apiserver/registry/gateway/rest/storage.go`
- Modify: `tools/hack/verify-generated.sh`

### Controller-manager entrypoint and app wiring
- Replace: `cmd/controller-manager/main.go`
- Create: `cmd/controller-manager/app/server.go`
- Create: `cmd/controller-manager/app/run.go`
- Create: `cmd/controller-manager/app/options/options.go`
- Create: `cmd/controller-manager/names/controller_names.go`
- Create: `internal/controlplane/controller/config/config.go`
- Create: `internal/controlplane/controller/health/server.go`

### Shared controller plumbing
- Create: `internal/controlplane/controller/shared/keys.go`
- Create: `internal/controlplane/controller/shared/queue.go`
- Create: `internal/controlplane/controller/shared/events.go`
- Create: `internal/controlplane/controller/index/index.go`
- Create: `internal/controlplane/controller/index/graph.go`
- Create: `internal/controlplane/controller/runtime/context.go`

### Trigger controllers
- Create: `internal/controlplane/controller/gateway/controller.go`
- Create: `internal/controlplane/controller/route/controller.go`
- Create: `internal/controlplane/controller/backend/controller.go`
- Create: `internal/controlplane/controller/certificate/controller.go`
- Create: `internal/controlplane/controller/authpolicy/controller.go`
- Create: `internal/controlplane/controller/trafficpolicy/controller.go`

### ResolvedGateway reconciliation
- Create: `internal/controlplane/controller/resolvedgateway/controller.go`
- Create: `internal/controlplane/controller/resolvedgateway/reconcile.go`
- Create: `internal/controlplane/controller/resolvedgateway/loader.go`
- Create: `internal/controlplane/controller/resolvedgateway/builder.go`
- Create: `internal/controlplane/controller/resolvedgateway/persist.go`
- Create: `internal/controlplane/controller/status/updater.go`

### XDS server bootstrap and consumption layer
- Replace: `cmd/xds-server/main.go`
- Create: `cmd/xds-server/app/server.go`
- Create: `cmd/xds-server/app/options/options.go`
- Create: `internal/controlplane/xds/config/config.go`
- Create: `internal/controlplane/xds/cache/cache.go`
- Create: `internal/controlplane/xds/watch/resolvedgateway.go`
- Create: `internal/controlplane/xds/publish/server.go`
- Create: `internal/controlplane/xds/translate/resolvedgateway.go`

### Verification and docs
- Create: `tools/hack/verify-controller-manager.sh`
- Create: `tools/hack/verify-xds-server.sh`
- Modify: `Makefile`
- Modify: `docs/superpowers/specs/03-control-plane.md`
- Create: `docs/superpowers/learning/controller-manager/01-how-to-run.md`

### Test files
- Create: `pkg/apis/gateway/validation/resolvedgateway_test.go`
- Create: `internal/controlplane/controller/index/index_test.go`
- Create: `internal/controlplane/controller/resolvedgateway/builder_test.go`
- Create: `internal/controlplane/controller/resolvedgateway/reconcile_test.go`
- Create: `internal/controlplane/xds/translate/resolvedgateway_test.go`

---

### Task 1: Add the `ResolvedGateway` API type and apiserver storage

**Files:**
- Create: `pkg/apis/gateway/v1alpha1/types_resolvedgateway.go`
- Modify: `pkg/apis/gateway/v1alpha1/resources.go`
- Modify: `pkg/apis/gateway/v1alpha1/register.go`
- Modify: `pkg/apis/gateway/defaulting/defaulting.go`
- Modify: `pkg/apis/gateway/validation/validation.go`
- Modify: `pkg/apis/gateway/validation/status.go`
- Create: `internal/controlplane/apiserver/registry/gateway/resolvedgateway/strategy.go`
- Create: `internal/controlplane/apiserver/registry/gateway/resolvedgateway/storage/storage.go`
- Modify: `internal/controlplane/apiserver/registry/gateway/rest/storage.go`
- Test: `pkg/apis/gateway/validation/resolvedgateway_test.go`

- [ ] **Step 1: Write the failing validation test for the new resource**

```go
func TestValidateResolvedGatewayRejectsMissingGatewayRef(t *testing.T) {
	rg := &v1alpha1.ResolvedGateway{
		Spec: v1alpha1.ResolvedGatewaySpec{},
	}

	errs := validation.ValidateResolvedGateway(rg)
	if len(errs) == 0 {
		t.Fatalf("expected validation error for missing gatewayRef")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails for the right reason**

Run: `go test ./pkg/apis/gateway/validation -run TestValidateResolvedGatewayRejectsMissingGatewayRef -v`
Expected: FAIL because `ResolvedGateway` type and validator do not exist yet.

- [ ] **Step 3: Add the minimal API type**

Add a first-cut shape in `types_resolvedgateway.go`:

```go
type ResolvedGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResolvedGatewaySpec   `json:"spec,omitempty"`
	Status ResolvedGatewayStatus `json:"status,omitempty"`
}

type ResolvedGatewaySpec struct {
	GatewayRef LocalObjectReference `json:"gatewayRef"`
	Version    string               `json:"version,omitempty"`
	Listeners  []ResolvedListener   `json:"listeners,omitempty"`
	Routes     []ResolvedRoute      `json:"routes,omitempty"`
	Backends   []ResolvedBackend    `json:"backends,omitempty"`
	Extensions []ResolvedExtension  `json:"extensions,omitempty"`
}
```

- [ ] **Step 4: Register the type and storage endpoints**

Implement list type, register kind/resource constants, and wire storage into `gateway/rest/storage.go` so `/apis/gateway.ingate.io/v1alpha1/resolvedgateways` exists.

- [ ] **Step 5: Add validation and status validation**

Minimum v1 checks:
- `spec.gatewayRef.name` required
- listener names unique
- route names unique within `spec.routes`
- backend names unique within `spec.backends`
- `status.conditions[*].observedGeneration` uses existing status validation helpers

- [ ] **Step 6: Generate helper code**

Run: `make generate`
Expected: updated deepcopy, defaults, openapi, clientset, informer, lister generated code for `ResolvedGateway`.

- [ ] **Step 7: Re-run validation test and generation verification**

Run: `go test ./pkg/apis/gateway/validation -run TestValidateResolvedGatewayRejectsMissingGatewayRef -v && make verify-generated`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/apis/gateway internal/controlplane/apiserver tools/hack/verify-generated.sh
 git commit -m "feat: add resolvedgateway api and storage"
```

### Task 2: Bootstrap a real `controller-manager` binary in the OneX style

**Files:**
- Replace: `cmd/controller-manager/main.go`
- Create: `cmd/controller-manager/app/server.go`
- Create: `cmd/controller-manager/app/run.go`
- Create: `cmd/controller-manager/app/options/options.go`
- Create: `cmd/controller-manager/names/controller_names.go`
- Create: `internal/controlplane/controller/config/config.go`
- Create: `internal/controlplane/controller/health/server.go`

- [ ] **Step 1: Write the failing startup smoke test**

```bash
go test ./cmd/controller-manager/app -run TestNewControllerManagerCommand -v
```

Create the test in `cmd/controller-manager/app/server_test.go` asserting the command exists and rejects positional args.

- [ ] **Step 2: Run the test to verify failure**

Expected: FAIL because `app` package does not exist.

- [ ] **Step 3: Add a thin `main.go` and Cobra command**

Follow the existing `cmd/apiserver` / `internal/adminapi` pattern:

```go
func main() {
	command := app.NewControllerManagerCommand()
	code := cli.Run(command)
	os.Exit(code)
}
```

- [ ] **Step 4: Implement options/config plumbing**

Support at least:
- apiserver address / kubeconfig
- leader election enable/name/namespace
- metrics bind address
- healthz bind address
- watch namespace (empty means all namespaces)
- worker count

- [ ] **Step 5: Add healthz/readyz endpoints**

Expose a small HTTP server for health probes so later verify scripts can test process readiness without reading logs.

- [ ] **Step 6: Re-run the smoke test**

Run: `go test ./cmd/controller-manager/app -run TestNewControllerManagerCommand -v`
Expected: PASS.

- [ ] **Step 7: Build the binary**

Run: `go build ./cmd/controller-manager`
Expected: build succeeds.

- [ ] **Step 8: Commit**

```bash
git add cmd/controller-manager internal/controlplane/controller/config internal/controlplane/controller/health
 git commit -m "feat: bootstrap controller manager binary"
```

### Task 3: Add shared informer, queue, and affected-gateway index plumbing

**Files:**
- Create: `internal/controlplane/controller/shared/keys.go`
- Create: `internal/controlplane/controller/shared/queue.go`
- Create: `internal/controlplane/controller/shared/events.go`
- Create: `internal/controlplane/controller/index/index.go`
- Create: `internal/controlplane/controller/index/graph.go`
- Create: `internal/controlplane/controller/runtime/context.go`
- Test: `internal/controlplane/controller/index/index_test.go`

- [ ] **Step 1: Write the failing index test**

```go
func TestAffectedGatewaysForBackendReturnsGatewayNames(t *testing.T) {
	idx := NewIndex()
	idx.TrackRoute("default", "catalog-route", []string{"public-edge"}, []string{"catalog-backend"})

	gateways := idx.AffectedGatewaysForBackend("default", "catalog-backend")
	if diff := cmp.Diff([]string{"default/public-edge"}, gateways); diff != "" {
		t.Fatalf("unexpected gateways (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/controlplane/controller/index -run TestAffectedGatewaysForBackendReturnsGatewayNames -v`
Expected: FAIL because the index package does not exist.

- [ ] **Step 3: Implement the minimal index graph**

Track these edges:
- route -> gateways
- route -> backends
- gateway -> certificates
- policy target -> gateways/routes/backends

Use namespace/name composite keys consistently.

- [ ] **Step 4: Add shared queue helpers**

Define helpers such as:

```go
type GatewayKey struct {
	Namespace string
	Name      string
}
```

and a rate-limited queue wrapper so every trigger controller enqueues the same key shape.

- [ ] **Step 5: Re-run the index test**

Run: `go test ./internal/controlplane/controller/index -run TestAffectedGatewaysForBackendReturnsGatewayNames -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/controlplane/controller/shared internal/controlplane/controller/index internal/controlplane/controller/runtime
 git commit -m "feat: add controller index and shared queue plumbing"
```

### Task 4: Add trigger controllers for gateway, route, backend, certificate, authpolicy, and trafficpolicy

**Files:**
- Create: `internal/controlplane/controller/gateway/controller.go`
- Create: `internal/controlplane/controller/route/controller.go`
- Create: `internal/controlplane/controller/backend/controller.go`
- Create: `internal/controlplane/controller/certificate/controller.go`
- Create: `internal/controlplane/controller/authpolicy/controller.go`
- Create: `internal/controlplane/controller/trafficpolicy/controller.go`
- Modify: `cmd/controller-manager/app/run.go`
- Modify: `cmd/controller-manager/names/controller_names.go`

- [ ] **Step 1: Write the failing enqueue behavior test for one trigger controller**

Create `internal/controlplane/controller/route/controller_test.go`:

```go
func TestRouteControllerEnqueuesAffectedGateway(t *testing.T) {
	// build fake index + fake queue
	// emit a route add/update event
	// assert gateway key was enqueued
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/controlplane/controller/route -run TestRouteControllerEnqueuesAffectedGateway -v`
Expected: FAIL because controller package does not exist.

- [ ] **Step 3: Implement one trigger controller fully**

Implement route watcher first; use generated informers and index lookups; keep logic tiny:
- on add/update/delete
- recompute index entries for that route
- enqueue affected gateways

- [ ] **Step 4: Clone the same pattern for the other five controllers**

Each controller should only do:
- informer handlers
- index maintenance for its resource
- enqueue affected gateway keys

- [ ] **Step 5: Register controllers in the manager run path**

Use a stable names file like OneX so logs, flags, and later metrics use canonical controller identifiers.

- [ ] **Step 6: Re-run the route controller test**

Run: `go test ./internal/controlplane/controller/route -run TestRouteControllerEnqueuesAffectedGateway -v`
Expected: PASS.

- [ ] **Step 7: Build the controller-manager binary**

Run: `go build ./cmd/controller-manager`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/controlplane/controller/gateway internal/controlplane/controller/route internal/controlplane/controller/backend internal/controlplane/controller/certificate internal/controlplane/controller/authpolicy internal/controlplane/controller/trafficpolicy cmd/controller-manager
 git commit -m "feat: add trigger controllers for resolvedgateway enqueue"
```

### Task 5: Implement `resolvedgateway` loading and builder logic

**Files:**
- Create: `internal/controlplane/controller/resolvedgateway/loader.go`
- Create: `internal/controlplane/controller/resolvedgateway/builder.go`
- Test: `internal/controlplane/controller/resolvedgateway/builder_test.go`

- [ ] **Step 1: Write the failing builder test**

```go
func TestBuildResolvedGatewayMergesRouteBackendAndPolicies(t *testing.T) {
	bundle := ResourceBundle{
		Gateway: gateway("public-edge"),
		Routes: []gatewayv1alpha1.Route{routeWithBackendAndRewrite(...)},
		Backends: []gatewayv1alpha1.Backend{staticBackend(...)},
		AuthPolicies: []policyv1alpha1.AuthPolicy{jwtPolicy(...)},
		TrafficPolicies: []policyv1alpha1.TrafficPolicy{timeoutPolicy(...)},
	}

	rg, err := Build(bundle)
	if err != nil { t.Fatalf("Build() error = %v", err) }
	if len(rg.Spec.Routes) != 1 { t.Fatalf("expected 1 resolved route") }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/controlplane/controller/resolvedgateway -run TestBuildResolvedGatewayMergesRouteBackendAndPolicies -v`
Expected: FAIL because builder/loader code does not exist.

- [ ] **Step 3: Implement the minimal loader**

Load only same-namespace objects in v1:
- the target `Gateway`
- routes attached to that gateway
- route-referenced backends
- listener certificates
- auth/traffic policies targeting gateway/route/backend

- [ ] **Step 4: Implement the minimal builder**

Produce a `ResolvedGateway` with:
- `spec.gatewayRef`
- listener protocol/port/hostnames/tls refs
- route matches, rewrite, header modifiers, auth summary, traffic summary
- backend endpoint lists and load-balance policy
- empty `extensions`

- [ ] **Step 5: Keep xDS details out of the builder**

Do not emit Envoy resources here. The output should remain a product-facing resolved model.

- [ ] **Step 6: Re-run the builder test**

Run: `go test ./internal/controlplane/controller/resolvedgateway -run TestBuildResolvedGatewayMergesRouteBackendAndPolicies -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/controlplane/controller/resolvedgateway/loader.go internal/controlplane/controller/resolvedgateway/builder.go internal/controlplane/controller/resolvedgateway/builder_test.go
 git commit -m "feat: build resolvedgateway from gateway resources"
```

### Task 6: Implement the `resolvedgateway` reconciliation loop and status updater

**Files:**
- Create: `internal/controlplane/controller/resolvedgateway/controller.go`
- Create: `internal/controlplane/controller/resolvedgateway/reconcile.go`
- Create: `internal/controlplane/controller/resolvedgateway/persist.go`
- Create: `internal/controlplane/controller/status/updater.go`
- Test: `internal/controlplane/controller/resolvedgateway/reconcile_test.go`

- [ ] **Step 1: Write the failing reconcile test**

```go
func TestReconcileCreatesOrUpdatesResolvedGateway(t *testing.T) {
	// fake clientset with gateway/route/backend/policies
	// run one reconcile for default/public-edge
	// assert ResolvedGateway exists and status fields are set
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/controlplane/controller/resolvedgateway -run TestReconcileCreatesOrUpdatesResolvedGateway -v`
Expected: FAIL because reconcile path is not implemented.

- [ ] **Step 3: Implement one-shot reconcile flow**

Minimal flow:
- dequeue one gateway key
- load bundle
- build `ResolvedGateway`
- create/update the object
- update `Accepted` and `Resolved` conditions on original resources and the resolved resource

- [ ] **Step 4: Handle failure path explicitly**

On missing refs or merge conflicts:
- persist failure conditions
- do not panic
- requeue using rate-limited queue rules only where retry makes sense

- [ ] **Step 5: Re-run the reconcile test**

Run: `go test ./internal/controlplane/controller/resolvedgateway -run TestReconcileCreatesOrUpdatesResolvedGateway -v`
Expected: PASS.

- [ ] **Step 6: Run all controller package tests**

Run: `go test ./internal/controlplane/controller/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/controlplane/controller/resolvedgateway internal/controlplane/controller/status
 git commit -m "feat: reconcile gateways into resolvedgateway resources"
```

### Task 7: Build the first real `xds-server` that watches `ResolvedGateway`

**Files:**
- Replace: `cmd/xds-server/main.go`
- Create: `cmd/xds-server/app/server.go`
- Create: `cmd/xds-server/app/options/options.go`
- Create: `internal/controlplane/xds/config/config.go`
- Create: `internal/controlplane/xds/cache/cache.go`
- Create: `internal/controlplane/xds/watch/resolvedgateway.go`
- Create: `internal/controlplane/xds/publish/server.go`
- Create: `internal/controlplane/xds/translate/resolvedgateway.go`
- Test: `internal/controlplane/xds/translate/resolvedgateway_test.go`

- [ ] **Step 1: Write the failing translation test**

```go
func TestTranslateResolvedGatewayProducesRuntimeArtifacts(t *testing.T) {
	rg := resolvedGatewayFixture()
	out, err := translate.FromResolvedGateway(rg)
	if err != nil { t.Fatalf("translate error = %v", err) }
	if len(out.Listeners) == 0 { t.Fatalf("expected listeners") }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/controlplane/xds/translate -run TestTranslateResolvedGatewayProducesRuntimeArtifacts -v`
Expected: FAIL because translation layer does not exist.

- [ ] **Step 3: Add a real xds-server command skeleton**

Match the style used for `apiserver` and planned `controller-manager` command layout.

- [ ] **Step 4: Implement the resolvedgateway watch cache**

Use generated informers/listers for `ResolvedGateway`; keep a local cache keyed by namespace/name and publish version.

- [ ] **Step 5: Implement minimal translation output**

V1 translation can be intentionally narrow:
- listeners
- routes
- backend clusters/endpoints
- auth/traffic placeholders enough for later xDS wiring

The translator output should be an internal runtime object, not direct protobuf fragments.

- [ ] **Step 6: Re-run the translation test**

Run: `go test ./internal/controlplane/xds/translate -run TestTranslateResolvedGatewayProducesRuntimeArtifacts -v`
Expected: PASS.

- [ ] **Step 7: Build the xds-server binary**

Run: `go build ./cmd/xds-server`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add cmd/xds-server internal/controlplane/xds
 git commit -m "feat: watch resolvedgateway in xds server"
```

### Task 8: Add end-to-end verification scripts and docs for local bring-up

**Files:**
- Create: `tools/hack/verify-controller-manager.sh`
- Create: `tools/hack/verify-xds-server.sh`
- Modify: `Makefile`
- Create: `docs/superpowers/learning/controller-manager/01-how-to-run.md`

- [ ] **Step 1: Write the failing verification shell flow first**

The script should assume:
- local apiserver already available or started by the script
- controller-manager binary path passed in via env
- xds-server binary path passed in via env

- [ ] **Step 2: Run the script and observe failure**

Run: `CONTROLLER_MANAGER_BIN=./_output/bin/controller-manager ./tools/hack/verify-controller-manager.sh`
Expected: FAIL because the controller-manager binary and behavior are not yet fully wired.

- [ ] **Step 3: Implement minimal verification checks**

Checks should cover:
- controller-manager starts and reports healthz/readyz
- creating `Gateway/Route/Backend/Certificate/AuthPolicy/TrafficPolicy` yields a `ResolvedGateway`
- `ResolvedGateway.status.Accepted/Resolved` become true
- xds-server starts, watches `ResolvedGateway`, and marks `Programmed`

- [ ] **Step 4: Add Makefile targets**

Add at least:
- `build-controller-manager`
- `build-xds-server`
- `verify-controller-manager`
- `verify-xds-server`

- [ ] **Step 5: Write the operator-facing runbook**

Document:
- required local dependencies
- start order
- how to inspect `ResolvedGateway`
- how to read controller and xds status

- [ ] **Step 6: Run the full verification sequence**

Run:
- `make build-controller-manager`
- `make build-xds-server`
- `make verify-controller-manager`
- `make verify-xds-server`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add Makefile tools/hack docs/superpowers/learning/controller-manager
 git commit -m "docs: add controller manager runbook and verification"
```

### Task 9: Final integration verification

**Files:**
- Modify as needed from previous tasks only; no new design changes in this task.

- [ ] **Step 1: Run generated-file verification**

Run: `make verify-generated`
Expected: PASS.

- [ ] **Step 2: Run controller-related unit tests**

Run: `go test ./internal/controlplane/controller/... ./internal/controlplane/xds/... ./pkg/apis/gateway/...`
Expected: PASS.

- [ ] **Step 3: Run existing API verification that could regress**

Run: `make verify-apiserver && make verify-admin-api`
Expected: PASS.

- [ ] **Step 4: Build all control-plane binaries**

Run: `go build ./cmd/apiserver ./cmd/admin-api ./cmd/controller-manager ./cmd/xds-server`
Expected: PASS.

- [ ] **Step 5: Review docs and examples for naming consistency**

Check that the codebase consistently uses:
- `ResolvedGateway`
- `resolvedgateway controller`
- `Accepted / Resolved / Programmed`

- [ ] **Step 6: Commit the integration pass**

```bash
git add .
 git commit -m "feat: wire resolvedgateway control plane v1"
```

## Notes for the Implementer

- Follow the existing `cmd/apiserver` and `internal/adminapi` command layout before inventing new patterns.
- Use generated clientset/informer/lister packages instead of ad-hoc polling.
- Keep `ResolvedGateway` free of Envoy/xDS protobuf details.
- Keep same-namespace semantics in v1 unless the task explicitly extends that boundary.
- If a task reveals the API shape is still too large, shrink the `ResolvedGateway.spec` surface rather than adding more abstraction.
- Do not skip the verification scripts; they are part of the deliverable.
- Prefer small focused files. If a file exceeds roughly 250-300 lines while still growing, split it.
