ARG GO_VERSION=1.26.0

FROM golang:${GO_VERSION}-bookworm AS plugin-builder

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
    && GOOS=wasip1 GOARCH=wasm go build -trimpath -buildmode=c-shared -o /out/acl.wasm ./plugins/acl \
    && GOOS=wasip1 GOARCH=wasm go build -trimpath -buildmode=c-shared -o /out/ai-proxy.wasm ./plugins/aiproxy

FROM higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/gateway@sha256:cf1fa0e100e79890b1de15c74f41a21fabba8f854b9d004d8f5b8ccb48a95c5d AS higress-envoy

FROM debian@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=higress-envoy /usr/local/bin/envoy /opt/ingate/envoy/bin/envoy
COPY --from=plugin-builder /out/ /opt/ingate/plugins/
COPY deploy/docker/envoy-bootstrap.yaml /opt/ingate/envoy/configs/bootstrap.yaml

STOPSIGNAL SIGTERM

ENTRYPOINT ["/opt/ingate/envoy/bin/envoy"]
CMD ["-c", "/opt/ingate/envoy/configs/bootstrap.yaml"]
