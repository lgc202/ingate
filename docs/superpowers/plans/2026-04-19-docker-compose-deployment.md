# Docker Compose Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a first-pass Docker Compose deployment bundle that starts etcd, ingate control-plane components, Envoy, and a sample backend so a user can proxy real HTTP traffic with one command.

**Architecture:** Add container images for `ingate-apiserver`, `ingate-controller-manager`, `ingate-xds-server`, and `ingate-admin-api`, then wire them together under `deploy/compose/compose.yaml`. Use a dedicated init container/script to wait for readiness and seed `Gateway`, `Route`, and `Backend` resources into the running stack, while Envoy consumes xDS from `xds-server` through a fixed bootstrap file.

**Tech Stack:** Docker, Docker Compose, Envoy v1.32.4, etcd, shell scripts, existing Go binaries, admin API / apiserver HTTP endpoints

---

### Task 1: Define deployment file layout

**Files:**
- Create: `deploy/compose/README.md`
- Create: `deploy/compose/.env.example`
- Create: `deploy/compose/compose.yaml`
- Create: `build/package/apiserver.Dockerfile`
- Create: `build/package/controller-manager.Dockerfile`
- Create: `build/package/xds-server.Dockerfile`
- Create: `build/package/admin-api.Dockerfile`
- Create: `build/package/sample-backend.Dockerfile`

- [ ] **Step 1: Write the failing deployment validation expectation**

Document the intended validation command in the README:

```bash
docker compose -f deploy/compose/compose.yaml config
```

Expected initial failure before files exist: missing compose file / missing build files.

- [ ] **Step 2: Add the file layout and environment contract**

Document exact services, ports, and required env vars:
- `etcd`
- `apiserver`
- `controller-manager`
- `xds-server`
- `admin-api`
- `envoy`
- `sample-backend`
- `init-control-plane`

- [ ] **Step 3: Add base compose file with service skeletons**

Create the compose file with named services, networks, healthchecks, bind mounts, and placeholders for build contexts/commands.

- [ ] **Step 4: Validate compose syntax**

Run: `docker compose -f deploy/compose/compose.yaml config`
Expected: valid rendered compose output.

### Task 2: Containerize each ingate component

**Files:**
- Create: `build/package/apiserver.Dockerfile`
- Create: `build/package/controller-manager.Dockerfile`
- Create: `build/package/xds-server.Dockerfile`
- Create: `build/package/admin-api.Dockerfile`
- Modify: `Makefile`
- Modify: `tools/hack/build.sh` (only if image build helpers are needed)

- [ ] **Step 1: Write the failing image build command list**

Document exact image build commands:

```bash
docker build -f build/package/apiserver.Dockerfile -t ingate/apiserver:dev .
docker build -f build/package/controller-manager.Dockerfile -t ingate/controller-manager:dev .
docker build -f build/package/xds-server.Dockerfile -t ingate/xds-server:dev .
docker build -f build/package/admin-api.Dockerfile -t ingate/admin-api:dev .
```

Expected initial failure before Dockerfiles exist: file not found.

- [ ] **Step 2: Implement minimal runtime images**

Use multi-stage builds that compile the target Go binary and copy only the resulting executable into a slim runtime image.

- [ ] **Step 3: Add compose build references**

Wire the compose services to the matching Dockerfiles, tags, commands, and runtime args.

- [ ] **Step 4: Validate image builds**

Run the four `docker build ...` commands above.
Expected: all images build successfully.

### Task 3: Add Envoy bootstrap and sample backend

**Files:**
- Create: `deploy/compose/envoy/envoy.yaml`
- Create: `build/package/sample-backend.Dockerfile`
- Create: `deploy/compose/sample-backend/server.py` or `server.go`

- [ ] **Step 1: Write the failing Envoy bootstrap expectation**

Validation target:

```bash
docker compose -f deploy/compose/compose.yaml config
```

Expected initial failure before bootstrap exists: missing mounted Envoy config file.

- [ ] **Step 2: Add a fixed bootstrap for compose networking**

Use `xds-server:19090` as ADS endpoint and expose Envoy admin/proxy ports to the host.

- [ ] **Step 3: Add sample backend image**

Provide a tiny HTTP service that returns a stable body for `/orders`.

- [ ] **Step 4: Validate that compose renders the bootstrap mount and backend service**

Run: `docker compose -f deploy/compose/compose.yaml config`
Expected: rendered service definitions include `envoy` volume mount and `sample-backend`.

### Task 4: Seed control-plane resources automatically

**Files:**
- Create: `deploy/compose/init/seed.sh`
- Create: `deploy/compose/init/resources.jsonnet` or `deploy/compose/init/resources.sh`
- Modify: `deploy/compose/compose.yaml`

- [ ] **Step 1: Write the failing seeding contract**

Document the expected runtime outcome:
- `Gateway` named `compose-gateway`
- `Route` named `compose-orders-route`
- `Backend` named `compose-backend`

Expected initial failure before init script exists: stack comes up but no route is programmable.

- [ ] **Step 2: Implement readiness-aware seeding**

The init service must:
- wait for apiserver HTTPS health
- wait for controller-manager health
- wait for xds-server health
- create `Gateway/Route/Backend` via the apiserver admin token

- [ ] **Step 3: Parameterize backend address**

Support both:
- internal sample backend (`sample-backend:8080`)
- external real business backend via env vars, without changing compose topology

- [ ] **Step 4: Validate seeding command shape**

Run: `docker compose -f deploy/compose/compose.yaml config`
Expected: `init-control-plane` has env vars, dependencies, and mounted init assets.

### Task 5: Add user-facing make targets and docs

**Files:**
- Modify: `Makefile`
- Create: `deploy/compose/README.md`
- Create: `deploy/compose/.env.example`
- Modify: `docs/superpowers/learning/controller-manager/01-how-to-run.md`

- [ ] **Step 1: Write the failing UX target expectation**

Planned commands:

```bash
make compose-up
make compose-down
make compose-logs
```

Expected initial failure before Makefile wiring: target not found.

- [ ] **Step 2: Add make targets for compose lifecycle**

Expose:
- `compose-build`
- `compose-up`
- `compose-down`
- `compose-logs`
- `compose-ps`

- [ ] **Step 3: Document basic and real-backend usage**

README must cover:
- prerequisites
- one-command startup
- sample curl
- how to point to a real backend address
- how to tear down

- [ ] **Step 4: Validate make target help text**

Run: `make help`
Expected: compose targets appear in the target list.

### Task 6: End-to-end compose verification

**Files:**
- Create: `tools/hack/verify-compose.sh`
- Modify: `Makefile`
- Test: `deploy/compose/compose.yaml`

- [ ] **Step 1: Write the failing verification flow**

Expected verification command:

```bash
make verify-compose
```

Expected initial failure before script exists: target not found.

- [ ] **Step 2: Implement compose verification script**

The script should:
- build required images
- run `docker compose up -d`
- wait for service health
- curl Envoy with `Host: api.example.com`
- assert HTTP `200`
- optionally check admin-api health
- tear down the stack on exit

- [ ] **Step 3: Run compose verification**

Run: `make verify-compose`
Expected: successful end-to-end proxy response through Envoy.

- [ ] **Step 4: Record final manual smoke command**

Document:

```bash
docker compose -f deploy/compose/compose.yaml up --build
curl -H 'Host: api.example.com' http://127.0.0.1:10080/orders
```

Expected: backend response body from the sample or configured real backend.
