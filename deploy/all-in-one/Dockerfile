FROM quay.io/coreos/etcd@sha256:8429758911821e36a15e2fe6ae75cfd563a501c97af0b3d119c74921f5fb6f19 AS etcd

FROM higress-registry.cn-hangzhou.cr.aliyuncs.com/higress/gateway@sha256:cf1fa0e100e79890b1de15c74f41a21fabba8f854b9d004d8f5b8ccb48a95c5d AS envoy

FROM redis@sha256:a8f08480e1f88f2647fed492d1178c06abb0d0c1fbf02c682a61e2f483fb3954 AS redis

FROM debian@sha256:0104b334637a5f19aa9c983a91b54c89887c0984081f2068983107a6f6c21eeb

RUN apt-get update \
	&& apt-get install -y --no-install-recommends bash ca-certificates curl \
	&& rm -rf /var/lib/apt/lists/*

COPY _output/all-in-one/bin/ingate-apiserver /usr/local/bin/ingate-apiserver
COPY _output/all-in-one/bin/ingate-admin-api /usr/local/bin/ingate-admin-api
COPY _output/all-in-one/bin/ingate-controller /usr/local/bin/ingate-controller
COPY --from=etcd /usr/local/bin/etcd /usr/local/bin/etcd
COPY --from=envoy /usr/local/bin/envoy /usr/local/bin/envoy
COPY --from=redis /usr/local/bin/redis-server /usr/local/bin/redis-server
COPY --from=redis /usr/local/bin/redis-cli /usr/local/bin/redis-cli

COPY deploy/all-in-one/envoy/bootstrap.yaml /etc/ingate/envoy/bootstrap.yaml
COPY deploy/all-in-one/redis/redis.conf /etc/ingate/redis/redis.conf
COPY deploy/all-in-one/entrypoint.sh /usr/local/bin/ingate-all-in-one
COPY web/console/dist /opt/ingate/console
COPY _output/plugins/ratelimit.wasm /opt/ingate/plugins/ratelimit.wasm
COPY _output/plugins/acl.wasm /opt/ingate/plugins/acl.wasm

RUN chmod +x /usr/local/bin/ingate-all-in-one

ENV INGATE_DATA_DIR=/var/lib/ingate
ENV INGATE_LOG_DIR=/var/log/ingate

EXPOSE 8001 8080
HEALTHCHECK --start-period=60s --interval=5s --timeout=3s --retries=6 \
	CMD redis-cli -h 127.0.0.1 ping >/dev/null \
	&& curl -fsS http://127.0.0.1:18080/readyz \
	&& curl -kfsS https://127.0.0.1:18443/readyz \
	&& curl -fsS http://127.0.0.1:8001/healthz \
	&& curl -fsS http://127.0.0.1:15000/ready
ENTRYPOINT ["/usr/local/bin/ingate-all-in-one"]
