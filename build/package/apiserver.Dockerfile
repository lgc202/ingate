FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl
WORKDIR /app
COPY _output/compose/ingate-apiserver /usr/local/bin/ingate-apiserver
ENTRYPOINT ["/usr/local/bin/ingate-apiserver"]
