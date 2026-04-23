FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl
WORKDIR /app
COPY _output/compose/ingate-xds-server /usr/local/bin/ingate-xds-server
ENTRYPOINT ["/usr/local/bin/ingate-xds-server"]
