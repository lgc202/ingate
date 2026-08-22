ARG GO_VERSION=1.26.6
ARG GOPROXY=https://proxy.golang.org,direct

FROM golang:${GO_VERSION}-bookworm AS builder

ARG GOPROXY
ARG PLUGIN
ENV GOPROXY=${GOPROXY}

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY plugins/ ./plugins/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eu; \
    case "${PLUGIN}" in transformer) ;; *) echo "unsupported plugin: ${PLUGIN}" >&2; exit 2 ;; esac; \
    GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath \
        -o /out/plugin.wasm "./plugins/${PLUGIN}"

# Envoy Gateway 和 Istio 均支持这种 Wasm Image Specification compat 布局
FROM scratch

COPY --from=builder /out/plugin.wasm /plugin.wasm
