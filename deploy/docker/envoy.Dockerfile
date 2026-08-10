ARG GO_VERSION=1.26.0
ARG GOPROXY=https://proxy.golang.org,direct

FROM golang:${GO_VERSION}-bookworm AS plugin-builder

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY pkg ./pkg
COPY plugins ./plugins
COPY internal/pkg/httpheader ./internal/pkg/httpheader

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out \
    && GOOS=wasip1 GOARCH=wasm go build -trimpath -buildmode=c-shared -o /out/ratelimit.wasm ./plugins/ratelimit \
    && GOOS=wasip1 GOARCH=wasm go build -trimpath -buildmode=c-shared -o /out/tokenquota.wasm ./plugins/tokenquota \
    && GOOS=wasip1 GOARCH=wasm go build -trimpath -buildmode=c-shared -o /out/iprestriction.wasm ./plugins/iprestriction

FROM higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/gateway:v2.2.3 AS higress-envoy

FROM debian:12.14-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=higress-envoy /usr/local/bin/envoy /opt/ingate/envoy/bin/envoy
COPY --from=plugin-builder /out/ /opt/ingate/plugins/
COPY deploy/docker/envoy-bootstrap.yaml /opt/ingate/envoy/configs/bootstrap.yaml

STOPSIGNAL SIGTERM

ENTRYPOINT ["/opt/ingate/envoy/bin/envoy"]
CMD ["-c", "/opt/ingate/envoy/configs/bootstrap.yaml"]
