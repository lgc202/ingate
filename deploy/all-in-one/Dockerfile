FROM quay.io/coreos/etcd:v3.5.11 AS etcd

FROM envoyproxy/envoy:v1.32-latest AS envoy

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends bash ca-certificates curl \
	&& rm -rf /var/lib/apt/lists/*

COPY _output/all-in-one/bin/ingate-apiserver /usr/local/bin/ingate-apiserver
COPY _output/all-in-one/bin/ingate-admin-api /usr/local/bin/ingate-admin-api
COPY _output/all-in-one/bin/ingate-controller /usr/local/bin/ingate-controller
COPY _output/all-in-one/bin/ingate-xds /usr/local/bin/ingate-xds
COPY _output/all-in-one/bin/ingate-httpbin /usr/local/bin/ingate-httpbin
COPY _output/all-in-one/bin/ingate-dataplane /usr/local/bin/ingate-dataplane
COPY --from=etcd /usr/local/bin/etcd /usr/local/bin/etcd
COPY --from=envoy /usr/local/bin/envoy /usr/local/bin/envoy

COPY deploy/all-in-one/default.env /etc/ingate/default.env
COPY deploy/all-in-one/envoy/bootstrap.yaml /etc/ingate/envoy/bootstrap.yaml
COPY deploy/all-in-one/entrypoint.sh /usr/local/bin/ingate-all-in-one
COPY web/console/dist /opt/ingate/console
COPY _output/plugins/ratelimit.wasm /opt/ingate/plugins/ratelimit.wasm
COPY _output/plugins/acl.wasm /opt/ingate/plugins/acl.wasm

RUN chmod +x /usr/local/bin/ingate-all-in-one

ENV INGATE_DATA_DIR=/var/lib/ingate
ENV INGATE_LOG_DIR=/var/log/ingate

EXPOSE 8001 8080 8443
ENTRYPOINT ["/usr/local/bin/ingate-all-in-one"]
