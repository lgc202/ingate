# Ingate Next

Ingate Next is a clean rewrite of Ingate: a declarative control plane for API gateways, AI gateways, and multi-target traffic runtimes.

## Direction

- Keep the core small and explicit.
- Start with resource modeling before data-plane integrations.
- Treat Envoy xDS as the first runtime target, not the only target.
- Keep AI gateway capabilities as first-class domain resources, not special backends.

## First Milestone

Build the smallest useful control-plane skeleton:

```text
Resource model -> compiler IR -> runtime snapshot -> target publisher
```

The first target will be `xds`.

