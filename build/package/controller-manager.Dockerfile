FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl
WORKDIR /app
COPY _output/compose/ingate-controller-manager /usr/local/bin/ingate-controller-manager
ENTRYPOINT ["/usr/local/bin/ingate-controller-manager"]
