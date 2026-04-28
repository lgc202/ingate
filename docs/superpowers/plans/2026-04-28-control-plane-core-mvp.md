# Control Plane Core MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first in-memory control-plane pipeline: Resource -> Compiler -> Logical IR -> Debug Target -> RuntimeSnapshot.

**Architecture:** Keep the core small and concrete. Resources describe declared user intent, the compiler resolves references into runtime-neutral IR, and the debug target translates that IR into a structured snapshot for tests.

**Tech Stack:** Go 1.26, standard library only, `go test`, `make test`, `make build`.

---

### Task 1: Resource And IR Types

**Files:**
- Create: `internal/core/resource/types.go`
- Create: `internal/core/ir/gateway.go`

- [ ] Define `Gateway`, `Route`, `Upstream`, and their specs in `resource`.
- [ ] Define `LogicalGateway`, `LogicalListener`, `LogicalRoute`, and `LogicalUpstream` in `ir`.
- [ ] Keep types concrete; do not add interfaces.

### Task 2: Compiler

**Files:**
- Create: `internal/core/compiler/compiler.go`
- Create: `internal/core/compiler/compiler_test.go`

- [ ] Write tests for successful gateway compilation.
- [ ] Write tests for missing gateway references.
- [ ] Write tests for missing upstream references.
- [ ] Implement `Compiler.CompileGateway`.
- [ ] Keep validation focused on real resource references and required names.

### Task 3: Runtime Snapshot And Debug Target

**Files:**
- Create: `internal/core/runtime/snapshot.go`
- Create: `internal/core/target/debug/translator.go`
- Create: `internal/core/target/debug/translator_test.go`

- [ ] Define `RuntimeSnapshot`.
- [ ] Write a test for translating logical IR into a debug snapshot.
- [ ] Implement `debug.Translator`.
- [ ] Preserve listener, route, and upstream data needed to verify the pipeline.

### Task 4: Verification And Commit

**Files:**
- Modify only files created by the previous tasks unless verification exposes a real issue.

- [ ] Run `gofmt -w` on created Go files.
- [ ] Run `make test`.
- [ ] Run `make build`.
- [ ] Commit the implementation as `feat: add control plane core mvp`.
