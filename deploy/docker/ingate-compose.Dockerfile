FROM alpine:3.22.1 AS service-runtime

RUN apk add --no-cache ca-certificates curl

STOPSIGNAL SIGTERM

FROM service-runtime AS apiserver

COPY _output/docker/bin/ingate-apiserver /opt/ingate/apiserver/bin/ingate-apiserver

ENTRYPOINT ["/opt/ingate/apiserver/bin/ingate-apiserver"]
CMD ["--config", "/opt/ingate/apiserver/configs/config.yaml"]

FROM service-runtime AS controller

COPY _output/docker/bin/ingate-controller /opt/ingate/controller/bin/ingate-controller

ENTRYPOINT ["/opt/ingate/controller/bin/ingate-controller"]
CMD ["--config", "/opt/ingate/controller/configs/config.yaml"]

FROM service-runtime AS admin-api

COPY _output/docker/bin/ingate-admin-api /opt/ingate/admin-api/bin/ingate-admin-api

ENTRYPOINT ["/opt/ingate/admin-api/bin/ingate-admin-api"]
CMD ["--config", "/opt/ingate/admin-api/configs/config.yaml"]

FROM service-runtime AS als

COPY _output/docker/bin/ingate-als /opt/ingate/als/bin/ingate-als

ENTRYPOINT ["/opt/ingate/als/bin/ingate-als"]
CMD ["--config", "/opt/ingate/als/configs/config.yaml"]

FROM service-runtime AS analytics

COPY _output/docker/bin/ingate-analytics /opt/ingate/analytics/bin/ingate-analytics

ENTRYPOINT ["/opt/ingate/analytics/bin/ingate-analytics"]
CMD ["--config", "/opt/ingate/analytics/configs/config.yaml"]

FROM service-runtime AS console

COPY _output/docker/bin/ingate-console /opt/ingate/console/bin/ingate-console
COPY _output/docker/web /opt/ingate/console/web

ENTRYPOINT ["/opt/ingate/console/bin/ingate-console"]
CMD ["--config", "/opt/ingate/console/configs/config.yaml"]
