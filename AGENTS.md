# Ingate Next Agent Instructions

This repository is a clean rewrite of Ingate.

## Project Direction

- Build a declarative control plane for API gateways, AI gateways, and multi-target traffic runtimes.
- Do not copy old Ingate naming or structure by default.
- Prefer precise gateway-domain names. For example, use `Upstream`, not `Backend`.
- Keep Envoy xDS as one target, not the core abstraction.

## Current Scope

The first implementation milestone is:

```text
Resource -> Compiler -> Logical IR -> Target Translator -> RuntimeSnapshot
```

Do not add apiserver, etcd, Envoy xDS, plugins, AI runtime, agent, or Kubernetes operator until the core MVP is implemented and reviewed.

## Development Rules

- Use Go 1.26.
- Keep packages small and purpose-specific.
- Prefer clear concrete types before introducing interfaces.
- Add tests with each behavior change.
- Run `make test` and `make build` before claiming work is complete.

## Git Rules

- Keep commits focused.
- Do not modify the old `../ingate` project from this repository.
- Do not commit generated build outputs or local caches.

