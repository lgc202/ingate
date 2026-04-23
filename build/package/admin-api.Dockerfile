FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl
WORKDIR /app
COPY _output/compose/ingate-admin-api /usr/local/bin/ingate-admin-api
ENTRYPOINT ["/usr/local/bin/ingate-admin-api"]
