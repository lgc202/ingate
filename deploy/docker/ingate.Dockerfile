ARG GO_VERSION=1.26.6
ARG NODE_VERSION=24
ARG GOPROXY=https://proxy.golang.org,direct

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS service-builder

ARG GOPROXY
ARG TARGETOS
ARG TARGETARCH
ARG COMPONENT
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
	--mount=type=cache,target=/tmp/go-build \
    set -eu; \
	case "${COMPONENT}" in \
		admin-api|ai-extproc|als|analytics|apiserver|assistant|authz|console|controller) ;; \
		*) echo "unsupported component: ${COMPONENT}" >&2; exit 2 ;; \
	esac; \
	export CGO_ENABLED=0 GOTMPDIR=/tmp/go-build; \
	ldflags="-X github.com/lgc202/ingate/internal/pkg/version.gitVersion=${GIT_VERSION} -X github.com/lgc202/ingate/internal/pkg/version.gitCommit=${GIT_COMMIT} -X github.com/lgc202/ingate/internal/pkg/version.gitTreeState=${GIT_TREE_STATE} -X github.com/lgc202/ingate/internal/pkg/version.buildDate=${BUILD_DATE}"; \
	GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -p=4 -trimpath -ldflags "${ldflags}" -o /out/ingate-service "./cmd/ingate-${COMPONENT}"

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-bookworm-slim AS console-builder

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

COPY --from=service-builder /out/ingate-service /opt/ingate/apiserver/bin/ingate-apiserver

ENTRYPOINT ["/opt/ingate/apiserver/bin/ingate-apiserver"]
CMD ["--config", "/opt/ingate/apiserver/configs/config.yaml"]

FROM service-runtime AS controller

COPY --from=service-builder /out/ingate-service /opt/ingate/controller/bin/ingate-controller

ENTRYPOINT ["/opt/ingate/controller/bin/ingate-controller"]
CMD ["--config", "/opt/ingate/controller/configs/config.yaml"]

FROM service-runtime AS admin-api

COPY --from=service-builder /out/ingate-service /opt/ingate/admin-api/bin/ingate-admin-api

ENTRYPOINT ["/opt/ingate/admin-api/bin/ingate-admin-api"]
CMD ["--config", "/opt/ingate/admin-api/configs/config.yaml"]

FROM service-runtime AS assistant

COPY --from=service-builder /out/ingate-service /opt/ingate/assistant/bin/ingate-assistant

ENTRYPOINT ["/opt/ingate/assistant/bin/ingate-assistant"]
CMD ["--config", "/opt/ingate/assistant/configs/config.yaml"]

FROM service-runtime AS als

COPY --from=service-builder /out/ingate-service /opt/ingate/als/bin/ingate-als

ENTRYPOINT ["/opt/ingate/als/bin/ingate-als"]
CMD ["--config", "/opt/ingate/als/configs/config.yaml"]

FROM service-runtime AS analytics

COPY --from=service-builder /out/ingate-service /opt/ingate/analytics/bin/ingate-analytics

ENTRYPOINT ["/opt/ingate/analytics/bin/ingate-analytics"]
CMD ["--config", "/opt/ingate/analytics/configs/config.yaml"]

FROM service-runtime AS ai-extproc

COPY --from=service-builder /out/ingate-service /opt/ingate/ai-extproc/bin/ingate-ai-extproc

ENTRYPOINT ["/opt/ingate/ai-extproc/bin/ingate-ai-extproc"]
CMD ["--config", "/opt/ingate/ai-extproc/configs/config.yaml"]

FROM service-runtime AS authz

COPY --from=service-builder /out/ingate-service /opt/ingate/authz/bin/ingate-authz

ENTRYPOINT ["/opt/ingate/authz/bin/ingate-authz"]
CMD ["--config", "/opt/ingate/authz/configs/config.yaml"]

FROM service-runtime AS console

COPY --from=service-builder /out/ingate-service /opt/ingate/console/bin/ingate-console
COPY --from=console-builder /src/dist /opt/ingate/console/web

ENTRYPOINT ["/opt/ingate/console/bin/ingate-console"]
CMD ["--config", "/opt/ingate/console/configs/config.yaml"]
