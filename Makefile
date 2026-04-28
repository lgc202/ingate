.PHONY: build test

GO_CACHE_DIR ?= /tmp/ingate-next-gocache

build:
	mkdir -p _output
	GOCACHE=$(GO_CACHE_DIR) go build -o _output/ingate ./cmd/ingate

test:
	GOCACHE=$(GO_CACHE_DIR) go test ./...
