ARG GO_VERSION=1.26.0
ARG NODE_VERSION=24
ARG GOPROXY=https://proxy.golang.org,direct

FROM golang:${GO_VERSION}-bookworm AS service-builder

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG GIT_VERSION=v0.0.0-unknown
ARG GIT_COMMIT=unknown
ARG GIT_TREE_STATE=unknown
ARG BUILD_DATE=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-X github.com/lgc202/go-kit/version.gitVersion=${GIT_VERSION} -X github.com/lgc202/go-kit/version.gitCommit=${GIT_COMMIT} -X github.com/lgc202/go-kit/version.gitTreeState=${GIT_TREE_STATE} -X github.com/lgc202/go-kit/version.buildDate=${BUILD_DATE}" \
      -o /out/ ./cmd/...

FROM node:${NODE_VERSION}-bookworm-slim AS console-builder

WORKDIR /src

COPY web/console/package.json web/console/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

COPY web/console/ ./
RUN npm run build

FROM debian:12.14-slim AS service-runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

STOPSIGNAL SIGTERM

FROM service-runtime AS apiserver

COPY --from=service-builder /out/ingate-apiserver /opt/ingate/apiserver/bin/ingate-apiserver

ENTRYPOINT ["/opt/ingate/apiserver/bin/ingate-apiserver"]
CMD ["--config", "/opt/ingate/apiserver/configs/config.yaml"]

FROM service-runtime AS controller

COPY --from=service-builder /out/ingate-controller /opt/ingate/controller/bin/ingate-controller

ENTRYPOINT ["/opt/ingate/controller/bin/ingate-controller"]
CMD ["--config", "/opt/ingate/controller/configs/config.yaml"]

FROM service-runtime AS admin-api

COPY --from=service-builder /out/ingate-admin-api /opt/ingate/admin-api/bin/ingate-admin-api

ENTRYPOINT ["/opt/ingate/admin-api/bin/ingate-admin-api"]
CMD ["--config", "/opt/ingate/admin-api/configs/config.yaml"]

FROM service-runtime AS console

COPY --from=service-builder /out/ingate-console /opt/ingate/console/bin/ingate-console
COPY --from=console-builder /src/dist /opt/ingate/console/web

ENTRYPOINT ["/opt/ingate/console/bin/ingate-console"]
CMD ["--config", "/opt/ingate/console/configs/config.yaml"]
