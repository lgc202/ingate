.PHONY: build generate test

GO_CACHE_DIR ?= /tmp/ingate-next-gocache

build:
	mkdir -p _output
	GOCACHE=$(GO_CACHE_DIR) go build -o _output/ingate ./cmd/ingate
	GOCACHE=$(GO_CACHE_DIR) go build ./cmd/...

generate:
	bash hack/update-codegen.sh

test:
	GOCACHE=$(GO_CACHE_DIR) go test ./...
